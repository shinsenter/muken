// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package ci

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestWorkflowIsValidYAML(t *testing.T) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(GitHubWorkflow("main", "v1.2.3")), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["name"] != "Deploy" {
		t.Fatalf("got %v", doc["name"])
	}
}

func TestReleaseVersionIsPinned(t *testing.T) {
	w := GitHubWorkflow("main", "v1.2.3")
	if !strings.Contains(w, "MUKEN_VERSION: v1.2.3\n") || !strings.Contains(w, "sha256sum --check") {
		t.Fatalf("workflow:\n%s", w)
	}
}

func TestDevelopmentBuildLeavesAPlaceholder(t *testing.T) {
	for _, v := range []string{"dev", "dev-abc1234", "v0.1.0-3-gabc1234", "v0.1.0-dirty"} {
		if w := GitHubWorkflow("main", v); !strings.Contains(w, "MUKEN_VERSION: vX.Y.Z") {
			t.Errorf("version %q must give a placeholder:\n%s", v, w)
		}
	}
}

func TestSecretsNeverGoInlineIntoShell(t *testing.T) {
	w := GitHubWorkflow("main", "v1.0.0")
	if strings.Contains(w, `"${{ secrets.`) {
		t.Fatal("a secret is expanded inside a shell script; pass it through env")
	}
}
