# Deploy Specification

## Purpose

Move a new release of the app live with no gap, and go back when a check fails.

## Requirements

### Requirement: Switch only after the health check
The system SHALL start the new release next to the old one and move traffic only after every new web container answers its health URL.

#### Scenario: A health check fails
- **WHEN** a new web container does not become healthy
- **THEN** the system removes the new containers
- **AND** the old release keeps serving

#### Scenario: A container exits at once
- **WHEN** a new container stops right after it starts
- **THEN** the deploy fails without waiting for the full timeout
- **AND** the error does not carry the container logs, which print before it

Tests: `TestHealthFailureLeavesOldReleaseAlone`, `TestCrashedContainerFailsFast`, `TestHealthFailureKeepsLogsOutOfTheError`

### Requirement: Migrate before the switch
The system SHALL run `web.migrate` in a throwaway container before it starts the new containers.

#### Scenario: The migration fails
- **WHEN** the migration command fails
- **THEN** no new container starts
- **AND** the old release keeps serving

#### Scenario: The user skips the migration
- **WHEN** the user runs `deploy --no-migrate`
- **THEN** the system does not run the migration

Tests: `TestMigrationFailureStopsBeforeStartingAnything`, `TestSkipMigrate`

### Requirement: Check the app after the switch
WHEN `web.smoke_path` is set, the system SHALL request it through the proxy after the switch.

#### Scenario: The smoke check fails
- **WHEN** the request fails
- **THEN** the system puts the previous route back
- **AND** the old release serves again

Tests: `TestSmokeFailureRestoresRoute`

### Requirement: One deploy at a time
The system SHALL hold a lock on the server for the whole deploy.

#### Scenario: A deploy is running
- **WHEN** a second deploy starts
- **THEN** it fails and says another deploy is running

#### Scenario: A deploy fails
- **WHEN** a deploy fails
- **THEN** the system releases the lock

Tests: `TestLockBlocksSecondDeploy`, `TestLockIsReleasedAfterFailure`

### Requirement: Keep the previous release
The system SHALL stop the old release after the switch and keep its containers. It SHALL remove releases older than `keep`. It SHALL record the previous release and image in the state.

#### Scenario: A second deploy
- **WHEN** a deploy succeeds with `keep: 2` and two older releases exist
- **THEN** every container of the old release is stopped, and the new release is not
- **AND** the containers of the release before it are removed
- **AND** the state names the previous release and its image

Tests: `TestSecondDeployStopsOldAndRecordsPrevious`

### Requirement: Roll back
The system SHALL start the previous release, check its health, move traffic to it and stop the failed release.

#### Scenario: A previous release exists
- **WHEN** the user runs `rollback`
- **THEN** the previous containers start and the route points at them
- **AND** the state swaps the current and the previous release

#### Scenario: There is no previous release
- **WHEN** the user runs `rollback` after a first deploy
- **THEN** the command fails and says to deploy at least twice

#### Scenario: The previous containers were pruned
- **WHEN** the user runs `rollback`
- **THEN** the command fails and names the `muken deploy --image <old image>` command to run

Tests: `TestRollback`, `TestRollbackWithoutHistory`, `TestRollbackAfterPrune`

### Requirement: Show the plan without acting
The system SHALL describe what a deploy would do when the user runs `deploy --plan`, and change nothing.

#### Scenario: A plan
- **WHEN** the user runs `deploy --plan`
- **THEN** the output shows the image change, the migration, the containers to start and the release kept for rollback

#### Scenario: No domain is set
- **WHEN** the config has no domain
- **THEN** the traffic line says any host name reaches the app, without an empty name

Tests: `TestPlanDescribesChange`, `TestPlanWithoutDomainsDoesNotPrintAnEmptyName`

### Requirement: Check the server first
The system SHALL stop before it changes anything when Docker, the `muken` network or the proxy is missing.

#### Scenario: The server is not set up
- **WHEN** a deploy starts on a server without Docker, the network or the proxy
- **THEN** it fails with a message that says to run `muken up`

Tests: `TestPreflightMessages` covers the missing Docker case only

### Requirement: Route every replica and domain
The system SHALL write one route file for the app that lists every web replica and every domain.

#### Scenario: Two replicas and two domains
- **WHEN** the app has two replicas and two domains
- **THEN** the route lists both containers and both domains

#### Scenario: No TLS
- **WHEN** `tls` is `none`
- **THEN** the route uses the plain HTTP entry point

Tests: `TestRouteListsAllReplicasAndDomains`, `TestRenderRouteWithoutTLS`

### Requirement: Build the environment file from all sources
The system SHALL merge accessory variables, defaults, config values and secrets into the environment file. Later sources win.

#### Scenario: Secrets and accessories
- **WHEN** the app has secrets and a database accessory
- **THEN** the file holds the secrets, the database URL, the release name and `TRUSTED_PROXIES`

Tests: `TestEnvFileMergesSecretsAndAccessories`
