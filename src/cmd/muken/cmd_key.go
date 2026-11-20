// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/shinsenter/muken/internal/project"
	"github.com/shinsenter/muken/internal/ui"
)

var keyName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func serverKey(p *ui.Printer, global commonFlags, name string) error {
	if name == "" {
		name = "id_ed25519"
	}
	privPath, err := createKey(filepath.Dir(global.config), name)
	if err != nil {
		return err
	}
	p.OK("Created the key %s", privPath)
	p.Info("Next: muken server create --ssh-pubkey %s.pub", privPath)
	p.Info("Then set `ssh_key: %s` under `server:` in %s.", privPath, global.config)
	p.Warn("The key has no passphrase. Add `.muken` to .dockerignore, so it never goes into your app image. The folder has its own .gitignore.")
	return nil
}

// createKey writes an ed25519 pair to <configDir>/.muken/.ssh. It never overwrites a key.
func createKey(configDir, name string) (string, error) {
	if !keyName.MatchString(name) {
		return "", ui.Errf("use letters, digits, '.', '-' and '_'", "bad key name %q", name)
	}
	root, err := project.EnsureDir(configDir)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, ".ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}

	privPEM, pubLine, err := newKeyPair("muken-" + name)
	if err != nil {
		return "", err
	}
	privPath := filepath.Join(dir, name)
	if err := writeNew(privPath, privPEM, 0o600); err != nil {
		return "", ui.Wrap(err, "pick another name (muken server key <name>), or delete the old key yourself", "cannot write %s", privPath)
	}
	return privPath, writeNew(privPath+".pub", []byte(pubLine), 0o644)
}

// newKeyPair returns the private key as PEM and the public key as an authorized_keys line.
func newKeyPair(comment string) ([]byte, string, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", err
	}
	block, err := ssh.MarshalPrivateKey(priv, comment)
	if err != nil {
		return nil, "", err
	}
	sshPub, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, "", err
	}
	line := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub))) + " " + comment + "\n"
	return pem.EncodeToMemory(block), line, nil
}

// writeNew creates the file and fails when it already exists.
func writeNew(path string, data []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
