// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/agent"
	"github.com/shinsenter/muken/internal/ci"
	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/k8s"
	"github.com/shinsenter/muken/internal/provider"
	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/secrets"
	"github.com/shinsenter/muken/internal/ui"
)

func publicKey(path string) (string, error) {
	paths := []string{path}
	if path == "" {
		home, _ := os.UserHomeDir()
		paths = nil
		for _, name := range []string{"id_ed25519.pub", "id_ecdsa.pub", "id_rsa.pub"} {
			paths = append(paths, filepath.Join(home, ".ssh", name))
		}
	}
	for _, keyFile := range paths {
		if data, err := os.ReadFile(keyFile); err == nil {
			return strings.TrimSpace(string(data)), nil
		}
	}
	return "", ui.Errf("create one with `ssh-keygen -t ed25519`, or pass --ssh-pubkey <file>", "no SSH public key found")
}

func pickChoice(p *ui.Printer, label string, list []provider.Choice, given string) (string, error) {
	if given != "" {
		return given, nil
	}
	if len(list) == 0 {
		return "", ui.Errf("", "the provider returned no choices for %s", label)
	}
	if !p.Interactive {
		return "", ui.Errf("pass the value as a flag, or run in a terminal to pick from a list", "%s is required", label)
	}
	options := make([]string, len(list))
	for i, choice := range list {
		options[i] = choice.ID + "  " + choice.Label
	}
	return list[p.Choose(label+"?", options, 0)].ID, nil
}

// serverFlags are the `server create` flags. Empty means ask.
type serverFlags struct {
	region, size, name, pubKey string
}

func cmdServer(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	var flags serverFlags
	flagSet := newFlags("server", &global)
	providerName := flagSet.String("provider", "", "digitalocean or hetzner")
	flagSet.StringVar(&flags.region, "region", "", "region or location")
	flagSet.StringVar(&flags.size, "size", "", "plan")
	flagSet.StringVar(&flags.name, "name", "", "server name")
	flagSet.StringVar(&flags.pubKey, "ssh-pubkey", "", "public key file to install")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	subcommand := flagSet.Arg(0)
	if subcommand == "" {
		return ui.Errf("example: muken server create", "use create, delete, regions, sizes or key")
	}
	if subcommand == "key" {
		return serverKey(p, global, flagSet.Arg(1))
	}
	if *providerName == "" {
		*providerName = provider.Names[p.Choose("Which provider?", provider.Names, 0)]
	}
	cloud, err := provider.New(*providerName)
	if err != nil {
		return err
	}

	switch subcommand {
	case "regions", "sizes":
		return listChoices(ctx, p, cloud, subcommand)
	case "delete":
		return deleteServer(ctx, p, cloud, flagSet.Args())
	case "create":
		return createServer(ctx, p, global, cloud, flags)
	}
	return ui.Errf("", "unknown subcommand %q", subcommand)
}

func listChoices(ctx context.Context, p *ui.Printer, cloud provider.Provider, what string) error {
	var list []provider.Choice
	var err error
	if what == "regions" {
		list, err = cloud.Regions(ctx)
	} else {
		list, err = cloud.Sizes(ctx)
	}
	if err != nil {
		return err
	}
	items := make([][2]string, len(list))
	for i, choice := range list {
		items[i] = [2]string{choice.ID, choice.Label}
	}
	p.List(items)
	return nil
}

// deleteServer expects args to be ["delete", "<id>"].
func deleteServer(ctx context.Context, p *ui.Printer, cloud provider.Provider, args []string) error {
	if len(args) < 2 {
		return ui.Errf("example: muken server delete 12345678", "give the server id")
	}
	id := args[1]
	if !p.Confirm("Delete server "+id+" and all its data?", false) {
		return ui.Errf("", "cancelled")
	}
	if err := cloud.Delete(ctx, id); err != nil {
		return err
	}
	p.OK("Deleted")
	return nil
}

