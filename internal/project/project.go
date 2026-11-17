// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package project knows the folder that muken keeps next to muken.yaml.
package project

import (
	"os"
	"path/filepath"
)

// Dir is the folder next to the config file that holds the keys of the project.
const Dir = ".muken"

// EnsureDir creates <base>/.muken, with a .gitignore that keeps everything in it out of git,
// and returns its path.
func EnsureDir(base string) (string, error) {
	root := filepath.Join(base, Dir)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", err
	}
	ignore := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(ignore); err != nil {
		if err := os.WriteFile(ignore, []byte("*\n"), 0o644); err != nil {
			return "", err
		}
	}
	return root, nil
}
