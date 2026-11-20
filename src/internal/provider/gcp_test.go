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
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gcpKeyFile(t *testing.T, tokenURI string) (string, *rsa.PublicKey) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKCS8PrivateKey(priv)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	raw, _ := json.Marshal(map[string]string{"client_email": "muken@proj.iam.gserviceaccount.com", "private_key": pemKey, "token_uri": tokenURI, "project_id": "proj"})
	path := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, &priv.PublicKey
}

func TestGCPCreate(t *testing.T) {
	var assertion string
	var instance, firewall map[string]any
	var polls int
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "urn:ietf:params:oauth:grant-type:jwt-bearer" {
			t.Errorf("grant_type %q", r.Form.Get("grant_type"))
		}
		assertion = r.Form.Get("assertion")
		io.WriteString(w, `{"access_token":"tok","expires_in":3600}`)
	})
	mux.HandleFunc("/projects/proj/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("missing token")
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/projects/proj/global/firewalls/muken-allow":
			w.WriteHeader(404)
			io.WriteString(w, `{"error":{"code":404}}`)
		case r.Method == "POST" && r.URL.Path == "/projects/proj/global/firewalls":
			json.NewDecoder(r.Body).Decode(&firewall)
			io.WriteString(w, `{"kind":"compute#operation"}`)
		case r.Method == "POST" && r.URL.Path == "/projects/proj/zones/asia-southeast1-b/instances":
			json.NewDecoder(r.Body).Decode(&instance)
			io.WriteString(w, `{"kind":"compute#operation"}`)
		case r.Method == "GET" && r.URL.Path == "/projects/proj/zones/asia-southeast1-b/instances/my-blog":
			polls++
			if polls == 1 {
				w.WriteHeader(404)
				io.WriteString(w, `{}`)
				return
			}
			if polls == 2 {
				io.WriteString(w, `{"status":"PROVISIONING","networkInterfaces":[{"accessConfigs":[]}]}`)
				return
			}
			io.WriteString(w, `{"status":"RUNNING","networkInterfaces":[{"accessConfigs":[{"natIP":"198.51.100.8"}]}]}`)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(500)
		}
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	creds, pub := gcpKeyFile(t, ts.URL+"/token")
	g := NewGCP(creds, "")
	g.apiBase, g.client.Sleep = ts.URL, func(time.Duration) {}
	srv, err := g.Create(context.Background(), CreateOptions{Name: "My_Blog", Region: "asia-southeast1-b", Size: "e2-small", PublicKey: pubKey(t)})
	if err != nil {
		t.Fatal(err)
	}
	if srv.ID != "asia-southeast1-b/my-blog" || srv.IP != "198.51.100.8" || srv.User != "ubuntu" {
		t.Fatalf("got %+v", srv)
	}
	if instance["machineType"] != "zones/asia-southeast1-b/machineTypes/e2-small" || firewall["name"] != "muken-allow" {
		t.Fatalf("instance %v firewall %v", instance, firewall)
	}
	checkJWT(t, assertion, pub)
}

// checkJWT verifies the RS256 signature and the claims that Google reads.
func checkJWT(t *testing.T, jwt string, pub *rsa.PublicKey) {
	parts := strings.Split(jwt, ".")
	if len(parts) != 3 {
		t.Fatalf("assertion %q", jwt)
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], signature); err != nil {
		t.Fatalf("bad signature: %v", err)
	}
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var claims map[string]any
	json.Unmarshal(raw, &claims)
	if claims["iss"] != "muken@proj.iam.gserviceaccount.com" || claims["scope"] != gcpScope || claims["aud"] == "" {
		t.Fatalf("claims %v", claims)
	}
	if claims["exp"].(float64)-claims["iat"].(float64) != 3600 {
		t.Fatalf("lifetime in %v", claims)
	}
}

func TestGCPDelete(t *testing.T) {
	var path string
	mux := http.NewServeMux()
	mux.HandleFunc("/token", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, `{"access_token":"tok"}`) })
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		path = r.Method + " " + r.URL.Path
		io.WriteString(w, `{}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()
	creds, _ := gcpKeyFile(t, ts.URL+"/token")
	g := NewGCP(creds, "other")
	g.apiBase = ts.URL
	if err := g.Delete(context.Background(), "us-east1-b/blog"); err != nil {
		t.Fatal(err)
	}
	if path != "DELETE /projects/other/zones/us-east1-b/instances/blog" {
		t.Fatalf("got %q", path)
	}
	if err := g.Delete(context.Background(), "blog"); err == nil {
		t.Fatal("an id without a zone must fail")
	}
}

func TestGCPNeedsCredentials(t *testing.T) {
	if _, err := NewGCP("", "").Regions(context.Background()); err == nil || !strings.Contains(err.Error(), "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Fatalf("got %v", err)
	}
}

func TestGCPName(t *testing.T) {
	cases := map[string]string{"My_Blog": "my-blog", "9lives": "muken-9lives", "ok-name": "ok-name", "": "muken", "---": "muken"}
	for in, want := range cases {
		if got := gcpName(in); got != want {
			t.Errorf("gcpName(%q) = %q, want %q", in, got, want)
		}
	}
}
