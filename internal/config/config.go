// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package config reads and checks muken.yaml.
package config

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/shinsenter/muken/internal/ui"
)

// DefaultFile is the config name looked up in the current folder.
const DefaultFile = "muken.yaml"

type Config struct {
	App         string      `yaml:"app"`
	Preset      string      `yaml:"preset,omitempty"`
	Image       string      `yaml:"image"`
	Domains     []string    `yaml:"domains,omitempty"`
	Email       string      `yaml:"email,omitempty"`
	TLS         string      `yaml:"tls,omitempty"` // letsencrypt, cloudflare or none
	Keep        int         `yaml:"keep,omitempty"`
	Server      Server      `yaml:"server"`
	Web         Web         `yaml:"web,omitempty"`
	Workers     []Worker    `yaml:"workers,omitempty"`
	Accessories []Accessory `yaml:"accessories,omitempty"`
	Cloudflare  Cloudflare  `yaml:"cloudflare,omitempty"`
	Agent       Agent       `yaml:"agent,omitempty"`
	Maintenance Maintenance `yaml:"maintenance,omitempty"`
}

type Server struct {
	Host         string `yaml:"host"`
	User         string `yaml:"user,omitempty"`
	Port         int    `yaml:"port,omitempty"`
	SSHKey       string `yaml:"ssh_key,omitempty"`
	TraefikImage string `yaml:"traefik_image,omitempty"`
}

type Web struct {
	Port          int               `yaml:"port,omitempty"`
	Replicas      int               `yaml:"replicas,omitempty"`
	HealthPath    string            `yaml:"health_path,omitempty"`
	HealthPort    int               `yaml:"health_port,omitempty"`
	HealthTimeout string            `yaml:"health_timeout,omitempty"`
	Drain         string            `yaml:"drain,omitempty"`
	StopGrace     string            `yaml:"stop_grace,omitempty"`
	SmokePath     string            `yaml:"smoke_path,omitempty"`
	Migrate       string            `yaml:"migrate,omitempty"`
	Memory        string            `yaml:"memory,omitempty"`
	CPUs          string            `yaml:"cpus,omitempty"`
	Env           map[string]string `yaml:"env,omitempty"`
	Volumes       []string          `yaml:"volumes,omitempty"`
}

type Worker struct {
	Name     string `yaml:"name"`
	Command  string `yaml:"command"`
	Replicas int    `yaml:"replicas,omitempty"`
	User     string `yaml:"user,omitempty"`
	Memory   string `yaml:"memory,omitempty"`
}

type Accessory struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"` // postgres, mysql, mariadb or redis
	Version string `yaml:"version,omitempty"`
}

type Cloudflare struct {
	Enabled       bool   `yaml:"enabled,omitempty"`
	Proxied       *bool  `yaml:"proxied,omitempty"`
	SSLMode       string `yaml:"ssl_mode,omitempty"`
	PurgeOnDeploy bool   `yaml:"purge_on_deploy,omitempty"`
}

type Maintenance struct {
	Page string `yaml:"page,omitempty"` // an HTML file, relative to muken.yaml
}

type Agent struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Port    int    `yaml:"port,omitempty"`
	Domain  string `yaml:"domain,omitempty"`
	Webhook string `yaml:"webhook,omitempty"`
}

