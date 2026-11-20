// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package deploy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	"github.com/shinsenter/muken/internal/remote"
	"github.com/shinsenter/muken/internal/ui"
)

type provisionStep struct {
	title string
	cmd   string
}

// traefikArgs returns the proxy flags. The Cloudflare address list is not here.
// The command adds it from a shell variable that it fills in on the server.
func (e *Engine) traefikArgs() []string {
	c := e.Cfg
	args := []string{
		"--providers.file.directory=/dynamic",
		"--providers.file.watch=true",
		"--entrypoints.web.address=:80",
		"--log.level=INFO",
	}
	if c.TLS == "none" {
		return args
	}
	args = append(args,
		"--entrypoints.websecure.address=:443",
		"--entrypoints.websecure.http3=true",
		"--entrypoints.web.http.redirections.entrypoint.to=websecure",
		"--entrypoints.web.http.redirections.entrypoint.scheme=https",
		"--certificatesresolvers.le.acme.email="+c.Email,
		"--certificatesresolvers.le.acme.storage=/acme/acme.json",
	)
	if c.TLS == "cloudflare" {
		return append(args, "--certificatesresolvers.le.acme.dnschallenge.provider=cloudflare")
	}
	return append(args, "--certificatesresolvers.le.acme.httpchallenge.entrypoint=web")
}

// traefikStep builds the command that starts the proxy. The container is
// replaced only when the flags, the image or the Cloudflare token change.
func (e *Engine) traefikStep(ctx context.Context) (step provisionStep, hash string, err error) {
	c := e.Cfg
	args := e.traefikArgs()
	hashInput := strings.Join(args, " ") + c.Server.TraefikImage
	envFlag := ""
	if c.TLS == "cloudflare" {
		tokenHash, err := e.uploadCloudflareToken(ctx)
		if err != nil {
			return provisionStep{}, "", err
		}
		hashInput += tokenHash
		envFlag = " --env-file " + quote(Base+"/traefik/cf.env")
	}
	sum := sha256.Sum256([]byte(hashInput))
	hash = hex.EncodeToString(sum[:])[:12]
	return provisionStep{"Starting the Traefik proxy", e.traefikCmd(args, hash, envFlag)}, hash, nil
}

func (e *Engine) uploadCloudflareToken(ctx context.Context) (string, error) {
	token := e.Getenv("CLOUDFLARE_API_TOKEN")
	if token == "" {
		return "", ui.Errf("create a token with Zone:DNS:Edit and export CLOUDFLARE_API_TOKEN", "CLOUDFLARE_API_TOKEN is not set")
	}
	if err := remote.Upload(ctx, e.R, Base+"/traefik/cf.env", []byte("CF_DNS_API_TOKEN="+token+"\n"), "600"); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:]), nil
}

func (e *Engine) traefikCmd(args []string, hash, envFlag string) string {
	prefix, extra := "", ""
	if e.Cfg.Cloudflare.Enabled {
		// Trust the visitor address that Cloudflare forwards.
		prefix = `CF="$(curl -fsS https://www.cloudflare.com/ips-v4 https://www.cloudflare.com/ips-v6 | tr '\n' ',' | sed 's/,$//')"; `
		extra = ` "--entrypoints.websecure.forwardedHeaders.trustedIPs=$CF"`
	}
	return fmt.Sprintf(`%scur=$(docker inspect -f '{{index .Config.Labels "muken.hash"}}' muken-traefik 2>/dev/null || true); `+
		`if [ "$cur" != %s ] || [ "$(docker inspect -f '{{.State.Running}}' muken-traefik 2>/dev/null)" != true ]; then `+
		`docker rm -f muken-traefik >/dev/null 2>&1 || true; `+
		`docker run -d --name muken-traefik --restart unless-stopped --network muken `+
		`-p 80:80 -p 443:443 -p 443:443/udp --label muken.hash=%s%s `+
		`-v %s:/dynamic:ro -v %s:/acme %s %s%s >/dev/null; fi`,
		prefix, quote(hash), hash, envFlag, quote(Base+"/traefik/dynamic"), quote(Base+"/acme"),
		quote(e.Cfg.Server.TraefikImage), cmdline(args...), extra)
}

// warnDockerRestart warns when the Docker step will restart a server that runs containers.
func (e *Engine) warnDockerRestart(ctx context.Context) {
	out, _ := e.R.Run(ctx, `command -v docker >/dev/null 2>&1 && [ ! -f /etc/docker/daemon.json ] && docker ps -q 2>/dev/null | wc -l`)
	n, _ := strconv.Atoi(strings.TrimSpace(out))
	if n == 0 {
		return
	}
	e.UI.Warn("Docker restarts to apply the log settings. The %d container(s) running on this server stop, and those without a restart policy stay stopped", n)
}

