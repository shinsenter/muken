// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package muken

import (
	"os/exec"
	"strings"
	"testing"
)

func TestLicenseTextsAreInTheBinary(t *testing.T) {
	cases := map[string]struct{ text, want string }{
		"LICENSE":    {License, "Apache License\n                           Version 2.0, January 2004"},
		"NOTICE":     {Notice, "Copyright 2026 Mai Nhut Tan"},
		"TRADEMARK":  {Trademark, `The name "muken"`},
		"THIRD_PART": {ThirdParty, "Go standard library"},
	}
	for name, c := range cases {
		if !strings.Contains(c.text, c.want) {
			t.Errorf("%s does not contain %q", name, c.want)
		}
	}
}

// A new dependency must come with its license text. Run scripts/third-party-notices.sh if this fails.
func TestEveryModuleHasItsNotice(t *testing.T) {
	for _, target := range [][2]string{{"linux", "amd64"}, {"linux", "arm64"}, {"darwin", "amd64"}, {"darwin", "arm64"}} {
		cmd := exec.Command("go", "list", "-deps", "-f", "{{with .Module}}{{if not .Main}}{{.Path}} {{.Version}}{{end}}{{end}}", "./cmd/muken")
		cmd.Env = append(cmd.Environ(), "GOOS="+target[0], "GOARCH="+target[1])
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("go list for %s/%s: %v", target[0], target[1], err)
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" && !strings.Contains(ThirdParty, line) {
				t.Errorf("THIRD_PARTY_NOTICES.txt has no entry for %q (%s/%s)", line, target[0], target[1])
			}
		}
	}
}
