// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shinsenter/muken/internal/config"
)

func hostRule(domains []string) string {
	if len(domains) == 0 {
		return "PathPrefix(`/`)"
	}
	parts := make([]string, len(domains))
	for i, d := range domains {
		parts[i] = "Host(`" + d + "`)"
	}
	return strings.Join(parts, " || ")
}

// RenderRoute returns the Traefik file that sends traffic to the given containers.
// Traefik watches the folder, so replacing this file switches the release.
func RenderRoute(cfg *config.Config, containers []string) string {
	sorted := append([]string(nil), containers...)
	sort.Strings(sorted)
	return renderRoute("muken-"+cfg.App, hostRule(cfg.Domains), cfg.TLS != "none" && len(cfg.Domains) > 0, cfg.Web.Port, sorted)
}

func RenderAgentRoute(cfg *config.Config) string {
	return renderRoute("muken-agent", hostRule([]string{cfg.Agent.Domain}), cfg.TLS != "none", cfg.Agent.Port, []string{"muken-agent"})
}

func renderRoute(id, rule string, secure bool, port int, servers []string) string {
	var b strings.Builder
	entry, tls := "web", ""
	if secure {
		entry = "websecure"
		tls = "      tls:\n        certResolver: le\n"
	}
	fmt.Fprintf(&b, "# Written by muken. Changes are overwritten on the next deploy.\nhttp:\n  routers:\n    %s:\n      rule: %q\n      entryPoints: [%s]\n      service: %s\n%s",
		id, rule, entry, id, tls)
	fmt.Fprintf(&b, "  services:\n    %s:\n      loadBalancer:\n        passHostHeader: true\n        servers:\n", id)
	for _, s := range servers {
		fmt.Fprintf(&b, "          - url: \"http://%s:%d\"\n", s, port)
	}
	return b.String()
}
