## Context
Read: `src/internal/provider/hetzner.go`, `src/internal/provider/provider.go`, `src/internal/provider/provider_test.go`
Skip: everything else

## 1. Client
- [x] 1.1 Add `src/internal/provider/vultr.go` and register `vultr` in `provider.go`. Check: `make t PKG=./internal/provider`
- [x] 1.2 Add Vultr tests in `provider_test.go`. Check: `make t PKG=./internal/provider RUN=Vultr`

## 2. Text and docs
- [x] 2.1 Update help strings in `src/cmd/muken/main.go` and `cmd_misc.go`. Check: `make check`
- [x] 2.2 Update `docs/guide.md`, `docs/development.md`, `README.md`, `CHANGELOG.md`. Check: read them again as the user

## Log
- Client written from the Vultr v2 docs as remembered. Not checked against the live API. OS id 2284 (Ubuntu 24.04) unverified.
- Next: archive the change after review.
