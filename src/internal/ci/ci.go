// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package ci writes the GitHub Actions workflow that runs muken.
package ci

import "strings"

// GitHubWorkflow returns a workflow that deploys on every push to the branch.
// It pins the muken release for version and checks its checksum. A development build has no release, so the user fills it in.
func GitHubWorkflow(branch, version string) string {
	note := ""
	if !isRelease(version) {
		version = "vX.Y.Z"
		note = "  # set a muken release from the Releases page"
	}
	return `# Written by muken. Add these repository secrets before the first run:
#   MUKEN_SSH_KEY   private key that can log in to the server
#   MUKEN_AGE_KEY   contents of .muken/age.key (only if you use muken.secrets.age)
name: Deploy
on:
  push:
    branches: [` + branch + `]
  workflow_dispatch:
concurrency:
  group: deploy
  cancel-in-progress: false
permissions:
  contents: read
jobs:
  deploy:
    runs-on: ubuntu-latest
    env:
      MUKEN_VERSION: ` + version + note + `
    steps:
      - uses: actions/checkout@v4
      - name: Install muken
        env:
          GH_TOKEN: ${{ github.token }}
        run: |
          gh release download "$MUKEN_VERSION" --repo shinsenter/muken \
            --pattern muken-linux-amd64 --pattern checksums.txt --dir /tmp/muken
          (cd /tmp/muken && sha256sum --check --ignore-missing checksums.txt)
          sudo install -m 755 /tmp/muken/muken-linux-amd64 /usr/local/bin/muken
      - name: Load the SSH key
        env:
          SSH_KEY: ${{ secrets.MUKEN_SSH_KEY }}
        run: |
          install -m 700 -d ~/.ssh
          printf '%s\n' "$SSH_KEY" > ~/.ssh/id_ed25519
          chmod 600 ~/.ssh/id_ed25519
      - name: Plan
        run: muken deploy --plan --accept-new
        env:
          MUKEN_AGE_KEY: ${{ secrets.MUKEN_AGE_KEY }}
      - name: Deploy
        run: muken deploy --yes --accept-new
        env:
          MUKEN_AGE_KEY: ${{ secrets.MUKEN_AGE_KEY }}
`
}

// isRelease is false for builds such as dev, v0.1.0-3-gabc1234 or v0.1.0-dirty.
func isRelease(version string) bool {
	if !strings.HasPrefix(version, "v") || strings.Contains(version, " ") {
		return false
	}
	return !strings.Contains(version, "-g") && !strings.HasSuffix(version, "-dirty")
}
