// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/shinsenter/muken/internal/config"
)

func TestRenderIsValidYAMLWithProbes(t *testing.T) {
	c, err := config.Parse([]byte(`
app: blog
image: shinsenter/laravel:latest
domains: [blog.example.com]
email: a@example.com
server: {host: 1.2.3.4}
web: {replicas: 3, env: {APP_ENV: production}}
workers: [{name: queue, command: "php artisan queue:work"}]
`))
	if err != nil {
		t.Fatal(err)
	}
	out := Render(c)
	dec := yaml.NewDecoder(bytes.NewReader([]byte(out)))
	kinds := map[string]int{}
	for {
		var doc map[string]any
		if err := dec.Decode(&doc); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatalf("invalid YAML: %v\n%s", err, out)
		}
		kinds[doc["kind"].(string)]++
	}
	if kinds["Deployment"] != 2 || kinds["Service"] != 1 || kinds["Ingress"] != 1 {
		t.Fatalf("kinds %v", kinds)
	}
	for _, want := range []string{"replicas: 3", `path: "/readyz", port: 2020`, "terminationGracePeriodSeconds: 35", "maxUnavailable: 0", "blog-env"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
}
