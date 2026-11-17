// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

// Package deploy turns muken.yaml into commands on the server.
package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/shinsenter/muken/internal/config"
	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/ui"
)

// Base is the folder muken owns on the server.
const Base = "/opt/muken"

// Engine runs deploy steps through a Runner. Now, Sleep and Getenv are fields so tests can replace them.
type Engine struct {
	Cfg     *config.Config
	R       remote.Runner
	UI      *ui.Printer
	Secrets map[string]string
	Now     func() time.Time
	Sleep   func(time.Duration)
	Getenv  func(string) string
}

func New(cfg *config.Config, r remote.Runner, p *ui.Printer, secrets map[string]string) *Engine {
	return &Engine{Cfg: cfg, R: r, UI: p, Secrets: secrets, Now: time.Now, Sleep: time.Sleep, Getenv: os.Getenv}
}

func quote(s string) string { return remote.Quote(s) }

func cmdline(args ...string) string {
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = quote(a)
	}
	return strings.Join(out, " ")
}

func (e *Engine) appDir() string    { return Base + "/apps/" + e.Cfg.App }
func (e *Engine) routeFile() string { return Base + "/traefik/dynamic/" + e.Cfg.App + ".yml" }
func (e *Engine) prevRoute() string { return e.appDir() + "/route.prev.yml" }

func (e *Engine) containerName(role, release string, i int) string {
	return fmt.Sprintf("muken-%s-%s-%s-%d", e.Cfg.App, role, release, i)
}

// State is what the server remembers between deploys.
type State struct {
	Release       string `json:"release"`
	Image         string `json:"image"`
	Previous      string `json:"previous,omitempty"`
	PreviousImage string `json:"previous_image,omitempty"`
	UpdatedAt     string `json:"updated_at"`
}

func (e *Engine) readState(ctx context.Context) (State, error) {
	var state State
	file := e.appDir() + "/state.json"
	out, err := e.R.Run(ctx, "cat "+quote(file)+" 2>/dev/null || true")
	if err != nil {
		return state, err
	}
	if strings.TrimSpace(out) == "" {
		return state, nil
	}
	if err := json.Unmarshal([]byte(out), &state); err != nil {
		return state, ui.Wrap(err, "inspect "+file+" on the server", "state file is damaged")
	}
	return state, nil
}

func (e *Engine) writeState(ctx context.Context, state State) error {
	state.UpdatedAt = e.Now().UTC().Format(time.RFC3339)
	data, _ := json.MarshalIndent(state, "", "  ")
	return remote.Upload(ctx, e.R, e.appDir()+"/state.json", data, "644")
}

func (e *Engine) history(ctx context.Context, release, image, status string) {
	line := strings.Join([]string{e.Now().UTC().Format(time.RFC3339), e.Cfg.App, release, image, status}, "|")
	_, _ = e.R.Run(ctx, "echo "+quote(line)+" >> "+quote(e.appDir()+"/history.log"))
}

type Container struct {
	Name, Release, Role, State string
}

func (e *Engine) containers(ctx context.Context) ([]Container, error) {
	out, err := e.R.Run(ctx, `docker ps -a --filter label=muken.app=`+e.Cfg.App+
		` --format '{{.Names}}|{{.Label "muken.release"}}|{{.Label "muken.role"}}|{{.State}}'`)
	if err != nil {
		return nil, err
	}
	var list []Container
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "|")
		if len(fields) == 4 {
			list = append(list, Container{Name: fields[0], Release: fields[1], Role: fields[2], State: fields[3]})
		}
	}
	return list, nil
}

func containerNames(containers []Container) []string {
	var out []string
	for _, c := range containers {
		out = append(out, c.Name)
	}
	sort.Strings(out) // a stable order keeps the commands the same between runs
	return out
}

func (e *Engine) lock(ctx context.Context) error {
	// mkdir fails when the folder exists, which makes it an atomic lock.
	dir := quote(e.appDir() + "/lock")
	info := quote(e.appDir() + "/lock/info")
	_, err := e.R.Run(ctx, "mkdir -p "+quote(e.appDir())+" && mkdir "+dir+" 2>/dev/null && date -u +%FT%TZ > "+info)
	if err == nil {
		return nil
	}
	started, _ := e.R.Run(ctx, "cat "+info+" 2>/dev/null")
	return ui.Errf("wait for it to finish, or run `muken unlock` if it crashed",
		"another deploy of %q is running (started %s)", e.Cfg.App, strings.TrimSpace(started))
}

