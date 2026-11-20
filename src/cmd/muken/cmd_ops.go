// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/cloudflare"
	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/deploy"
	"github.com/shinsenter/muken/internal/ui"
)

type session struct {
	cfg    *config.Config
	engine *deploy.Engine
	global commonFlags
	p      *ui.Printer
}

func (sess *session) Close() { _ = sess.engine.R.Close() }

func connectSession(ctx context.Context, p *ui.Printer, global commonFlags) (*session, error) {
	cfg, secretValues, err := loadConfigAndSecrets(global)
	if err != nil {
		return nil, err
	}
	runner, err := dialServer(ctx, p, cfg, global)
	if err != nil {
		return nil, err
	}
	return &session{cfg: cfg, engine: deploy.New(cfg, runner, p, secretValues), global: global, p: p}, nil
}

// openSessionFromArgs is for commands that take only the common flags.
func openSessionFromArgs(ctx context.Context, p *ui.Printer, name string, args []string) (*session, error) {
	var global commonFlags
	flagSet := newFlags(name, &global)
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return nil, err
	}
	return connectSession(ctx, p, global)
}

func cmdUnlock(ctx context.Context, p *ui.Printer, args []string) error {
	sess, err := openSessionFromArgs(ctx, p, "unlock", args)
	if err != nil {
		return err
	}
	defer sess.Close()
	if err := sess.engine.Unlock(ctx); err != nil {
		return err
	}
	p.OK("Lock removed")
	return nil
}

func printPlan(p *ui.Printer, lines []string) {
	p.Section("Plan")
	for _, line := range lines {
		p.Info("  - %s", line)
	}
}

func cmdDeploy(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("deploy", &global)
	image := flagSet.String("image", "", "deploy this image instead of the one in the config")
	skipPull := flagSet.Bool("skip-pull", false, "use the image already on the server")
	noMigrate := flagSet.Bool("no-migrate", false, "skip the migration command")
	plan := flagSet.Bool("plan", false, "show what would change and change nothing")
	keepMaintenance := flagSet.Bool("keep-maintenance", false, "if maintenance is on, keep the maintenance page after the deploy")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()
	opts := deploy.DeployOptions{Image: *image, SkipPull: *skipPull, SkipMigrate: *noMigrate, KeepMaintenance: *keepMaintenance}
	if *plan {
		lines, err := sess.engine.Plan(ctx, opts)
		if err != nil {
			return err
		}
		printPlan(p, lines)
		return nil
	}
	return doDeploy(ctx, sess, opts, true)
}

func deploySummary(cfg *config.Config, result *deploy.Result, took time.Duration) (string, [][2]string) {
	scheme := "https"
	if cfg.TLS == "none" {
		scheme = "http"
	}
	var urls []string
	for _, domain := range cfg.Domains {
		urls = append(urls, scheme+"://"+domain)
	}
	if len(urls) == 0 {
		urls = append(urls, "http://"+cfg.Server.Host)
	}
	details := [][2]string{
		{"Image", result.Image},
		{"Time", took.Round(time.Second).String()},
		{"Open", strings.Join(urls, ", ")},
	}
	next := "muken status, muken logs -f"
	if result.Previous != "" {
		next += ", muken rollback"
	}
	return "Release " + result.Release + " is live", append(details, [2]string{"Next", next})
}

// doDeploy shows the plan, deploys and clears the cache. askFirst is false when the user already agreed.
func doDeploy(ctx context.Context, sess *session, opts deploy.DeployOptions, askFirst bool) error {
	lines, err := sess.engine.Plan(ctx, opts)
	if err != nil {
		return err
	}
	printPlan(sess.p, lines)
	if askFirst && !sess.p.Confirm("Deploy now?", true) {
		return ui.Errf("", "cancelled")
	}

	start := time.Now()
	result, err := sess.engine.Deploy(ctx, opts)
	if err != nil {
		return err
	}
	title, details := deploySummary(sess.cfg, result, time.Since(start))
	sess.p.Summary(title, details)
	if result.MaintenanceOn {
		sess.p.Warn("Maintenance is still on. Run `muken maintenance off` to send visitors to the new release.")
	}

	if sess.cfg.Cloudflare.Enabled && sess.cfg.Cloudflare.PurgeOnDeploy {
		if err := purgeCache(ctx, sess.cfg); err != nil {
			sess.p.Warn("deployed, but the Cloudflare cache purge failed: %v", err)
		} else {
			sess.p.OK("Cloudflare cache cleared")
		}
	}
	return nil
}

