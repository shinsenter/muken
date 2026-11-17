// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package provider creates servers at cloud providers.
package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/shinsenter/muken/internal/ui"
)

type CreateOptions struct {
	Name      string
	Region    string
	Size      string
	PublicKey string // authorized_keys line
}

type Server struct {
	ID   string
	IP   string
	User string // SSH user when it is not root
}

type Choice struct {
	ID    string
	Label string
}

type Provider interface {
	Name() string
	TokenEnv() string
	Regions(ctx context.Context) ([]Choice, error)
	Sizes(ctx context.Context) ([]Choice, error)
	Create(ctx context.Context, o CreateOptions) (Server, error)
	Delete(ctx context.Context, id string) error
}

var Names = []string{"digitalocean", "hetzner", "vultr", "aws"}

func New(name string) (Provider, error) {
	switch name {
	case "digitalocean", "do":
		return NewDigitalOcean(os.Getenv("DIGITALOCEAN_TOKEN")), nil
	case "hetzner":
		return NewHetzner(os.Getenv("HCLOUD_TOKEN")), nil
	case "aws":
		return awsFromEnv(), nil
	case "vultr":
		return NewVultr(os.Getenv("VULTR_API_KEY")), nil
	}
	return nil, ui.Errf("use one of: "+strings.Join(Names, ", "), "unknown provider %q", name)
}

// api is a small JSON client shared by the providers.
// Sleep and Tries exist so tests can poll without waiting.
type api struct {
	base  string
	token string
	http  *http.Client
	Sleep func(time.Duration)
	Tries int
}

func newAPI(base, token string) *api {
	return &api{
		base:  base,
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
		Sleep: time.Sleep,
		Tries: 60,
	}
}

// do sends one request and decodes the JSON answer into out (if not nil).
// It returns the HTTP status too, so callers can react to a 404.
func (client *api) do(ctx context.Context, method, path string, body, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, client.base+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+client.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.http.Do(req)
	if err != nil {
		return 0, ui.Wrap(err, "check the network", "cannot reach %s", client.base)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))

	if resp.StatusCode >= 300 {
		return resp.StatusCode, statusError(method, path, resp.StatusCode, raw)
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return resp.StatusCode, fmt.Errorf("unreadable response from %s: %w", path, err)
		}
	}
	return resp.StatusCode, nil
}

func statusError(method, path string, status int, raw []byte) error {
	msg := strings.TrimSpace(string(raw))
	if len(msg) > 300 {
		msg = msg[:300]
	}
	hint := ""
	switch status {
	case http.StatusUnauthorized:
		hint = "the API token is wrong or expired: create a new one and export it again"
	case http.StatusForbidden:
		// A 403 means the token works but lacks one permission, which the message names.
		hint = "the token is valid but lacks the permission named above. A custom-scope DigitalOcean token needs droplet (create, read, delete), ssh_key (create, read), tag (create), regions and sizes (read)"
	}
	return ui.Errf(hint, "%s %s: HTTP %d %s", method, path, status, msg)
}

func (client *api) requireToken(env string) error {
	if client.token == "" {
		return ui.Errf("create an API token with write access and run: export "+env+"=...", "%s is not set", env)
	}
	return nil
}

// Fingerprint returns the MD5 fingerprint that DigitalOcean uses.
func Fingerprint(authorizedKey string) (string, error) {
	key, _, _, _, err := ssh.ParseAuthorizedKey([]byte(authorizedKey))
	if err != nil {
		return "", ui.Wrap(err, "pass a public key file such as ~/.ssh/id_ed25519.pub", "cannot read the public key")
	}
	return strings.TrimPrefix(ssh.FingerprintLegacyMD5(key), "MD5:"), nil
}
