// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"github.com/shinsenter/muken/internal/ui"
)

type DigitalOcean struct{ client *api }

func NewDigitalOcean(token string) *DigitalOcean {
	return &DigitalOcean{client: newAPI("https://api.digitalocean.com/v2", token)}
}

func (d *DigitalOcean) Name() string     { return "digitalocean" }
func (d *DigitalOcean) TokenEnv() string { return "DIGITALOCEAN_TOKEN" }

func (d *DigitalOcean) Regions(ctx context.Context) ([]Choice, error) {
	if err := d.client.requireToken(d.TokenEnv()); err != nil {
		return nil, err
	}
	var response struct {
		Regions []struct {
			Slug      string `json:"slug"`
			Name      string `json:"name"`
			Available bool   `json:"available"`
		} `json:"regions"`
	}
	if _, err := d.client.do(ctx, "GET", "/regions?per_page=200", nil, &response); err != nil {
		return nil, err
	}
	var out []Choice
	for _, region := range response.Regions {
		if region.Available {
			out = append(out, Choice{region.Slug, region.Name})
		}
	}
	return out, nil
}

// Sizes returns the 8 cheapest plans with at least 1 GB of memory.
func (d *DigitalOcean) Sizes(ctx context.Context) ([]Choice, error) {
	if err := d.client.requireToken(d.TokenEnv()); err != nil {
		return nil, err
	}
	var response struct {
		Sizes []struct {
			Slug      string  `json:"slug"`
			Memory    int     `json:"memory"`
			VCPUs     int     `json:"vcpus"`
			Disk      int     `json:"disk"`
			Price     float64 `json:"price_monthly"`
			Available bool    `json:"available"`
		} `json:"sizes"`
	}
	if _, err := d.client.do(ctx, "GET", "/sizes?per_page=200", nil, &response); err != nil {
		return nil, err
	}
	sort.Slice(response.Sizes, func(i, j int) bool { return response.Sizes[i].Price < response.Sizes[j].Price })
	var out []Choice
	for _, size := range response.Sizes {
		if !size.Available || size.Memory < 1024 || len(out) >= 8 {
			continue
		}
		label := fmt.Sprintf("%d vCPU, %d MB RAM, %d GB disk, $%.0f/month", size.VCPUs, size.Memory, size.Disk, size.Price)
		out = append(out, Choice{size.Slug, label})
	}
	return out, nil
}

// ensureKey uploads the SSH key unless DigitalOcean already knows it. It returns the fingerprint.
func (d *DigitalOcean) ensureKey(ctx context.Context, publicKey string) (string, error) {
	fingerprint, err := Fingerprint(publicKey)
	if err != nil {
		return "", err
	}
	status, err := d.client.do(ctx, "GET", "/account/keys/"+url.PathEscape(fingerprint), nil, nil)
	if err == nil {
		return fingerprint, nil
	}
	if status != http.StatusNotFound {
		return "", err
	}
	_, err = d.client.do(ctx, "POST", "/account/keys", map[string]string{"name": "muken", "public_key": publicKey}, nil)
	return fingerprint, err
}

func (d *DigitalOcean) Create(ctx context.Context, opts CreateOptions) (Server, error) {
	if err := d.client.requireToken(d.TokenEnv()); err != nil {
		return Server{}, err
	}
	fingerprint, err := d.ensureKey(ctx, opts.PublicKey)
	if err != nil {
		return Server{}, err
	}
	id, err := d.createDroplet(ctx, opts, fingerprint)
	if err != nil {
		return Server{}, err
	}
	return d.waitActive(ctx, id)
}

func (d *DigitalOcean) createDroplet(ctx context.Context, opts CreateOptions, fingerprint string) (string, error) {
	body := map[string]any{
		"name":       opts.Name,
		"region":     opts.Region,
		"size":       opts.Size,
		"image":      "ubuntu-24-04-x64",
		"ssh_keys":   []string{fingerprint},
		"tags":       []string{"muken"},
		"monitoring": true,
	}
	var created struct {
		Droplet struct {
			ID int `json:"id"`
		} `json:"droplet"`
	}
	if _, err := d.client.do(ctx, "POST", "/droplets", body, &created); err != nil {
		return "", err
	}
	return strconv.Itoa(created.Droplet.ID), nil
}

// waitActive polls every 5 seconds until the droplet is active and has a public IPv4 address.
func (d *DigitalOcean) waitActive(ctx context.Context, id string) (Server, error) {
	for i := 0; i < d.client.Tries; i++ {
		var response struct {
			Droplet struct {
				Status   string `json:"status"`
				Networks struct {
					V4 []struct {
						IP   string `json:"ip_address"`
						Type string `json:"type"`
					} `json:"v4"`
				} `json:"networks"`
			} `json:"droplet"`
		}
		if _, err := d.client.do(ctx, "GET", "/droplets/"+id, nil, &response); err != nil {
			return Server{ID: id}, err
		}
		if response.Droplet.Status == "active" {
			for _, n := range response.Droplet.Networks.V4 {
				if n.Type == "public" {
					return Server{ID: id, IP: n.IP}, nil
				}
			}
		}
		d.client.Sleep(5 * time.Second)
	}
	return Server{ID: id}, ui.Errf("check the DigitalOcean console; delete the droplet with `muken server delete`", "droplet %s did not become active in time", id)
}

func (d *DigitalOcean) Delete(ctx context.Context, id string) error {
	if err := d.client.requireToken(d.TokenEnv()); err != nil {
		return err
	}
	_, err := d.client.do(ctx, "DELETE", "/droplets/"+id, nil, nil)
	return err
}
