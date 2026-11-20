// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/ui"
)

// releaseLayout makes release names that sort by time.
const releaseLayout = "20060102T150405"

type DeployOptions struct {
	Image       string
	SkipPull    bool
	SkipMigrate bool
	// KeepMaintenance leaves visitors on the maintenance page. It does nothing when maintenance is off.
	KeepMaintenance bool
}

type Result struct {
	Release       string
	Image         string
	Previous      string
	MaintenanceOn bool
}

type Status struct {
	State       State
	Containers  []Container
	Proxy       bool
	Maintenance bool
}

// Deploy moves traffic to the new release after the health check.
// A failure before the switch leaves the old release untouched.
func (e *Engine) Deploy(ctx context.Context, opts DeployOptions) (*Result, error) {
	image := e.imageFor(opts)
	if err := e.lock(ctx); err != nil {
		return nil, err
	}
	defer e.unlock()

	e.UI.Step("Checking the server")
	if err := e.preflight(ctx); err != nil {
		return nil, err
	}
	state, err := e.readState(ctx)
	if err != nil {
		return nil, err
	}
	release := e.Now().UTC().Format(releaseLayout)
	hold := e.maintenanceActive(ctx) && opts.KeepMaintenance

	if err := e.prepare(ctx, image, release, opts); err != nil {
		return nil, err
	}
	created, err := e.startAndSwitch(ctx, image, release, hold)
	if err != nil {
		e.removeContainers(created)
		e.history(context.Background(), release, image, "failed")
		return nil, err
	}
	if !hold {
		e.endMaintenance(ctx)
	}

	all, _ := e.containers(ctx)
	e.stopOld(ctx, all, state.Release)
	e.prune(ctx, all, release)

	next := State{Release: release, Image: image, Previous: state.Release, PreviousImage: state.Image}
	if err := e.writeState(ctx, next); err != nil {
		return nil, err
	}
	e.history(ctx, release, image, "ok")
	return &Result{Release: release, Image: image, Previous: state.Release, MaintenanceOn: hold}, nil
}

func (e *Engine) imageFor(opts DeployOptions) string {
	if opts.Image != "" {
		return opts.Image
	}
	return e.Cfg.Image
}

// prepare does what the running release does not depend on: accessories, env file, pull, migration.
func (e *Engine) prepare(ctx context.Context, image, release string, opts DeployOptions) error {
	accessoryVars, err := e.ensureAccessories(ctx)
	if err != nil {
		return err
	}
	env, err := e.buildEnv(accessoryVars, release)
	if err != nil {
		return err
	}
	if err := remote.Upload(ctx, e.R, e.appDir()+"/env", []byte(renderEnv(env)), "600"); err != nil {
		return err
	}
	if !opts.SkipPull {
		e.UI.Step("Pulling %s", image)
		if _, err := e.R.Run(ctx, "docker pull "+quote(image)); err != nil {
			return ui.Wrap(err, "check the image name and `docker login` on the server for private images", "cannot pull %s", image)
		}
	}
	if opts.SkipMigrate {
		return nil
	}
	return e.migrate(ctx, image)
}

func (e *Engine) migrate(ctx context.Context, image string) error {
	script := strings.TrimSpace(e.Cfg.Web.Migrate)
	if script == "" {
		return nil
	}
	e.UI.Step("Running migrations")
	args := []string{"docker", "run", "--rm", "--network", "muken", "--env-file", e.appDir() + "/env", "--user", "www-data"}
	args = append(args, e.mounts()...)
	args = append(args, "--entrypoint", "sh", image, "-c", script)
	out, err := e.R.Run(ctx, cmdline(args...))
	if err != nil {
		return ui.Wrap(err, "nothing changed on the server. Fix the migration and deploy again", "migration failed:\n%s", strings.TrimSpace(out+"\n"+err.Error()))
	}
	return nil
}

// startAndSwitch returns every container it created, even on error, so the caller can remove them.
// With hold set, traffic stays on the maintenance page and the smoke check is skipped.
func (e *Engine) startAndSwitch(ctx context.Context, image, release string, hold bool) ([]string, error) {
	e.UI.Step("Starting release %s", release)
	web, err := e.startWeb(ctx, image, release)
	created := web
	if err != nil {
		return created, err
	}
	e.UI.Step("Waiting for health checks (up to %s)", e.Cfg.Web.HealthTimeout)
	if err := e.waitHealthy(ctx, web); err != nil {
		return created, err
	}
	workers, err := e.startWorkers(ctx, image, release)
	created = append(created, workers...)
	if err != nil {
		return created, err
	}
	if err := e.checkRunning(ctx, workers); err != nil {
		return created, err
	}
	if hold {
		e.UI.Step("Keeping the maintenance page. `muken maintenance off` sends traffic to the new release")
		return created, nil
	}
	e.UI.Step("Switching traffic")
	return created, e.switchRoute(ctx, web)
}

