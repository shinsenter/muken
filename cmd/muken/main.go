// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Command muken deploys the shinsenter/php images to your own servers.
package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/secrets"
	"github.com/shinsenter/muken/internal/ui"
)

var version = "dev"

type command struct {
	run   func(ctx context.Context, p *ui.Printer, args []string) error
	short string
	group string
}

var commands = map[string]command{
	"init":        {cmdInit, "Create muken.yaml by answering a few questions", "Start"},
	"server":      {cmdServer, "Create or delete a server (DigitalOcean, Hetzner, Vultr, AWS), or an SSH key for it", "Start"},
	"up":          {cmdUp, "First time on a server: prepare it, set DNS, deploy", "Start"},
	"doctor":      {cmdDoctor, "Check config, SSH, Docker, DNS and disk", "Deploy"},
	"deploy":      {cmdDeploy, "Ship a new version with no downtime, roll back on failure", "Deploy"},
	"rollback":    {cmdRollback, "Return to the previous release", "Deploy"},
	"scale":       {cmdScale, "Change replicas, e.g. muken scale web=3 queue=2", "Deploy"},
	"maintenance": {cmdMaintenance, "Turn the maintenance page on or off", "Deploy"},
	"status":      {cmdStatus, "Show the running release and containers", "Operate"},
	"logs":        {cmdLogs, "Show logs, e.g. muken logs -f web", "Operate"},
	"exec":        {cmdExec, "Run a command in a container", "Operate"},
	"ssh":         {cmdSSH, "Open a shell on the server", "Operate"},
	"unlock":      {cmdUnlock, "Remove a stale deploy lock", "Operate"},
	"secrets":     {cmdSecrets, "Keep secrets encrypted in the repository", "Configure"},
	"dns":         {cmdDNS, "Sync Cloudflare DNS records or purge the cache", "Configure"},
	"agent":       {cmdAgent, "Install or open the monitoring sidecar", "Configure"},
	"render":      {cmdRender, "Print Kubernetes manifests or a GitHub Actions workflow", "Configure"},
	"version":     {cmdVersion, "Print the version, or the license with --license", "Other"},
}

var groupOrder = []string{"Start", "Deploy", "Operate", "Configure", "Other"}

func main() { os.Exit(run(os.Args[1:])) }

// run returns the exit code: 0 ok, 1 failed, 2 unknown command, 130 cancelled.
func run(args []string) int {
	p := ui.New()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(p)
		return 0
	}
	cmd, ok := commands[args[0]]
	if !ok {
		p.Fail(ui.Errf("run `muken help` to see the commands", "unknown command %q", args[0]))
		return 2
	}

	// A second Ctrl+C must kill the process if a step ignores the first one.
	context.AfterFunc(ctx, stop)
	err := cmd.run(ctx, p, args[1:])
	p.Finish(err == nil)
	switch {
	case err == nil, errors.Is(err, flag.ErrHelp):
		return 0
	case errors.Is(err, context.Canceled):
		p.Warn("cancelled")
		return 130
	}
	p.Fail(err)
	return 1
}