func cmdRollback(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("rollback", &global)
	keepMaintenance := flagSet.Bool("keep-maintenance", false, "if maintenance is on, keep the maintenance page after the rollback")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()
	if !p.Confirm("Go back to the previous release?", true) {
		return ui.Errf("", "cancelled")
	}
	result, err := sess.engine.Rollback(ctx, *keepMaintenance)
	if err != nil {
		return err
	}
	p.Summary("Back on release "+result.Release, [][2]string{{"Image", result.Image}, {"Next", "muken status, muken logs -f"}})
	if result.MaintenanceOn {
		p.Warn("Maintenance is still on. Run `muken maintenance off` to send visitors to the old release.")
	}
	return nil
}

func cmdStatus(ctx context.Context, p *ui.Printer, args []string) error {
	sess, err := openSessionFromArgs(ctx, p, "status", args)
	if err != nil {
		return err
	}
	defer sess.Close()
	status, err := sess.engine.Status(ctx)
	if err != nil {
		return err
	}

	p.Section("Application")
	p.Row("App", sess.cfg.App)
	p.Row("Server", sess.cfg.Server.Host)
	if status.State.Release == "" {
		p.Row("Release", p.Paint(ui.Warning, "NONE YET"))
	} else {
		p.Row("Release", status.State.Release)
		p.Row("Image", status.State.Image)
		if line := rollbackLine(status); line != "" {
			p.Info("  %s", p.Paint(ui.Dim, line))
		}
	}
	if status.Proxy {
		p.Row("Proxy", p.Paint(ui.Accent, "RUNNING"))
	} else {
		p.Row("Proxy", p.Paint(ui.Danger, "NOT RUNNING"))
	}
	if status.Maintenance {
		p.Row("Maintenance", p.Paint(ui.Warning, "ON (visitors see the maintenance page)"))
	} else {
		p.Row("Maintenance", "off")
	}

	p.Section("Containers")
	rows := make([][]string, len(status.Containers))
	for i, container := range status.Containers {
		live := container.Release
		if container.Release == status.State.Release {
			live += " (live)"
		}
		rows[i] = []string{container.Name, container.Role, container.State, live}
	}
	p.Table([]string{"CONTAINER", "ROLE", "STATE", "RELEASE"}, rows)
	return nil
}

// rollbackLine names the redeploy command when the previous containers were pruned.
func rollbackLine(status *deploy.Status) string {
	previous := status.State.Previous
	if previous == "" {
		return ""
	}
	for _, container := range status.Containers {
		if container.Release == previous {
			return "Rollback to: " + previous
		}
	}
	if status.State.PreviousImage != "" {
		return "Rollback: release " + previous + " was pruned. Redeploy it with: muken deploy --image " + status.State.PreviousImage
	}
	return "Rollback: release " + previous + " was pruned"
}

// parseScale turns ["web=3", "queue=2"] into a map of name to replicas.
func parseScale(args []string) (map[string]int, error) {
	const hint = "example: muken scale web=3 queue=2"
	out := map[string]int{}
	for _, arg := range args {
		name, count, ok := strings.Cut(arg, "=")
		n, err := strconv.Atoi(count)
		if !ok || err != nil || n < 1 {
			return nil, ui.Errf(hint, "cannot read %q", arg)
		}
		out[name] = n
	}
	if len(out) == 0 {
		return nil, ui.Errf(hint, "say what to scale")
	}
	return out, nil
}

