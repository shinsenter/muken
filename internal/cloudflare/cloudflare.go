// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package cloudflare calls the few Cloudflare API endpoints muken needs.
package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/ui"
)

type Client struct {
	Token string
	Base  string
	HTTP  *http.Client
}

func New(token string) *Client {
	return &Client{Token: token, Base: "https://api.cloudflare.com/client/v4", HTTP: &http.Client{Timeout: 30 * time.Second}}
}

type envelope struct {
	Success bool            `json:"success"`
	Errors  []apiError      `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// do sends one request and decodes the "result" part of the answer into out (if not nil).
func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var reader io.Reader
	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ui.Wrap(err, "check the network", "cannot reach Cloudflare")
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	var wrapper envelope
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return fmt.Errorf("cloudflare returned HTTP %d with an unreadable body", resp.StatusCode)
	}
	if !wrapper.Success {
		return apiFailure(resp.StatusCode, wrapper.Errors)
	}
	if out != nil {
		return json.Unmarshal(wrapper.Result, out)
	}
	return nil
}

func apiFailure(status int, errs []apiError) error {
	var parts []string
	for _, apiErr := range errs {
		parts = append(parts, fmt.Sprintf("%s (code %d)", apiErr.Message, apiErr.Code))
	}
	msg := strings.Join(parts, "; ")
	if msg == "" {
		msg = fmt.Sprintf("HTTP %d", status)
	}
	hint := ""
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		hint = "the token needs Zone:Read, DNS:Edit, Zone Settings:Edit and Cache Purge for this zone"
	}
	return ui.Errf(hint, "Cloudflare: %s", msg)
}

// ZoneID finds the zone that owns host. It tries www.example.com, then example.com,
// and stops before the bare top-level name.
func (c *Client) ZoneID(ctx context.Context, host string) (string, error) {
	labels := strings.Split(host, ".")
	for i := 0; i < len(labels)-1; i++ {
		name := strings.Join(labels[i:], ".")
		var zones []struct {
			ID string `json:"id"`
		}
		if err := c.do(ctx, "GET", "/zones?name="+url.QueryEscape(name), nil, &zones); err != nil {
			return "", err
		}
		if len(zones) > 0 {
			return zones[0].ID, nil
		}
	}
	return "", ui.Errf("add the domain to Cloudflare first, or check the token's zone access", "no Cloudflare zone found for %s", host)
}

type record struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
}

// UpsertA creates or updates the A record for name. It returns what it did.
func (c *Client) UpsertA(ctx context.Context, zoneID, name, ip string, proxied bool) (string, error) {
	var found []record
	query := fmt.Sprintf("/zones/%s/dns_records?name=%s&type=A", zoneID, url.QueryEscape(name))
	if err := c.do(ctx, "GET", query, nil, &found); err != nil {
		return "", err
	}
	payload := map[string]any{"type": "A", "name": name, "content": ip, "ttl": 1, "proxied": proxied}
	if len(found) == 0 {
		return "created", c.do(ctx, "POST", fmt.Sprintf("/zones/%s/dns_records", zoneID), payload, nil)
	}
	if found[0].Content == ip && found[0].Proxied == proxied {
		return "unchanged", nil
	}
	return "updated", c.do(ctx, "PUT", fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, found[0].ID), payload, nil)
}

// SetSSLMode sets the zone's SSL mode: off, flexible, full or strict.
func (c *Client) SetSSLMode(ctx context.Context, zoneID, mode string) error {
	return c.do(ctx, "PATCH", fmt.Sprintf("/zones/%s/settings/ssl", zoneID), map[string]string{"value": mode}, nil)
}

func (c *Client) PurgeHosts(ctx context.Context, zoneID string, hosts []string) error {
	return c.do(ctx, "POST", fmt.Sprintf("/zones/%s/purge_cache", zoneID), map[string]any{"hosts": hosts}, nil)
}