func (e *Engine) stopOld(ctx context.Context, all []Container, release string) {
	if release == "" {
		return
	}
	old := containersOfRelease(all, release)
	if len(old) == 0 {
		return
	}
	e.UI.Step("Draining the old release for %s", e.Cfg.Web.Drain)
	e.Sleep(e.Cfg.Drain())
	if err := e.stopContainers(ctx, containerNames(old)); err != nil {
		e.UI.Warn("could not stop the old release cleanly: %v", err)
	}
}

// prune removes the containers of releases older than the newest cfg.Keep.
func (e *Engine) prune(ctx context.Context, all []Container, current string) {
	releases := []string{current}
	for _, container := range all {
		if container.Release != "" && !hasString(releases, container.Release) {
			releases = append(releases, container.Release)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(releases)))

	keep := map[string]bool{}
	for i, release := range releases {
		if i < e.Cfg.Keep {
			keep[release] = true
		}
	}
	var drop []string
	for _, container := range all {
		if !keep[container.Release] {
			drop = append(drop, container.Name)
		}
	}
	e.removeContainers(drop)
}

func hasString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

func containersOfRelease(all []Container, release string) []Container {
	var out []Container
	for _, container := range all {
		if container.Release == release {
			out = append(out, container)
		}
	}
	return out
}

func webContainerNames(list []Container) []string {
	var out []string
	for _, container := range list {
		if container.Role == "web" {
			out = append(out, container.Name)
		}
	}
	return out
}

func (e *Engine) mounts() []string {
	var args []string
	for _, v := range e.Cfg.Web.Volumes {
		args = append(args, "-v", v)
	}
	return args
}

func (e *Engine) limits(memory string) []string {
	if memory == "" {
		return nil
	}
	return []string{"--memory", memory}
}

// runBase returns the docker run flags that web and worker containers share.
func (e *Engine) runBase(name, role, release string) []string {
	args := []string{
		"docker", "run", "-d",
		"--name", name,
		"--network", "muken",
		"--restart", "unless-stopped",
		"--env-file", e.appDir() + "/env",
		"--stop-timeout", strconv.Itoa(int(e.Cfg.StopGrace().Seconds())),
		"--label", "muken.app=" + e.Cfg.App,
		"--label", "muken.release=" + release,
		"--label", "muken.role=" + role,
	}
	return append(args, e.mounts()...)
}

func (e *Engine) startWeb(ctx context.Context, image, release string) ([]string, error) {
	web := e.Cfg.Web
	var started []string
	for i := 1; i <= web.Replicas; i++ {
		name := e.containerName("web", release, i)
		args := e.runBase(name, "web", release)
		args = append(args,
			"--label", "muken.health="+web.HealthPath,
			"--label", "muken.health_port="+strconv.Itoa(web.HealthPort))
		args = append(args, e.limits(web.Memory)...)
		if web.CPUs != "" {
			args = append(args, "--cpus", web.CPUs)
		}
		args = append(args, image)
		if _, err := e.R.Run(ctx, cmdline(args...)); err != nil {
			return started, ui.Wrap(err, "check the image name and server disk space", "cannot start %s", name)
		}
		started = append(started, name)
	}
	return started, nil
}

func (e *Engine) startWorkers(ctx context.Context, image, release string) ([]string, error) {
	var started []string
	for _, worker := range e.Cfg.Workers {
		for i := 1; i <= worker.Replicas; i++ {
			name := e.containerName(worker.Name, release, i)
			args := e.runBase(name, worker.Name, release)
			args = append(args, "--user", worker.User)
			args = append(args, e.limits(worker.Memory)...)
			// Replacing the entrypoint runs only the command, not the web server.
			args = append(args, "--entrypoint", "sh", image, "-c", worker.Command)
			if _, err := e.R.Run(ctx, cmdline(args...)); err != nil {
				return started, ui.Wrap(err, "", "cannot start worker %s", name)
			}
			started = append(started, name)
		}
	}
	return started, nil
}

