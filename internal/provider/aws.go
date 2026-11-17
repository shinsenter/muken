// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/ui"
)

const (
	ec2Version     = "2016-11-15"
	awsDefaultZone = "us-east-1"
	awsContentType = "application/x-www-form-urlencoded; charset=utf-8"
	// canonicalOwner is the AWS account that publishes the Ubuntu AMIs.
	canonicalOwner = "099720109477"
	awsAMIName     = "ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"
)

type AWS struct {
	keyID, secret, region string
	endpoint              func(region string) string
	http                  *http.Client
	now                   func() time.Time
	Sleep                 func(time.Duration)
	Tries                 int
}

func NewAWS(keyID, secret, region string) *AWS {
	if region == "" {
		region = awsDefaultZone
	}
	return &AWS{
		keyID:    keyID,
		secret:   secret,
		region:   region,
		endpoint: func(r string) string { return "https://ec2." + r + ".amazonaws.com/" },
		http:     &http.Client{Timeout: 30 * time.Second},
		now:      time.Now,
		Sleep:    time.Sleep,
		Tries:    60,
	}
}

func (a *AWS) Name() string     { return "aws" }
func (a *AWS) TokenEnv() string { return "AWS_ACCESS_KEY_ID" }

func (a *AWS) requireKeys() error {
	if a.keyID == "" || a.secret == "" {
		return ui.Errf("create an access key and run: export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=...", "AWS credentials are not set")
	}
	return nil
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func awsSigningKey(secret, date, region, service string) []byte {
	key := hmacSHA256([]byte("AWS4"+secret), date)
	key = hmacSHA256(key, region)
	key = hmacSHA256(key, service)
	return hmacSHA256(key, "aws4_request")
}

// awsAuthorization builds the SigV4 Authorization header.
// The signed headers are always content-type, host and x-amz-date.
func awsAuthorization(method, query string, body []byte, host, amzDate, region, service, keyID, secret string) string {
	date := amzDate[:8]
	payload := sha256.Sum256(body)
	canonical := strings.Join([]string{
		method, "/", query,
		"content-type:" + awsContentType + "\nhost:" + host + "\nx-amz-date:" + amzDate + "\n",
		"content-type;host;x-amz-date",
		hex.EncodeToString(payload[:]),
	}, "\n")
	hashed := sha256.Sum256([]byte(canonical))
	scope := date + "/" + region + "/" + service + "/aws4_request"
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(hashed[:])
	signature := hex.EncodeToString(hmacSHA256(awsSigningKey(secret, date, region, service), toSign))
	return "AWS4-HMAC-SHA256 Credential=" + keyID + "/" + scope + ", SignedHeaders=content-type;host;x-amz-date, Signature=" + signature
}

type awsError struct {
	Code, Message string
}

func (e *awsError) Error() string { return "AWS " + e.Code + ": " + e.Message }

// call sends one EC2 Query API request and decodes the XML answer into out (if not nil).
func (a *AWS) call(ctx context.Context, region, action string, params url.Values, out any) error {
	params.Set("Action", action)
	params.Set("Version", ec2Version)
	body := []byte(params.Encode())
	req, err := http.NewRequestWithContext(ctx, "POST", a.endpoint(region), bytes.NewReader(body))
	if err != nil {
		return err
	}
	amzDate := a.now().UTC().Format("20060102T150405Z")
	req.Header.Set("Content-Type", awsContentType)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("Authorization", awsAuthorization("POST", "", body, req.URL.Host, amzDate, region, "ec2", a.keyID, a.secret))

	resp, err := a.http.Do(req)
	if err != nil {
		return ui.Wrap(err, "check the network", "cannot reach %s", req.URL.Host)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode >= 300 {
		return awsFailure(resp.StatusCode, raw)
	}
	if out != nil {
		if err := xml.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("unreadable response to %s: %w", action, err)
		}
	}
	return nil
}

func awsFailure(status int, raw []byte) error {
	var body struct {
		Errors []struct {
			Code    string `xml:"Code"`
			Message string `xml:"Message"`
		} `xml:"Errors>Error"`
	}
	if xml.Unmarshal(raw, &body) != nil || len(body.Errors) == 0 {
		return ui.Errf("", "AWS answered HTTP %d", status)
	}
	e := &awsError{body.Errors[0].Code, body.Errors[0].Message}
	switch e.Code {
	case "AuthFailure", "InvalidClientTokenId", "SignatureDoesNotMatch":
		return ui.Wrap(e, "the access key is wrong or expired: create a new one and export it again", "AWS refused the credentials")
	case "UnauthorizedOperation":
		return ui.Wrap(e, "the IAM user needs EC2 permissions: AmazonEC2FullAccess is enough", "AWS refused the request")
	}
	return e
}

func awsCode(err error) string {
	var e *awsError
	if errors.As(err, &e) {
		return e.Code
	}
	return ""
}

func (a *AWS) Regions(ctx context.Context) ([]Choice, error) {
	if err := a.requireKeys(); err != nil {
		return nil, err
	}
	var response struct {
		Regions []struct {
			Name string `xml:"regionName"`
		} `xml:"regionInfo>item"`
	}
	if err := a.call(ctx, a.region, "DescribeRegions", url.Values{}, &response); err != nil {
		return nil, err
	}
	var out []Choice
	for _, r := range response.Regions {
		out = append(out, Choice{r.Name, r.Name})
	}
	return out, nil
}

// Sizes is a fixed list, because the EC2 API has no price list.
func (a *AWS) Sizes(context.Context) ([]Choice, error) {
	return []Choice{
		{"t3.micro", "2 vCPU, 1 GB RAM"},
		{"t3.small", "2 vCPU, 2 GB RAM"},
		{"t3.medium", "2 vCPU, 4 GB RAM"},
		{"t3.large", "2 vCPU, 8 GB RAM"},
		{"t3.xlarge", "4 vCPU, 16 GB RAM"},
	}, nil
}

func (a *AWS) findImage(ctx context.Context, region string) (string, error) {
	params := url.Values{}
	params.Set("Owner.1", canonicalOwner)
	params.Set("Filter.1.Name", "name")
	params.Set("Filter.1.Value.1", awsAMIName)
	params.Set("Filter.2.Name", "state")
	params.Set("Filter.2.Value.1", "available")
	var response struct {
		Images []struct {
			ID      string `xml:"imageId"`
			Created string `xml:"creationDate"`
		} `xml:"imagesSet>item"`
	}
	if err := a.call(ctx, region, "DescribeImages", params, &response); err != nil {
		return "", err
	}
	best, bestDate := "", ""
	for _, img := range response.Images {
		if img.Created > bestDate {
			best, bestDate = img.ID, img.Created
		}
	}
	if best == "" {
		return "", ui.Errf("pick another region", "no Ubuntu 24.04 image found in %s", region)
	}
	return best, nil
}

// ensureKey imports the SSH key. It returns the key pair name.
func (a *AWS) ensureKey(ctx context.Context, region, publicKey string) (string, error) {
	fingerprint, err := Fingerprint(publicKey)
	if err != nil {
		return "", err
	}
	name := "muken-" + strings.ReplaceAll(fingerprint, ":", "")[:12]
	params := url.Values{}
	params.Set("KeyName", name)
	params.Set("PublicKeyMaterial", base64.StdEncoding.EncodeToString([]byte(publicKey)))
	err = a.call(ctx, region, "ImportKeyPair", params, nil)
	if err != nil && awsCode(err) != "InvalidKeyPair.Duplicate" {
		return "", err
	}
	return name, nil
}

// ensureGroup finds the security group "muken" or creates it with ports 22, 80 and 443 open.
// Without a VPC id AWS uses the default VPC.
func (a *AWS) ensureGroup(ctx context.Context, region string) (string, error) {
	find := url.Values{}
	find.Set("Filter.1.Name", "group-name")
	find.Set("Filter.1.Value.1", "muken")
	var found struct {
		Groups []struct {
			ID string `xml:"groupId"`
		} `xml:"securityGroupInfo>item"`
	}
	if err := a.call(ctx, region, "DescribeSecurityGroups", find, &found); err != nil {
		return "", err
	}
	if len(found.Groups) > 0 {
		return found.Groups[0].ID, nil
	}
	create := url.Values{}
	create.Set("GroupName", "muken")
	create.Set("GroupDescription", "muken: SSH, HTTP and HTTPS")
	var created struct {
		ID string `xml:"groupId"`
	}
	if err := a.call(ctx, region, "CreateSecurityGroup", create, &created); err != nil {
		return "", err
	}
	open := url.Values{}
	open.Set("GroupId", created.ID)
	for i, port := range []string{"22", "80", "443"} {
		prefix := "IpPermissions." + strconv.Itoa(i+1)
		open.Set(prefix+".IpProtocol", "tcp")
		open.Set(prefix+".FromPort", port)
		open.Set(prefix+".ToPort", port)
		open.Set(prefix+".IpRanges.1.CidrIp", "0.0.0.0/0")
	}
	return created.ID, a.call(ctx, region, "AuthorizeSecurityGroupIngress", open, nil)
}

func (a *AWS) Create(ctx context.Context, opts CreateOptions) (Server, error) {
	if err := a.requireKeys(); err != nil {
		return Server{}, err
	}
	image, err := a.findImage(ctx, opts.Region)
	if err != nil {
		return Server{}, err
	}
	keyName, err := a.ensureKey(ctx, opts.Region, opts.PublicKey)
	if err != nil {
		return Server{}, err
	}
	group, err := a.ensureGroup(ctx, opts.Region)
	if err != nil {
		return Server{}, err
	}
	params := url.Values{}
	params.Set("ImageId", image)
	params.Set("InstanceType", opts.Size)
	params.Set("MinCount", "1")
	params.Set("MaxCount", "1")
	params.Set("KeyName", keyName)
	params.Set("SecurityGroupId.1", group)
	params.Set("TagSpecification.1.ResourceType", "instance")
	params.Set("TagSpecification.1.Tag.1.Key", "Name")
	params.Set("TagSpecification.1.Tag.1.Value", opts.Name)
	params.Set("TagSpecification.1.Tag.2.Key", "managed-by")
	params.Set("TagSpecification.1.Tag.2.Value", "muken")
	var run struct {
		Instances []struct {
			ID string `xml:"instanceId"`
		} `xml:"instancesSet>item"`
	}
	if err := a.call(ctx, opts.Region, "RunInstances", params, &run); err != nil {
		return Server{}, err
	}
	if len(run.Instances) == 0 {
		return Server{}, ui.Errf("check the EC2 console", "AWS returned no instance")
	}
	return a.waitRunning(ctx, opts.Region, run.Instances[0].ID)
}

// waitRunning polls every 5 seconds until the instance runs and has a public IPv4 address.
func (a *AWS) waitRunning(ctx context.Context, region, id string) (Server, error) {
	full := region + "/" + id
	for i := 0; i < a.Tries; i++ {
		params := url.Values{}
		params.Set("InstanceId.1", id)
		var response struct {
			Instances []struct {
				State string `xml:"instanceState>name"`
				IP    string `xml:"ipAddress"`
			} `xml:"reservationSet>item>instancesSet>item"`
		}
		if err := a.call(ctx, region, "DescribeInstances", params, &response); err != nil {
			// A new instance can be unknown to DescribeInstances for a moment.
			if awsCode(err) != "InvalidInstanceID.NotFound" {
				return Server{ID: full}, err
			}
		} else if len(response.Instances) > 0 {
			inst := response.Instances[0]
			if inst.State == "running" && inst.IP != "" {
				return Server{ID: full, IP: inst.IP, User: "ubuntu"}, nil
			}
		}
		a.Sleep(5 * time.Second)
	}
	return Server{ID: full}, ui.Errf("check the EC2 console; delete the instance with `muken server delete`", "instance %s did not start in time", id)
}

// Delete takes the id that Create returned: <region>/<instance-id>.
func (a *AWS) Delete(ctx context.Context, id string) error {
	if err := a.requireKeys(); err != nil {
		return err
	}
	region, instance, ok := strings.Cut(id, "/")
	if !ok {
		return ui.Errf("use the id that `server create` printed, for example us-east-1/i-0123456789abcdef0", "bad AWS instance id %q", id)
	}
	params := url.Values{}
	params.Set("InstanceId.1", instance)
	return a.call(ctx, region, "TerminateInstances", params, nil)
}

func awsFromEnv() *AWS {
	return NewAWS(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), os.Getenv("AWS_REGION"))
}
