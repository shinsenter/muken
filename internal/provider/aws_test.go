// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// The key and the signature come from the AWS Signature Version 4 documentation.
func TestAWSSigningKey(t *testing.T) {
	key := awsSigningKey("wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY", "20150830", "us-east-1", "iam")
	want := "c4afb1cc5771d871763a393e44b703571b55cc28424d1a5e86da6ed3c154a4b9"
	if hex.EncodeToString(key) != want {
		t.Fatalf("got %x", key)
	}
}

func TestAWSAuthorization(t *testing.T) {
	got := awsAuthorization("GET", "Action=ListUsers&Version=2010-05-08", nil, "iam.amazonaws.com", "20150830T123600Z", "us-east-1", "iam",
		"AKIDEXAMPLE", "wJalrXUtnFEMI/K7MDENG+bPxRfiCYEXAMPLEKEY")
	want := "Signature=5d672d79c15b13162d9279b0855cfba6789a8edb4c82c400e06b5924a6f2b5d7"
	if !strings.HasSuffix(got, want) || !strings.Contains(got, "Credential=AKIDEXAMPLE/20150830/us-east-1/iam/aws4_request") {
		t.Fatalf("got %s", got)
	}
}

func awsTestServer(t *testing.T, calls *[]url.Values) *httptest.Server {
	var polls int
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(raw))
		*calls = append(*calls, form)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 Credential=AKID/") {
			t.Error("request is not signed")
		}
		switch form.Get("Action") {
		case "DescribeImages":
			io.WriteString(w, `<DescribeImagesResponse><imagesSet>
			 <item><imageId>ami-old</imageId><creationDate>2026-01-01T00:00:00.000Z</creationDate></item>
			 <item><imageId>ami-new</imageId><creationDate>2026-06-01T00:00:00.000Z</creationDate></item></imagesSet></DescribeImagesResponse>`)
		case "ImportKeyPair":
			w.WriteHeader(400)
			io.WriteString(w, `<Response><Errors><Error><Code>InvalidKeyPair.Duplicate</Code><Message>exists</Message></Error></Errors></Response>`)
		case "DescribeSecurityGroups":
			io.WriteString(w, `<DescribeSecurityGroupsResponse><securityGroupInfo></securityGroupInfo></DescribeSecurityGroupsResponse>`)
		case "CreateSecurityGroup":
			io.WriteString(w, `<CreateSecurityGroupResponse><groupId>sg-1</groupId></CreateSecurityGroupResponse>`)
		case "AuthorizeSecurityGroupIngress", "TerminateInstances":
			io.WriteString(w, `<Response><return>true</return></Response>`)
		case "RunInstances":
			io.WriteString(w, `<RunInstancesResponse><instancesSet><item><instanceId>i-abc</instanceId></item></instancesSet></RunInstancesResponse>`)
		case "DescribeInstances":
			polls++
			if polls < 3 {
				io.WriteString(w, `<DescribeInstancesResponse><reservationSet><item><instancesSet><item><instanceState><name>pending</name></instanceState></item></instancesSet></item></reservationSet></DescribeInstancesResponse>`)
				return
			}
			io.WriteString(w, `<DescribeInstancesResponse><reservationSet><item><instancesSet><item><instanceState><name>running</name></instanceState><ipAddress>203.0.113.5</ipAddress></item></instancesSet></item></reservationSet></DescribeInstancesResponse>`)
		default:
			t.Errorf("unexpected action %s", form.Get("Action"))
			w.WriteHeader(500)
		}
	}))
}

func newTestAWS(url string) *AWS {
	a := NewAWS("AKID", "secret", "")
	a.endpoint = func(string) string { return url + "/" }
	a.Sleep = func(time.Duration) {}
	return a
}

func TestAWSCreate(t *testing.T) {
	var calls []url.Values
	ts := awsTestServer(t, &calls)
	defer ts.Close()
	srv, err := newTestAWS(ts.URL).Create(context.Background(), CreateOptions{Name: "blog", Region: "ap-southeast-1", Size: "t3.small", PublicKey: pubKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if srv.ID != "ap-southeast-1/i-abc" || srv.IP != "203.0.113.5" || srv.User != "ubuntu" {
		t.Fatalf("got %+v", srv)
	}
	byAction := map[string]url.Values{}
	for _, c := range calls {
		byAction[c.Get("Action")] = c
	}
	run := byAction["RunInstances"]
	if run.Get("ImageId") != "ami-new" || run.Get("InstanceType") != "t3.small" || run.Get("SecurityGroupId.1") != "sg-1" || run.Get("Version") != ec2Version {
		t.Fatalf("RunInstances %v", run)
	}
	open := byAction["AuthorizeSecurityGroupIngress"]
	if open.Get("IpPermissions.1.FromPort") != "22" || open.Get("IpPermissions.2.FromPort") != "80" || open.Get("IpPermissions.3.FromPort") != "443" {
		t.Fatalf("ingress %v", open)
	}
}

func TestAWSDelete(t *testing.T) {
	var calls []url.Values
	ts := awsTestServer(t, &calls)
	defer ts.Close()
	a := newTestAWS(ts.URL)
	if err := a.Delete(context.Background(), "eu-west-1/i-abc"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0].Get("InstanceId.1") != "i-abc" {
		t.Fatalf("calls %v", calls)
	}
	if err := a.Delete(context.Background(), "i-abc"); err == nil {
		t.Fatal("an id without a region must fail")
	}
}

func TestAWSNeedsKeys(t *testing.T) {
	if _, err := NewAWS("", "", "").Regions(context.Background()); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("got %v", err)
	}
}