func createServer(ctx context.Context, p *ui.Printer, global commonFlags, cloud provider.Provider, flags serverFlags) error {
	key, err := publicKey(flags.pubKey)
	if err != nil {
		return err
	}
	regions, err := cloud.Regions(ctx)
	if err != nil {
		return err
	}
	region, err := pickChoice(p, "Region", regions, flags.region)
	if err != nil {
		return err
	}
	sizes, err := cloud.Sizes(ctx)
	if err != nil {
		return err
	}
	size, err := pickChoice(p, "Plan", sizes, flags.size)
	if err != nil {
		return err
	}

	// The config file may not exist yet. Then we only print the host.
	cfg, cfgErr := config.Load(global.config)
	name := flags.name
	if name == "" {
		defaultName := "muken"
		if cfgErr == nil {
			defaultName = cfg.App
		}
		name = p.Ask("Server name", defaultName)
	}
	if !p.Confirm(fmt.Sprintf("Create %s (%s, %s) at %s? This costs money until you delete it.", name, region, size, cloud.Name()), false) {
		return ui.Errf("", "cancelled")
	}

	p.Step("Creating the server (about a minute)")
	created, err := cloud.Create(ctx, provider.CreateOptions{Name: name, Region: region, Size: size, PublicKey: key})
	if err != nil {
		if created.ID != "" {
			p.Warn("a server with id %s may exist; remove it with: muken server delete --provider %s %s", created.ID, cloud.Name(), created.ID)
		}
		return err
	}
	p.OK("Server %s created at %s (id %s)", name, created.IP, created.ID)

	if cfgErr != nil {
		p.Info("\nPut this in muken.yaml:\n  server:\n    host: %s", created.IP)
		return nil
	}
	cfg.Server.Host = created.IP
	if err := writeConfig(global.config, "# muken config. Reference: https://github.com/shinsenter/muken/blob/main/docs/guide.md\n", cfg); err != nil {
		return err
	}
	p.OK("Saved server.host in %s", global.config)
	p.Step("Waiting for SSH")
	if err := waitSSH(ctx, p, cfg); err != nil {
		return err
	}
	p.Info("\nNext: muken up")
	return nil
}

