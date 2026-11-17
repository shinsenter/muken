// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDirKeepsTheFolderOutOfGit(t *testing.T) {
	base := t.TempDir()
	root, err := EnsureDir(base)
	if err != nil || root != filepath.Join(base, ".muken") {
		t.Fatalf("root %q, err %v", root, err)
	}
	if info, _ := os.Stat(root); info.Mode().Perm() != 0o700 {
		t.Fatalf("folder mode %v", info.Mode().Perm())
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".gitignore")); string(got) != "*\n" {
		t.Fatalf(".gitignore = %q", got)
	}
}

func TestEnsureDirKeepsAnExistingGitignore(t *testing.T) {
	base := t.TempDir()
	root, _ := EnsureDir(base)
	custom := []byte("*\n!keep\n")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), custom, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := EnsureDir(base); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".gitignore")); string(got) != string(custom) {
		t.Fatalf("the .gitignore was rewritten: %q", got)
	}
}
