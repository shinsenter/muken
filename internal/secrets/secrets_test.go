// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTripAndFileIsEncrypted(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MUKEN_AGE_KEY", "")
	t.Setenv("MUKEN_AGE_KEY_FILE", filepath.Join(dir, "age.key"))
	if _, err := Init(dir, filepath.Join(dir, File)); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(dir, filepath.Join(dir, File)); err == nil {
		t.Fatal("Init must not overwrite a key")
	}
	st, _ := os.Stat(KeyPath(dir))
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v", st.Mode())
	}
	path := filepath.Join(dir, File)
	if err := Save(path, map[string]string{"APP_KEY": "base64:abc=", "DB_PASSWORD": "p=w"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "base64:abc") || !strings.Contains(string(raw), "BEGIN AGE ENCRYPTED FILE") {
		t.Fatal("the file must be encrypted")
	}
	m, err := Load(path)
	if err != nil || m["APP_KEY"] != "base64:abc=" || m["DB_PASSWORD"] != "p=w" {
		t.Fatalf("got %v %v", m, err)
	}
}

func TestLoadWithoutFileIsEmpty(t *testing.T) {
	m, err := Load(filepath.Join(t.TempDir(), "none.age"))
	if err != nil || len(m) != 0 {
		t.Fatalf("got %v %v", m, err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MUKEN_AGE_KEY", "")
	t.Setenv("MUKEN_AGE_KEY_FILE", filepath.Join(dir, "a.key"))
	Init(dir, filepath.Join(dir, File))
	path := filepath.Join(dir, File)
	Save(path, map[string]string{"K": "v"})
	t.Setenv("MUKEN_AGE_KEY_FILE", filepath.Join(dir, "b.key"))
	Init(dir, filepath.Join(dir, File))
	if _, err := Load(path); err == nil {
		t.Fatal("another key must not decrypt")
	}
}

func TestKeyFromEnvironmentForCI(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MUKEN_AGE_KEY", "")
	t.Setenv("MUKEN_AGE_KEY_FILE", filepath.Join(dir, "age.key"))
	Init(dir, filepath.Join(dir, File))
	path := filepath.Join(dir, File)
	Save(path, map[string]string{"K": "v"})
	key, _ := os.ReadFile(KeyPath(dir))
	os.Remove(KeyPath(dir))
	t.Setenv("MUKEN_AGE_KEY", string(key))
	m, err := Load(path)
	if err != nil || m["K"] != "v" {
		t.Fatalf("got %v %v", m, err)
	}
}

// isolate gives the test its own home and no key in the environment.
func isolate(t *testing.T) (home string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MUKEN_AGE_KEY", "")
	t.Setenv("MUKEN_AGE_KEY_FILE", "")
	return home
}

// userKey creates a key in the old place, ~/.config/muken/age.key.
func userKey(t *testing.T, home string) string {
	t.Helper()
	path := filepath.Join(home, ".config", "muken", "age.key")
	t.Setenv("MUKEN_AGE_KEY_FILE", path)
	if _, err := Init(t.TempDir(), "none"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MUKEN_AGE_KEY_FILE", "")
	return path
}

func TestInitCreatesTheKeyInTheProject(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	file := filepath.Join(dir, File)
	keyPath, err := Init(dir, file)
	if err != nil || keyPath != filepath.Join(dir, ".muken", "age.key") {
		t.Fatalf("key %q, err %v", keyPath, err)
	}
	if info, _ := os.Stat(keyPath); info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode %v", info.Mode().Perm())
	}
	if got, _ := os.ReadFile(filepath.Join(dir, ".muken", ".gitignore")); string(got) != "*\n" {
		t.Fatalf(".gitignore = %q", got)
	}
	if err := Save(file, map[string]string{"K": "v"}); err != nil {
		t.Fatal(err)
	}
	if m, err := Load(file); err != nil || m["K"] != "v" {
		t.Fatalf("got %v %v", m, err)
	}
}

func TestInitWarnsInsteadOfOverwritingTheProjectKey(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	file := filepath.Join(dir, File)
	keyPath, _ := Init(dir, file)
	before, _ := os.ReadFile(keyPath)
	_, err := Init(dir, file)
	if err == nil || !strings.Contains(err.Error(), "does not overwrite") {
		t.Fatalf("want a warning that init does not overwrite, got %v", err)
	}
	if after, _ := os.ReadFile(keyPath); string(after) != string(before) {
		t.Fatal("the key was overwritten")
	}
}

func TestUserKeyIsTheFallback(t *testing.T) {
	home := isolate(t)
	old := userKey(t, home)
	dir := t.TempDir()
	if got := KeyPath(dir); got != old {
		t.Fatalf("KeyPath = %q, want the key in the config folder %q", got, old)
	}
	file := filepath.Join(dir, File)
	if err := Save(file, map[string]string{"K": "v"}); err != nil {
		t.Fatal(err)
	}
	if m, err := Load(file); err != nil || m["K"] != "v" {
		t.Fatalf("got %v %v", m, err)
	}
}

func TestInitRefusesToShadowTheKeyOfAnExistingSecretsFile(t *testing.T) {
	home := isolate(t)
	old := userKey(t, home)
	dir := t.TempDir()
	file := filepath.Join(dir, File)
	if err := Save(file, map[string]string{"K": "v"}); err != nil {
		t.Fatal(err)
	}
	_, err := Init(dir, file)
	if err == nil || !strings.Contains(err.Error(), "unreadable") || !strings.Contains(err.Error(), old) {
		t.Fatalf("want a refusal that names the old key, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, ".muken", "age.key")); statErr == nil {
		t.Fatal("no new key may be created")
	}
}

func TestInitWithoutSecretsFileMayCreateAProjectKeyNextToAnOldOne(t *testing.T) {
	home := isolate(t)
	userKey(t, home)
	dir := t.TempDir()
	if _, err := Init(dir, filepath.Join(dir, File)); err != nil {
		t.Fatalf("a new project with no secrets file may have its own key: %v", err)
	}
}

func TestEnvironmentFileWinsOverTheProjectKey(t *testing.T) {
	isolate(t)
	dir := t.TempDir()
	if _, err := Init(dir, filepath.Join(dir, File)); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(t.TempDir(), "other.key")
	t.Setenv("MUKEN_AGE_KEY_FILE", other)
	if got := KeyPath(dir); got != other {
		t.Fatalf("KeyPath = %q, want %q", got, other)
	}
}