var (
	nameRe    = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,30}$`)
	hostRe    = regexp.MustCompile(`^([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)+[a-z]{2,}$`)
	envRefRe  = regexp.MustCompile(`\$\{env:([A-Za-z_][A-Za-z0-9_]*)\}`)
	envNameRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	memRe     = regexp.MustCompile(`^[0-9]+[kmg]$`)       // 512m, 1g
	cpuRe     = regexp.MustCompile(`^[0-9]+(\.[0-9]+)?$`) // 0.5, 2
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ui.Errf("run `muken init` to create it", "%s not found", path)
		}
		return nil, err
	}
	return Parse(data)
}

// Parse decodes YAML text. ${env:NAME} is replaced by the environment value.
func Parse(data []byte) (*Config, error) {
	data, err := expandEnv(data)
	if err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var c Config
	if err := decoder.Decode(&c); err != nil {
		return nil, ui.Wrap(err, "check spelling and indentation (2 spaces)", "cannot read muken.yaml")
	}
	c.ApplyDefaults()
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return &c, nil
}

func expandEnv(data []byte) ([]byte, error) {
	var missing []string
	data = envRefRe.ReplaceAllFunc(data, func(ref []byte) []byte {
		name := string(envRefRe.FindSubmatch(ref)[1])
		value, ok := os.LookupEnv(name)
		if !ok {
			missing = append(missing, name)
		}
		return []byte(value)
	})
	if len(missing) > 0 {
		return nil, ui.Errf("export the variable before running muken", "environment variable %s is not set", strings.Join(missing, ", "))
	}
	return data, nil
}

func (c *Config) ApplyDefaults() {
	if c.TLS == "" {
		c.TLS = "letsencrypt"
	}
	if c.Keep == 0 {
		c.Keep = 2
	}
	if c.Agent.Port == 0 {
		c.Agent.Port = 9100
	}
	c.applyServerDefaults()
	c.applyWebDefaults()
	c.applyListDefaults()
	if c.Cloudflare.Enabled {
		c.applyCloudflareDefaults()
	}
}

func (c *Config) applyServerDefaults() {
	s := &c.Server
	if s.User == "" {
		s.User = "root"
	}
	if s.Port == 0 {
		s.Port = 22
	}
	if s.TraefikImage == "" {
		s.TraefikImage = "traefik:v3"
	}
}

func (c *Config) applyWebDefaults() {
	w := &c.Web
	if w.Port == 0 {
		w.Port = 80
	}
	if w.Replicas == 0 {
		w.Replicas = 1
	}
	if w.HealthPath == "" {
		w.HealthPath = "/readyz"
	}
	if w.HealthPort == 0 {
		w.HealthPort = 2020
	}
	if w.HealthTimeout == "" {
		w.HealthTimeout = "120s"
	}
	if w.Drain == "" {
		w.Drain = "10s"
	}
	if w.StopGrace == "" {
		w.StopGrace = "30s"
	}
}

func (c *Config) applyListDefaults() {
	for i := range c.Workers {
		w := &c.Workers[i]
		if w.Replicas == 0 {
			w.Replicas = 1
		}
		if w.User == "" {
			w.User = "www-data"
		}
	}
	for i := range c.Accessories {
		a := &c.Accessories[i]
		if a.Name == "" {
			a.Name = a.Type
		}
	}
}

func (c *Config) applyCloudflareDefaults() {
	if c.Cloudflare.Proxied == nil {
		proxied := true
		c.Cloudflare.Proxied = &proxied
	}
	if c.Cloudflare.SSLMode == "" {
		c.Cloudflare.SSLMode = "strict"
	}
}

// Validate returns one error that lists every problem, each with a fix.
func (c *Config) Validate() error {
	var problems []string
	problems = append(problems, c.checkBasics()...)
	problems = append(problems, c.checkTLS()...)
	problems = append(problems, c.checkWeb()...)
	problems = append(problems, c.checkWebFields()...)
	problems = append(problems, c.checkWorkersAndAccessories()...)
	problems = append(problems, c.checkCloudflareAndAgent()...)
	if len(problems) == 0 {
		return nil
	}
	return ui.Errf("edit muken.yaml, then run again", "muken.yaml has %d problem(s):\n  - %s", len(problems), strings.Join(problems, "\n  - "))
}

func (c *Config) checkBasics() []string {
	var problems []string
	if !nameRe.MatchString(c.App) {
		problems = append(problems, fmt.Sprintf("app: use 1-31 lowercase letters, digits or '-', starting with a letter or digit (got %q)", c.App))
	}
	if strings.TrimSpace(c.Image) == "" {
		problems = append(problems, "image: required, for example shinsenter/wordpress:latest")
	}
	if c.Server.Host == "" {
		problems = append(problems, "server.host: required (IP or hostname). Use `muken server create` to get one")
	}
	return problems
}

func (c *Config) checkTLS() []string {
	var problems []string
	if c.TLS != "letsencrypt" && c.TLS != "cloudflare" && c.TLS != "none" {
		problems = append(problems, fmt.Sprintf("tls: must be letsencrypt, cloudflare or none (got %q)", c.TLS))
	}
	for _, d := range c.Domains {
		if !hostRe.MatchString(d) {
			problems = append(problems, fmt.Sprintf("domains: %q is not a valid host name", d))
		}
	}
	if len(c.Domains) == 0 && c.TLS != "none" {
		problems = append(problems, "domains: add at least one, or set tls: none to serve on the server IP over HTTP")
	}
	if c.TLS == "letsencrypt" && c.Email == "" {
		problems = append(problems, "email: required for Let's Encrypt notices")
	}
	if c.TLS == "cloudflare" && !c.Cloudflare.Enabled {
		problems = append(problems, "tls: cloudflare needs cloudflare.enabled: true")
	}
	return problems
}

func (c *Config) checkWeb() []string {
	var problems []string
	if c.Web.Replicas < 1 || c.Web.Replicas > 20 {
		problems = append(problems, "web.replicas: use 1 to 20")
	}
	for _, port := range []int{c.Web.Port, c.Web.HealthPort, c.Server.Port, c.Agent.Port} {
		if port < 1 || port > 65535 {
			problems = append(problems, fmt.Sprintf("port %d is out of range", port))
		}
	}
	problems = append(problems, checkDuration("web.health_timeout", c.Web.HealthTimeout)...)
	problems = append(problems, checkDuration("web.drain", c.Web.Drain)...)
	problems = append(problems, checkDuration("web.stop_grace", c.Web.StopGrace)...)
	return problems
}

func checkDuration(field, value string) []string {
	if _, err := time.ParseDuration(value); err != nil {
		return []string{fmt.Sprintf("%s: %q is not a duration such as 30s or 2m", field, value)}
	}
	return nil
}

func (c *Config) checkWebFields() []string {
	var problems []string
	if !strings.HasPrefix(c.Web.HealthPath, "/") {
		problems = append(problems, "web.health_path: must start with /")
	}
	if c.Web.SmokePath != "" && !strings.HasPrefix(c.Web.SmokePath, "/") {
		problems = append(problems, "web.smoke_path: must start with /")
	}
	if c.Web.Memory != "" && !memRe.MatchString(c.Web.Memory) {
		problems = append(problems, "web.memory: use a value such as 512m or 1g")
	}
	if c.Web.CPUs != "" && !cpuRe.MatchString(c.Web.CPUs) {
		problems = append(problems, "web.cpus: use a number such as 0.5 or 2")
	}
	for k, v := range c.Web.Env {
		if !envNameRe.MatchString(k) || strings.ContainsAny(v, "\n\r") {
			problems = append(problems, fmt.Sprintf("web.env: %q is not a valid variable (names use letters, digits, '_'; values stay on one line)", k))
		}
	}
	return problems
}

func (c *Config) checkWorkersAndAccessories() []string {
	var problems []string
	// Workers and accessories share one name space with the web container.
	used := map[string]bool{"web": true}
	for _, w := range c.Workers {
		switch {
		case !nameRe.MatchString(w.Name):
			problems = append(problems, fmt.Sprintf("workers: name %q is not valid", w.Name))
		case used[w.Name]:
			problems = append(problems, fmt.Sprintf("workers: name %q is used twice (or is reserved: web)", w.Name))
		case strings.TrimSpace(w.Command) == "":
			problems = append(problems, fmt.Sprintf("workers.%s: command is required", w.Name))
		}
		used[w.Name] = true
		if w.Replicas < 1 || w.Replicas > 50 {
			problems = append(problems, fmt.Sprintf("workers.%s: replicas must be 1 to 50", w.Name))
		}
	}
	for _, a := range c.Accessories {
		switch a.Type {
		case "postgres", "mysql", "mariadb", "redis":
		default:
			problems = append(problems, fmt.Sprintf("accessories: type %q is not supported (postgres, mysql, mariadb, redis)", a.Type))
		}
		if !nameRe.MatchString(a.Name) || used[a.Name] {
			problems = append(problems, fmt.Sprintf("accessories: name %q is invalid or already used", a.Name))
		}
		used[a.Name] = true
	}
	return problems
}

func (c *Config) checkCloudflareAndAgent() []string {
	var problems []string
	if c.Cloudflare.Enabled {
		switch c.Cloudflare.SSLMode {
		case "off", "flexible", "full", "strict":
		default:
			problems = append(problems, "cloudflare.ssl_mode: use off, flexible, full or strict")
		}
	}
	if c.Agent.Domain != "" && !hostRe.MatchString(c.Agent.Domain) {
		problems = append(problems, fmt.Sprintf("agent.domain: %q is not a valid host name", c.Agent.Domain))
	}
	return problems
}

func (c *Config) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(c); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Drain returns web.drain as a duration. Validate has already checked it.
func (c *Config) Drain() time.Duration {
	duration, _ := time.ParseDuration(c.Web.Drain)
	return duration
}

func (c *Config) StopGrace() time.Duration {
	duration, _ := time.ParseDuration(c.Web.StopGrace)
	return duration
}

func (c *Config) HealthTimeout() time.Duration {
	duration, _ := time.ParseDuration(c.Web.HealthTimeout)
	return duration
}

func (c *Config) IsProxied() bool { return c.Cloudflare.Proxied != nil && *c.Cloudflare.Proxied }
