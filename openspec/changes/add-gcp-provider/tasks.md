## Context
Read: `internal/provider/aws.go`, `internal/provider/provider.go`
Skip: everything else

## 1. Client
- [x] 1.1 Add `internal/provider/gcp.go` with the JWT token flow and the Compute calls. Check: `make t PKG=./internal/provider RUN=GCP`

## 2. Text and docs
- [x] 2.1 Update help strings and docs. Check: `make check`

## Log
- Image family `ubuntu-2404-lts-amd64` in `ubuntu-os-cloud` checked against Ubuntu and Google pages. The JWT flow, firewall and instance fields follow the Google API docs as remembered, not re-read. The test verifies the RS256 signature and the claims, not Google behavior.
