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

	"golang.org/x/crypto/ssh"

	"github.com/shinsenter/muken/internal/provider"
	"github.com/shinsenter/muken/internal/ui"
)

func runKey(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err := cmdServer(context.Background(), &ui.Printer{Out: &out, Err: &errOut}, args)
	return out.String() + errOut.String(), err
}

func TestKeyCreate(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "muken.yaml")
	if _, err := runKey(t, "-c", cfg, "key", "acme"); err != nil {
		t.Fatal(err)
	}
	keys := filepath.Join(dir, ".muken", ".ssh")

	info, err := os.Stat(filepath.Join(keys, "acme"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("private key: %v, %v", err, info)
	}
	if info, _ := os.Stat(keys); info.Mode().Perm() != 0o700 {
		t.Fatalf("key folder mode %v", info.Mode().Perm())
	}
	ignore, _ := os.ReadFile(filepath.Join(dir, ".muken", ".gitignore"))
	if string(ignore) != "*\n" {
		t.Fatalf(".gitignore = %q", ignore)
	}

	privData, _ := os.ReadFile(filepath.Join(keys, "acme"))
	signer, err := ssh.ParsePrivateKey(privData)
	if err != nil {
		t.Fatal(err)
	}
	pubLine, _ := os.ReadFile(filepath.Join(keys, "acme.pub"))
	pub, comment, _, _, err := ssh.ParseAuthorizedKey(pubLine)
	if err != nil || comment != "muken-acme" {
		t.Fatalf("public key: %v, comment %q", err, comment)
	}
	if !bytes.Equal(pub.Marshal(), signer.PublicKey().Marshal()) {
		t.Fatal("the .pub file does not match the private key")
	}
	// `server create` passes this line to the provider.
	if _, err := provider.Fingerprint(string(pubLine)); err != nil {
		t.Fatal(err)
	}
}

func TestKeyCreateKeepsAnExistingKey(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "muken.yaml")
	if _, err := runKey(t, "-c", cfg, "key"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(cfg), ".muken", ".ssh", "id_ed25519")
	before, _ := os.ReadFile(path)
	if _, err := runKey(t, "-c", cfg, "key"); err == nil {
		t.Fatal("a second create must fail")
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Fatal("the existing key was overwritten")
	}
}

func TestKeyCreateRejectsBadNames(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "muken.yaml")
	for _, name := range []string{"../evil", "a/b", ".hidden"} {
		if _, err := runKey(t, "-c", cfg, "key", name); err == nil || !strings.Contains(err.Error(), "bad key name") {
			t.Errorf("name %q: want a bad key name error, got %v", name, err)
		}
	}
}

// `server key` must not ask for a provider or call a cloud API.
func TestServerKeyAsksNothing(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "muken.yaml")
	out, err := runKey(t, "-c", cfg, "key")
	if err != nil || !strings.Contains(out, "Created the key") {
		t.Fatalf("got %q, %v", out, err)
	}
}
