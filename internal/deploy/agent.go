// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/ui"
)

func (e *Engine) ServerArch(ctx context.Context) (string, error) {
	out, err := e.R.Run(ctx, "uname -m")
	if err != nil {
		return "", err
	}
	switch strings.TrimSpace(out) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	}
	return "", ui.Errf("", "unsupported server architecture %q", strings.TrimSpace(out))
}

// InstallAgent copies muken to the server and runs it as the sidecar.
// The container has no app label, so deploys never touch it.
func (e *Engine) InstallAgent(ctx context.Context, binary io.Reader) error {
	streamer, ok := e.R.(remote.Streamer)
	if !ok {
		return ui.Errf("", "this connection cannot upload files")
	}
	e.UI.Step("Uploading the agent binary")
	if err := e.uploadAgentBinary(ctx, streamer, binary); err != nil {
		return err
	}
	e.UI.Step("Creating the agent token")
	if err := e.writeAgentEnv(ctx); err != nil {
		return err
	}
	e.UI.Step("Starting the agent")
	if err := e.startAgent(ctx); err != nil {
		return err
	}
	return e.writeAgentRoute(ctx)
}

const agentBinary = Base + "/bin/muken"

func (e *Engine) uploadAgentBinary(ctx context.Context, s remote.Streamer, binary io.Reader) error {
	bin := quote(agentBinary)
	cmd := fmt.Sprintf("mkdir -p %s && cat > %s.tmp && chmod 755 %s.tmp && mv -f %s.tmp %s", quote(Base+"/bin"), bin, bin, bin, bin)
	if err := s.RunStdin(ctx, cmd, binary); err != nil {
		return ui.Wrap(err, "", "upload failed")
	}
	return nil
}

// writeAgentEnv reuses the token file when it exists, so a reinstall keeps the same token.
func (e *Engine) writeAgentEnv(ctx context.Context) error {
	file := quote(Base + "/agent.token")
	out, err := e.R.Run(ctx, fmt.Sprintf("{ [ -s %s ] || (umask 077; openssl rand -hex 24 > %s); } && cat %s", file, file, file))
	if err != nil {
		return err
	}
	env := "MUKEN_AGENT_TOKEN=" + strings.TrimSpace(out) + "\n"
	if hook := e.Cfg.Agent.Webhook; hook != "" {
		env += "MUKEN_WEBHOOK=" + hook + "\n"
	}
	return remote.Upload(ctx, e.R, Base+"/agent.env", []byte(env), "600")
}

func (e *Engine) startAgent(ctx context.Context) error {
	port := e.Cfg.Agent.Port
	cmd := fmt.Sprintf("docker rm -f muken-agent >/dev/null 2>&1; docker run -d --name muken-agent --network muken --restart unless-stopped "+
		"-p 127.0.0.1:%d:%d --env-file %s -v /var/run/docker.sock:/var/run/docker.sock:ro -v %s:%s:ro -v %s:/muken:ro "+
		"--entrypoint /muken busybox:stable agent --addr :%d --disk-path %s --base-dir %s >/dev/null",
		port, port, quote(Base+"/agent.env"), quote(Base), quote(Base), quote(agentBinary), port, quote(Base), quote(Base))
	if _, err := e.R.Run(ctx, cmd); err != nil {
		return ui.Wrap(err, "check that Docker can pull busybox", "cannot start the agent")
	}
	return nil
}

func (e *Engine) writeAgentRoute(ctx context.Context) error {
	route := Base + "/traefik/dynamic/muken-agent.yml"
	if e.Cfg.Agent.Domain != "" {
		return remote.Upload(ctx, e.R, route, []byte(RenderAgentRoute(e.Cfg)), "644")
	}
	_, err := e.R.Run(ctx, "rm -f "+quote(route))
	return err
}

func (e *Engine) AgentToken(ctx context.Context) (string, error) {
	out, err := e.R.Run(ctx, "cat "+quote(Base+"/agent.token"))
	if err != nil {
		return "", ui.Errf("run `muken agent install` first", "no agent token on the server")
	}
	return strings.TrimSpace(out), nil
}

