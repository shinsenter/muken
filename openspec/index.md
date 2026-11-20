# Index

Pick the capability, open its spec. Do not search the tree.

| Capability | Spec | Code | Tests |
|---|---|---|---|
| Deploy, rollback, plan | `specs/deploy/spec.md` | `src/internal/deploy/deploy.go`, `engine.go`, `env.go`, `traefik.go`, `src/cmd/muken/cmd_ops.go` | `src/internal/deploy/deploy_test.go` |
| Maintenance | `specs/maintenance/spec.md` | `src/internal/deploy/agent.go`, `maintenancePage` in `src/cmd/muken/cmd_ops.go` | `src/internal/deploy/deploy_test.go`, `src/cmd/muken/cmd_maintenance_test.go` |
| Setup: `up`, `doctor` | `specs/setup/spec.md` | `src/internal/deploy/provision.go`, `cmdUp` in `cmd_ops.go`, `cmdDoctor` in `cmd_misc.go` | `src/internal/deploy/deploy_test.go`, `src/cmd/muken/cmd_ops_test.go` |
| Keys: age key, SSH key | `specs/keys/spec.md` | `src/internal/secrets/`, `src/internal/project/`, `src/cmd/muken/cmd_key.go` | the same folders, `src/cmd/muken/cmd_key_test.go` |
| Image | `specs/image/spec.md` | `src/Dockerfile`, `.github/workflows/image.yml` | smoke test in `image.yml` |

No spec yet. Read the code and one section of the guide.

| Area | Code | Guide section |
|---|---|---|
| Config `muken.yaml` | `src/internal/config/` | `## muken.yaml` |
| Monitoring agent | `src/internal/agent/`, `agent*` in `src/cmd/muken/cmd_misc.go` | `## Monitoring` |
| Cloudflare and DNS | `src/internal/cloudflare/` | `## Cloudflare` |
| Servers at providers | `src/internal/provider/` | `### 2. Get a server` |
| Kubernetes, CI file | `src/internal/k8s/`, `src/internal/ci/` | `## Kubernetes`, `### Deploy from GitHub Actions` |
| SSH runner | `src/internal/remote/` | none |
| Console output, prompts | `src/internal/ui/` | none |
| Presets | `src/internal/catalog/` | none |

Do not read: `dist/`, `THIRD_PARTY_NOTICES.txt`, `LICENSE`, `NOTICE`. Do not read `docs/guide.md` whole. Run `grep -n '^## ' docs/guide.md` and read one section.
