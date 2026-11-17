# AGENTS.md

muken is a Go CLI that deploys a Docker app to one Debian or Ubuntu server over SSH, with no gap between releases.

<!-- OPENSPEC:START -->
## Specs

Specs are the source of truth. They live in `openspec/`.

- Read `openspec/project.md` for context, conventions and decisions.
- Read `openspec/specs/<capability>/spec.md` before you change that capability.
- Read `openspec/AGENTS.md` before you plan, propose or change behavior.
- Read `openspec/loop.md` before you run a change with several tasks, or a `/loop`.
- Read `openspec/roadmap.md` for the 1.0 plan. Target: early 2027.
<!-- OPENSPEC:END -->

## Commands

```shell
go vet ./...
go test ./...
gofmt -l .          # must print nothing
make dist           # binaries in dist/
```

Report test results only after a real run. A failing test is a finding. Fix the cause or tell the user. Do not edit a test to make it pass.

## Behavior

- Think first. State assumptions, ask when unclear, show tradeoffs.
- Keep it simple. Write the minimum code. No speculative features or abstractions.
- Change surgically. Touch only what the request needs. Match the existing style.
- Work toward a goal. Define a check for each step and run it.

## Rules

- Do not call a real server, a cloud API (DigitalOcean, Hetzner, Cloudflare) or Docker on your own. Ask first, with the reason and the command. Tests use `remote.Fake` and `httptest`.
- Work on a `0.x` branch (`0.1` now). Never commit or push to `main`.
- Use Conventional Commits: `feat:` `fix:` `refactor:` `docs:` `test:` `chore:`. One line, 60 characters or fewer, no body, no co-author line. `feat:` is a minor bump, `fix:` a patch bump, `!` a major bump.
- Every commit carries a valid signature. `git log --format='%h %G? %s'` shows `G`.
- Commit only when the user asks. Group a change into a few commits by topic and keep the history lean.
- Push only when asked. Force push only when asked, with `--force-with-lease`, and never on `main`.
- Do not touch `.env`, keys, tokens or the age key file.
- Do not tag or publish a release without being asked. A tag `vX.Y.Z` on `main` starts the `release` workflow.
- After a dependency change, run `scripts/third-party-notices.sh` and commit `THIRD_PARTY_NOTICES.txt`.
- Keep dependencies and the base image current, so known CVEs do not ship. Run `go run golang.org/x/vuln/cmd/govulncheck@latest ./...` after a dependency change and before a release.
- Change `docs/guide.md` and `CHANGELOG.md` with the code.
- Do not remove or rewrite the notices in `LICENSE`, `NOTICE` or the file headers. The name "muken" is covered by `TRADEMARK.md`.
- Do not accept code that you cannot license under `CONTRIBUTING.md`, for example code copied from a GPL project.
