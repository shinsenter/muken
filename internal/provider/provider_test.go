// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/shinsenter/muken/internal/ui"
)

func pubKey(t *testing.T) string {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k)))
}

func TestFingerprintFormat(t *testing.T) {
	fp, err := Fingerprint(pubKey(t))
	if err != nil || len(strings.Split(fp, ":")) != 16 {
		t.Fatalf("got %q %v", fp, err)
	}
	if _, err := Fingerprint("not a key"); err == nil {
		t.Fatal("want an error")
	}
}

func TestDigitalOceanCreate(t *testing.T) {
	var created map[string]any
	var polls int
	var keyPosted bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("missing token")
		}
		switch {
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, "/account/keys/"):
			w.WriteHeader(404)
			io.WriteString(w, `{"id":"not_found"}`)
		case r.Method == "POST" && r.URL.Path == "/account/keys":
			keyPosted = true
			io.WriteString(w, `{}`)
		case r.Method == "POST" && r.URL.Path == "/droplets":
			json.NewDecoder(r.Body).Decode(&created)
			io.WriteString(w, `{"droplet":{"id":42}}`)
		case r.Method == "GET" && r.URL.Path == "/droplets/42":
			polls++
			if polls < 3 {
				io.WriteString(w, `{"droplet":{"status":"new","networks":{"v4":[]}}}`)
				return
			}
			io.WriteString(w, `{"droplet":{"status":"active","networks":{"v4":[{"ip_address":"10.0.0.2","type":"private"},{"ip_address":"203.0.113.7","type":"public"}]}}}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer ts.Close()
	d := NewDigitalOcean("tok")
	d.client.base, d.client.Sleep = ts.URL, func(time.Duration) {}
	srv, err := d.Create(context.Background(), CreateOptions{Name: "blog", Region: "sgp1", Size: "s-1vcpu-1gb", PublicKey: pubKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if srv.ID != "42" || srv.IP != "203.0.113.7" {
		t.Fatalf("got %+v", srv)
	}
	if !keyPosted || created["region"] != "sgp1" || created["size"] != "s-1vcpu-1gb" || created["image"] != "ubuntu-24-04-x64" {
		t.Fatalf("request %v keyPosted=%v", created, keyPosted)
	}
}

func TestDigitalOceanNeedsToken(t *testing.T) {
	if _, err := NewDigitalOcean("").Regions(context.Background()); err == nil || !strings.Contains(err.Error(), "DIGITALOCEAN_TOKEN") {
		t.Fatalf("got %v", err)
	}
}

func TestDigitalOceanSizesAreCheapestFirst(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"sizes":[
		 {"slug":"big","memory":8192,"vcpus":4,"disk":160,"price_monthly":48,"available":true},
		 {"slug":"tiny","memory":512,"vcpus":1,"disk":10,"price_monthly":4,"available":true},
		 {"slug":"small","memory":1024,"vcpus":1,"disk":25,"price_monthly":6,"available":true},
		 {"slug":"gone","memory":2048,"vcpus":1,"disk":50,"price_monthly":12,"available":false}]}`)
	}))
	defer ts.Close()
	d := NewDigitalOcean("tok")
	d.client.base = ts.URL
	list, err := d.Sizes(context.Background())
	if err != nil || len(list) != 2 || list[0].ID != "small" || list[1].ID != "big" {
		t.Fatalf("got %v %v", list, err)
	}
}

