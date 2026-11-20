# Web Specification

## Purpose
A browser UI for the functions of muken, in a Laravel app inside the image.

## Requirements

### Requirement: Stack
The system SHALL build the UI with Laravel, Livewire and Tailwind CSS, with Livewire components, Blade views and Vite, as the framework documents them. It SHALL use no login and no user table.

#### Scenario: Open the site
- **WHEN** the browser opens `/`
- **THEN** the status page shows with no login

Tests: none yet

### Requirement: Pages
The system SHALL have a page for each function: Status (`/`), Deploy with plan (`/deploy`), Rollback, Scale, Maintenance, Up, Doctor, Logs, Config (read only) and History. Each page SHALL call muken only through the API in `specs/api`.

#### Scenario: Deploy
- **WHEN** the user opens Deploy
- **THEN** the page shows the plan from `GET /v1/plan` and a button
- **AND** the button starts a job and the page shows its lines as they arrive

Tests: none yet

### Requirement: Confirm first
The system SHALL ask for a second click on Deploy, Rollback, Scale, Up and Maintenance on, with a panel that names the server and the action.

#### Scenario: Rollback
- **WHEN** the user clicks Rollback
- **THEN** nothing runs until the user confirms in the panel

Tests: none yet

### Requirement: Config is a file
The system SHALL read settings from the fixed file `/work/muken.yaml`, through the API. It SHALL NOT edit that file. Laravel session, cache and queue SHALL use `file` and `sync` drivers, never the database.

#### Scenario: No SQLite file
- **WHEN** the history file does not exist
- **THEN** every page except History works the same

Tests: none yet

### Requirement: The API is down
The system SHALL show a page that says muken serve does not answer, with the command to check it, and not a server error.

#### Scenario: The service stopped
- **WHEN** a call to `127.0.0.1:7600` fails
- **THEN** the page shows the message and keeps the menu

Tests: none yet
