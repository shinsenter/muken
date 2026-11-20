// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

// Every webapp folder in src/webapps must have a preset, and the reverse.
func TestPresetsMatchWebappFolders(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "..", "src", "webapps"))
	if err != nil {
		t.Skip("run inside the repository:", err)
	}
	dirs := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() {
			dirs[e.Name()] = true
			if preset, ok := Get(e.Name()); !ok || preset.Kind != "app" {
				t.Errorf("src/webapps/%s has no app preset", e.Name())
			}
		}
	}
	for _, preset := range All() {
		if preset.Kind == "app" && !dirs[preset.Name] {
			t.Errorf("preset %s has no folder in src/webapps", preset.Name)
		}
	}
}

func TestPresetFields(t *testing.T) {
	seen := map[string]bool{}
	for _, preset := range All() {
		if seen[preset.Name] {
			t.Errorf("duplicate preset %s", preset.Name)
		}
		seen[preset.Name] = true
		if preset.Image == "" || preset.HealthPath == "" || preset.Title == "" {
			t.Errorf("preset %s is incomplete: %+v", preset.Name, preset)
		}
	}
}

func TestImageFor(t *testing.T) {
	preset, _ := Get("php-nginx")
	if got := preset.ImageFor("8.3"); got != "shinsenter/php:8.3-fpm-nginx" {
		t.Fatalf("got %s", got)
	}
	if got := preset.ImageFor(PHPReleaseCandidate); got != "shinsenter/php:8.6-fpm-nginx" {
		t.Fatalf("release candidate: got %s", got)
	}
	if got := preset.ImageFor(""); got != "shinsenter/php:"+DefaultPHP+"-fpm-nginx" {
		t.Fatalf("default: got %s", got)
	}
	w, _ := Get("wordpress")
	if got := w.ImageFor("8.3"); got != "shinsenter/wordpress:latest" {
		t.Fatalf("got %s", got)
	}
	rr, _ := Get("php-roadrunner")
	if rr.HealthPath != "/health?plugin=http" {
		t.Fatalf("roadrunner health path is %s", rr.HealthPath)
	}
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	if Detect(dir) != "" {
		t.Fatal("empty folder must give no guess")
	}
	os.WriteFile(filepath.Join(dir, "composer.json"), []byte(`{"require":{"laravel/framework":"^11"}}`), 0o644)
	if got := Detect(dir); got != "laravel" {
		t.Fatalf("got %q", got)
	}
	dir2 := t.TempDir()
	os.MkdirAll(filepath.Join(dir2, "wp-content"), 0o755)
	if got := Detect(dir2); got != "wordpress" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultPHPIsListedAndRCIsOffered(t *testing.T) {
	found, rc := false, PHPReleaseCandidate == ""
	for _, v := range PHPVersions {
		found = found || v == DefaultPHP
		rc = rc || v == PHPReleaseCandidate
	}
	if !found || !rc {
		t.Fatalf("PHPVersions %v must hold the default %s and a release candidate", PHPVersions, DefaultPHP)
	}
}
