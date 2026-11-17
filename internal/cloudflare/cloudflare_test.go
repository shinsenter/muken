// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package cloudflare

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shinsenter/muken/internal/ui"
)

func fake(t *testing.T, h http.HandlerFunc) *Client {
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	c := New("tok")
	c.Base = ts.URL
	return c
}

func reply(w http.ResponseWriter, result string) {
	io.WriteString(w, `{"success":true,"errors":[],"result":`+result+`}`)
}

func TestZoneIDWalksUpTheName(t *testing.T) {
	var asked []string
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Error("token not sent")
		}
		n := r.URL.Query().Get("name")
		asked = append(asked, n)
		if n == "example.com" {
			reply(w, `[{"id":"z1"}]`)
			return
		}
		reply(w, `[]`)
	})
	id, err := c.ZoneID(context.Background(), "a.b.example.com")
	if err != nil || id != "z1" {
		t.Fatalf("got %q %v", id, err)
	}
	if strings.Join(asked, ",") != "a.b.example.com,b.example.com,example.com" {
		t.Fatalf("asked %v", asked)
	}
}

func TestZoneIDNotFound(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, _ *http.Request) { reply(w, `[]`) })
	if _, err := c.ZoneID(context.Background(), "x.example.com"); err == nil {
		t.Fatal("want an error")
	}
}

func TestUpsertA(t *testing.T) {
	var method string
	var body map[string]any
	existing := `[]`
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			reply(w, existing)
			return
		}
		method = r.Method
		json.NewDecoder(r.Body).Decode(&body)
		reply(w, `{}`)
	})
	ctx := context.Background()
	if a, err := c.UpsertA(ctx, "z", "a.example.com", "1.2.3.4", true); err != nil || a != "created" || method != "POST" {
		t.Fatalf("create: %s %s %v", a, method, err)
	}
	if body["content"] != "1.2.3.4" || body["proxied"] != true || body["type"] != "A" {
		t.Fatalf("body %v", body)
	}
	existing = `[{"id":"r1","type":"A","content":"1.2.3.4","proxied":true}]`
	method = ""
	if a, _ := c.UpsertA(ctx, "z", "a.example.com", "1.2.3.4", true); a != "unchanged" || method != "" {
		t.Fatalf("same record must not be written: %s %s", a, method)
	}
	if a, _ := c.UpsertA(ctx, "z", "a.example.com", "5.6.7.8", true); a != "updated" || method != "PUT" {
		t.Fatalf("update: %s %s", a, method)
	}
}

func TestSSLAndPurge(t *testing.T) {
	var got []string
	c := fake(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.Path+" "+string(b))
		reply(w, `{}`)
	})
	c.SetSSLMode(context.Background(), "z", "strict")
	c.PurgeHosts(context.Background(), "z", []string{"a.example.com"})
	if got[0] != `PATCH /zones/z/settings/ssl {"value":"strict"}` || got[1] != `POST /zones/z/purge_cache {"hosts":["a.example.com"]}` {
		t.Fatalf("%v", got)
	}
}

func TestAuthErrorHasHint(t *testing.T) {
	c := fake(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(403)
		io.WriteString(w, `{"success":false,"errors":[{"code":9109,"message":"Invalid access token"}]}`)
	})
	_, err := c.ZoneID(context.Background(), "a.example.com")
	he, ok := err.(*ui.HintError)
	if !ok || !strings.Contains(he.Msg, "Invalid access token") || !strings.Contains(he.Hint, "DNS:Edit") {
		t.Fatalf("got %#v", err)
	}
}
