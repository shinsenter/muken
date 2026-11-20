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
The system SHALL read settings from the fixed file `/work/muken.yaml`, through the API, and SHALL show the files of the working folder `/work` (read only, through `GET /v1/files`). It SHALL NOT write any file of that folder except the history database, and, through the setup form while `muken.yaml` is missing, `muken.yaml` and an SSH key in `.muken/.ssh`. Laravel session, cache and queue SHALL use `file` and `sync` drivers, never the database.

#### Scenario: The working folder
- **WHEN** the user opens Config and a folder
- **THEN** the page lists its files and shows a text file
- **AND** `.muken/age.key` and the SSH key are not listed

#### Scenario: No SQLite file
- **WHEN** the history file does not exist
- **THEN** every page except History works the same

Tests: PHP, `PagesTest::test_every_page_opens_without_the_database`. Run with `docker build --target web-test`: 11 tests pass.

### Requirement: First setup in the browser
The system SHALL send the first visit to `/setup` when `muken.yaml` is missing. The page SHALL list the presets, take the answers of `muken init`, offer to make an SSH key and show its public key (never the private key), and send them to `POST /v1/init`. It SHALL show no form when `muken.yaml` exists.

#### Scenario: A new working folder
- **WHEN** the user opens `/` and `muken.yaml` is missing
- **THEN** the browser goes to `/setup`
- **AND** after the form is sent the user lands on Status

#### Scenario: The file exists
- **WHEN** the user opens `/setup` and `muken.yaml` exists
- **THEN** the page says so and shows no form

Tests: PHP, `PagesTest::test_the_setup_page_lists_the_presets_when_there_is_no_config`, `PagesTest::test_the_setup_page_never_offers_a_form_when_the_config_exists`, `PagesTest::test_the_setup_form_sends_init_and_goes_to_status`, `PagesTest::test_the_setup_form_checks_the_name_before_it_calls_the_api`, `PagesTest::test_every_page_opens_without_the_database`. Run with `docker build --target web-test`: 15 tests pass.

### Requirement: Footer
The system SHALL show on every page a footer with the version of muken (from `GET /v1/version`) and `Copyright © 2026 Mai Nhut Tan`, `Apache License 2.0`. When the API does not answer, the footer SHALL still show without the version.

#### Scenario: The version
- **WHEN** a page opens and the API answers `v9.9.9`
- **THEN** the footer shows `v9.9.9` and the copyright

Tests: PHP, `PagesTest::test_the_status_page_needs_no_login`, `PagesTest::test_a_stopped_api_shows_a_message_and_keeps_the_menu`. Run with `docker build --target web-test`: 11 tests pass.

### Requirement: The API is down
The system SHALL show a page that says muken serve does not answer, with the command to check it, and not a server error.

#### Scenario: The service stopped
- **WHEN** a call to `127.0.0.1:7600` fails
- **THEN** the page shows the message and keeps the menu

Tests: PHP, `PagesTest`. Run with `docker build --target web-test`: 11 tests pass.