// defaultMaintenancePage is shown when muken.yaml has no maintenance.page.
const defaultMaintenancePage = `<!doctype html><meta charset=utf-8><title>Maintenance</title><body style="font:16px system-ui;text-align:center;padding:4rem"><h1>Back soon</h1><p>We are updating this site.</p>`

// maintenanceConf answers every request with 503 and the page from the mounted folder.
const maintenanceConf = `server { listen 80; error_page 503 /maintenance.html; location = /maintenance.html { internal; root /usr/share/nginx/html; } location / { return 503; } }`

// maintenanceDir holds maintenance.html on the server. The container mounts the folder,
// so a second `maintenance on` with another page needs no new container.
const maintenanceDir = Base + "/maintenance"

// Maintenance sends every visitor a 503 page, or puts the app back. An empty page means the built-in one.
func (e *Engine) Maintenance(ctx context.Context, on bool, page []byte) error {
	if err := e.lock(ctx); err != nil {
		return err
	}
	defer e.unlock()
	if on {
		return e.maintenanceOn(ctx, page)
	}
	return e.maintenanceOff(ctx)
}

func (e *Engine) maintenanceOn(ctx context.Context, page []byte) error {
	if len(page) == 0 {
		page = []byte(defaultMaintenancePage)
	}
	if _, err := e.R.Run(ctx, "mkdir -p "+quote(maintenanceDir)); err != nil {
		return ui.Wrap(err, "", "cannot create %s", maintenanceDir)
	}
	if err := remote.Upload(ctx, e.R, maintenanceDir+"/maintenance.html", page, "644"); err != nil {
		return ui.Wrap(err, "", "cannot upload the maintenance page")
	}
	script := quote("printf '%s' " + quote(maintenanceConf) + " > /etc/nginx/conf.d/default.conf && exec nginx -g 'daemon off;'")
	cmd := "docker inspect muken-maintenance >/dev/null 2>&1 || docker run -d --name muken-maintenance --network muken --restart unless-stopped -v " + quote(maintenanceDir) + ":/usr/share/nginx/html:ro --entrypoint sh nginx:alpine -c " + script + " >/dev/null"
	if _, err := e.R.Run(ctx, cmd); err != nil {
		return ui.Wrap(err, "", "cannot start the maintenance page")
	}
	secure := e.Cfg.TLS != "none" && len(e.Cfg.Domains) > 0
	route := renderRoute("muken-"+e.Cfg.App, hostRule(e.Cfg.Domains), secure, 80, []string{"muken-maintenance"})
	return remote.Upload(ctx, e.R, e.routeFile(), []byte(route), "644")
}

func (e *Engine) maintenanceActive(ctx context.Context) bool {
	out, _ := e.R.Run(ctx, "docker ps -q --filter "+quote("name=^muken-maintenance$"))
	return strings.TrimSpace(out) != ""
}

// endMaintenance removes the maintenance container after traffic moved to the new release.
func (e *Engine) endMaintenance(ctx context.Context) {
	if !e.maintenanceActive(ctx) {
		return
	}
	e.UI.Step("Turning maintenance off")
	_, _ = e.R.Run(ctx, "docker rm -f muken-maintenance >/dev/null 2>&1")
}

func (e *Engine) maintenanceOff(ctx context.Context) error {
	state, err := e.readState(ctx)
	if err != nil {
		return err
	}
	all, err := e.containers(ctx)
	if err != nil {
		return err
	}
	web := webContainerNames(containersOfRelease(all, state.Release))
	if len(web) == 0 {
		return ui.Errf("run `muken deploy`", "there is no running release to put back")
	}
	if err := remote.Upload(ctx, e.R, e.routeFile(), []byte(RenderRoute(e.Cfg, web)), "644"); err != nil {
		return err
	}
	_, _ = e.R.Run(ctx, "docker rm -f muken-maintenance >/dev/null 2>&1")
	return nil
}