func usage(p *ui.Printer) {
	first := [][2]string{
		{"muken init", "Answer a few questions"},
		{"muken server create", "Optional: get a server at DigitalOcean, Hetzner, Vultr or AWS"},
		{"muken up", "Prepare the server and deploy"},
	}
	flags := [][2]string{
		{"-c <file>", "Config file"},
		{"--yes", "Answer yes to questions"},
		{"--accept-new", "Trust the SSH key of a server seen for the first time"},
		{"-v, --verbose", "Print every remote command and its output"},
	}
	groups := make([][][2]string, len(groupOrder))
	for i, group := range groupOrder {
		var names []string
		for name, cmd := range commands {
			if cmd.group == group {
				names = append(names, name)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			groups[i] = append(groups[i], [2]string{name, commands[name].short})
		}
	}

	// One width for every list, so all descriptions start in the same column.
	width := 0
	for _, list := range append([][][2]string{first, flags}, groups...) {
		for _, item := range list {
			width = max(width, len(item[0]))
		}
	}

	p.Info("muken %s: deploy the shinsenter/php images with no downtime", version)
	p.Section("First time")
	p.ListWidth(width, first)
	for i, group := range groupOrder {
		p.Section(group)
		p.ListWidth(width, groups[i])
	}
	p.Section("Common flags (put them before other arguments)")
	p.ListWidth(width, flags)
	p.Section("Docs")
	p.Info("  https://github.com/shinsenter/muken/blob/main/docs/guide.md")
}

type commonFlags struct {
	config    string
	yes       bool
	acceptNew bool
	verbose   bool
}

func newFlags(name string, global *commonFlags) *flag.FlagSet {
	flagSet := flag.NewFlagSet(name, flag.ContinueOnError)
	flagSet.SetOutput(os.Stderr)
	flagSet.Usage = func() { flagUsage(flagSet) }
	flagSet.StringVar(&global.config, "c", config.DefaultFile, "config file")
	flagSet.StringVar(&global.config, "config", config.DefaultFile, "config file")
	flagSet.BoolVar(&global.yes, "yes", false, "answer yes to questions")
	flagSet.BoolVar(&global.verbose, "v", false, "print every remote command and its output")
	flagSet.BoolVar(&global.verbose, "verbose", false, "print every remote command and its output")
	flagSet.BoolVar(&global.acceptNew, "accept-new", false, "trust the SSH key of a server seen for the first time")
	return flagSet
}

func flagUsage(flagSet *flag.FlagSet) {
	p := ui.New()
	p.Out = flagSet.Output()
	p.Info("Usage: muken %s [flags] [arguments]", flagSet.Name())
	var items [][2]string
	flagSet.VisitAll(func(opt *flag.Flag) {
		name := "-" + opt.Name
		if _, isBool := opt.Value.(interface{ IsBoolFlag() bool }); !isBool {
			name += " <value>"
		}
		desc := upperFirst(opt.Usage)
		if opt.DefValue != "" && opt.DefValue != "false" {
			desc += " (default " + opt.DefValue + ")"
		}
		items = append(items, [2]string{name, desc})
	})
	p.Section("Flags")
	p.List(items)
}

func parseFlags(flagSet *flag.FlagSet, global *commonFlags, p *ui.Printer, args []string) error {
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	p.Yes = global.yes
	p.Verbose = global.verbose || os.Getenv("MUKEN_VERBOSE") == "1"
	return nil
}

// secretsPath maps muken.yaml to muken.secrets.age and staging.yaml to staging.secrets.age.
func secretsPath(global commonFlags) string {
	base := strings.TrimSuffix(filepath.Base(global.config), filepath.Ext(global.config))
	dir := filepath.Dir(global.config)
	if base == "muken" {
		return filepath.Join(dir, secrets.File)
	}
	return filepath.Join(dir, base+".secrets.age")
}

func loadConfigAndSecrets(global commonFlags) (*config.Config, map[string]string, error) {
	cfg, err := config.Load(global.config)
	if err != nil {
		return nil, nil, err
	}
	secretValues, err := secrets.Load(secretsPath(global))
	if err != nil {
		return nil, nil, err
	}
	return cfg, secretValues, nil
}

func dialServer(ctx context.Context, p *ui.Printer, cfg *config.Config, global commonFlags) (remote.Runner, error) {
	p.Step("Connecting to %s@%s", cfg.Server.User, cfg.Server.Host)
	runner, err := remote.Dial(ctx, dialOptions(p, cfg, global.acceptNew))
	if err != nil {
		return nil, err
	}
	if !p.Verbose {
		return runner, nil
	}
	// Secrets travel inside uploaded files, so the commands are redacted before they are shown.
	return &remote.Logged{Runner: runner, Log: func(cmd, out string, err error, took time.Duration) {
		p.Command(remote.Redact(cmd), out, err, took)
	}}, nil
}

func dialOptions(p *ui.Printer, cfg *config.Config, acceptNew bool) remote.DialOptions {
	// --yes never trusts a new host key on its own.
	confirm := func(host, fingerprint string) bool {
		if !p.Interactive {
			return false
		}
		p.Info("First connection to %s, key %s", host, fingerprint)
		return strings.EqualFold(p.Ask("Trust this server key? (yes/no)", "no"), "yes")
	}
	return remote.DialOptions{
		Host:      cfg.Server.Host,
		Port:      cfg.Server.Port,
		User:      cfg.Server.User,
		KeyPath:   cfg.Server.SSHKey,
		AcceptNew: acceptNew,
		Confirm:   confirm,
	}
}

func cmdVersion(_ context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("version", &global)
	showLicense := flagSet.Bool("license", false, "show the license, the notices and the name policy")
	thirdParty := flagSet.Bool("third-party", false, "show the licenses of the software inside muken")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	if *showLicense || *thirdParty {
		printLicense(p, *thirdParty)
		return nil
	}
	p.Info("muken %s", version)
	p.Info("Copyright 2026 Mai Nhut Tan. Apache License 2.0. Run `muken version --license` for the terms.")
	return nil
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToUpper(r)) + s[size:]
}
