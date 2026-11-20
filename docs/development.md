# muken, for developers

This page is for the person who maintains muken. It assumes you know PHP well and are learning Go. The user guide is [guide.md](guide.md).

muken is a command-line program. It reads `muken.yaml`, connects to a server over SSH and runs shell commands there: it installs Docker and Traefik, starts the new version of your app next to the old one, switches traffic when the new one is healthy, and goes back when it is not. Almost everything it does on the server is a shell command that the code builds as a string.

## Work on it

You need Go 1.26 or newer. Run `make` from the root of the repository. Run `go` commands inside `src/`, where `go.mod` is.

```shell
cd src                                 # go.mod is here
go test ./...                          # all tests, offline, a few seconds
make check                             # vet, tests and gofmt in one command. It prints only problems
make check-ci                          # the same with no home and no ssh-agent, like CI
go test ./internal/deploy -run Rollback -v   # one test, with its output
go vet ./...                           # the checks the compiler does not do
gofmt -l .                             # lists files that are not formatted. gofmt -w . fixes them
go run golang.org/x/vuln/cmd/govulncheck@latest ./...   # known vulnerabilities in the code and the dependencies
go run ./cmd/muken help                # run it without building
make dist                              # (from the root) binaries for linux and macOS in dist/
```

The tests never touch a real server, Docker or the network.

## Where things are

| Folder | What it does | In PHP terms |
|---|---|---|
| `src/cmd/muken/` | The commands (`init`, `deploy`, `status`, ...). `main.go` has the list of commands | The controllers, plus `index.php` |
| `src/internal/config/` | Reads `muken.yaml`, fills defaults, checks it and explains each mistake | A form request with validation rules |
| `src/internal/catalog/` | The images and apps muken knows (WordPress, Laravel, ...) with their defaults | A config file with a lookup function |
| `src/internal/deploy/` | The part that matters: `Provision`, `Plan`, `Deploy`, `Rollback`, `Maintenance` | The service class |
| `src/internal/remote/` | Runs a command on the server over SSH. Also a fake one for tests | An interface with two implementations |
| `src/internal/provider/` | Creates servers at DigitalOcean, Hetzner, Vultr, AWS and Google Cloud | An API client per vendor |
| `src/internal/cloudflare/` | DNS records, SSL mode, cache purge | An API client |
| `src/internal/secrets/` | Keeps secrets encrypted in the repository (age) | A small vault |
| `src/internal/project/` | The `.muken` folder next to `muken.yaml` that holds the keys, with its own `.gitignore` | A small helper |
| `src/internal/agent/` | The monitoring sidecar: reads Docker, shows a page, sends alerts | A separate small app |
| `src/internal/k8s/`, `src/internal/ci/` | Print Kubernetes manifests and a GitHub Actions file | Template renderers |
| `src/internal/ui/` | Prints progress, sections and rows, framed prompts with arrow keys, formats errors with a hint. The colors are in `theme.go` | A console helper |
| `src/legal/` and the root texts | The license, notice, name policy and third-party licenses, compiled into the binary (`muken version --license`) | `resources/` files read at start |
| `src/Dockerfile` | The Docker image `cli` (`docker build -f src/Dockerfile --target cli .`) | The Dockerfile of a PHP app |
| `.github/workflows/` | `ci` (test and scan), `release` (binaries), `image` (Docker image), `cleanup` (deletes old workflow runs every week) | The pipeline files |
| `openspec/` | The specs of each capability, the changes in progress and the roadmap to 1.0. `AGENTS.md` points here | A requirements folder next to the code |
| `src/scripts/` | `third-party-notices.sh` collects the licenses of the dependencies | A build helper |

Each folder is one Go package. A package is a namespace: the files in it share names, and the folder name is the name you use when you call it (`config.Load`, `deploy.New`).

## One deploy, step by step

`Engine.Deploy` in `src/internal/deploy/deploy.go` is the best place to start reading.

1. Take a lock on the server (a folder that only one process can create).
2. Check that Docker, the `muken` network and the proxy exist.
3. Create the databases that do not exist yet. Write the environment file.
4. Pull the image. Run the migration in a throwaway container.
5. Start the new containers and wait until their health URL answers.
6. Write the Traefik route file. Traefik reloads it, and traffic moves. With `--keep-maintenance` and maintenance on, this step and the next one are skipped.
7. Optional smoke check through the proxy (`web.smoke_path`). If it fails, the old route comes back.
8. If maintenance was on, remove the maintenance container.
9. Wait, then stop the old containers (they stay, stopped, for `rollback`).
10. Write the state file and the history line.

If a step before 6 fails, the new containers are removed and the old release never stopped serving. Test names in `deploy_test.go` follow these steps.

## Go for a PHP developer

