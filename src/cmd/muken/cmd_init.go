// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shinsenter/muken/internal/catalog"
	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/ui"
)

func listPresets(p *ui.Printer) {
	presets := catalog.All()
	p.Section("PHP images (any PHP app)")
	printPresets(p, presets, "server")
	p.Section("Apps and frameworks")
	printPresets(p, presets, "app")
	p.Section("Use one with")
	p.Info("  muken init --preset <name>")
}

func printPresets(p *ui.Printer, presets []catalog.Preset, kind string) {
	var items [][2]string
	for _, preset := range presets {
		if preset.Kind == kind {
			items = append(items, [2]string{preset.Name, preset.Title})
		}
	}
	p.List(items)
}

var nonNameChars = regexp.MustCompile(`[^a-z0-9-]+`)

func suggestName(dir string) string {
	name := nonNameChars.ReplaceAllString(strings.ToLower(filepath.Base(dir)), "-")
	name = strings.Trim(name, "-")
	if name == "" || name == "muken" {
		return "app"
	}
	if len(name) > 31 {
		name = name[:31]
	}
	return name
}

// splitList splits on commas and spaces: "a, b" gives [a b].
func splitList(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == ' ' })
}

// initFlags are the command line answers. Empty means ask the user.
type initFlags struct {
	preset, app, domains, email, host string
	php, image, dbs, workers          string
	sshKey                            string
	cloudflare, agent, force, list    bool
}

func cmdInit(_ context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	var flags initFlags
	flagSet := newFlags("init", &global)
	flagSet.StringVar(&flags.preset, "preset", "", "image or app (see `muken init --list`)")
	flagSet.StringVar(&flags.app, "app", "", "app name")
	flagSet.StringVar(&flags.domains, "domain", "", "domain names, comma separated")
	flagSet.StringVar(&flags.email, "email", "", "email for Let's Encrypt")
	flagSet.StringVar(&flags.host, "host", "", "server IP or host name")
	flagSet.StringVar(&flags.php, "php", "", "PHP version for the plain PHP images")
	flagSet.StringVar(&flags.image, "image", "", "your own image")
	flagSet.StringVar(&flags.dbs, "db", "", "databases: postgres, mysql, mariadb, redis")
	flagSet.StringVar(&flags.workers, "workers", "", "background workers from the preset, e.g. queue,scheduler")
	flagSet.StringVar(&flags.sshKey, "ssh-key", "", "private key file for the server (see `muken server key`)")
	flagSet.BoolVar(&flags.cloudflare, "cloudflare", false, "use Cloudflare for DNS and TLS")
	flagSet.BoolVar(&flags.agent, "agent", false, "enable the monitoring sidecar")
	flagSet.BoolVar(&flags.force, "force", false, "overwrite an existing file")
	flagSet.BoolVar(&flags.list, "list", false, "list the images and apps muken knows, then stop")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	if flags.list {
		listPresets(p)
		return nil
	}
	if _, err := os.Stat(global.config); err == nil && !flags.force {
		return ui.Errf("use --force to overwrite, or edit the file", "%s already exists", global.config)
	}
	cwd, _ := os.Getwd()

	preset, err := choosePreset(p, flags.preset, cwd)
	if err != nil {
		return err
	}
	cfg := &config.Config{Preset: preset.Name, App: flags.app}
	if cfg.App == "" {
		cfg.App = p.Ask("App name (letters, digits, '-')", suggestName(cwd))
	}
	cfg.Image = askImage(p, preset, flags)
	askAccess(p, cfg, flags)
	askServices(p, cfg, preset, flags)
	if err := addWorkers(p, cfg, preset, flags.workers); err != nil {
		return err
	}
	cfg.Server.SSHKey = flags.sshKey
	cfg.Agent.Enabled = flags.agent
	if !flags.agent && p.Interactive {
		cfg.Agent.Enabled = p.Confirm("Install the monitoring page and alerts (muken agent)?", false)
	}

	header := "# muken config. Reference: https://github.com/shinsenter/muken/blob/main/docs/guide.md\n# Secrets do not belong here: use `muken secrets set KEY`.\n"
	if err := writeConfig(global.config, header, cfg); err != nil {
		return err
	}
	p.OK("Wrote %s", global.config)
	warnWithoutCode(p, cfg, flags.image, cwd)
	printNextSteps(p, cfg)
	return nil
}

// warnWithoutCode explains why a stock image with no code fails a first deploy:
// the migration and the workers run before the image creates a project.
func warnWithoutCode(p *ui.Printer, cfg *config.Config, ownImage, dir string) {
	if ownImage != "" || (cfg.Web.Migrate == "" && len(cfg.Workers) == 0) || catalog.Detect(dir) != "" {
		return
	}
	p.Warn("No application code in this folder, and the image %s has none. The migration and the workers run before the image creates a project, so the first deploy would stop at a missing file such as artisan.", cfg.Image)
	p.Info("  Build your own image from it and set `image:`, or delete `web.migrate` and `workers` in %s for a first look.", "muken.yaml")
}

func choosePreset(p *ui.Printer, name, dir string) (catalog.Preset, error) {
	if name == "" {
		all := catalog.All()
		defaultIndex := 0
		if detected := catalog.Detect(dir); detected != "" {
			for i, candidate := range all {
				if candidate.Name == detected {
					defaultIndex = i
				}
			}
			p.Info("Found a %s project in this folder.", detected)
		}
		options := make([]string, len(all))
		for i, candidate := range all {
			options[i] = fmt.Sprintf("%-16s %s", candidate.Name, candidate.Title)
		}
		name = all[p.Choose("What do you want to run?", options, defaultIndex)].Name
	}
	preset, ok := catalog.Get(name)
	if !ok {
		return preset, ui.Errf("run `muken init --list` to see the list", "unknown preset %q", name)
	}
	return preset, nil
}

