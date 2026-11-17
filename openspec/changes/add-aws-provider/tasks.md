## Context
Read: `internal/provider/vultr.go`, `internal/provider/provider.go`, `createServer` in `cmd/muken/cmd_misc.go`
Skip: everything else

## 1. Client
- [x] 1.1 Add `internal/provider/aws.go` with SigV4 and the EC2 calls. Check: `make t PKG=./internal/provider RUN=AWS`
- [x] 1.2 Add `User` to `Server` and write it to `server.user` in `createServer`. Check: `make t PKG=./cmd/muken RUN=Server`

## 2. Text and docs
- [x] 2.1 Update help strings and docs. Check: `make check`

## Log
- SigV4 checked against the AWS documentation vector (signing key and signature). EC2 action names and fields follow the API reference; ImportKeyPair was read there. The AMI name pattern, the Duplicate error code and the default VPC rule were not checked against a live account.
- Next: GCP change.