func (e *Engine) warnProxyRestart(ctx context.Context, hash string) {
	out, _ := e.R.Run(ctx, `docker inspect -f '{{.State.Running}} {{index .Config.Labels "muken.hash"}}' muken-traefik 2>/dev/null`)
	running, current, _ := strings.Cut(strings.TrimSpace(out), " ")
	if running != "true" || current == hash {
		return
	}
	e.UI.Warn("The proxy settings changed, so the proxy restarts. Every site on this server drops open connections for a few seconds")
}

// provisionSteps lists the server setup in the order it runs. Each command
// can run again without harm.
func (e *Engine) provisionSteps() []provisionStep {
	return []provisionStep{
		{"Checking the operating system", `. /etc/os-release && case "$ID $ID_LIKE" in *debian*|*ubuntu*) ;; *) echo "muken supports Debian and Ubuntu servers, found $ID" >&2; exit 1;; esac`},
		{"Installing base packages", `dpkg -s curl ca-certificates ufw openssl unattended-upgrades >/dev/null 2>&1 || { export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq curl ca-certificates ufw openssl unattended-upgrades >/dev/null; }`},
		{"Installing Docker", `command -v docker >/dev/null 2>&1 || curl -fsSL https://get.docker.com | sh >/dev/null 2>&1; ` +
			// Restart only after writing the log settings. A restart stops every container on the server.
			`new=; [ -f /etc/docker/daemon.json ] || { mkdir -p /etc/docker; printf '{"log-driver":"json-file","log-opts":{"max-size":"10m","max-file":"3"}}' > /etc/docker/daemon.json; new=1; }; ` +
			`if command -v systemctl >/dev/null 2>&1; then systemctl enable --now docker >/dev/null 2>&1; [ -z "$new" ] || systemctl restart docker; fi; docker info >/dev/null`},
		{"Turning on the firewall", fmt.Sprintf(`ufw allow %d/tcp >/dev/null && ufw allow 80/tcp >/dev/null && ufw allow 443/tcp >/dev/null && ufw allow 443/udp >/dev/null && ufw --force enable >/dev/null`, e.Cfg.Server.Port)},
		{"Turning on automatic security updates", `printf 'APT::Periodic::Update-Package-Lists "1";\nAPT::Periodic::Unattended-Upgrade "1";\n' > /etc/apt/apt.conf.d/20auto-upgrades`},
		{"Adding swap when there is none", `if [ -z "$(swapon --show 2>/dev/null)" ]; then fallocate -l 1G /swapfile && chmod 600 /swapfile && mkswap /swapfile >/dev/null && swapon /swapfile; grep -q /swapfile /etc/fstab || echo '/swapfile none swap sw 0 0' >> /etc/fstab; fi`},
		{"Creating folders and the Docker network", fmt.Sprintf(`mkdir -p %s/traefik/dynamic %s/acme %s/apps %s/tmp && touch %s/acme/acme.json && chmod 600 %s/acme/acme.json && (docker network inspect muken >/dev/null 2>&1 || docker network create muken >/dev/null)`, Base, Base, Base, Base, Base, Base)},
	}
}

// Provision prepares a Debian or Ubuntu server. It is safe to run again.
func (e *Engine) Provision(ctx context.Context) error {
	return e.provision(ctx, e.provisionSteps())
}

// Prepare is Provision for `muken up`. It skips the base steps when Docker and the muken network exist,
// but still runs the proxy step, which applies proxy changes. It reports whether it skipped them.
func (e *Engine) Prepare(ctx context.Context) (skipped bool, err error) {
	if _, err := e.R.Run(ctx, `docker info >/dev/null 2>&1 && docker network inspect muken >/dev/null 2>&1`); err != nil {
		return false, e.Provision(ctx)
	}
	return true, e.provision(ctx, nil)
}

func (e *Engine) provision(ctx context.Context, steps []provisionStep) error {
	if len(steps) > 0 {
		e.warnDockerRestart(ctx)
	}
	for _, s := range steps {
		e.UI.Step("%s", s.title)
		if out, err := e.R.Run(ctx, s.cmd); err != nil {
			return ui.Wrap(err, "fix the cause and run `muken up` again; it is safe to repeat",
				"%s failed%s", strings.ToLower(s.title), tail(out))
		}
	}
	proxy, hash, err := e.traefikStep(ctx)
	if err != nil {
		return err
	}
	e.warnProxyRestart(ctx, hash)
	e.UI.Step("%s", proxy.title)
	if _, err := e.R.Run(ctx, proxy.cmd); err != nil {
		return ui.Wrap(err, "check traefik_image and that ports 80 and 443 are free", "cannot start the proxy")
	}
	return nil
}

func tail(out string) string {
	out = strings.TrimSpace(out)
	if out == "" {
		return ""
	}
	return ":\n" + out
}