func askImage(p *ui.Printer, preset catalog.Preset, flags initFlags) string {
	if flags.image != "" {
		return flags.image
	}
	if preset.Kind != "server" {
		return p.Ask("Docker image (your own image built FROM shinsenter/"+preset.Name+", or the stock one)", preset.ImageFor(""))
	}
	version := flags.php
	if version == "" {
		version = askPHPVersion(p)
	}
	image := preset.ImageFor(version)
	p.Info("Your app code must be in the image or a volume. Set `image:` to your own image built FROM %s.", image)
	return image
}

func askPHPVersion(p *ui.Printer) string {
	options := make([]string, len(catalog.PHPVersions))
	defaultIndex := 0
	for i, phpVersion := range catalog.PHPVersions {
		options[i] = phpVersion
		if phpVersion == catalog.PHPReleaseCandidate {
			options[i] += "  release candidate, for testing"
		}
		if phpVersion == catalog.DefaultPHP {
			defaultIndex = i
		}
	}
	return catalog.PHPVersions[p.Choose("PHP version?", options, defaultIndex)]
}

func askAccess(p *ui.Printer, cfg *config.Config, flags initFlags) {
	domains := flags.domains
	if domains == "" {
		domains = p.Ask("Domain name(s), comma separated (empty = server IP over HTTP)", "")
	}
	cfg.Domains = splitList(domains)
	cfg.TLS = "letsencrypt"
	if len(cfg.Domains) == 0 {
		cfg.TLS = "none"
	}

	useCloudflare := flags.cloudflare
	if !useCloudflare && len(cfg.Domains) > 0 && p.Interactive {
		useCloudflare = p.Confirm("Is the domain on Cloudflare (muken can set DNS and certificates)?", false)
	}
	if useCloudflare {
		cfg.Cloudflare = config.Cloudflare{Enabled: true, PurgeOnDeploy: true}
		cfg.TLS = "cloudflare"
	}

	if cfg.TLS != "none" {
		cfg.Email = flags.email
		if cfg.Email == "" {
			cfg.Email = p.Ask("Email for certificate notices", "")
		}
	}
	cfg.Server.Host = flags.host
	if cfg.Server.Host == "" {
		cfg.Server.Host = p.Ask("Server IP (empty = create one later with `muken server create`)", "")
	}
}

func askServices(p *ui.Printer, cfg *config.Config, preset catalog.Preset, flags initFlags) {
	cfg.Web.Env, cfg.Web.Migrate = preset.Env, preset.Migrate
	if preset.HealthPath != "/readyz" {
		cfg.Web.HealthPath = preset.HealthPath
	}
	if preset.Stateful {
		cfg.Web.Volumes = []string{cfg.App + "-data:/var/www/html"}
	}

	picked := splitList(flags.dbs)
	if flags.dbs == "" && p.Interactive {
		databases := []string{"postgres", "mysql", "mariadb"}
		options := append([]string{"No database"}, databases...)
		if i := p.Choose("Which database on the same server?", options, 0); i > 0 {
			picked = append(picked, databases[i-1])
		}
		if p.Confirm("Add Redis for cache and queues?", false) {
			picked = append(picked, "redis")
		}
	}
	for _, kind := range picked {
		cfg.Accessories = append(cfg.Accessories, config.Accessory{Name: kind, Type: kind})
	}
}

func addWorkers(p *ui.Printer, cfg *config.Config, preset catalog.Preset, flagValue string) error {
	names := splitList(flagValue)
	if len(preset.Workers) > 0 && flagValue == "" {
		options := make([]string, len(preset.Workers))
		for i, worker := range preset.Workers {
			options[i] = worker.Name + "  " + worker.Title
		}
		for _, i := range p.Multi("Run background workers?", options) {
			names = append(names, preset.Workers[i].Name)
		}
	}
	for _, name := range names {
		found := false
		for _, worker := range preset.Workers {
			if worker.Name == name {
				cfg.Workers = append(cfg.Workers, config.Worker{Name: worker.Name, Command: worker.Command})
				found = true
			}
		}
		if !found {
			return ui.Errf("this preset has: "+workerNames(preset), "no worker template %q", name)
		}
	}
	return nil
}

func workerNames(preset catalog.Preset) string {
	if len(preset.Workers) == 0 {
		return "none"
	}
	names := make([]string, len(preset.Workers))
	for i, worker := range preset.Workers {
		names[i] = worker.Name
	}
	return strings.Join(names, ", ")
}

func printNextSteps(p *ui.Printer, cfg *config.Config) {
	p.Info("\nNext:")
	n := 1
	if cfg.Server.Host == "" {
		p.Info("  %d. muken server create     get a server and fill in server.host", n)
		n++
	}
	if cfg.Cloudflare.Enabled {
		p.Info("  %d. export CLOUDFLARE_API_TOKEN=...   (Zone:DNS:Edit, Zone Settings:Edit, Cache Purge)", n)
		n++
	}
	p.Info("  %d. muken doctor            check everything before you deploy", n)
	p.Info("  %d. muken up                prepare the server and deploy", n+1)
}

func writeConfig(path, header string, cfg *config.Config) error {
	data, err := cfg.Marshal()
	if err != nil {
		return err
	}
	return os.WriteFile(path, append([]byte(header), data...), 0o644)
}
