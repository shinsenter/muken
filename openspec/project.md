# Project

Update this file when a decision or the layout changes.

## Purpose

muken puts a Docker app on one server you own, with no gap between releases. It connects over SSH, prepares a Debian or Ubuntu server (Docker, firewall, Traefik), starts the new release next to the old one, switches traffic after a health check, and goes back when a check fails.

It was made for the images of [shinsenter/php](https://github.com/shinsenter/php). It works with any app that answers an HTTP health check. muken never builds images.

Users read `docs/guide.md`. The maintainer, who is learning Go from this code, reads `docs/development.md`.

## Tech

- Go. The version is in `src/go.mod`.
- Four dependencies: SSH (`x/crypto`), YAML, age and `golang.org/x/term` (raw keyboard mode and terminal width for the prompts in `src/internal/ui`). A new one needs a reason in the change proposal.
- The web UI is PHP (Laravel, Livewire, Tailwind) in `web/`. It runs only in the `web` image and adds no Go dependency. `muken serve` uses `net/http` of the standard library.
- muken runs on the user's machine or in the Docker image. It acts on the server over SSH, with `docker` commands.
- Server: Debian or Ubuntu. Proxy: Traefik. One server per app.
- CI: GitHub Actions. `ci` tests and scans, `release` builds binaries on a `v*` tag, `image` builds the Docker image on a `v*` tag, `cleanup` deletes finished workflow runs older than N days (default 7) every Monday.
- External services: DigitalOcean, Hetzner and Cloudflare APIs, ghcr.io, GitHub. Never call a real one without asking.

## Layout

| Path | Purpose |
|---|---|
| `src/cmd/muken/` | The commands. `main.go` has the table of commands |
| `src/internal/config/` | Reads and checks `muken.yaml` |
| `src/internal/catalog/` | The images and apps muken knows, with their defaults |
| `src/internal/deploy/` | `Provision`, `Plan`, `Deploy`, `Rollback`, `Maintenance` |
| `src/internal/remote/` | SSH runner and the fake one for tests |
| `src/internal/provider/`, `src/internal/cloudflare/` | DigitalOcean, Hetzner and Cloudflare clients |
| `src/internal/secrets/` | Encrypted secrets (age) |
| `src/internal/project/` | The `.muken` folder next to `muken.yaml` that holds the keys |
| `src/internal/agent/` | The monitoring sidecar |
| `src/internal/api/` | The JSON API of `muken serve`, for the web UI |
| `web/` | The web UI overlay: Laravel, Livewire, Tailwind. See `openspec/upstream.md` for the base image |
| `third_party/shinsenter-php/` | Read only. Context for the images. Updated at session start |
| `src/internal/k8s/`, `src/internal/ci/`, `src/internal/ui/` | Manifests, the CI workflow file, console output |
| `src/legal/`, `LICENSE`, `NOTICE`, `TRADEMARK.md`, `THIRD_PARTY_NOTICES.txt` | Legal texts, compiled into the binary. The root files are the source, `make sync-legal` copies them to `src/legal/` |
| `src/scripts/third-party-notices.sh` | Writes `THIRD_PARTY_NOTICES.txt` |
| `src/Dockerfile` | The images: `cli` and `web` (build from the root: `docker build -f src/Dockerfile --target web .`). `cli` is on `alpine` with `openssh-client` and `ca-certificates`. `web` is on `shinsenter/laravel`: target `cli`, on `alpine` with `openssh-client` and `ca-certificates` |
| `.github/workflows/` | `ci`, `release`, `image`, `cleanup` |
| `openspec/` | Specs, changes, roadmap |

## Principles

- Small and readable. A person who knows PHP and is new to Go can follow the code from top to bottom.
- Everything that changes the server goes through `remote.Runner`, so a test can watch it with `remote.Fake`.
- A failure before the traffic switch leaves the old release serving. Keep that true.
- Errors tell the user what to do next. A user-facing error comes from `ui.Errf(hint, ...)`, with the fix in the hint.
- Tests are offline: no server, no Docker, no network.
- Tests do not depend on the machine: no real `~/.ssh`, ssh-agent or environment. Use `t.TempDir()` and `t.Setenv`. CI has none of them. Check with `env HOME=$(mktemp -d) SSH_AUTH_SOCK= GOCACHE=$(go env GOCACHE) GOMODCACHE=$(go env GOMODCACHE) go test ./...`.
- Few commands. Prefer a flag or a subcommand of an existing command over a new command. Do not add a flag that few people need.
- The guide promises no gap between releases. Any command that can interrupt a running app warns before it acts, in its output and in the guide section "When an interruption is possible".

## Code style

- Go files start with the two-line header: `Copyright 2026 Mai Nhut Tan` and `SPDX-License-Identifier: Apache-2.0`.
- Comments say why, not what. One short sentence per line. No comment that repeats the next line or the name.
- A function does one job and fits on a screen (about 40 lines). Use early returns.
- Plain code over clever code: no `map[bool]string{...}[cond]`, no closures that capture many variables, no factories for commands.
- `gofmt` is the only formatter. Names are ordinary and short in small scopes.

## Docs style

- The reader is an engineer who uses Docker. Write English, in short paragraphs of about three lines, with tables for options and steps. Explain a term once, in "Words used in this guide".
- Highlight with GitHub alerts: `> [!NOTE]`, `> [!TIP]`, `> [!IMPORTANT]`, `> [!WARNING]`, `> [!CAUTION]`. The Pages site draws them with the script in `_includes/head-custom.html`. The theme is `jekyll-theme-midnight`.
- Use placeholders that cannot be mistaken for real values, because people copy commands: `YOUR_SERVER_IP`, `example.com`.
- The Docker example mounts only what muken needs. The age key is `.muken/age.key` and the SSH key is `.muken/.ssh`, so `-v "$PWD":/work` carries both.
- After a docs change, read it again as the user. Look for contradictions, old versions (the Go version comes from `src/go.mod`) and claims that the code does not back.

## Task rules

- Dependency change: run `src/scripts/third-party-notices.sh` and commit `THIRD_PARTY_NOTICES.txt`. Run `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` after the change and before a release. Keep dependencies and the base image current, so known CVEs do not ship.
- Code change: change `docs/guide.md` and `CHANGELOG.md` with the code.
- Commit types follow semver: `feat:` is a minor bump, `fix:` a patch bump, `!` a major bump.
- Tag `vX.Y.Z` on `main` starts the `release` workflow. Never tag without being asked.
- Do not accept code that you cannot license under `CONTRIBUTING.md`, for example code copied from a GPL project.
- The name "muken" is covered by `TRADEMARK.md`, not by the license.

## Constraints

One server per app. Debian and Ubuntu servers only. No load balancer over several servers. No database backups. No migration rollback. No Cloud Run.

## Decisions

| Decision | Why |
|---|---|
| Similar jobs share one command: `deploy --plan`, `init --list`, `up`, `server key`, `render ci`, `version --license` | Fewer commands to learn |
| `up` has no setup flag | `up` is safe to repeat. A failed setup leaves the `muken` network missing, so the next `up` runs the full setup again |
| Keys live in `.muken/` next to `muken.yaml`: `.muken/age.key` and `.muken/.ssh/`. muken still reads `~/.config/muken/age.key` when a project has no key | One context per project or customer. `-v "$PWD":/work` carries the keys into Docker |
| `secrets init` and `server key` never overwrite a key. `secrets init` also stops when a new key would hide the key that an existing secrets file needs | An overwritten key makes the secrets unreadable |
| `deploy` and `rollback` turn maintenance off after the switch. `--keep-maintenance` keeps the page. `up` and `scale` always turn it off. `status` shows it | The user decides what visitors see. Hidden state surprises people |
| Neither image has a Docker CLI or a `dind` target. `cli` builds on `alpine:latest` with `apk upgrade` | muken runs docker on the server over SSH. A small image with the newest base has fewer CVEs |
| `latest` is the `web` image, on `shinsenter/laravel` with SSH on. The small image is `:cli` | The maintainer wants one image for the CLI and the site. Anyone who runs `docker run ghcr.io/shinsenter/muken deploy` must use `:cli` now |
| The web has no login. The API has no login and refuses a non-loopback address. A change asks for a second click | A login is not wanted. The risk is in the guide, next to the example |
| Only the history is in SQLite. Config is the file `muken.yaml`. Session, cache and queue use files | A missing SQLite file must never break the app |
| The specs of this repo cache the facts of shinsenter/php in `openspec/upstream.md` | A session reads one small file, and the hook says when to refresh it |
| `up` warns before it restarts Docker or replaces the proxy | The guide promises no gap |
| `doctor` only warns when Docker or the proxy is missing on a server that `up` never set up | The guide says to run `doctor` before the first `up` |
| Tests do not read the machine's `~/.ssh` or environment | The CI runner has none, and two tests failed there |
| Comments say why, not what | The maintainer wants short comments that help a developer |
| Group commits by topic and keep the history lean. Push only when asked | The maintainer's preference |
| Specs follow OpenSpec. The task loop is in `openspec/loop.md` | Agents read the spec instead of the code, which costs fewer tokens |
