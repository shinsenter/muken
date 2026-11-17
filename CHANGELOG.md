# Changelog

All notable changes are in this file.

## [Unreleased]
- First version (0.1). Commands: `init`, `server`, `up`, `doctor`, `deploy`, `rollback`, `scale`, `maintenance`, `status`, `logs`, `exec`, `ssh`, `unlock`, `secrets`, `dns`, `agent`, `render`, `version`. Similar jobs share one command: `deploy --plan`, `init --list`, `server key`, `render ci`, `version --license`.
- Deploy with no downtime through Traefik, with a health check before the switch and an automatic return when a check fails.
- Servers at DigitalOcean, Hetzner and Vultr (`--provider vultr`, token in `VULTR_API_KEY`), DNS and certificates through Cloudflare, secrets encrypted with age, a monitoring sidecar and Kubernetes manifests.
- Docker image for amd64 and arm64: `ghcr.io/shinsenter/muken` (on `alpine` with `openssh-client`, no Docker CLI). Scanned and signed.
- `muken server key [name]` makes an SSH key for one project in `.muken/.ssh` next to `muken.yaml`, with a `.gitignore` so it is not committed. Pass the `.pub` file to `server create --ssh-pubkey`.
- `muken secrets init` keeps the key in `.muken/age.key` next to `muken.yaml`, with a `.gitignore`. It never overwrites a key, and it stops if a new key would hide the one that an existing `muken.secrets.age` needs. muken still reads `~/.config/muken/age.key` when a project has no key.
- `maintenance.page` in `muken.yaml` sets the HTML file that `muken maintenance on` shows (HTTP 503, 1 byte to 64 KB).
- `muken deploy` turns maintenance off after it moves traffic to the new release. `--keep-maintenance` leaves the maintenance page in place until `muken maintenance off`.
- `muken rollback` turns maintenance off too, and `--keep-maintenance` keeps the page. `muken status` shows whether maintenance is on.
- Apache License 2.0. The license, the notices and the name policy are inside the binary (`muken version --license`).
- Fix: `muken up` no longer hangs on a server that does not answer SSH. The handshake times out after 15 seconds and Ctrl+C stops it at once.
- Console output in the style of Laravel: the questions are framed prompts you answer with the arrow keys, space and Enter, and `ui` has sections and dotted rows. Output in CI and pipes has no colors and no prompts. Ctrl+C at a prompt ends the command with exit code 130.
- The accent color is lime, set in `internal/ui/theme.go`. `help`, `status`, `up`, `doctor` and the `-h` page of each command use sections and dotted rows.
- `muken init` offers the PHP release candidate (`8.6`) and asks for one database or none, then asks about Redis. `muken init --list` uses sections.
