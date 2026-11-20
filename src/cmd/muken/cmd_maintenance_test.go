// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shinsenter/muken/internal/config"
)

func TestMaintenancePageFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "muken.yaml")
	cfg := &config.Config{}

	if page, err := maintenancePage(cfg, cfgFile); page != nil || err != nil {
		t.Fatalf("no setting: want nil, nil; got %q, %v", page, err)
	}

	cfg.Maintenance.Page = "down.html"
	if _, err := maintenancePage(cfg, cfgFile); err == nil || !strings.Contains(err.Error(), "cannot read the maintenance page") {
		t.Fatalf("missing file: got %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "down.html"), []byte("<p>down</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if page, err := maintenancePage(cfg, cfgFile); err != nil || string(page) != "<p>down</p>" {
		t.Fatalf("relative path: %q, %v", page, err)
	}

	cfg.Maintenance.Page = filepath.Join(dir, "down.html")
	if page, err := maintenancePage(cfg, "elsewhere/muken.yaml"); err != nil || string(page) != "<p>down</p>" {
		t.Fatalf("absolute path: %q, %v", page, err)
	}
}

func TestMaintenancePageSizeLimits(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Maintenance.Page = "p.html"
	for name, size := range map[string]int{"empty": 0, "too big": maxMaintenancePage + 1} {
		if err := os.WriteFile(filepath.Join(dir, "p.html"), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := maintenancePage(cfg, filepath.Join(dir, "muken.yaml")); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "p.html"), make([]byte, maxMaintenancePage), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := maintenancePage(cfg, filepath.Join(dir, "muken.yaml")); err != nil {
		t.Errorf("exactly the limit must work: %v", err)
	}
}