func cmdScale(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("scale", &global)
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	want, err := parseScale(flagSet.Args())
	if err != nil {
		return err
	}
	cfg, err := config.Load(global.config)
	if err != nil {
		return err
	}
	for name, n := range want {
		if !setReplicas(cfg, name, n) {
			return ui.Errf("use web or a worker name from muken.yaml", "nothing called %q to scale", name)
		}
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := writeConfig(global.config, "# muken config. Reference: https://github.com/shinsenter/muken/blob/main/docs/guide.md\n", cfg); err != nil {
		return err
	}
	p.OK("Saved the new replica counts to %s (comments in the file are not kept)", global.config)

	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()
	return doDeploy(ctx, sess, deploy.DeployOptions{SkipPull: true, SkipMigrate: true}, false)
}

func setReplicas(cfg *config.Config, name string, n int) bool {
	found := false
	if name == "web" {
		cfg.Web.Replicas = n
		found = true
	}
	for i := range cfg.Workers {
		if cfg.Workers[i].Name == name {
			cfg.Workers[i].Replicas = n
			found = true
		}
	}
	return found
}

// maxMaintenancePage keeps the upload under the size limit of one shell argument.
const maxMaintenancePage = 64 << 10

// maintenancePage reads maintenance.page, relative to the config file. Nil means the built-in page.
func maintenancePage(cfg *config.Config, configFile string) ([]byte, error) {
	path := cfg.Maintenance.Page
	if path == "" {
		return nil, nil
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(configFile), path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, ui.Wrap(err, "fix maintenance.page in muken.yaml", "cannot read the maintenance page %s", path)
	}
	if len(data) == 0 || len(data) > maxMaintenancePage {
		return nil, ui.Errf("use one HTML file of 1 byte to 64 KB, with the CSS and images inline", "the maintenance page %s is %d bytes", path, len(data))
	}
	return data, nil
}

func cmdMaintenance(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("maintenance", &global)
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	mode := flagSet.Arg(0)
	if flagSet.NArg() != 1 || (mode != "on" && mode != "off") {
		for _, arg := range flagSet.Args() {
			if strings.HasPrefix(arg, "-") {
				return ui.Errf("put flags before the word: muken maintenance "+arg+" on", "the flag %s must come before on or off", arg)
			}
		}
		return ui.Errf("example: muken maintenance on", "say on or off")
	}
	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()

	on := mode == "on"
	var page []byte
	if on {
		if page, err = maintenancePage(sess.cfg, global.config); err != nil {
			return err
		}
	}
	if err := sess.engine.Maintenance(ctx, on, page); err != nil {
		return err
	}
	if on {
		p.OK("Visitors now see the maintenance page (HTTP 503)")
	} else {
		p.OK("The app is back")
	}
	return nil
}

func sshArgs(cfg *config.Config, tty bool) []string {
	args := []string{"ssh"}
	if tty {
		args = append(args, "-t")
	}
	args = append(args, "-p", strconv.Itoa(cfg.Server.Port))
	if cfg.Server.SSHKey != "" {
		args = append(args, "-i", cfg.Server.SSHKey)
	}
	return append(args, cfg.Server.User+"@"+cfg.Server.Host)
}

func runSSH(args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

func sudoIfNeeded(cfg *config.Config, cmd string) string {
	if cfg.Server.User != "root" {
		return "sudo -n " + cmd
	}
	return cmd
}

func liveContainer(ctx context.Context, sess *session, role string) (string, error) {
	status, err := sess.engine.Status(ctx)
	if err != nil {
		return "", err
	}
	for _, container := range status.Containers {
		if container.Release == status.State.Release && container.Role == role && container.State == "running" {
			return container.Name, nil
		}
	}
	return "", ui.Errf("run `muken status` to see what runs", "no running %q container in the live release", role)
}

func cmdLogs(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("logs", &global)
	follow := flagSet.Bool("f", false, "follow")
	lines := flagSet.Int("n", 100, "lines")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	role := "web"
	if flagSet.NArg() > 0 {
		role = flagSet.Arg(0)
	}
	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()
	name, err := liveContainer(ctx, sess, role)
	if err != nil {
		return err
	}

	followFlag := ""
	if *follow {
		followFlag = "-f "
	}
	cmd := fmt.Sprintf("docker logs --tail %d %s%s 2>&1", *lines, followFlag, name)
	if *follow {
		return runSSH(append(sshArgs(sess.cfg, false), sudoIfNeeded(sess.cfg, cmd)))
	}
	out, err := sess.engine.R.Run(ctx, cmd)
	fmt.Fprint(p.Out, out)
	return err
}

func cmdExec(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("exec", &global)
	role := flagSet.String("role", "web", "container role")
	user := flagSet.String("user", "", "run as this user (default: the image user)")
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	sess, err := connectSession(ctx, p, global)
	if err != nil {
		return err
	}
	defer sess.Close()
	name, err := liveContainer(ctx, sess, *role)
	if err != nil {
		return err
	}

	cmd := execCommand(name, *user, flagSet.Args(), p.Interactive)
	return runSSH(append(sshArgs(sess.cfg, p.Interactive), sudoIfNeeded(sess.cfg, shellJoin(cmd))))
}

// execCommand asks for a terminal (-t) only when muken has one, so scripts and CI work.
func execCommand(container, user string, args []string, tty bool) []string {
	ttyFlag := "-i"
	if tty {
		ttyFlag = "-it"
	}
	cmd := []string{"docker", "exec", ttyFlag}
	if user != "" {
		cmd = append(cmd, "-u", user)
	}
	cmd = append(cmd, container)
	if len(args) == 0 {
		return append(cmd, "sh")
	}
	return append(cmd, args...)
}

func shellJoin(words []string) string {
	quoted := make([]string, len(words))
	for i, w := range words {
		quoted[i] = "'" + strings.ReplaceAll(w, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}

func cmdSSH(_ context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("ssh", &global)
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	cfg, err := config.Load(global.config)
	if err != nil {
		return err
	}
	return runSSH(sshArgs(cfg, true))
}

func cloudflareClient() (*cloudflare.Client, error) {
	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	if token == "" {
		return nil, ui.Errf("create a token with Zone:Read, DNS:Edit, Zone Settings:Edit, Cache Purge and run: export CLOUDFLARE_API_TOKEN=...", "CLOUDFLARE_API_TOKEN is not set")
	}
	return cloudflare.New(token), nil
}

func serverIP(cfg *config.Config) (string, error) {
	host := cfg.Server.Host
	if net.ParseIP(host) != nil {
		return host, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return "", ui.Wrap(err, "use the server IP in server.host", "cannot resolve %s", host)
	}
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4.String(), nil
		}
	}
	return "", ui.Errf("use an IPv4 address in server.host", "%s has no IPv4 address", host)
}

func hostsOf(cfg *config.Config) []string {
	hosts := append([]string(nil), cfg.Domains...)
	if cfg.Agent.Domain != "" {
		hosts = append(hosts, cfg.Agent.Domain)
	}
	return hosts
}

func syncDNS(ctx context.Context, p *ui.Printer, cfg *config.Config) error {
	client, err := cloudflareClient()
	if err != nil {
		return err
	}
	ip, err := serverIP(cfg)
	if err != nil {
		return err
	}
	sslDone := map[string]bool{}
	for _, host := range hostsOf(cfg) {
		zone, err := client.ZoneID(ctx, host)
		if err != nil {
			return err
		}
		action, err := client.UpsertA(ctx, zone, host, ip, cfg.IsProxied())
		if err != nil {
			return err
		}
		p.OK("%s -> %s (%s)", host, ip, action)

		// Set the SSL mode once per zone.
		if sslDone[zone] {
			continue
		}
		sslDone[zone] = true
		if err := client.SetSSLMode(ctx, zone, cfg.Cloudflare.SSLMode); err != nil {
			return err
		}
		p.OK("SSL mode of the zone: %s", cfg.Cloudflare.SSLMode)
	}
	return nil
}

func purgeCache(ctx context.Context, cfg *config.Config) error {
	client, err := cloudflareClient()
	if err != nil {
		return err
	}
	hostsByZone := map[string][]string{}
	for _, host := range hostsOf(cfg) {
		zone, err := client.ZoneID(ctx, host)
		if err != nil {
			return err
		}
		hostsByZone[zone] = append(hostsByZone[zone], host)
	}
	for zone, hosts := range hostsByZone {
		if err := client.PurgeHosts(ctx, zone, hosts); err != nil {
			return err
		}
	}
	return nil
}

func cmdDNS(ctx context.Context, p *ui.Printer, args []string) error {
	var global commonFlags
	flagSet := newFlags("dns", &global)
	if err := parseFlags(flagSet, &global, p, args); err != nil {
		return err
	}
	cfg, err := config.Load(global.config)
	if err != nil {
		return err
	}
	if !cfg.Cloudflare.Enabled {
		return ui.Errf("set cloudflare.enabled: true in muken.yaml", "Cloudflare is not enabled")
	}
	switch flagSet.Arg(0) {
	case "sync":
		return syncDNS(ctx, p, cfg)
	case "purge":
		if err := purgeCache(ctx, cfg); err != nil {
			return err
		}
		p.OK("Cache cleared for %s", strings.Join(hostsOf(cfg), ", "))
		return nil
	}
	return ui.Errf("example: muken dns sync", "use `sync` or `purge`")
}

func cmdUp(ctx context.Context, p *ui.Printer, args []string) error {
	sess, err := openSessionFromArgs(ctx, p, "up", args)
	if err != nil {
		return err
	}
	defer sess.Close()

	p.Section("1/3 Server")
	skipped, err := sess.engine.Prepare(ctx)
	if err != nil {
		return err
	}
	if skipped {
		p.Info("  Docker is already set up. Skipped the base steps")
	}

	p.Section("2/3 DNS")
	if sess.cfg.Cloudflare.Enabled {
		if err := syncDNS(ctx, p, sess.cfg); err != nil {
			return err
		}
	} else {
		ip, _ := serverIP(sess.cfg)
		for _, domain := range sess.cfg.Domains {
			p.Info("  Point %s to %s (A record) before the first HTTPS request", domain, ip)
		}
	}

	p.Section("3/3 Deploy")
	return doDeploy(ctx, sess, deploy.DeployOptions{}, false)
}
