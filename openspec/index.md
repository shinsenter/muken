# Index

Pick the capability, open its spec. Do not search the tree.

| Capability | Spec | Code | Tests |
|---|---|---|---|
| Deploy, rollback, plan | `specs/deploy/spec.md` | `internal/deploy/deploy.go`, `engine.go`, `env.go`, `traefik.go`, `cmd/muken/cmd_ops.go` | `internal/deploy/deploy_test.go` |
| Maintenance | `specs/maintenance/spec.md` | `internal/deploy/agent.go`, `maintenancePage` in `cmd/muken/cmd_ops.go` | `internal/deploy/deploy_test.go`, `cmd/muken/cmd_maintenance_test.go` |
| Setup: `up`, `doctor` | `specs/setup/spec.md` | `internal/deploy/provision.go`, `cmdUp` in `cmd_ops.go`, `cmdDoctor` in `cmd_misc.go` | `internal/deploy/deploy_test.go`, `cmd/muken/cmd_ops_test.go` |
| Keys: age key, SSH key | `specs/keys/spec.md` | `internal/secrets/`, `internal/project/`, `cmd/muken/cmd_key.go` | the same folders, `cmd/muken/cmd_key_test.go` |
| Image | `specs/image/spec.md` | `Dockerfile`, `.github/workflows/image.yml` | smoke test in `image.yml` |

No spec yet. Read the code and one section of the guide.

| Area | Code | Guide section |
|---|---|---|
| Config `muken.yaml` | `internal/config/` | `## muken.yaml` |
| Monitoring agent | `internal/agent/`, `agent*` in `cmd/muken/cmd_misc.go` | `## Monitoring` |
| Cloudflare and DNS | `internal/cloudflare/` | `## Cloudflare` |
| Servers at providers | `internal/provider/` | `### 2. Get a server` |
| Kubernetes, CI file | `internal/k8s/`, `internal/ci/` | `## Kubernetes`, `### Deploy from GitHub Actions` |
| SSH runner | `internal/remote/` | none |
| Console output, prompts | `internal/ui/` | none |
| Presets | `internal/catalog/` | none |

Do not read: `dist/`, `THIRD_PARTY_NOTICES.txt`, `LICENSE`, `NOTICE`. Do not read `docs/guide.md` whole. Run `grep -n '^## ' docs/guide.md` and read one section.