| PHP | Go |
|---|---|
| `namespace App\Foo` and `use` | `package foo` and `import` |
| `composer.json` | `src/go.mod` (and `src/go.sum`, like `composer.lock`) |
| A class with methods | A `struct` and functions with a receiver: `func (e *Engine) Deploy(...)` |
| `private` and `public` | A lowercase name is private to the package. A Capital name is public |
| An interface you `implements` | An interface is satisfied by any type that has the methods. `remote.Runner` is one: `SSH` and `Fake` both fit |
| `throw` and `catch` | A function returns an `error` as its last value. You check it: `if err != nil { return err }` |
| `null` | `nil` for pointers, maps, slices and errors. Other types have a zero value (`""`, `0`, `false`) |
| An array | A slice (`[]string`) for a list, a map (`map[string]string`) for key and value |
| `try { } finally { }` | `defer cleanup()` runs when the function ends |
| `foreach ($a as $k => $v)` | `for k, v := range a` |
| `sprintf` | `fmt.Sprintf`, with `%s`, `%d`, `%v` |
| `var_dump($x)` | `fmt.Printf("%+v\n", x)` |
| PHPUnit | `go test`. A test is a function `TestSomething(t *testing.T)` in a `_test.go` file |

Three habits you will see all the time:

- Errors are values. A function that can fail returns `(result, error)`. The caller decides what to do.
- No exceptions and no magic. If something happens, a line of code did it.
- Small structs and plain functions instead of deep class trees.

Good places to learn: the [Tour of Go](https://go.dev/tour/) (two hours) and [Go by Example](https://gobyexample.com).

## How the tests work

Nothing real is called. `remote.Fake` is a stand-in for the SSH connection: it records every command it receives and answers from a list of rules.

```go
f := (&remote.Fake{}).On("docker pull", "", errors.New("denied"))
```

means: when a command contains `docker pull`, fail with `denied`. After the call, `f.Index("docker pull")` tells where in the order a command ran and `f.Count(...)` how often. That is how the tests prove that the new containers start before traffic moves, and that a failed health check removes them.

The cloud clients (`provider`, `cloudflare`) are tested against a small local HTTP server from `net/http/httptest`.

A test must not depend on the machine that runs it. Do not read the real `~/.ssh`, the ssh-agent or your environment. Use `t.TempDir()` and `t.Setenv(...)`. `isolateSSH` in `src/internal/remote/remote_test.go` is the example. CI runs on a runner with no SSH key, so a test that quietly used your own key passes on your laptop and fails there. To check, run the tests with an empty home:

```shell
env HOME=$(mktemp -d) SSH_AUTH_SOCK= GOCACHE=$(go env GOCACHE) GOMODCACHE=$(go env GOMODCACHE) go test ./...
```

## Recipes

Add a preset (an image or app): add one entry in `src/internal/catalog/catalog.go`. A test checks that every folder in `src/webapps/` has a preset.

Add a setting to `muken.yaml`: add the field to the struct in `src/internal/config/config.go` (with a `yaml:"name"` tag), give it a default in `ApplyDefaults`, check it in `Validate`, use it, add a test, and describe it in `docs/guide.md` and `CHANGELOG.md`. An unknown key in the file is an error on purpose, so users see typos.

Add a command: first check that a flag or a subcommand of an existing command does not fit. The list stays short on purpose (`deploy --plan`, `init --list`, `server key`, `render ci`, `version --license`). If a new command is needed, write a function in `src/cmd/muken/` that takes `(ctx, printer, args)` and returns an `error`, then add it to the `commands` map in `main.go`. The help text comes from that map.

Add a cloud provider: implement the `Provider` interface in `src/internal/provider/`, add its name to `provider.New`, and copy the test of the DigitalOcean client.

## The license inside the binary

`LICENSE`, `NOTICE`, `TRADEMARK.md` and `THIRD_PARTY_NOTICES.txt` at the root are the source. `go:embed` cannot read a parent folder, so `make sync-legal` copies them to `src/legal/`, where `legal.go` compiles them into the binary. `muken version --license` prints them. `TestCopiesMatchTheRootFiles` fails when a copy differs.

When you add or update a dependency, run `src/scripts/third-party-notices.sh` and commit the new `THIRD_PARTY_NOTICES.txt` (the script also updates the copy in `src/legal/`). A test (`TestEveryModuleHasItsNotice`) fails when a module that goes into a release binary has no entry.

## Rules this code follows

- Tell the user what to do next. A user-facing error comes from `ui.Errf(hint, ...)`, with the fix in the hint. Tests and docs quote these messages, so change them on purpose.
- Anything that changes the server goes through `remote.Runner`, so a test can watch it.
- A new dependency needs a reason. Today there are four: SSH (`x/crypto`), YAML, age and `x/term` (raw keyboard mode and terminal width for the prompts).
- A command that can interrupt a running app warns before it acts, in its output and in the guide section "When an interruption is possible".
- Keep tests offline.
- Every Go file starts with the two-line header (`Copyright 2026 Mai Nhut Tan` and the SPDX line `Apache-2.0`).
- Contributions come with the agreement in `CONTRIBUTING.md`, so the license can change later.
