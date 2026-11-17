// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"os"
	"strings"
	"testing"
)

const minimal = `
app: blog
image: shinsenter/wordpress:latest
domains: [blog.example.com]
email: me@example.com
server:
  host: 203.0.113.10
`

func TestParseMinimalGetsDefaults(t *testing.T) {
	cfg, err := Parse([]byte(minimal))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.User != "root" || cfg.Server.Port != 22 || cfg.Web.Port != 80 || cfg.Web.HealthPort != 2020 ||
		cfg.Web.HealthPath != "/readyz" || cfg.TLS != "letsencrypt" || cfg.Keep != 2 || cfg.Web.Replicas != 1 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestParseReportsEveryProblemWithFixes(t *testing.T) {
	_, err := Parse([]byte("app: Bad_Name\nimage: \"\"\nserver: {host: \"\"}\ndomains: [not a host]\ntls: weird\n"))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"app:", "image:", "server.host", "domains:", "tls:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not mention %q:\n%v", want, err)
		}
	}
}

func TestMaintenancePageKey(t *testing.T) {
	cfg, err := Parse([]byte(minimal + "maintenance:\n  page: maintenance.html\n"))
	if err != nil || cfg.Maintenance.Page != "maintenance.html" {
		t.Fatalf("page = %q, err %v", cfg.Maintenance.Page, err)
	}
}

func TestParseRejectsUnknownKey(t *testing.T) {
	if _, err := Parse([]byte(minimal + "imagee: typo\n")); err == nil {
		t.Fatal("a typo in a key must be an error")
	}
}

func TestEnvReference(t *testing.T) {
	t.Setenv("MUKEN_TEST_HOOK", "https://hooks.example.com/x")
	cfg, err := Parse([]byte(minimal + "agent:\n  webhook: ${env:MUKEN_TEST_HOOK}\n"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Agent.Webhook != "https://hooks.example.com/x" {
		t.Fatalf("got %q", cfg.Agent.Webhook)
	}
	os.Unsetenv("MUKEN_TEST_HOOK")
	if _, err := Parse([]byte(minimal + "agent:\n  webhook: ${env:MUKEN_TEST_HOOK}\n")); err == nil {
		t.Fatal("a missing variable must be an error")
	}
}

func TestDollarInCommandsIsKept(t *testing.T) {
	cfg, err := Parse([]byte(minimal + "web:\n  migrate: cd \"$APP_PATH\" && php artisan migrate\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cfg.Web.Migrate, "$APP_PATH") {
		t.Fatalf("migrate was changed: %q", cfg.Web.Migrate)
	}
}

func TestTLSNoneAllowsNoDomain(t *testing.T) {
	if _, err := Parse([]byte("app: a\nimage: x\ntls: none\nserver: {host: 1.2.3.4}\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse([]byte("app: a\nimage: x\nserver: {host: 1.2.3.4}\n")); err == nil {
		t.Fatal("HTTPS without a domain must be an error")
	}
}

func TestWorkerAndAccessoryChecks(t *testing.T) {
	bad := minimal + "workers:\n  - {name: web, command: x}\naccessories:\n  - {type: oracle}\n"
	if _, err := Parse([]byte(bad)); err == nil {
		t.Fatal("reserved worker name and unknown accessory must fail")
	}
	good := minimal + "workers:\n  - {name: queue, command: php artisan queue:work}\naccessories:\n  - {type: postgres}\n"
	cfg, err := Parse([]byte(good))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workers[0].User != "www-data" || cfg.Accessories[0].Name != "postgres" {
		t.Fatalf("defaults missing: %+v %+v", cfg.Workers, cfg.Accessories)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	cfg, _ := Parse([]byte(minimal))
	data, err := cfg.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(data); err != nil {
		t.Fatalf("own output does not parse: %v\n%s", err, data)
	}
}