// checkRunning catches workers that exit right after they start.
func (e *Engine) checkRunning(ctx context.Context, list []string) error {
	if len(list) == 0 {
		return nil
	}
	e.Sleep(3 * time.Second)
	for _, name := range list {
		out, _ := e.R.Run(ctx, "docker inspect -f '{{.State.Running}}' "+quote(name))
		if strings.TrimSpace(out) != "true" {
			return ui.Errf("fix the command in workers, then deploy again. The old version still runs",
				"worker %s exited right after start. Last log lines:\n%s", name, e.logsOf(ctx, name))
		}
	}
	return nil
}

func (e *Engine) smokeCmd() string {
	scheme, flags := "http", "-fsS"
	if e.Cfg.TLS != "none" && len(e.Cfg.Domains) > 0 {
		scheme, flags = "https", "-fsSk"
	}
	host := ""
	if len(e.Cfg.Domains) > 0 {
		host = " -H " + quote("Host: "+e.Cfg.Domains[0])
	}
	url := scheme + "://127.0.0.1" + e.Cfg.Web.SmokePath
	return fmt.Sprintf("curl %s -m 10 -o /dev/null%s %s", flags, host, quote(url))
}

func (e *Engine) smoke(ctx context.Context) error {
	if e.Cfg.Web.SmokePath == "" {
		return nil
	}
	cmd := e.smokeCmd()
	var err error
	// Traefik needs a moment to reload the route file.
	for i := 0; i < 5; i++ {
		if _, err = e.R.Run(ctx, cmd); err == nil {
			return nil
		}
		e.Sleep(time.Second)
	}
	return err
}

// switchRoute sends traffic to the web containers and checks it. On failure the previous route returns.
func (e *Engine) switchRoute(ctx context.Context, web []string) error {
	_, _ = e.R.Run(ctx, "cp -f "+quote(e.routeFile())+" "+quote(e.prevRoute())+" 2>/dev/null || rm -f "+quote(e.prevRoute()))
	if err := remote.Upload(ctx, e.R, e.routeFile(), []byte(RenderRoute(e.Cfg, web)), "644"); err != nil {
		return err
	}
	if err := e.smoke(ctx); err != nil {
		e.restoreRoute()
		return ui.Wrap(err, "the old version is back. Check web.smoke_path and the app logs", "smoke check %s failed after the switch", e.Cfg.Web.SmokePath)
	}
	return nil
}

func (e *Engine) restoreRoute() {
	prev, route := quote(e.prevRoute()), quote(e.routeFile())
	_, _ = e.R.Run(context.Background(), "if [ -f "+prev+" ]; then cp -f "+prev+" "+route+"; else rm -f "+route+"; fi")
}

// Rollback needs the containers of the previous release, which muken keeps stopped.
// With keepMaintenance and maintenance on, visitors stay on the maintenance page.
func (e *Engine) Rollback(ctx context.Context, keepMaintenance bool) (*Result, error) {
	if err := e.lock(ctx); err != nil {
		return nil, err
	}
	defer e.unlock()
	if err := e.preflight(ctx); err != nil {
		return nil, err
	}
	state, err := e.readState(ctx)
	if err != nil {
		return nil, err
	}
	if state.Previous == "" {
		return nil, ui.Errf("deploy at least twice before rolling back", "there is no previous release for %q", e.Cfg.App)
	}
	all, err := e.containers(ctx)
	if err != nil {
		return nil, err
	}
	target := containersOfRelease(all, state.Previous)
	web := webContainerNames(target)
	if len(web) == 0 {
		return nil, ui.Errf("redeploy it with: muken deploy --image "+state.PreviousImage,
			"the containers of release %s were pruned", state.Previous)
	}
	hold := e.maintenanceActive(ctx) && keepMaintenance
	if err := e.startPrevious(ctx, state.Previous, target, web, hold); err != nil {
		return nil, err
	}
	if !hold {
		e.endMaintenance(ctx)
	}

	e.Sleep(e.Cfg.Drain())
	if err := e.stopContainers(ctx, containerNames(containersOfRelease(all, state.Release))); err != nil {
		e.UI.Warn("could not stop the failed release cleanly: %v", err)
	}
	next := State{Release: state.Previous, Image: state.PreviousImage, Previous: state.Release, PreviousImage: state.Image}
	if err := e.writeState(ctx, next); err != nil {
		return nil, err
	}
	e.history(ctx, state.Previous, state.PreviousImage, "rollback")
	return &Result{Release: state.Previous, Image: state.PreviousImage, Previous: state.Release, MaintenanceOn: hold}, nil
}

