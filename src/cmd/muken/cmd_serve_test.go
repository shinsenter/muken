// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/shinsenter/muken/internal/api"
	"github.com/shinsenter/muken/internal/ui"
)

func TestServeRefusesAPublicAddress(t *testing.T) {
	err := cmdServe(context.Background(), ui.New(), []string{"--addr", "0.0.0.0:7600"})
	var he *ui.HintError
	if err == nil || !strings.Contains(err.Error(), "0.0.0.0") {
		t.Fatalf("want a refusal, got %v", err)
	}
	if !asHint(err, &he) || !strings.Contains(he.Hint, "loopback") {
		t.Errorf("the hint should name loopback: %v", err)
	}
}

func TestServeBackendRunsACommand(t *testing.T) {
	var out api.Lines
	b := &serveBackend{config: "muken.yaml"}
	if err := b.Run(context.Background(), &out, []string{"version"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "muken") || strings.Contains(out.String(), "\x1b[") {
		t.Errorf("version output: %q", out.String())
	}
}

func TestServeBackendReportsAnErrorWithItsHint(t *testing.T) {
	var out api.Lines
	b := &serveBackend{config: t.TempDir() + "/missing.yaml"}
	err := b.Run(context.Background(), &out, []string{"doctor"})
	if err == nil || out.String() == "" {
		t.Fatalf("want an error and text, got %v %q", err, out.String())
	}
	if err := b.Run(context.Background(), &out, []string{"nope"}); err == nil {
		t.Error("an unknown command should fail")
	}
}

func asHint(err error, target **ui.HintError) bool { return errors.As(err, target) }