func (e *Engine) unlock() {
	_, _ = e.R.Run(context.Background(), "rm -rf "+quote(e.appDir()+"/lock"))
}

func (e *Engine) Unlock(ctx context.Context) error {
	_, err := e.R.Run(ctx, "rm -rf "+quote(e.appDir()+"/lock"))
	return err
}

func (e *Engine) preflight(ctx context.Context) error {
	if _, err := e.R.Run(ctx, "docker info >/dev/null 2>&1"); err != nil {
		return ui.Errf("run `muken up` first", "Docker is not available on %s", e.Cfg.Server.Host)
	}
	if _, err := e.R.Run(ctx, "docker network inspect muken >/dev/null 2>&1"); err != nil {
		return ui.Errf("run `muken up` first", "network 'muken' is missing on the server")
	}
	out, _ := e.R.Run(ctx, "docker inspect -f '{{.State.Running}}' muken-traefik 2>/dev/null")
	if strings.TrimSpace(out) != "true" {
		return ui.Errf("run `muken up` to start the proxy", "the proxy container muken-traefik is not running")
	}
	return nil
}

func (e *Engine) logsOf(ctx context.Context, name string) string {
	out, _ := e.R.Run(ctx, "docker logs --tail 25 "+quote(name)+" 2>&1")
	return strings.TrimSpace(out)
}

// showLogs prints the last lines of a container before the error, so the
// error and its hint stay the last thing the user reads.
func (e *Engine) showLogs(ctx context.Context, name string) {
	lines := strings.Split(e.logsOf(ctx, name), "\n")
	if len(lines) > 10 {
		lines = lines[len(lines)-10:]
	}
	e.UI.Info("Last log lines of %s:", name)
	for _, l := range lines {
		e.UI.Info("  | %s", l)
	}
}

func (e *Engine) probeCmd(name string) string {
	web := e.Cfg.Web
	return fmt.Sprintf(`ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' %s) && [ -n "$ip" ] && curl -fsS -m 3 -o /dev/null "http://$ip:%d%s"`,
		quote(name), web.HealthPort, web.HealthPath)
}

func (e *Engine) waitHealthy(ctx context.Context, list []string) error {
	const interval = 2 * time.Second
	attempts := max(int(e.Cfg.HealthTimeout()/interval), 1)
	pending := list
	for i := 0; i < attempts; i++ {
		var err error
		pending, err = e.probeAll(ctx, pending)
		if err != nil {
			return err
		}
		if len(pending) == 0 {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		e.Sleep(interval)
	}
	e.showLogs(ctx, pending[0])
	return ui.Errf("raise web.health_timeout, or check web.health_path and web.health_port. The old version still serves traffic",
		"%s did not become healthy in %s", pending[0], e.Cfg.Web.HealthTimeout)
}

// probeAll returns the containers that are not healthy yet. It fails at once when one has stopped.
func (e *Engine) probeAll(ctx context.Context, pending []string) ([]string, error) {
	var still []string
	for _, name := range pending {
		status, _ := e.R.Run(ctx, "docker inspect -f '{{.State.Status}}' "+quote(name))
		if s := strings.TrimSpace(status); s == "exited" || s == "dead" {
			e.showLogs(ctx, name)
			return nil, ui.Errf("fix the error above, then deploy again; the old version still serves traffic",
				"%s stopped during start", name)
		}
		if _, err := e.R.Run(ctx, e.probeCmd(name)); err != nil {
			still = append(still, name)
		}
	}
	return still, nil
}

func (e *Engine) removeContainers(list []string) {
	if len(list) == 0 {
		return
	}
	_, _ = e.R.Run(context.Background(), "docker rm -f "+cmdline(list...))
}

func (e *Engine) stopContainers(ctx context.Context, list []string) error {
	if len(list) == 0 {
		return nil
	}
	grace := int(e.Cfg.StopGrace().Seconds())
	_, err := e.R.Run(ctx, fmt.Sprintf("docker stop -t %d %s", grace+5, cmdline(list...)))
	return err
}
