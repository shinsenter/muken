# add-web-ui

## Why
muken is a CLI. Some people want to deploy, roll back and switch maintenance from a browser, and run muken from one Docker image. The images of shinsenter/php already carry Nginx, PHP-FPM and an SSH switch, so one image can be both the muken CLI and a small PHP site.

## What changes
- Image: the final image builds on `shinsenter/laravel` (alpine variant), turns SSH on by default, and holds the `muken` binary and a Laravel app. The old small `cli` target stays.
- API: a new command `muken serve` runs a JSON HTTP API on loopback. The PHP app calls it. The API reuses `internal/deploy`, so the CLI and the web do the same thing.
- Web: Laravel + Livewire + Tailwind CSS in `web/`, served by the image (Nginx + PHP-FPM). No login. It does what the CLI does for one server: status, plan, deploy, rollback, scale, maintenance, up, doctor, logs.
- Config: `muken.yaml` is the only config. It is a fixed file, `/work/muken.yaml`. The web reads it. The only write is the setup form, which makes it when it is missing and never overwrites it.
- Setup: when `muken.yaml` is missing, the first visit goes to a form (`/setup`) that makes it with `muken init`, and can make an SSH key. `muken init` gets a flag `--ssh-key`.
- History: only the history of operations is stored, in SQLite. A deleted file is recreated by a migration with an empty history. A DB error never breaks a page or an operation.
- Upstream: `openspec/upstream.md` caches the facts of shinsenter/php. The session hook tells the agent to update the specs when the submodule changes.

## Impact
- Specs: `image` (modified), `api`, `web`, `history` (new).
- Code: `Dockerfile`, `.github/workflows/image.yml`, `src/cmd/muken/cmd_serve.go`, `src/internal/api/`, `web/`
- Docs: `docs/guide.md` (new section Web), `docs/development.md`, `README.md`, `CHANGELOG.md`, `openspec/project.md` (Layout, Tech, Decisions), `openspec/index.md`
- Risk: no login. Anyone who reaches the web port can deploy and read logs. The image publishes no port by default, and the guide says to put it behind a private network or an auth proxy.
- Risk: the image is far bigger than alpine + muken, and Trivy may report more CVEs from the base. The scan gate stays.
- Roadmap: not in the 1.0 plan yet. Ask before adding it.
