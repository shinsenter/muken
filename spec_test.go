// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package muken

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	testFunc = regexp.MustCompile(`(?m)^func (Test\w+)\(`)
	testName = regexp.MustCompile("`(Test\\w+)`")
)

// knownTests lists the name of every Test function in the repository.
func knownTests(t *testing.T) map[string]bool {
	t.Helper()
	known := map[string]bool{}
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "dist") {
			return filepath.SkipDir
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, m := range testFunc.FindAllStringSubmatch(string(data), -1) {
			known[m[1]] = true
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return known
}

// checkSpec returns one message per problem in the text of a spec file.
func checkSpec(text string, known map[string]bool) []string {
	var problems []string
	for _, part := range strings.Split(text, "\n### Requirement: ")[1:] {
		name, body, _ := strings.Cut(part, "\n")
		statement, _, _ := strings.Cut(body, "#### Scenario:")
		if !strings.Contains(statement, "SHALL") {
			problems = append(problems, name+": no SHALL")
		}
		if !strings.Contains(body, "#### Scenario:") {
			problems = append(problems, name+": no scenario")
		}
		_, rest, found := strings.Cut(body, "\nTests:")
		if !found {
			problems = append(problems, name+": no Tests line")
			continue
		}
		line, _, _ := strings.Cut(rest, "\n")
		for _, m := range testName.FindAllStringSubmatch(line, -1) {
			if !known[m[1]] {
				problems = append(problems, name+": unknown test "+m[1])
			}
		}
	}
	return problems
}

func TestSpecsAreWellFormed(t *testing.T) {
	files, err := filepath.Glob("openspec/specs/*/spec.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no specs found: %v", err)
	}
	known := knownTests(t)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, problem := range checkSpec(string(data), known) {
			t.Errorf("%s: %s", file, problem)
		}
	}
}

func TestCheckSpecFindsProblems(t *testing.T) {
	known := map[string]bool{"TestReal": true}
	const good = "\n### Requirement: A\nThe system SHALL do it.\n\n#### Scenario: S\n- **WHEN** x\n- **THEN** y\n\nTests: `TestReal`\n"
	cases := []struct {
		name, text, want string
	}{
		{"well formed", good, ""},
		{"no tests yet", strings.Replace(good, "`TestReal`", "none yet", 1), ""},
		{"no SHALL", strings.Replace(good, "SHALL", "does", 1), "no SHALL"},
		{"no scenario", strings.Replace(good, "#### Scenario:", "#### Case:", 1), "no scenario"},
		{"no Tests line", strings.Replace(good, "Tests: `TestReal`\n", "", 1), "no Tests line"},
		{"unknown test", strings.Replace(good, "`TestReal`", "`TestInvented`", 1), "unknown test TestInvented"},
	}
	for _, c := range cases {
		problems := strings.Join(checkSpec(c.text, known), "; ")
		if (c.want == "") != (problems == "") || !strings.Contains(problems, c.want) {
			t.Errorf("%s: want %q, got %q", c.name, c.want, problems)
		}
	}
}
