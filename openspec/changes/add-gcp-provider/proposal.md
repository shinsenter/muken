# add-gcp-provider

## Why
`muken server create` has no Google Cloud option. Compute Engine needs an OAuth token from a service account key, so it does not fit the shared Bearer-token client alone.

## What changes
- `--provider gcp` creates and deletes Compute Engine instances. `GOOGLE_APPLICATION_CREDENTIALS` is the path of a service account key file. `GOOGLE_CLOUD_PROJECT` sets the project and falls back to `project_id` in that file.
- The client signs the JWT and gets the token itself. No SDK, so `go.mod` does not change.
- The "region" the user picks is a zone, such as `asia-southeast1-b`. The server runs the newest Ubuntu 24.04 image (`ubuntu-2404-lts-amd64` in `ubuntu-os-cloud`).
- muken creates a firewall rule `muken-allow` for tag `muken` that opens ports 22, 80 and 443 on the `default` network.
- The server id is `<zone>/<name>`. `server create` writes `server.user: ubuntu` to `muken.yaml`.
- Sizes are a fixed list of `e2` types.

## Impact
- Specs: none. No spec covers servers at providers yet.
- Code: `src/internal/provider/gcp.go`, `provider.go`, `src/cmd/muken/cmd_misc.go`, `src/cmd/muken/main.go`
- Docs: `docs/guide.md`, `docs/development.md`, `README.md`, `CHANGELOG.md`
- Roadmap: none. The client passes against mocks only.