// waitSSH retries for about three minutes while the new server boots.
func waitSSH(ctx context.Context, p *ui.Printer, cfg *config.Config) error {
	var last error
	for i := 0; i < 40; i++ {
		// We created this server a minute ago, so trust its first key.
		conn, err := remote.Dial(ctx, dialOptions(p, cfg, true))
		if err == nil {
			conn.Close()
			p.OK("SSH is ready")
			return nil
		}
		last = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
	return ui.Wrap(last, "try `muken doctor` in a minute", "SSH did not come up")
}

type doctor struct {
	p      *ui.Printer
	failed int
}

func (d *doctor) fail(err error) {
	d.failed++
	d.p.Fail(err)
}

func (d *doctor) result() error {
	if d.failed > 0 {
		return ui.Errf("fix the items above, then run `muken doctor` again", "%d check(s) failed", d.failed)
	}
	return nil
}

func cmdDoctor(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("doctor", &global)
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	cfg, err := config.Load(global.config)
	if err != nil {
		return err
	}
	p.Section("Config")
	p.Check("File", global.config+" is valid")
	p.Row("App", cfg.App)
	p.Row("Image", cfg.Image)
	p.Section("Local")

	d := &doctor{p: p}
	d.checkLocal(cfg, global)

	runner, err := dialServer(ctx, p, cfg, global)
	if err != nil {
		d.fail(err)
		return d.result()
	}
	defer runner.Close()
	p.Section("Server")
	p.Check("SSH login", "OK")
	d.checkServer(ctx, runner)
	d.checkDomains(cfg)
	return d.result()
}

func (d *doctor) checkLocal(cfg *config.Config, global commonFlags) {
	path := secretsPath(global)
	if _, err := secrets.Load(path); err != nil {
		d.fail(err)
	} else if _, err := os.Stat(path); err == nil {
		d.p.Check("Secrets file", "can be decrypted")
	}
	switch {
	case !cfg.Cloudflare.Enabled:
		d.p.Row("Cloudflare", "not used")
	case os.Getenv("CLOUDFLARE_API_TOKEN") == "":
		d.fail(ui.Errf("export CLOUDFLARE_API_TOKEN=...", "Cloudflare is enabled but CLOUDFLARE_API_TOKEN is not set"))
	default:
		d.p.Check("Cloudflare token", "set")
	}
}

func (d *doctor) checkServer(ctx context.Context, runner remote.Runner) {
	run := func(cmd string) string {
		out, _ := runner.Run(ctx, cmd)
		return strings.TrimSpace(out)
	}

	if dockerVersion := run("docker version --format '{{.Server.Version}}' 2>/dev/null"); dockerVersion != "" {
		d.p.Check("Docker", dockerVersion)
	} else {
		// A new server has no Docker yet. That is not a fault, so it is not counted as a failure.
		d.p.Warn("Docker is not installed. This server is not set up yet. Run `muken up` to set it up")
		return
	}

	switch {
	case run("docker inspect -f '{{.State.Running}}' muken-traefik 2>/dev/null") == "true":
		d.p.Check("Proxy", "RUNNING")
	case run("docker network inspect -f '{{.Name}}' muken 2>/dev/null") == "":
		// Docker is there but muken never set the server up, the same test that `up` uses.
		d.p.Warn("The proxy is not installed yet. Run `muken up` to set the server up")
	default:
		d.fail(ui.Errf("run `muken up` to set the server up again. It is safe to repeat", "the proxy is not running"))
	}

	free := run("df -Pm /opt 2>/dev/null | awk 'NR==2{print $4}'")
	if free == "" {
		return
	}
	var mb int
	fmt.Sscan(free, &mb)
	if mb < 2048 {
		d.fail(ui.Errf("free some space: docker system prune", "only %d MB free on the server", mb))
	} else {
		d.p.Check("Disk free", fmt.Sprintf("%d MB", mb))
	}
}

// checkDomains only warns, because DNS may not be ready yet.
func (d *doctor) checkDomains(cfg *config.Config) {
	ip, ipErr := serverIP(cfg)
	if len(cfg.Domains) > 0 {
		d.p.Section("Domains")
	}
	for _, domain := range cfg.Domains {
		addrs, err := net.LookupHost(domain)
		switch {
		case err != nil:
			advice := "Add an A record to " + ip + "."
			if cfg.Cloudflare.Enabled {
				advice = "Run `muken dns sync`."
			}
			d.p.Warn("%s does not resolve yet (Let's Encrypt needs DNS). %s", domain, advice)
		case cfg.Cloudflare.Enabled:
			d.p.Check(domain, "resolves to "+addrs[0])
		case ipErr == nil && !containsString(addrs, ip):
			d.p.Warn("%s points to %s, not to the server %s", domain, strings.Join(addrs, ", "), ip)
		default:
			d.p.Check(domain, "points to the server")
		}
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func cmdSecrets(_ context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("secrets", &global)
	reveal := flagSet.Bool("reveal", false, "print values in `list`")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	path := secretsPath(global)
	switch flagSet.Arg(0) {
	case "init":
		return secretsInit(p, path)
	case "set":
		return secretsSet(p, path, flagSet.Args())
	case "unset":
		return secretsUnset(path, flagSet.Arg(1))
	case "list":
		return secretsList(p, path, *reveal)
	}
	return ui.Errf("example: muken secrets init", "use init, set, unset or list")
}

func secretsInit(p *ui.Printer, secretsFile string) error {
	keyPath, err := secrets.Init(filepath.Dir(secretsFile), secretsFile)
	if err != nil {
		return err
	}
	p.OK("Created the key %s", keyPath)
	p.Info("Back it up. Without it nobody can read %s. Share it with teammates over a safe channel, and give CI the same text as MUKEN_AGE_KEY.", secrets.File)
	p.Warn("Add `.muken` to .dockerignore, so the key never goes into your app image. The folder has its own .gitignore.")
	return nil
}

// secretsSet takes KEY=VALUE, or KEY and reads the value from stdin.
// args is ["set", "<arg>"].
func secretsSet(p *ui.Printer, path string, args []string) error {
	if len(args) < 2 {
		return ui.Errf("example: muken secrets set APP_KEY=base64:...   or   printf %s \"$VALUE\" | muken secrets set APP_KEY", "give a KEY or KEY=VALUE")
	}
	key, value, hasValue := strings.Cut(args[1], "=")
	if !hasValue {
		input, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		if err != nil {
			return err
		}
		value = strings.TrimRight(string(input), "\r\n")
	}
	if key == "" || strings.ContainsAny(value, "\r\n") {
		return ui.Errf("keep the value on one line", "invalid key or value")
	}
	entries, err := secrets.Load(path)
	if err != nil {
		return err
	}
	entries[key] = value
	if err := secrets.Save(path, entries); err != nil {
		return err
	}
	p.OK("Saved %s in %s", key, path)
	return nil
}

func secretsUnset(path, key string) error {
	entries, err := secrets.Load(path)
	if err != nil {
		return err
	}
	delete(entries, key)
	return secrets.Save(path, entries)
}

func secretsList(p *ui.Printer, path string, reveal bool) error {
	entries, err := secrets.Load(path)
	if err != nil {
		return err
	}
	for key, value := range entries {
		if reveal {
			p.Info("%s=%s", key, value)
		} else {
			p.Info("%s", key)
		}
	}
	return nil
}

func cmdRender(_ context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("render", &global)
	branch := flagSet.String("branch", "main", "branch that triggers a deploy (for ci)")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	switch flagSet.Arg(0) {
	case "k8s":
		cfg, err := config.Load(global.config)
		if err != nil {
			return err
		}
		fmt.Fprint(p.Out, k8s.Render(cfg))
		return nil
	case "ci":
		fmt.Fprint(p.Out, ci.GitHubWorkflow(*branch, version))
		fmt.Fprintln(p.Err, "Add the repository secrets MUKEN_SSH_KEY and (if you use secrets) MUKEN_AGE_KEY.")
		return nil
	}
	return ui.Errf("example: muken render k8s > k8s.yaml", "use k8s or ci")
}

// agentBinaryPaths lists where to look for a linux muken binary, best first.
func agentBinaryPaths(arch string) []string {
	var paths []string
	if v := os.Getenv("MUKEN_AGENT_BIN"); v != "" {
		paths = append(paths, v)
	}
	name := "muken-linux-" + arch
	exe, exeErr := os.Executable()
	if exeErr == nil {
		dir := filepath.Dir(exe)
		paths = append(paths, filepath.Join(dir, name), filepath.Join(dir, "dist", name))
	}
	paths = append(paths, filepath.Join("dist", name), filepath.Join("muken", "dist", name))
	// This binary is itself the agent when we already run on the same linux arch.
	if exeErr == nil && runtime.GOOS == "linux" && runtime.GOARCH == arch {
		paths = append(paths, exe)
	}
	return paths
}

func agentBinary(arch string) (io.ReadCloser, string, error) {
	for _, path := range agentBinaryPaths(arch) {
		if file, err := os.Open(path); err == nil {
			return file, path, nil
		}
	}
	return nil, "", ui.Errf("build it with: make -C muken dist   (or set MUKEN_AGENT_BIN to a linux/"+arch+" muken binary)",
		"no linux/%s muken binary found to upload", arch)
}

// agentRunFlags only apply to `muken agent run`, which runs on the server.
type agentRunFlags struct {
	addr       string
	interval   time.Duration
	diskPath   string
	baseDir    string
	dockerSock string
}

func cmdAgent(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	var flags agentRunFlags
	flagSet := newFlags("agent", &global)
	flagSet.StringVar(&flags.addr, "addr", "127.0.0.1:9100", "listen address (run)")
	flagSet.DurationVar(&flags.interval, "interval", 15*time.Second, "poll interval (run)")
	flagSet.StringVar(&flags.diskPath, "disk-path", "/opt/muken", "path whose disk usage is watched (run)")
	flagSet.StringVar(&flags.baseDir, "base-dir", "/opt/muken", "muken folder (run)")
	flagSet.StringVar(&flags.dockerSock, "docker-sock", "/var/run/docker.sock", "Docker socket (run)")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	switch flagSet.Arg(0) {
	case "run":
		return agentRun(ctx, flags)
	case "install":
		return agentInstall(ctx, p, global)
	case "token":
		return agentToken(ctx, p, global)
	case "open":
		return agentOpen(ctx, p, global)
	}
	return ui.Errf("example: muken agent install", "use install, open, token or run")
}

func agentRun(ctx context.Context, flags agentRunFlags) error {
	host, _ := os.Hostname()
	monitor, err := agent.New(agent.Options{
		Addr:     flags.addr,
		Token:    os.Getenv("MUKEN_AGENT_TOKEN"),
		Interval: flags.interval,
		Collector: &agent.Collector{
			Docker:   agent.NewUnixDocker(flags.dockerSock),
			DiskPath: flags.diskPath,
			BaseDir:  flags.baseDir,
		},
		Webhook: &agent.Webhook{URL: os.Getenv("MUKEN_WEBHOOK"), Host: host},
		Logf:    func(format string, a ...any) { fmt.Fprintf(os.Stderr, format+"\n", a...) },
	})
	if err != nil {
		return err
	}
	return monitor.Run(ctx)
}

func agentInstall(ctx context.Context, p *ui.Printer, global commonFlags) error {
	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()
	arch, err := sess.engine.ServerArch(ctx)
	if err != nil {
		return err
	}
	bin, from, err := agentBinary(arch)
	if err != nil {
		return err
	}
	defer bin.Close()

	p.Info("Using %s", from)
	if err := sess.engine.InstallAgent(ctx, bin); err != nil {
		return err
	}
	p.OK("Agent installed. View it with: muken agent open")
	if sess.cfg.Agent.Domain != "" {
		p.Info("Public page: https://%s (log in with any user name, the token as password; see `muken agent token`)", sess.cfg.Agent.Domain)
	}
	return nil
}

func agentToken(ctx context.Context, p *ui.Printer, global commonFlags) error {
	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()
	token, err := sess.engine.AgentToken(ctx)
	if err != nil {
		return err
	}
	p.Info("%s", token)
	return nil
}

func agentOpen(ctx context.Context, p *ui.Printer, global commonFlags) error {
	cfg, err := config.Load(global.config)
	if err != nil {
		return err
	}
	port := cfg.Agent.Port
	p.Info("Open http://localhost:%d (log in with any user name; the password is the output of `muken agent token`). Press Ctrl-C to close the tunnel.", port)

	args := []string{"-N", "-L", fmt.Sprintf("%d:127.0.0.1:%d", port, port), "-p", fmt.Sprint(cfg.Server.Port)}
	if cfg.Server.SSHKey != "" {
		args = append(args, "-i", cfg.Server.SSHKey)
	}
	args = append(args, cfg.Server.User+"@"+cfg.Server.Host)
	return exec.CommandContext(ctx, "ssh", args...).Run()
}
