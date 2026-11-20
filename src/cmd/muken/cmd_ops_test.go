// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/deploy"
	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/ui"
)

func TestExecCommand(t *testing.T) {
	cases := []struct {
		name string
		user string
		args []string
		tty  bool
		want []string
	}{
		{"terminal shell", "", nil, true, []string{"docker", "exec", "-it", "web", "sh"}},
		{"script command", "", []string{"php", "artisan", "about"}, false, []string{"docker", "exec", "-i", "web", "php", "artisan", "about"}},
		{"user", "www-data", []string{"id"}, false, []string{"docker", "exec", "-i", "-u", "www-data", "web", "id"}},
	}
	for _, tc := range cases {
		got := execCommand("web", tc.user, tc.args, tc.tty)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestDoctorNewServerIsNotAFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &doctor{p: &ui.Printer{Out: &out, Err: &errOut}}
	d.checkServer(context.Background(), &remote.Fake{})
	if d.failed != 0 || d.result() != nil {
		t.Fatalf("a new server must not fail doctor, failed=%d", d.failed)
	}
	if !strings.Contains(errOut.String(), "not set up yet") {
		t.Fatalf("expected a hint to run `muken up`, got %q", errOut.String())
	}
}

func TestDoctorBrokenProxyFails(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &doctor{p: &ui.Printer{Out: &out, Err: &errOut}}
	// muken set the server up before (its network exists), but the proxy is down.
	fake := (&remote.Fake{}).On("docker version", "27.0.1", nil).On("docker network inspect", "muken", nil)
	d.checkServer(context.Background(), fake)
	if d.failed != 1 {
		t.Fatalf("a proxy that stopped on a prepared server is a fault, failed=%d", d.failed)
	}
}

// A server with Docker but without the muken network was never set up by `muken up`.
// Running doctor before the first `up` must not fail.
func TestDoctorDockerWithoutProxyIsNotAFailureBeforeFirstUp(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &doctor{p: &ui.Printer{Out: &out, Err: &errOut}}
	d.checkServer(context.Background(), (&remote.Fake{}).On("docker version", "27.0.1", nil))
	if d.failed != 0 || d.result() != nil {
		t.Fatalf("doctor before the first up must not fail, failed=%d", d.failed)
	}
	if !strings.Contains(errOut.String(), "Run `muken up` to set the server up") {
		t.Fatalf("expected a hint to run `muken up`, got %q", errOut.String())
	}
}

func TestDoctorRunningProxyPasses(t *testing.T) {
	var out, errOut bytes.Buffer
	d := &doctor{p: &ui.Printer{Out: &out, Err: &errOut}}
	fake := (&remote.Fake{}).On("docker version", "27.0.1", nil).On("muken-traefik", "true", nil)
	d.checkServer(context.Background(), fake)
	if d.failed != 0 || !strings.Contains(out.String(), "RUNNING") {
		t.Fatalf("failed=%d out=%q", d.failed, out.String())
	}
}

func TestWarnWithoutCode(t *testing.T) {
	cfg := &config.Config{Image: "shinsenter/laravel:latest"}
	cfg.Web.Migrate = "php artisan migrate --force"
	run := func(own, dir string, withCfg *config.Config) string {
		var out, errOut bytes.Buffer
		warnWithoutCode(&ui.Printer{Out: &out, Err: &errOut}, withCfg, own, dir)
		return errOut.String()
	}
	if got := run("", t.TempDir(), cfg); !strings.Contains(got, "No application code") {
		t.Fatalf("expected a warning, got %q", got)
	}
	if got := run("ghcr.io/me/app:1", t.TempDir(), cfg); got != "" {
		t.Fatalf("own image: no warning expected, got %q", got)
	}
	if got := run("", t.TempDir(), &config.Config{}); got != "" {
		t.Fatalf("nothing to run: no warning expected, got %q", got)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "artisan"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := run("", dir, cfg); got != "" {
		t.Fatalf("project present: no warning expected, got %q", got)
	}
}

func TestRollbackLine(t *testing.T) {
	status := &deploy.Status{State: deploy.State{Release: "r2", Previous: "r1", PreviousImage: "app:1"}}
	if got := rollbackLine(status); !strings.Contains(got, "pruned") || !strings.Contains(got, "--image app:1") {
		t.Fatalf("pruned release: %q", got)
	}
	status.Containers = []deploy.Container{{Name: "c", Release: "r1"}}
	if got := rollbackLine(status); got != "Rollback to: r1" {
		t.Fatalf("kept release: %q", got)
	}
	if got := rollbackLine(&deploy.Status{}); got != "" {
		t.Fatalf("no previous release: %q", got)
	}
}

func TestMaintenanceFlagAfterWord(t *testing.T) {
	var out, errOut bytes.Buffer
	p := &ui.Printer{Out: &out, Err: &errOut}
	err := cmdMaintenance(context.Background(), p, []string{"on", "--yes"})
	var hintErr *ui.HintError
	if err == nil || !errors.As(err, &hintErr) || !strings.Contains(hintErr.Hint, "muken maintenance --yes on") {
		t.Fatalf("expected a hint about flag order, got %v", err)
	}
}

func TestDeploySummary(t *testing.T) {
	cfg := &config.Config{TLS: "none"}
	cfg.Server.Host = "203.0.113.9"
	title, details := deploySummary(cfg, &deploy.Result{Release: "r2", Image: "app:2", Previous: "r1"}, 74*time.Second)
	if title != "Release r2 is live" {
		t.Fatalf("title: %q", title)
	}
	got := map[string]string{}
	for _, detail := range details {
		got[detail[0]] = detail[1]
	}
	if got["Open"] != "http://203.0.113.9" || got["Time"] != "1m14s" || !strings.Contains(got["Next"], "muken rollback") {
		t.Fatalf("details: %v", got)
	}
	cfg = &config.Config{TLS: "letsencrypt", Domains: []string{"a.example.com", "b.example.com"}}
	_, details = deploySummary(cfg, &deploy.Result{Release: "r1", Image: "app:1"}, time.Second)
	for _, detail := range details {
		if detail[0] == "Open" && detail[1] != "https://a.example.com, https://b.example.com" {
			t.Fatalf("open: %q", detail[1])
		}
		if detail[0] == "Next" && strings.Contains(detail[1], "rollback") {
			t.Fatalf("no rollback hint on a first deploy: %q", detail[1])
		}
	}
}

func TestVerboseFlagAndEnvironment(t *testing.T) {
	for _, tc := range []struct {
		args []string
		env  string
		want bool
	}{{[]string{"-v"}, "", true}, {[]string{"--verbose"}, "", true}, {nil, "1", true}, {nil, "", false}} {
		t.Setenv("MUKEN_VERBOSE", tc.env)
		var global commonFlags
		p := &ui.Printer{Out: &bytes.Buffer{}, Err: &bytes.Buffer{}}
		flagSet := newFlags("test", &global)
		if err := parseFlags(flagSet, &global, p, tc.args); err != nil {
			t.Fatal(err)
		}
		if p.Verbose != tc.want {
			t.Errorf("args=%v env=%q: Verbose=%v, want %v", tc.args, tc.env, p.Verbose, tc.want)
		}
	}
}
