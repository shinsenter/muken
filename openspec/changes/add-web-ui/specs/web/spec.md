# Web Specification

## Purpose
A browser UI for the functions of muken, in a Laravel app inside the image.

## Requirements

### Requirement: Stack
The system SHALL build the UI with Laravel, Livewire and Tailwind CSS, with Livewire components, Blade views and Vite, as the framework documents them. It SHALL use no login and no user table.

#### Scenario: Open the site
- **WHEN** the browser opens `/`
- **THEN** the status page shows with no login

Tests: PHP, `PagesTest`. Run with `docker build --target web-test`: 11 tests pass.

### Requirement: Pages
The system SHALL have a page for each function: Status (`/`), Deploy with plan (`/deploy`), Rollback, Scale, Maintenance, Up, Doctor, Logs, Config with the working folder (read only) and History. Each page SHALL call muken only through the API in `specs/api`.

#### Scenario: Deploy
- **WHEN** the user opens Deploy
- **THEN** the page shows the plan from `GET /v1/plan` and a button
- **AND** the button starts a job and the page shows its lines as they arrive

Tests: PHP, `PagesTest`. Run with `docker build --target web-test`: 11 tests pass.

### Requirement: Confirm first
The system SHALL ask for a second click on Deploy, Rollback, Scale, Up and Maintenance on, with a panel that names the server and the action.

#### Scenario: Rollback
- **WHEN** the user clicks Rollback
- **THEN** nothing runs until the user confirms in the panel

Tests: PHP, `PagesTest`. Run with `docker build --target web-test`: 11 tests pass.

### Requirement: Config is a file
The system SHALL read settings from the fixed file `/work/muken.yaml`, through the API, and SHALL show the files of the working folder `/work` (read only, through `GET /v1/files`). It SHALL NOT write any file of that folder except the history database. Laravel session, cache and queue SHALL use `file` and `sync` drivers, never the database.

#### Scenario: The working folder
- **WHEN** the user opens Config and a folder
- **THEN** the page lists its files and shows a text file
- **AND** `.muken/age.key` and the SSH key are not listed

#### Scenario: No SQLite file
- **WHEN** the history file does not exist
- **THEN** every page except History works the same

Tests: PHP, `PagesTest::test_every_page_opens_without_the_database`. Run with `docker build --target web-test`: 11 tests pass.

### Requirement: The API is down
The system SHALL show a page that says muken serve does not answer, with the command to check it, and not a server error.

#### Scenario: The service stopped
- **WHEN** a call to `127.0.0.1:7600` fails
- **THEN** the page shows the message and keeps the menu

Tests: PHP, `PagesTest`. Run with `docker build --target web-test`: 11 tests pass.
