## Context
Read: `openspec/upstream.md`, `specs/image/spec.md`, `src/cmd/muken/main.go`, `src/internal/deploy/engine.go`
Skip: everything else. Do not read `third_party/` unless a detail is missing from `upstream.md`.

## 1. Image
- [x] 1.1 Add the targets `web`, `web-test` to `src/Dockerfile`. Check: `docker build -f src/Dockerfile --target web-test .` (11 tests pass) and `--target web .` (builds)
- [x] 1.2 Add the post-boot hook `src/docker/web/rootfs/etc/hooks/post-boot/50-muken-api`. Check: `docker run` with a temp folder: `muken serve` answers on 7600, pages open, key login over SSH works
- [x] 1.3 Update `image.yml`: build, smoke test and scan `web` and `cli`. Publish `web` as the main tags and `cli` as `-cli` tags. Check: YAML parses. The run on GitHub is not done

## 2. API
- [x] 2.1 Add `src/internal/api/` with the routes in `specs/api/spec.md` and the job runner. Check: `make t PKG=./internal/api`
- [x] 2.2 Add `muken serve` in `src/cmd/muken/cmd_serve.go` and register it in `main.go`. Check: `make t PKG=./cmd/muken RUN=Serve`

## 3. Web
- [x] 3.1 Create `web/` (an overlay, see `web/README.md`) with `MukenClient`, the pages in `specs/web/spec.md`, and file drivers for session, cache and queue. Check: `docker build --target web-test`, `PagesTest` passes
- [x] 3.2 Add `HistoryStore` and its migration. Check: `HistoryStoreTest` passes (5 tests)

## 4. Docs
- [x] 4.1 Add the Web section to `docs/guide.md`. Update `development.md`, `README.md`, `CHANGELOG.md`, `project.md`, `index.md`. Check: `make check`

## Log
- 2026-11-20: `muken serve` smoke-tested on loopback with a temp folder: version, `no_config`, file read, path escape, refusal of `0.0.0.0`. Status, deploy and the rest need a server and were not run.
- The web is an overlay on `composer create-project laravel/laravel`, because the machine has no PHP. The framework versions are whatever is latest at build time. The built CSS has the Tailwind classes of the views, so the new-project setup includes Tailwind.
- 2026-11-20, Docker on a Mac, image `web` with a temp `/work` and no `muken.yaml`: every page answers 200 and shows the `no_config` message. `age.key` is not listed in Config. Deleting `history.sqlite` while the container runs gives an empty History and a new file. SSH: key login works, no key means `Permission denied`. No error in the container log.
- Bugs the run found: the Livewire default layout is `layouts::app` in the newest version (now `#[Layout]` on each component). `php`, `composer` and `artisan` are aliases that run in `APP_PATH` (build stages set `APP_PATH=/work`). `init-laravel` could not write `.env` as the app user, so every page was a 500 without `APP_KEY` (the hook makes the key as root).
- 2026-11-20 later: the app is copied from `/source/`. `INITIAL_PROJECT=""` did not stop the base from making a Laravel project (the default comes back), `cp -n` kept its `routes/web.php` and `composer update` removed Livewire, so `/deploy` was a 404. `DISABLE_AUTORUN_CREATING_PROJECT=1` fixes it. `composer.json` and `package.json` are removed so the first start runs no `composer update` or `npm install`. `openssh-server` is baked in. With `--network none`: pages 200, sshd runs, footer shows. Footer added: version and copyright.
- Not run: a deploy, rollback or status against a real server through the web, arm64, Trivy on the `web` image, the GitHub workflow.
- `docker run ghcr.io/shinsenter/muken deploy` now starts the web image. The guide uses `:cli` for the CLI.
- Facts about shinsenter/laravel come from `openspec/upstream.md` (commit `49c8243`). Docker Hub was not checked.
