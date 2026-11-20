// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/shinsenter/muken/internal/ui"
)

type Hetzner struct{ client *api }

func NewHetzner(token string) *Hetzner {
	return &Hetzner{client: newAPI("https://api.hetzner.cloud/v1", token)}
}

func (h *Hetzner) Name() string     { return "hetzner" }
func (h *Hetzner) TokenEnv() string { return "HCLOUD_TOKEN" }

func (h *Hetzner) Regions(ctx context.Context) ([]Choice, error) {
	if err := h.client.requireToken(h.TokenEnv()); err != nil {
		return nil, err
	}
	var response struct {
		Locations []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"locations"`
	}
	if _, err := h.client.do(ctx, "GET", "/locations", nil, &response); err != nil {
		return nil, err
	}
	var out []Choice
	for _, l := range response.Locations {
		out = append(out, Choice{l.Name, l.Description})
	}
	return out, nil
}

type hetznerType struct {
	Name       string  `json:"name"`
	Cores      int     `json:"cores"`
	Memory     float64 `json:"memory"`
	Disk       int     `json:"disk"`
	Deprecated bool    `json:"deprecated"`
	Prices     []struct {
		Monthly struct {
			Gross string `json:"gross"`
		} `json:"price_monthly"`
	} `json:"prices"`
}

type pricedChoice struct {
	choice Choice
	price  float64
}

// Sizes returns the 8 cheapest plans with at least 1 GB of memory.
func (h *Hetzner) Sizes(ctx context.Context) ([]Choice, error) {
	if err := h.client.requireToken(h.TokenEnv()); err != nil {
		return nil, err
	}
	var response struct {
		Types []hetznerType `json:"server_types"`
	}
	if _, err := h.client.do(ctx, "GET", "/server_types?per_page=50", nil, &response); err != nil {
		return nil, err
	}
	var rows []pricedChoice
	for _, t := range response.Types {
		if t.Deprecated || t.Memory < 1 || len(t.Prices) == 0 {
			continue
		}
		price, _ := strconv.ParseFloat(t.Prices[0].Monthly.Gross, 64)
		label := fmt.Sprintf("%d vCPU, %.0f GB RAM, %d GB disk, EUR %.2f/month", t.Cores, t.Memory, t.Disk, price)
		rows = append(rows, pricedChoice{Choice{t.Name, label}, price})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].price < rows[j].price })
	if len(rows) > 8 {
		rows = rows[:8]
	}
	var out []Choice
	for _, row := range rows {
		out = append(out, row.choice)
	}
	return out, nil
}

// ensureKey finds the SSH key by fingerprint or uploads it. It returns the key id.
func (h *Hetzner) ensureKey(ctx context.Context, publicKey string) (int, error) {
	fingerprint, err := Fingerprint(publicKey)
	if err != nil {
		return 0, err
	}
	var list struct {
		Keys []struct {
			ID int `json:"id"`
		} `json:"ssh_keys"`
	}
	if _, err := h.client.do(ctx, "GET", "/ssh_keys?fingerprint="+fingerprint, nil, &list); err != nil {
		return 0, err
	}
	if len(list.Keys) > 0 {
		return list.Keys[0].ID, nil
	}
	var created struct {
		Key struct {
			ID int `json:"id"`
		} `json:"ssh_key"`
	}
	body := map[string]string{"name": "muken-" + fingerprint[:8], "public_key": publicKey}
	if _, err := h.client.do(ctx, "POST", "/ssh_keys", body, &created); err != nil {
		return 0, err
	}
	return created.Key.ID, nil
}

func (h *Hetzner) Create(ctx context.Context, opts CreateOptions) (Server, error) {
	if err := h.client.requireToken(h.TokenEnv()); err != nil {
		return Server{}, err
	}
	keyID, err := h.ensureKey(ctx, opts.PublicKey)
	if err != nil {
		return Server{}, err
	}
	id, err := h.createServer(ctx, opts, keyID)
	if err != nil {
		return Server{}, err
	}
	return h.waitRunning(ctx, id)
}

func (h *Hetzner) createServer(ctx context.Context, opts CreateOptions, keyID int) (string, error) {
	body := map[string]any{
		"name":        opts.Name,
		"server_type": opts.Size,
		"location":    opts.Region,
		"image":       "ubuntu-24.04",
		"ssh_keys":    []int{keyID},
		"labels":      map[string]string{"managed-by": "muken"},
	}
	var created struct {
		Server struct {
			ID int `json:"id"`
		} `json:"server"`
	}
	if _, err := h.client.do(ctx, "POST", "/servers", body, &created); err != nil {
		return "", err
	}
	return strconv.Itoa(created.Server.ID), nil
}

// waitRunning polls every 5 seconds until the server runs and has an IPv4 address.
func (h *Hetzner) waitRunning(ctx context.Context, id string) (Server, error) {
	for i := 0; i < h.client.Tries; i++ {
		var response struct {
			Server struct {
				Status    string `json:"status"`
				PublicNet struct {
					IPv4 struct {
						IP string `json:"ip"`
					} `json:"ipv4"`
				} `json:"public_net"`
			} `json:"server"`
		}
		if _, err := h.client.do(ctx, "GET", "/servers/"+id, nil, &response); err != nil {
			return Server{ID: id}, err
		}
		if ip := response.Server.PublicNet.IPv4.IP; response.Server.Status == "running" && ip != "" {
			return Server{ID: id, IP: ip}, nil
		}
		h.client.Sleep(5 * time.Second)
	}
	return Server{ID: id}, ui.Errf("check the Hetzner console", "server %s did not start in time", id)
}

func (h *Hetzner) Delete(ctx context.Context, id string) error {
	if err := h.client.requireToken(h.TokenEnv()); err != nil {
		return err
	}
	_, err := h.client.do(ctx, "DELETE", "/servers/"+id, nil, nil)
	return err
}
