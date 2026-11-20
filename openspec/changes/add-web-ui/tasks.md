## Context
Read: `openspec/upstream.md`, `specs/image/spec.md`, `src/cmd/muken/main.go`, `src/internal/deploy/engine.go`
Skip: everything else. Do not read `third_party/` unless a detail is missing from `upstream.md`.

## 1. Image
- [ ] 1.1 Add the target `web` to `src/Dockerfile`: node stage for Vite, composer stage, final stage on `shinsenter/laravel` alpine, `ENABLE_SSHD=1`, `openssh-client`, `ca-certificates`, the muken binary. Check: ask before `docker build`
- [ ] 1.2 Add the post-boot hook that starts `muken serve`. Check: ask before `docker run`
- [ ] 1.3 Update `image.yml`: build, smoke test and scan `web`. Publish `web` as the main tags and `cli` as `-cli` tags. Check: read the workflow again

## 2. API
- [ ] 2.1 Add `src/internal/api/` with the routes in `specs/api/spec.md` and the job runner. Check: `make t PKG=./internal/api`
- [ ] 2.2 Add `muken serve` in `src/cmd/muken/cmd_serve.go` and register it in `main.go`. Check: `make t PKG=./cmd/muken RUN=Serve`

## 3. Web
- [ ] 3.1 Create `web/` (Laravel, Livewire, Tailwind) with `MukenClient`, the pages in `specs/web/spec.md`, and file drivers for session, cache and queue. Check: `cd web && php artisan test`
- [ ] 3.2 Add `HistoryStore` and its migration. Check: `php artisan test --filter=History`

## 4. Docs
- [ ] 4.1 Add the Web section to `docs/guide.md`. Update `development.md`, `README.md`, `CHANGELOG.md`, `project.md`, `index.md`. Check: `make check`

## Log
- Facts about shinsenter/laravel come from `openspec/upstream.md` (commit `49c8243`). Docker Hub was not checked.
