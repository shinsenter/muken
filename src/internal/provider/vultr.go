// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/ui"
)

// vultrUbuntu2404 is the id of "Ubuntu 24.04 LTS x64" in the Vultr OS list.
const vultrUbuntu2404 = 2284

type Vultr struct{ client *api }

func NewVultr(token string) *Vultr {
	return &Vultr{client: newAPI("https://api.vultr.com/v2", token)}
}

func (v *Vultr) Name() string     { return "vultr" }
func (v *Vultr) TokenEnv() string { return "VULTR_API_KEY" }

func (v *Vultr) Regions(ctx context.Context) ([]Choice, error) {
	if err := v.client.requireToken(v.TokenEnv()); err != nil {
		return nil, err
	}
	var response struct {
		Regions []struct {
			ID      string `json:"id"`
			City    string `json:"city"`
			Country string `json:"country"`
		} `json:"regions"`
	}
	if _, err := v.client.do(ctx, "GET", "/regions?per_page=500", nil, &response); err != nil {
		return nil, err
	}
	var out []Choice
	for _, r := range response.Regions {
		out = append(out, Choice{r.ID, r.City + ", " + r.Country})
	}
	return out, nil
}

// Sizes returns the 8 cheapest regular cloud plans with at least 1 GB of memory.
func (v *Vultr) Sizes(ctx context.Context) ([]Choice, error) {
	if err := v.client.requireToken(v.TokenEnv()); err != nil {
		return nil, err
	}
	var response struct {
		Plans []struct {
			ID    string  `json:"id"`
			VCPUs int     `json:"vcpu_count"`
			RAM   int     `json:"ram"`
			Disk  int     `json:"disk"`
			Price float64 `json:"monthly_cost"`
		} `json:"plans"`
	}
	if _, err := v.client.do(ctx, "GET", "/plans?type=vc2&per_page=500", nil, &response); err != nil {
		return nil, err
	}
	var rows []pricedChoice
	for _, p := range response.Plans {
		if p.RAM < 1024 {
			continue
		}
		label := fmt.Sprintf("%d vCPU, %d MB RAM, %d GB disk, $%.0f/month", p.VCPUs, p.RAM, p.Disk, p.Price)
		rows = append(rows, pricedChoice{Choice{p.ID, label}, p.Price})
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

// keyBody drops the comment, so the same key matches whatever its comment says.
func keyBody(authorizedKey string) string {
	fields := strings.Fields(authorizedKey)
	if len(fields) > 2 {
		fields = fields[:2]
	}
	return strings.Join(fields, " ")
}

// ensureKey finds the SSH key by its content or uploads it. It returns the key id.
// Vultr has no fingerprint filter, so the list is searched here.
func (v *Vultr) ensureKey(ctx context.Context, publicKey string) (string, error) {
	if _, err := Fingerprint(publicKey); err != nil {
		return "", err
	}
	var list struct {
		Keys []struct {
			ID  string `json:"id"`
			Key string `json:"ssh_key"`
		} `json:"ssh_keys"`
	}
	if _, err := v.client.do(ctx, "GET", "/ssh-keys?per_page=500", nil, &list); err != nil {
		return "", err
	}
	want := keyBody(publicKey)
	for _, k := range list.Keys {
		if keyBody(k.Key) == want {
			return k.ID, nil
		}
	}
	var created struct {
		Key struct {
			ID string `json:"id"`
		} `json:"ssh_key"`
	}
	body := map[string]string{"name": "muken", "ssh_key": publicKey}
	if _, err := v.client.do(ctx, "POST", "/ssh-keys", body, &created); err != nil {
		return "", err
	}
	return created.Key.ID, nil
}

func (v *Vultr) Create(ctx context.Context, opts CreateOptions) (Server, error) {
	if err := v.client.requireToken(v.TokenEnv()); err != nil {
		return Server{}, err
	}
	keyID, err := v.ensureKey(ctx, opts.PublicKey)
	if err != nil {
		return Server{}, err
	}
	id, err := v.createInstance(ctx, opts, keyID)
	if err != nil {
		return Server{}, err
	}
	return v.waitActive(ctx, id)
}

func (v *Vultr) createInstance(ctx context.Context, opts CreateOptions, keyID string) (string, error) {
	body := map[string]any{
		"region":    opts.Region,
		"plan":      opts.Size,
		"os_id":     vultrUbuntu2404,
		"label":     opts.Name,
		"hostname":  opts.Name,
		"sshkey_id": []string{keyID},
		"tags":      []string{"muken"},
	}
	var created struct {
		Instance struct {
			ID string `json:"id"`
		} `json:"instance"`
	}
	if _, err := v.client.do(ctx, "POST", "/instances", body, &created); err != nil {
		return "", err
	}
	return created.Instance.ID, nil
}

// waitActive polls every 5 seconds until the instance runs and has a public IPv4 address.
// Vultr reports 0.0.0.0 until the address is assigned.
func (v *Vultr) waitActive(ctx context.Context, id string) (Server, error) {
	for i := 0; i < v.client.Tries; i++ {
		var response struct {
			Instance struct {
				Status string `json:"status"`
				Power  string `json:"power_status"`
				IP     string `json:"main_ip"`
			} `json:"instance"`
		}
		if _, err := v.client.do(ctx, "GET", "/instances/"+id, nil, &response); err != nil {
			return Server{ID: id}, err
		}
		inst := response.Instance
		if inst.Status == "active" && inst.Power == "running" && inst.IP != "" && inst.IP != "0.0.0.0" {
			return Server{ID: id, IP: inst.IP}, nil
		}
		v.client.Sleep(5 * time.Second)
	}
	return Server{ID: id}, ui.Errf("check the Vultr console; delete the instance with `muken server delete`", "instance %s did not become active in time", id)
}

func (v *Vultr) Delete(ctx context.Context, id string) error {
	if err := v.client.requireToken(v.TokenEnv()); err != nil {
		return err
	}
	_, err := v.client.do(ctx, "DELETE", "/instances/"+id, nil, nil)
	return err
}
