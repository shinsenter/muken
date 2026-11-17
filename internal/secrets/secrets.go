// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package secrets keeps environment secrets in an encrypted file that can be committed.
package secrets

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
	"filippo.io/age/armor"

	"github.com/shinsenter/muken/internal/project"
	"github.com/shinsenter/muken/internal/ui"
)

// File is the encrypted secrets file next to muken.yaml.
const File = "muken.secrets.age"

const KeyName = "age.key"

// userKeyPath is where earlier versions kept the key. It is still read when a project has none.
func userKeyPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "muken", KeyName)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// KeyPath returns the key file for the secrets file in dir, the folder of muken.yaml.
// Order: MUKEN_AGE_KEY_FILE, <dir>/.muken/age.key, then the key in the user's config folder.
// With no key it returns the project path, where Init writes. MUKEN_AGE_KEY (the text, for CI) is read by loadIdentity.
func KeyPath(dir string) string {
	if path := os.Getenv("MUKEN_AGE_KEY_FILE"); path != "" {
		return path
	}
	own := filepath.Join(dir, project.Dir, KeyName)
	if !exists(own) && exists(userKeyPath()) {
		return userKeyPath()
	}
	return own
}

func loadIdentity(dir string) (*age.X25519Identity, error) {
	text := os.Getenv("MUKEN_AGE_KEY")
	if text == "" {
		keyData, err := os.ReadFile(KeyPath(dir))
		if err != nil {
			return nil, ui.Errf("run `muken secrets init`, or copy the key file from a teammate into .muken/"+KeyName, "no secrets key at %s", KeyPath(dir))
		}
		text = string(keyData)
	}
	ids, err := age.ParseIdentities(strings.NewReader(text))
	if err != nil || len(ids) == 0 {
		return nil, ui.Wrap(err, "the key file must hold one AGE-SECRET-KEY line", "cannot read the secrets key")
	}
	identity, ok := ids[0].(*age.X25519Identity)
	if !ok {
		return nil, ui.Errf("", "unsupported key type")
	}
	return identity, nil
}

// Init creates a key in <dir>/.muken, or at MUKEN_AGE_KEY_FILE. It never overwrites a key,
// and it refuses to create one that would hide the key that secretsFile needs.
func Init(dir, secretsFile string) (string, error) {
	keyPath := os.Getenv("MUKEN_AGE_KEY_FILE")
	if keyPath == "" {
		root, err := project.EnsureDir(dir)
		if err != nil {
			return "", err
		}
		keyPath = filepath.Join(root, KeyName)
	}
	if exists(keyPath) {
		return keyPath, ui.Errf("keep the key. To start again, move the file away yourself, run `muken secrets init` and set the secrets again",
			"a key already exists at %s. `muken secrets init` does not overwrite it: the secrets it protects could not be read again", keyPath)
	}
	if current := KeyPath(dir); current != keyPath && exists(current) && exists(secretsFile) {
		return keyPath, ui.Errf("copy that key instead: cp "+current+" "+keyPath,
			"%s was most likely encrypted with the key at %s. A new key at %s would make it unreadable", filepath.Base(secretsFile), current, keyPath)
	}
	identity, err := age.GenerateX25519Identity()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return "", err
	}
	return keyPath, os.WriteFile(keyPath, []byte(identity.String()+"\n"), 0o600)
}

// Load decrypts the file. A missing file gives an empty map.
func Load(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()
	identity, err := loadIdentity(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	decrypted, err := age.Decrypt(armor.NewReader(f), identity)
	if err != nil {
		return nil, ui.Wrap(err, "this key cannot open the file. Use the key that created it", "cannot decrypt %s", path)
	}
	plain, err := io.ReadAll(decrypted)
	if err != nil {
		return nil, err
	}
	return parseLines(string(plain)), nil
}

// parseLines reads KEY=value lines. Blank lines and lines that start with # are skipped.
func parseLines(text string) map[string]string {
	values := map[string]string{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(key)] = value
		}
	}
	return values
}

// Save encrypts the map to the key's own recipient.
func Save(path string, values map[string]string) error {
	identity, err := loadIdentity(filepath.Dir(path))
	if err != nil {
		return err
	}
	armored, err := encrypt(renderLines(values), identity.Recipient())
	if err != nil {
		return err
	}
	return os.WriteFile(path, armored, 0o644)
}

// renderLines writes KEY=value lines in sorted order, so the file changes only when a value does.
func renderLines(values map[string]string) []byte {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	for _, key := range keys {
		fmt.Fprintf(&buf, "%s=%s\n", key, values[key])
	}
	return buf.Bytes()
}

func encrypt(plain []byte, recipient age.Recipient) ([]byte, error) {
	var out bytes.Buffer
	armored := armor.NewWriter(&out)
	writer, err := age.Encrypt(armored, recipient)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(plain); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	if err := armored.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