func TestHetznerCreate(t *testing.T) {
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/ssh_keys":
			io.WriteString(w, `{"ssh_keys":[]}`)
		case r.Method == "POST" && r.URL.Path == "/ssh_keys":
			io.WriteString(w, `{"ssh_key":{"id":9}}`)
		case r.Method == "POST" && r.URL.Path == "/servers":
			json.NewDecoder(r.Body).Decode(&body)
			io.WriteString(w, `{"server":{"id":7}}`)
		case r.Method == "GET" && r.URL.Path == "/servers/7":
			io.WriteString(w, `{"server":{"status":"running","public_net":{"ipv4":{"ip":"198.51.100.4"}}}}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer ts.Close()
	h := NewHetzner("tok")
	h.client.base, h.client.Sleep = ts.URL, func(time.Duration) {}
	srv, err := h.Create(context.Background(), CreateOptions{Name: "blog", Region: "nbg1", Size: "cx22", PublicKey: pubKey(t)})
	if err != nil || srv.ID != "7" || srv.IP != "198.51.100.4" {
		t.Fatalf("got %+v %v", srv, err)
	}
	if body["server_type"] != "cx22" || body["location"] != "nbg1" {
		t.Fatalf("request %v", body)
	}
}

func TestVultrCreate(t *testing.T) {
	var body map[string]any
	var polls int
	key := pubKey(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("missing token")
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/ssh-keys":
			io.WriteString(w, `{"ssh_keys":[{"id":"other","ssh_key":"ssh-ed25519 AAAA other"}]}`)
		case r.Method == "POST" && r.URL.Path == "/ssh-keys":
			io.WriteString(w, `{"ssh_key":{"id":"key-1"}}`)
		case r.Method == "POST" && r.URL.Path == "/instances":
			json.NewDecoder(r.Body).Decode(&body)
			io.WriteString(w, `{"instance":{"id":"abc"}}`)
		case r.Method == "GET" && r.URL.Path == "/instances/abc":
			polls++
			if polls < 3 {
				io.WriteString(w, `{"instance":{"status":"pending","power_status":"stopped","main_ip":"0.0.0.0"}}`)
				return
			}
			io.WriteString(w, `{"instance":{"status":"active","power_status":"running","main_ip":"192.0.2.9"}}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	}))
	defer ts.Close()
	v := NewVultr("tok")
	v.client.base, v.client.Sleep = ts.URL, func(time.Duration) {}
	srv, err := v.Create(context.Background(), CreateOptions{Name: "blog", Region: "sgp", Size: "vc2-1c-1gb", PublicKey: key})
	if err != nil || srv.ID != "abc" || srv.IP != "192.0.2.9" {
		t.Fatalf("got %+v %v", srv, err)
	}
	if body["region"] != "sgp" || body["plan"] != "vc2-1c-1gb" || body["os_id"] != float64(vultrUbuntu2404) {
		t.Fatalf("request %v", body)
	}
	if ids, _ := body["sshkey_id"].([]any); len(ids) != 1 || ids[0] != "key-1" {
		t.Fatalf("sshkey_id %v", body["sshkey_id"])
	}
}

func TestVultrReusesKnownKey(t *testing.T) {
	key := pubKey(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/ssh-keys" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"ssh_keys": []map[string]string{{"id": "known", "ssh_key": key + " old-comment"}}})
	}))
	defer ts.Close()
	v := NewVultr("tok")
	v.client.base = ts.URL
	id, err := v.ensureKey(context.Background(), key)
	if err != nil || id != "known" {
		t.Fatalf("got %q %v", id, err)
	}
}

func TestVultrNeedsToken(t *testing.T) {
	if _, err := NewVultr("").Regions(context.Background()); err == nil || !strings.Contains(err.Error(), "VULTR_API_KEY") {
		t.Fatalf("got %v", err)
	}
}

func TestVultrSizesAreCheapestFirst(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"plans":[
		 {"id":"big","vcpu_count":4,"ram":8192,"disk":160,"monthly_cost":48},
		 {"id":"tiny","vcpu_count":1,"ram":512,"disk":10,"monthly_cost":2.5},
		 {"id":"small","vcpu_count":1,"ram":1024,"disk":25,"monthly_cost":6}]}`)
	}))
	defer ts.Close()
	v := NewVultr("tok")
	v.client.base = ts.URL
	list, err := v.Sizes(context.Background())
	if err != nil || len(list) != 2 || list[0].ID != "small" || list[1].ID != "big" {
		t.Fatalf("got %v %v", list, err)
	}
}

func TestNew(t *testing.T) {
	for _, n := range []string{"digitalocean", "do", "hetzner", "vultr", "aws"} {
		if _, err := New(n); err != nil {
			t.Errorf("%s: %v", n, err)
		}
	}
	if _, err := New("nope"); err == nil {
		t.Fatal("unknown provider must fail")
	}
}

func TestStatusErrorHints(t *testing.T) {
	forbidden := statusError("POST", "/droplets", http.StatusForbidden, []byte(`{"message":"You are missing the required permission tag:create."}`))
	var h *ui.HintError
	if !errors.As(forbidden, &h) || !strings.Contains(h.Hint, "lacks the permission") || strings.Contains(h.Hint, "read-only") {
		t.Fatalf("403 hint: %v", forbidden)
	}
	unauth := statusError("GET", "/regions", http.StatusUnauthorized, nil)
	if !errors.As(unauth, &h) || !strings.Contains(h.Hint, "wrong or expired") {
		t.Fatalf("401 hint: %v", unauth)
	}
}
