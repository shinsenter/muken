# Maintenance Specification

## Purpose

Show visitors a maintenance page while the app keeps running, and let the user decide when it ends.

## Requirements

### Requirement: Answer with HTTP 503
The system SHALL answer every request to the app's domains with HTTP 503 and an HTML page when the user runs `maintenance on`. The web and worker containers SHALL keep running.

#### Scenario: Turn it on and off
- **WHEN** the user runs `maintenance on`
- **THEN** the route points at the `muken-maintenance` container
- **WHEN** the user runs `maintenance off`
- **THEN** the route points at the live release again
- **AND** the maintenance container is removed

Tests: `TestMaintenanceOnAndOff`

### Requirement: Show the user's page
WHEN `maintenance.page` is set, the system SHALL upload that file and show it. Without it the system SHALL show the built-in page.

#### Scenario: A custom page
- **WHEN** `maintenance.page` names an HTML file from 1 byte to 64 KB
- **THEN** the system uploads it to the server and mounts it in the maintenance container
- **AND** the status stays 503

#### Scenario: The path is relative
- **WHEN** `maintenance.page` is a relative path
- **THEN** the system resolves it against the folder of the config file

#### Scenario: The file is empty or too large
- **WHEN** the file is empty or larger than 64 KB
- **THEN** the command fails before it changes the server

#### Scenario: No page is set
- **WHEN** `maintenance.page` is empty
- **THEN** the system shows the built-in page that says "Back soon"

Tests: `TestMaintenancePageIsUploadedAndMounted`, `TestMaintenanceDefaultPage`, `TestMaintenancePageFile`, `TestMaintenancePageSizeLimits`, `TestMaintenancePageKey`

### Requirement: Deploy and rollback end maintenance by default
The system SHALL turn maintenance off after `deploy` or `rollback` moves traffic. `--keep-maintenance` SHALL leave visitors on the maintenance page.

#### Scenario: Deploy without the flag
- **WHEN** maintenance is on and the user runs `deploy`
- **THEN** traffic moves to the new release
- **AND** the maintenance container is removed after the switch

#### Scenario: Deploy with the flag
- **WHEN** maintenance is on and the user runs `deploy --keep-maintenance`
- **THEN** the new release starts and is saved as the current one
- **AND** the route does not change, the smoke check is skipped and the maintenance container stays

#### Scenario: Rollback with and without the flag
- **WHEN** maintenance is on and the user runs `rollback`
- **THEN** traffic moves to the previous release and the maintenance container is removed
- **WHEN** the user adds `--keep-maintenance`
- **THEN** the route and the maintenance container stay as they are

#### Scenario: Maintenance is off
- **WHEN** maintenance is off and the user passes `--keep-maintenance`
- **THEN** the deploy behaves as usual

Tests: `TestDeployTurnsMaintenanceOff`, `TestDeployKeepsMaintenanceWhenAsked`, `TestKeepMaintenanceDoesNothingWhenMaintenanceIsOff`, `TestRollbackTurnsMaintenanceOff`, `TestRollbackKeepsMaintenanceWhenAsked`

### Requirement: Other deploys end maintenance
The system SHALL turn maintenance off after `up` and `scale`, which deploy without `--keep-maintenance`.

#### Scenario: `up` or `scale` while maintenance is on
- **WHEN** maintenance is on and the user runs `up` or `scale`
- **THEN** maintenance is off after the deploy

Tests: none yet

### Requirement: Show the state
The system SHALL show whether maintenance is on in `status`, and mention it in `deploy --plan`.

#### Scenario: Status and plan
- **WHEN** maintenance is on
- **THEN** `status` says it is on
- **AND** `deploy --plan` says whether it will turn maintenance off or keep it
- **WHEN** maintenance is off
- **THEN** the plan does not mention maintenance

Tests: `TestStatusShowsMaintenance`, `TestPlanMentionsMaintenance`
