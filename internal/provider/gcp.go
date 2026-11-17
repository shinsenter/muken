// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/ui"
)

const (
	gcpComputeBase = "https://compute.googleapis.com/compute/v1"
	gcpScope       = "https://www.googleapis.com/auth/compute"
	gcpImage       = "projects/ubuntu-os-cloud/global/images/family/ubuntu-2404-lts-amd64"
	gcpFirewall    = "muken-allow"
)

type GCP struct {
	client    *api
	credsFile string
	project   string
	apiBase   string
	now       func() time.Time
}

func NewGCP(credsFile, project string) *GCP {
	return &GCP{client: newAPI(gcpComputeBase, ""), credsFile: credsFile, project: project, apiBase: gcpComputeBase, now: time.Now}
}

func (g *GCP) Name() string     { return "gcp" }
func (g *GCP) TokenEnv() string { return "GOOGLE_APPLICATION_CREDENTIALS" }

type gcpKey struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
	ProjectID   string `json:"project_id"`
}

func b64(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

// assertion builds the signed JWT (RS256) that Google exchanges for an access token.
func (g *GCP) assertion(key gcpKey) (string, error) {
	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return "", ui.Errf("download a JSON key again from the service account page", "the private key in %s is not PEM", g.credsFile)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if err != nil || !ok {
		return "", ui.Errf("use a service account key of type JSON", "cannot read the private key in %s", g.credsFile)
	}
	issued := g.now().Unix()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT"})
	claims, _ := json.Marshal(map[string]any{
		"iss": key.ClientEmail, "scope": gcpScope, "aud": key.TokenURI, "iat": issued, "exp": issued + 3600,
	})
	signing := b64(header) + "." + b64(claims)
	digest := sha256.Sum256([]byte(signing))
	signature, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return signing + "." + b64(signature), nil
}

// auth reads the key file once, gets an access token and points the client at the project.
func (g *GCP) auth(ctx context.Context) error {
	if g.client.token != "" {
		return nil
	}
	if g.credsFile == "" {
		return ui.Errf("create a service account key (JSON) and run: export GOOGLE_APPLICATION_CREDENTIALS=/path/key.json", "GOOGLE_APPLICATION_CREDENTIALS is not set")
	}
	raw, err := os.ReadFile(g.credsFile)
	if err != nil {
		return ui.Wrap(err, "check the path in GOOGLE_APPLICATION_CREDENTIALS", "cannot read %s", g.credsFile)
	}
	var key gcpKey
	if err := json.Unmarshal(raw, &key); err != nil || key.ClientEmail == "" || key.TokenURI == "" {
		return ui.Errf("use a service account key of type JSON", "%s is not a service account key", g.credsFile)
	}
	project := g.project
	if project == "" {
		project = key.ProjectID
	}
	if project == "" {
		return ui.Errf("export GOOGLE_CLOUD_PROJECT=your-project-id", "the Google Cloud project is not set")
	}
	jwt, err := g.assertion(key)
	if err != nil {
		return err
	}
	token, err := g.exchange(ctx, key.TokenURI, jwt)
	if err != nil {
		return err
	}
	g.client.token = token
	g.client.base = g.apiBase + "/projects/" + url.PathEscape(project)
	return nil
}

func (g *GCP) exchange(ctx context.Context, tokenURI, jwt string) (string, error) {
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {jwt}}
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.client.http.Do(req)
	if err != nil {
		return "", ui.Wrap(err, "check the network", "cannot reach the Google token service")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var answer struct {
		Token string `json:"access_token"`
		Error string `json:"error_description"`
	}
	json.Unmarshal(raw, &answer)
	if resp.StatusCode >= 300 || answer.Token == "" {
		return "", ui.Errf("check that the key is active and the Compute Engine API is enabled", "Google refused the key: %s", strings.TrimSpace(answer.Error))
	}
	return answer.Token, nil
}

// Regions returns zones, because an instance lives in a zone.
func (g *GCP) Regions(ctx context.Context) ([]Choice, error) {
	if err := g.auth(ctx); err != nil {
		return nil, err
	}
	var response struct {
		Items []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"items"`
	}
	if _, err := g.client.do(ctx, "GET", "/zones?maxResults=500", nil, &response); err != nil {
		return nil, err
	}
	var out []Choice
	for _, z := range response.Items {
		if z.Status == "UP" {
			out = append(out, Choice{z.Name, z.Name})
		}
	}
	return out, nil
}

// Sizes is a fixed list, because the Compute API has no price list.
func (g *GCP) Sizes(context.Context) ([]Choice, error) {
	return []Choice{
		{"e2-micro", "2 shared vCPU, 1 GB RAM"},
		{"e2-small", "2 shared vCPU, 2 GB RAM"},
		{"e2-medium", "2 shared vCPU, 4 GB RAM"},
		{"e2-standard-2", "2 vCPU, 8 GB RAM"},
		{"e2-standard-4", "4 vCPU, 16 GB RAM"},
	}, nil
}

var gcpBadName = regexp.MustCompile(`[^a-z0-9-]+`)

// gcpName turns a name into one that Compute accepts: lower case, digits and dashes, starts with a letter.
func gcpName(name string) string {
	name = strings.Trim(gcpBadName.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if name == "" {
		return "muken"
	}
	if name[0] < 'a' || name[0] > 'z' {
		name = "muken-" + name
	}
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	return name
}

// ensureFirewall creates the rule that opens ports 22, 80 and 443 for instances with the tag "muken".
func (g *GCP) ensureFirewall(ctx context.Context) error {
	status, err := g.client.do(ctx, "GET", "/global/firewalls/"+gcpFirewall, nil, nil)
	if err == nil {
		return nil
	}
	if status != http.StatusNotFound {
		return err
	}
	body := map[string]any{
		"name":         gcpFirewall,
		"network":      "global/networks/default",
		"direction":    "INGRESS",
		"allowed":      []map[string]any{{"IPProtocol": "tcp", "ports": []string{"22", "80", "443"}}},
		"sourceRanges": []string{"0.0.0.0/0"},
		"targetTags":   []string{"muken"},
	}
	_, err = g.client.do(ctx, "POST", "/global/firewalls", body, nil)
	return err
}

func (g *GCP) Create(ctx context.Context, opts CreateOptions) (Server, error) {
	if err := g.auth(ctx); err != nil {
		return Server{}, err
	}
	if _, err := Fingerprint(opts.PublicKey); err != nil {
		return Server{}, err
	}
	if err := g.ensureFirewall(ctx); err != nil {
		return Server{}, err
	}
	name := gcpName(opts.Name)
	body := map[string]any{
		"name":        name,
		"machineType": "zones/" + opts.Region + "/machineTypes/" + opts.Size,
		"disks": []map[string]any{{
			"boot": true, "autoDelete": true,
			"initializeParams": map[string]any{"sourceImage": gcpImage, "diskSizeGb": "20"},
		}},
		"networkInterfaces": []map[string]any{{
			"network":       "global/networks/default",
			"accessConfigs": []map[string]string{{"name": "External NAT", "type": "ONE_TO_ONE_NAT"}},
		}},
		"metadata": map[string]any{"items": []map[string]string{{"key": "ssh-keys", "value": "ubuntu:" + strings.TrimSpace(opts.PublicKey)}}},
		"tags":     map[string]any{"items": []string{"muken"}},
		"labels":   map[string]string{"managed-by": "muken"},
	}
	if _, err := g.client.do(ctx, "POST", "/zones/"+opts.Region+"/instances", body, nil); err != nil {
		return Server{}, err
	}
	return g.waitRunning(ctx, opts.Region, name)
}

// waitRunning polls every 5 seconds until the instance runs and has an external IPv4 address.
// The insert call returns before the instance exists, so a 404 at first is normal.
func (g *GCP) waitRunning(ctx context.Context, zone, name string) (Server, error) {
	id := zone + "/" + name
	for i := 0; i < g.client.Tries; i++ {
		var response struct {
			Status string `json:"status"`
			NICs   []struct {
				Access []struct {
					IP string `json:"natIP"`
				} `json:"accessConfigs"`
			} `json:"networkInterfaces"`
		}
		status, err := g.client.do(ctx, "GET", "/zones/"+zone+"/instances/"+name, nil, &response)
		if err != nil && status != http.StatusNotFound {
			return Server{ID: id}, err
		}
		if err == nil && response.Status == "RUNNING" && len(response.NICs) > 0 && len(response.NICs[0].Access) > 0 && response.NICs[0].Access[0].IP != "" {
			return Server{ID: id, IP: response.NICs[0].Access[0].IP, User: "ubuntu"}, nil
		}
		g.client.Sleep(5 * time.Second)
	}
	return Server{ID: id}, ui.Errf("check the Compute Engine console; delete the instance with `muken server delete`", "instance %s did not start in time", name)
}

// Delete takes the id that Create returned: <zone>/<name>.
func (g *GCP) Delete(ctx context.Context, id string) error {
	if err := g.auth(ctx); err != nil {
		return err
	}
	zone, name, ok := strings.Cut(id, "/")
	if !ok {
		return ui.Errf("use the id that `server create` printed, for example asia-southeast1-b/blog", "bad GCP instance id %q", id)
	}
	_, err := g.client.do(ctx, "DELETE", "/zones/"+zone+"/instances/"+name, nil, nil)
	return err
}

func gcpFromEnv() *GCP {
	return NewGCP(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"), os.Getenv("GOOGLE_CLOUD_PROJECT"))
}
