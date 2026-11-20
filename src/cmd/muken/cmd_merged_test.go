// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shinsenter/muken/internal/ui"
)

func runCmd(t *testing.T, fn func(context.Context, *ui.Printer, []string) error, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = fn(context.Background(), &ui.Printer{Out: &out, Err: &errOut}, args)
	return out.String(), errOut.String(), err
}

func TestVersionShowsLicense(t *testing.T) {
	out, _, err := runCmd(t, cmdVersion)
	if err != nil || !strings.Contains(out, "muken ") || !strings.Contains(out, "muken version --license") {
		t.Fatalf("version: %q, %v", out, err)
	}
	out, _, err = runCmd(t, cmdVersion, "--license")
	if err != nil || !strings.Contains(out, "Apache License") {
		t.Fatalf("--license: %v", err)
	}
	out, _, err = runCmd(t, cmdVersion, "--third-party")
	if err != nil || out == "" {
		t.Fatalf("--third-party: %q, %v", out, err)
	}
}

func TestInitList(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "muken.yaml")
	out, _, err := runCmd(t, cmdInit, "-c", cfg, "--list")
	if err != nil || !strings.Contains(out, "muken init --preset <name>") {
		t.Fatalf("init --list: %q, %v", out, err)
	}
	if _, err := os.Stat(cfg); err == nil {
		t.Fatal("--list must not write muken.yaml")
	}
}

func TestInitWritesTheSSHKeyPath(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "muken.yaml")
	key := filepath.Join(dir, ".muken", ".ssh", "id_ed25519")
	_, _, err := runCmd(t, cmdInit, "-c", cfg, "--preset", "laravel", "--app", "shop", "--host", "203.0.113.7", "--ssh-key", key)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(cfg)
	if !strings.Contains(string(data), "ssh_key: "+key) {
		t.Fatalf("muken.yaml has no ssh_key:\n%s", data)
	}
	if _, _, err := runCmd(t, cmdInit, "-c", cfg, "--preset", "laravel", "--app", "shop"); err == nil {
		t.Fatal("init must not overwrite an existing file")
	}
}

func TestRenderCIPrintsTheWorkflow(t *testing.T) {
	t.Chdir(t.TempDir())
	out, errOut, err := runCmd(t, cmdRender, "--branch", "release", "ci")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "branches: [release]") || !strings.Contains(out, "muken deploy --plan") {
		t.Fatalf("workflow: %q", out)
	}
	if !strings.Contains(errOut, "Add the repository secrets") || strings.Contains(out, "Add the repository secrets") {
		t.Fatalf("the secrets hint belongs on stderr, got stderr %q", errOut)
	}
	if entries, _ := os.ReadDir("."); len(entries) != 0 {
		t.Fatalf("render must not write files, found %v", entries)
	}
	if _, _, err := runCmd(t, cmdRender, "nope"); err == nil {
		t.Fatal("an unknown target must fail")
	}
}
