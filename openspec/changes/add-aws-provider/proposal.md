# add-aws-provider

## Why
`muken server create` has no AWS option. AWS needs request signing, so it does not fit the shared Bearer-token client.

## What changes
- `--provider aws` creates and terminates EC2 instances. Credentials come from `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY`. `AWS_REGION` sets the region that the region list is read from (default `us-east-1`).
- The client signs requests with SigV4 itself. No SDK, so `go.mod` does not change.
- The server runs the newest Ubuntu 24.04 AMI from Canonical. muken imports the SSH key and creates a security group `muken` that opens ports 22, 80 and 443.
- The server id is `<region>/<instance-id>`, so `server delete` needs no region flag.
- Ubuntu on AWS has no root login. `server create` writes `server.user: ubuntu` to `muken.yaml`.
- Sizes are a fixed list of `t3` types. The EC2 API has no price list.

## Impact
- Specs: none. No spec covers servers at providers yet.
- Code: `internal/provider/aws.go`, `provider.go`, `cmd/muken/cmd_misc.go`, `cmd/muken/main.go`
- Docs: `docs/guide.md`, `docs/development.md`, `README.md`, `CHANGELOG.md`
- Roadmap: none. The client passes against mocks only.