// startPrevious starts the stopped containers and moves traffic to them, unless hold is set.
// On failure it stops them again.
func (e *Engine) startPrevious(ctx context.Context, release string, target []Container, web []string, hold bool) error {
	e.UI.Step("Starting release %s", release)
	list := cmdline(containerNames(target)...)
	if _, err := e.R.Run(ctx, "docker start "+list); err != nil {
		return ui.Wrap(err, "", "cannot start the previous release")
	}
	err := e.waitHealthy(ctx, web)
	if err == nil && hold {
		e.UI.Step("Keeping the maintenance page. `muken maintenance off` sends traffic to the old release")
	} else if err == nil {
		e.UI.Step("Switching traffic")
		err = e.switchRoute(ctx, web)
	}
	if err != nil {
		_, _ = e.R.Run(context.Background(), "docker stop "+list)
	}
	return err
}

// Plan describes what a deploy would do. It changes nothing.
func (e *Engine) Plan(ctx context.Context, opts DeployOptions) ([]string, error) {
	image := e.imageFor(opts)
	if err := e.preflight(ctx); err != nil {
		return nil, err
	}
	state, err := e.readState(ctx)
	if err != nil {
		return nil, err
	}
	web := e.Cfg.Web
	lines := []string{e.planHeadline(state, image)}
	for _, a := range e.Cfg.Accessories {
		lines = append(lines, fmt.Sprintf("Keep %s %s (created only if missing)", a.Type, a.Name))
	}
	if !opts.SkipPull {
		lines = append(lines, "Pull the image on the server")
	}
	if web.Migrate != "" && !opts.SkipMigrate {
		lines = append(lines, "Run migration: "+web.Migrate)
	}
	lines = append(lines, fmt.Sprintf("Start %d web container(s), wait up to %s for %s on port %d",
		web.Replicas, web.HealthTimeout, web.HealthPath, web.HealthPort))
	for _, worker := range e.Cfg.Workers {
		lines = append(lines, fmt.Sprintf("Start %d worker(s) %q", worker.Replicas, worker.Name))
	}
	lines = append(lines, e.planSwitch(opts, e.maintenanceActive(ctx))...)
	if state.Release != "" {
		lines = append(lines, fmt.Sprintf("Drain %s, then stop release %s (kept for rollback)", web.Drain, state.Release))
	}
	return lines, nil
}

func (e *Engine) planSwitch(opts DeployOptions, maintenance bool) []string {
	if maintenance && opts.KeepMaintenance {
		return []string{"Maintenance is on: keep the maintenance page. Traffic moves on `muken maintenance off`"}
	}
	line := "Switch traffic (no domain set: any host name reaches this app)"
	if len(e.Cfg.Domains) > 0 {
		line = "Switch traffic for " + strings.Join(e.Cfg.Domains, ", ")
	}
	if maintenance {
		return []string{line, "Maintenance is on: turn it off after the switch"}
	}
	return []string{line}
}

func (e *Engine) planHeadline(state State, image string) string {
	switch {
	case state.Release == "":
		return fmt.Sprintf("First deploy of %q to %s", e.Cfg.App, e.Cfg.Server.Host)
	case state.Image == image:
		return fmt.Sprintf("Redeploy %s (running release %s, same image)", image, state.Release)
	}
	return fmt.Sprintf("Image: %s -> %s (running release %s)", state.Image, image, state.Release)
}

func (e *Engine) Status(ctx context.Context) (*Status, error) {
	state, err := e.readState(ctx)
	if err != nil {
		return nil, err
	}
	list, err := e.containers(ctx)
	if err != nil {
		return nil, err
	}
	out, _ := e.R.Run(ctx, "docker inspect -f '{{.State.Running}}' muken-traefik 2>/dev/null")
	sort.Slice(list, func(i, j int) bool { return list[i].Release+list[i].Name > list[j].Release+list[j].Name })
	return &Status{State: state, Containers: list, Proxy: strings.TrimSpace(out) == "true", Maintenance: e.maintenanceActive(ctx)}, nil
}
