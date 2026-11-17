# Setup Specification

## Purpose

Prepare a server for muken with `up`, check it with `doctor`, and warn before setup can interrupt anything.

## Requirements

### Requirement: Prepare a new server, skip a prepared one
The system SHALL run every base step on a server that muken has not set up. It SHALL skip the base steps when Docker and the `muken` network exist, and still run the proxy step.

#### Scenario: A new server
- **WHEN** `up` runs on a server without Docker or without the `muken` network
- **THEN** every base step runs

#### Scenario: A prepared server
- **WHEN** `up` runs on a server with Docker and the `muken` network
- **THEN** the packages, Docker, firewall and swap steps do not run
- **AND** the proxy step still applies changes to the proxy settings

Tests: `TestPrepareRunsEverythingOnNewServer`, `TestPrepareSkipsBaseStepsOnPreparedServer`

### Requirement: Restart Docker only after new settings
The system SHALL restart Docker only after it writes `/etc/docker/daemon.json` itself.

#### Scenario: daemon.json exists
- **WHEN** `/etc/docker/daemon.json` exists
- **THEN** the system does not restart Docker

Tests: `TestProvisionRestartsDockerOnlyAfterNewSettings`

### Requirement: Warn before an interruption
The system SHALL warn before setup restarts Docker on a server that runs containers, and before it replaces a running proxy.

#### Scenario: Docker will restart
- **WHEN** Docker is installed, `daemon.json` is missing and containers are running
- **THEN** the system warns and says how many containers stop

#### Scenario: Nothing runs
- **WHEN** no container is running
- **THEN** the system prints no warning

#### Scenario: The proxy will restart
- **WHEN** the proxy is running and its settings changed
- **THEN** the system warns that every site on the server drops connections for a few seconds

#### Scenario: The proxy settings are the same
- **WHEN** the proxy settings did not change
- **THEN** the system prints no warning

Tests: `TestSetupWarnsBeforeRestartingDocker`, `TestSetupDoesNotWarnWithoutRunningContainers`, `TestSetupWarnsWhenTheProxyWillRestart`, `TestSetupDoesNotWarnWhenTheProxySettingsAreTheSame`

### Requirement: Configure the proxy from the config
The system SHALL start Traefik with flags that follow `tls` and `cloudflare`, and SHALL refuse a Cloudflare DNS challenge without a token. The proxy SHALL NOT get the Docker socket.

#### Scenario: Let's Encrypt over HTTP
- **WHEN** `tls` is `letsencrypt`
- **THEN** the proxy uses the HTTP challenge with the address from `email`
- **AND** it publishes port 443 over UDP too
- **AND** it gets no Docker socket

#### Scenario: A Cloudflare challenge without a token
- **WHEN** `tls` is `cloudflare` and `CLOUDFLARE_API_TOKEN` is not set
- **THEN** the setup fails and names the variable

Tests: `TestProvisionTraefikFlags`, `TestProvisionCloudflareNeedsToken`

### Requirement: Keep SSH reachable
The system SHALL allow the SSH port in the firewall before it turns the firewall on.

#### Scenario: The firewall turns on
- **WHEN** the setup enables `ufw`
- **THEN** the rule for `server.port` comes first

Tests: `TestProvisionTraefikFlags`

### Requirement: Tell a new server from a broken one
The system SHALL treat a missing Docker or a missing proxy as a warning on a server that `up` never set up, and a stopped proxy as an error on a server that `up` did set up.

#### Scenario: A new server
- **WHEN** `doctor` runs on a server without Docker
- **THEN** it warns and says to run `muken up`
- **AND** it does not count a failure

#### Scenario: Docker without the muken network
- **WHEN** `doctor` runs on a server with Docker and no `muken` network
- **THEN** it warns that the proxy is not installed and says to run `muken up`
- **AND** it does not count a failure

#### Scenario: A prepared server with a stopped proxy
- **WHEN** the `muken` network exists and the proxy is not running
- **THEN** `doctor` fails and says to run `muken up`

#### Scenario: A running proxy
- **WHEN** the proxy is running
- **THEN** `doctor` reports it as running

Tests: `TestDoctorNewServerIsNotAFailure`, `TestDoctorDockerWithoutProxyIsNotAFailureBeforeFirstUp`, `TestDoctorBrokenProxyFails`, `TestDoctorRunningProxyPasses`
