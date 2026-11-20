# API Specification

## Purpose
Let the PHP web app do what the CLI does, through JSON over loopback HTTP.

## Requirements

### Requirement: Serve on loopback only
The system SHALL add `muken serve`. It SHALL listen on `MUKEN_API_ADDR` (default `127.0.0.1:7600`) and SHALL refuse an address that is not a loopback address. It SHALL read `muken.yaml` from `MUKEN_CONFIG` (default `/work/muken.yaml`).

#### Scenario: A non-loopback address
- **WHEN** `MUKEN_API_ADDR` is `0.0.0.0:7600`
- **THEN** `muken serve` exits with an error that says the API has no login and must stay on loopback

Tests: `TestCheckLoopback`, `TestServeRefusesAPublicAddress`

### Requirement: Read endpoints
The system SHALL answer these with JSON. `plan`, `doctor` and `logs` give the text of the CLI command in `output`. `status` gives the same data as `muken status`: `GET /v1/status`, `GET /v1/plan`, `GET /v1/doctor`, `GET /v1/logs?service=<name>&tail=<n>`, `GET /v1/config`, `GET /v1/version`. `GET /v1/config` SHALL hide every secret value.

#### Scenario: Status
- **WHEN** the client calls `GET /v1/status`
- **THEN** it gets the release, the containers and the maintenance flag that `muken status` shows

Tests: `TestAPIStatusAndText`, `TestAPIConfigHidesSecrets`

### Requirement: Change endpoints are jobs
The system SHALL run `POST /v1/deploy`, `/v1/rollback`, `/v1/scale`, `/v1/maintenance` (body `{"on": true}`) and `/v1/up` as jobs. Each returns `202` and `{"job_id": "..."}` at once. It SHALL run one change job at a time and answer `409` with the running `job_id` to a second one.

#### Scenario: Two deploys
- **WHEN** a deploy runs and the client starts another deploy
- **THEN** the answer is `409` and names the first job

Tests: `TestAPIJobsRunOneAtATime`, `TestAPIBodies`, `TestAPIFailedJob`

### Requirement: Job output
The system SHALL give `GET /v1/jobs/{id}?after=<n>` with `state` (`running`, `ok`, `failed`), `lines` after line `n`, and `next`. It SHALL give the same lines as server-sent events on `GET /v1/jobs/{id}/events`. It SHALL keep the last 50 jobs in memory.

#### Scenario: Polling
- **WHEN** the client polls with `after=10`
- **THEN** it gets the lines from 11 on and the new `next`

Tests: `TestAPIJobsRunOneAtATime`

### Requirement: Read the working folder
The system SHALL give `GET /v1/files?path=<relative path>` for the folder of `muken.yaml`: a list for a folder and the text for a file, read only. It SHALL refuse a path that leaves the folder (including through a symlink), a file over 256 KB, a binary file, and these files: `.muken/age.key`, `.muken/.ssh/`, `.env*`, `*.pem`, `*.key`, `*.secrets.age`, `id_*`. It SHALL NOT list the refused files.

#### Scenario: A private key
- **WHEN** the client asks for `.muken/age.key`
- **THEN** the answer is `403` and the key is not in any listing

#### Scenario: A path outside
- **WHEN** the client asks for `../../etc/passwd`, or a symlink that points outside
- **THEN** the answer is `404` or `403` and no content

Tests: `TestAPIFilesAreReadOnlyAndStayInside`

### Requirement: First setup
The system SHALL give `GET /v1/presets`, `POST /v1/init`, `GET` and `POST /v1/server-key`. `POST /v1/init` SHALL run `muken init` with the fields of the body (preset, app, host, domain, email, image, php, db, workers, cloudflare, agent, and `ssh_key`, the name of a key made by `server-key`). It SHALL check every field against a strict pattern and pass each as one argument `--name=value`. It SHALL answer `409` when `muken.yaml` exists, and SHALL NOT pass `--force`. `server-key` SHALL run `muken server key <name>`, never replace a key, and give the public key only.

#### Scenario: The first muken.yaml
- **WHEN** there is no `muken.yaml` and the client posts a preset, an app name and a host
- **THEN** `muken.yaml` is written, with `server.ssh_key` when a key name was given

#### Scenario: A file exists
- **WHEN** `muken.yaml` exists
- **THEN** `POST /v1/init` answers `409 config_exists` and runs nothing

#### Scenario: A value that looks like a flag
- **WHEN** a field is `--force` or `-oProxyCommand=x`
- **THEN** the answer is `400` and nothing runs

Tests: `TestAPIInitMakesTheFirstConfigOnly`, `TestAPIInitRefusesBadFields`, `TestAPIServerKeyShowsOnlyThePublicKey`, `TestServeBackendInitMakesTheFileAndNeverOverwrites`, `TestInitWritesTheSSHKeyPath`

### Requirement: Errors tell what to do
The system SHALL answer an error as `{"error": {"code": "...", "message": "...", "hint": "..."}}` with the hint that the CLI would print.

#### Scenario: No muken.yaml
- **WHEN** the file does not exist
- **THEN** every endpoint except `GET /v1/version` answers `409` with code `no_config` and a hint to create `muken.yaml`

Tests: `TestAPINoConfigIsConflict`, `TestAPIFailureCarriesTheHint`, `TestServeBackendReportsAnErrorWithItsHint`
