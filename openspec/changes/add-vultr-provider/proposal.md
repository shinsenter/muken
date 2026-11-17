# add-vultr-provider

## Why
`muken server create` works at DigitalOcean and Hetzner only. Vultr is a common third choice with a simple API.

## What changes
- `--provider vultr` creates, deletes and lists regions and sizes at Vultr. The token is read from `VULTR_API_KEY`.
- The server runs Ubuntu 24.04, like the other providers.
- The SSH key is found by its content, or uploaded when missing.

## Impact
- Specs: none. No spec covers servers at providers yet.
- Code: `internal/provider/vultr.go`, `provider.go`, `cmd/muken/main.go`, `cmd/muken/cmd_misc.go`
- Docs: `docs/guide.md`, `docs/development.md`, `README.md`, `CHANGELOG.md`
- Roadmap: none. The client passes against mocks only, like Hetzner.
