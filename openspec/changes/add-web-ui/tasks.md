## Context
Read: `openspec/upstream.md`, `specs/image/spec.md`, `src/cmd/muken/main.go`, `src/internal/deploy/engine.go`
Skip: everything else. Do not read `third_party/` unless a detail is missing from `upstream.md`.

## 1. Image
- [ ] 1.1 Add the targets `web`, `web-test` to `src/Dockerfile` (written). Check: ask before `docker build -f src/Dockerfile --target web-test .` and `--target web .`
- [ ] 1.2 Add the post-boot hook `src/docker/web/rootfs/etc/hooks/post-boot/50-muken-api` (written). Check: ask before `docker run`
- [x] 1.3 Update `image.yml`: build, smoke test and scan `web` and `cli`. Publish `web` as the main tags and `cli` as `-cli` tags. Check: YAML parses. The run on GitHub is not done

## 2. API
- [x] 2.1 Add `src/internal/api/` with the routes in `specs/api/spec.md` and the job runner. Check: `make t PKG=./internal/api`
- [x] 2.2 Add `muken serve` in `src/cmd/muken/cmd_serve.go` and register it in `main.go`. Check: `make t PKG=./cmd/muken RUN=Serve`

## 3. Web
- [ ] 3.1 Create `web/` (an overlay, see `web/README.md`) with `MukenClient`, the pages in `specs/web/spec.md`, and file drivers for session, cache and queue (written). Check: `docker build --target web-test`. Not run: no `php` or `composer` on this machine
- [ ] 3.2 Add `HistoryStore` and its migration (written). Check: `php artisan test --filter=History`. Not run, same reason

## 4. Docs
- [x] 4.1 Add the Web section to `docs/guide.md`. Update `development.md`, `README.md`, `CHANGELOG.md`, `project.md`, `index.md`. Check: `make check`

## Log
- 2026-11-20: `muken serve` smoke-tested on loopback with a temp folder: version, `no_config`, file read, path escape, refusal of `0.0.0.0`. Status, deploy and the rest need a server and were not run.
- The web is an overlay on `composer create-project laravel/laravel`, because the machine has no PHP. The framework versions are whatever is latest at build time. Unverified: that Laravel's new-project Vite setup includes Tailwind CSS. If not, add `tailwindcss` and `@tailwindcss/vite` to the overlay.
- `docker run ghcr.io/shinsenter/muken deploy` now starts the web image. The guide uses `:cli` for the CLI.
- Facts about shinsenter/laravel come from `openspec/upstream.md` (commit `49c8243`). Docker Hub was not checked.
