// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package k8s renders Kubernetes manifests from muken.yaml.
package k8s

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shinsenter/muken/internal/config"
)

func envBlock(env map[string]string, indent string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	out.WriteString(indent + "env:\n")
	for _, key := range keys {
		fmt.Fprintf(&out, "%s  - name: %s\n%s    value: %q\n", indent, key, indent, env[key])
	}
	return out.String()
}

// Render returns a multi-document YAML: Deployments, Service and Ingress.
// Secrets are expected in a Secret named <app>-env, which you create yourself.
func Render(cfg *config.Config) string {
	var out strings.Builder
	out.WriteString(webManifest(cfg))
	for _, worker := range cfg.Workers {
		out.WriteString(workerManifest(cfg, worker))
	}
	if len(cfg.Domains) > 0 {
		out.WriteString(ingressManifest(cfg))
	}
	return out.String()
}

func probe(cfg *config.Config, period int) string {
	return fmt.Sprintf("httpGet: {path: %q, port: %d}\n            periodSeconds: %d", cfg.Web.HealthPath, cfg.Web.HealthPort, period)
}

// graceSeconds is the stop grace plus 5 seconds for the preStop sleep.
func graceSeconds(cfg *config.Config) int {
	return int(cfg.StopGrace().Seconds()) + 5
}

func webManifest(cfg *config.Config) string {
	return fmt.Sprintf(`# Written by muken. Create the secret first:
#   kubectl create secret generic %[1]s-env --from-literal=KEY=value
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s-web
  labels: {app: %[1]s, role: web}
spec:
  replicas: %[2]d
  strategy:
    type: RollingUpdate
    rollingUpdate: {maxUnavailable: 0, maxSurge: 1}
  selector:
    matchLabels: {app: %[1]s, role: web}
  template:
    metadata:
      labels: {app: %[1]s, role: web}
    spec:
      terminationGracePeriodSeconds: %[3]d
      containers:
        - name: web
          image: %[4]s
          ports:
            - containerPort: %[5]d
          envFrom:
            - secretRef: {name: %[1]s-env, optional: true}
%[6]s          readinessProbe:
            %[7]s
          livenessProbe:
            %[8]s
            failureThreshold: 6
          lifecycle:
            preStop:
              exec: {command: ["sleep", "5"]}
---
apiVersion: v1
kind: Service
metadata:
  name: %[1]s-web
spec:
  selector: {app: %[1]s, role: web}
  ports:
    - port: 80
      targetPort: %[5]d
`, cfg.App, cfg.Web.Replicas, graceSeconds(cfg), cfg.Image, cfg.Web.Port, envBlock(cfg.Web.Env, "          "), probe(cfg, 5), probe(cfg, 15))
}

func workerManifest(cfg *config.Config, worker config.Worker) string {
	return fmt.Sprintf(`---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s-%[2]s
  labels: {app: %[1]s, role: %[2]s}
spec:
  replicas: %[3]d
  selector:
    matchLabels: {app: %[1]s, role: %[2]s}
  template:
    metadata:
      labels: {app: %[1]s, role: %[2]s}
    spec:
      terminationGracePeriodSeconds: %[4]d
      containers:
        - name: %[2]s
          image: %[5]s
          command: ["sh", "-c", %[6]q]
          envFrom:
            - secretRef: {name: %[1]s-env, optional: true}
`, cfg.App, worker.Name, worker.Replicas, graceSeconds(cfg), cfg.Image, worker.Command)
}

func ingressManifest(cfg *config.Config) string {
	var out strings.Builder
	fmt.Fprintf(&out, "---\napiVersion: networking.k8s.io/v1\nkind: Ingress\nmetadata:\n  name: %s\nspec:\n  rules:\n", cfg.App)
	for _, domain := range cfg.Domains {
		fmt.Fprintf(&out, "    - host: %s\n      http:\n        paths:\n          - path: /\n            pathType: Prefix\n            backend:\n              service: {name: %s-web, port: {number: 80}}\n", domain, cfg.App)
	}
	if cfg.TLS != "none" {
		fmt.Fprintf(&out, "  tls:\n    - secretName: %s-tls\n      hosts: [%s]\n", cfg.App, strings.Join(cfg.Domains, ", "))
	}
	return out.String()
}
