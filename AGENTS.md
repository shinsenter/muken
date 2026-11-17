# AGENTS.md

Project context for coding agents and human contributors. Update it when a decision or the layout changes.

## Project

muken is a Go command-line tool that puts a Docker app on a server you own, with no downtime. It connects over SSH, prepares a Debian or Ubuntu server (Docker, firewall, Traefik), starts the new release next to the old one, switches traffic after a health check, and goes back when a check fails. It was made for the images of [shinsenter/php](https://github.com/shinsenter/php) and works with any app that answers an HTTP health check.

Guides: `docs/guide.md` for users, `docs/development.md` for the maintainer, who is learning Go from this code.

## Design principles

- Small and readable. A person who knows PHP and is new to Go must be able to follow the code from top to bottom.
- Everything that changes the server goes through `remote.Runner`, so a test can watch it with `remote.Fake`.
- A failure before the traffic switch must leave the old release serving. Keep that true.
- Tell the user what to do next. A user-facing error comes from `ui.Errf(hint, ...)`, with the fix in the hint.
- A new dependency needs a reason. Today there are four: SSH, YAML, age and `golang.org/x/term` (raw keyboard mode and terminal width for the prompts in `internal/ui`).
- Tests are offline: no server, no Docker, no network.
- Tests do not depend on the machine. No real `~/.ssh`, ssh-agent or environment: use `t.TempDir()` and `t.Setenv`. CI has none of them, so check with `env HOME=$(mktemp -d) SSH_AUTH_SOCK= GOCACHE=$(go env GOCACHE) GOMODCACHE=$(go env GOMODCACHE) go test ./...`.
- Few commands. Prefer a flag or a subcommand of an existing command over a new command, and do not add a flag that few people need. Similar jobs already share one command: `deploy --plan`, `init --list`, `server key`, `render ci`, `version --license`.
- The guide promises no gap between releases, so any command that can interrupt a running app warns before it acts, in its output and in the guide section "When an interruption is possible". Today that covers a Docker restart and a proxy restart during `up`, and `maintenance on`.
- Maintenance is the user's choice. `deploy` and `rollback` turn it off after the switch, `--keep-maintenance` keeps the page, and `status` shows it. `maintenance.page` is one HTML file, 64 KB at most.
- Keys of a project live in `.muken/` next to `muken.yaml`: `.muken/age.key` and `.muken/.ssh/`. `internal/project` creates the folder with its own `.gitignore`. `secrets init` and `server key` never overwrite a key. muken still reads `~/.config/muken/age.key` when a project has no key.
- `up` has no setup flag. It skips the base setup when Docker and the `muken` network exist, the same test that `doctor` uses to tell a new server from a broken one.

## Behavioral guidelines

- Think first: state assumptions, ask when unclear, show tradeoffs.
- Keep it simple: minimum code, no speculative features or abstractions.
- Change surgically: touch only what the request needs, match the existing style.
- Work toward a goal: define a check for each step and run it.

## Layout

| Path | Purpose |
|---|---|
| `cmd/muken/` | The commands. `main.go` has the table of commands |
| `internal/config/` | Reads and checks `muken.yaml` |
| `internal/catalog/` | The images and apps muken knows, with their defaults |
| `internal/deploy/` | `Provision`, `Plan`, `Deploy`, `Rollback`, `Maintenance` |
| `internal/remote/` | SSH runner and the fake one for tests |
| `internal/provider/`, `internal/cloudflare/` | DigitalOcean, Hetzner and Cloudflare clients |
| `internal/secrets/` | Encrypted secrets (age) |
| `internal/project/` | The `.muken` folder next to `muken.yaml` that holds the keys |
| `internal/agent/` | The monitoring sidecar |
| `internal/k8s/`, `internal/ci/`, `internal/ui/` | Manifests, the CI workflow file, console output |
| `legal.go`, `LICENSE`, `NOTICE`, `TRADEMARK.md`, `THIRD_PARTY_NOTICES.txt` | Legal texts, compiled into the binary |
| `scripts/third-party-notices.sh` | Writes `THIRD_PARTY_NOTICES.txt` |
| `Dockerfile` | The image: target `cli` (on `alpine` with `openssh-client` and `ca-certificates`) |
| `.github/workflows/` | `ci` (test and scan), `release` (binaries on a `v*` tag), `image` (Docker image on a `v*` tag), `cleanup` (every Monday, deletes finished workflow runs older than N days, default 7) |

## Build and test

```shell
go vet ./...
go test ./...
gofmt -l .          # must print nothing
make dist           # binaries in dist/
```

Report test results only after a real run. A failing test is a finding: fix the cause or tell the user. Do not edit a test to make it pass.

## Working rules

- Do not call a real server, a cloud API (DigitalOcean, Hetzner, Cloudflare) or Docker on your own. Ask first, with the reason and the command. Tests use `remote.Fake` and `httptest`.
- Work on a `0.x` branch (`0.1` now). Never commit or push to `main`.
- Use Conventional Commits: `feat:` `fix:` `refactor:` `docs:` `test:` `chore:`. One line, 60 characters or fewer, no body, no co-author line. The type follows semver: `feat:` is a minor bump, `fix:` a patch bump, `!` a major bump.
- Every commit must carry a valid signature (`git log --format='%h %G? %s'` shows `G`). Commit only when the user asks.
- Do not touch `.env`, keys, tokens or the age key file.
- Do not tag or publish a release without being asked. A tag `vX.Y.Z` on `main` starts the `release` workflow.
- After a dependency change, run `scripts/third-party-notices.sh` and commit `THIRD_PARTY_NOTICES.txt`.
- Change `docs/guide.md` and `CHANGELOG.md` with the code.
- Group a change into a few commits by topic and keep the history lean. Push only when asked. Force push only when asked, with `--force-with-lease`, and never on `main`.
- Keep the dependencies and the base image current so known CVEs do not ship. The image builds on `alpine:latest` with `apk upgrade`. Run `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` after a dependency change and before a release.

## Docs

- The reader is an engineer who uses Docker. Write English, in short paragraphs of about three lines, with tables for options and steps. Explain a term once, in "Words used in this guide".
- Highlight what needs attention with GitHub alerts: `> [!NOTE]`, `> [!TIP]`, `> [!IMPORTANT]`, `> [!WARNING]`, `> [!CAUTION]`. The Pages site draws them with the script in `_includes/head-custom.html`. The theme is `jekyll-theme-midnight`.
- People copy commands from the docs, so use placeholders that cannot be mistaken for real values: `YOUR_SERVER_IP`, `example.com`.
- The Docker example mounts only what muken needs. The age key is `.muken/age.key` and the SSH key is `.muken/.ssh`, so `-v "$PWD":/work` carries both. It does not mount `~/.config/muken`.
- After a docs change, read it again as the user: contradictions, old versions (the Go version comes from `go.mod`), and claims that the code does not back.

## Code style

- Go files start with the two-line header: `Copyright 2026 Mai Nhut Tan` and `SPDX-License-Identifier: Apache-2.0`.
- Comments say why, not what. One short sentence per line. No comment that repeats the next line.
- A function does one job and fits on a screen (about 40 lines). Use early returns.
- Plain code over clever code: no `map[bool]string{...}[cond]`, no closures that capture many variables, no factories for commands.
- `gofmt` is the only formatter. Names are ordinary and short in small scopes.

## License and name

muken is released under the Apache License 2.0. Do not remove or rewrite the notices in `LICENSE`, `NOTICE` or the file headers. The name "muken" is covered by `TRADEMARK.md`, not by the license: do not use it for another product or fork.

Contributions come with the agreement in `CONTRIBUTING.md`. Do not accept code that you cannot license under it (for example code copied from a GPL project).
