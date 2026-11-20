# AGENTS.md

muken is a Go CLI that deploys a Docker app to one Debian or Ubuntu server over SSH, with no gap between releases.

## Route

| You are about to | Read this, and nothing else first |
|---|---|
| Change behavior | `openspec/index.md`, then one spec |
| Plan or propose | `openspec/AGENTS.md` |
| Run several tasks | `openspec/loop.md` |
| Need a convention, a decision or a task rule | `openspec/project.md`. Run `grep -n '^## '`, then read one section |
| Check the 1.0 plan | `openspec/roadmap.md` |
| Need the shinsenter/php or shinsenter/laravel image behavior | `openspec/upstream.md` (a digest). The submodule `third_party/shinsenter-php` is read-only, open it only for a missing detail. The hook in `.claude/settings.json` updates it each session |

## Commands

- `make check`: vet, tests and gofmt. Prints only problems, then `check ok`.
- `make t PKG=./internal/deploy RUN=Maintenance`: a few tests, last 20 lines.
- `make check-ci`: the same check with no home and no ssh-agent, like CI.

Report results only after a real run. A failing test is a finding. Fix the cause or tell the user. Do not edit a test to make it pass.

## Behavior

- Think first. State assumptions, ask when unclear, show tradeoffs.
- Keep it simple. Write the minimum code. No speculative features.
- Change surgically. Touch only what the request needs. Match the style.
- Work toward a goal. Define a check for each step and run it.

## Rules

- Do not call a real server, a cloud API (DigitalOcean, Hetzner, Cloudflare) or Docker on your own. Ask first, with the reason and the command.
- Work on a `0.x` branch (`0.1` now). Never commit or push to `main`.
- Use Conventional Commits: `feat:` `fix:` `refactor:` `docs:` `test:` `chore:`. One line, 60 characters or fewer, no body, no co-author line.
- Every commit is signed. Commit only when the user asks. Group a change into a few commits by topic.
- Push only when asked. Force push only when asked, with `--force-with-lease`, never on `main`.
- Do not touch `.env`, keys, tokens or the age key file.
- Do not tag or publish a release without being asked.
- Keep the notices in `LICENSE`, `NOTICE` and the file headers.
- Dependency, docs and license rules for a task are in `openspec/project.md`, section "Task rules".
