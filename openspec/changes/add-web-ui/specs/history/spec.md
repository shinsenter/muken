# History Specification

## Purpose
Keep a list of what people did in the web UI, and survive the loss of the file.

## Requirements

### Requirement: One table in SQLite
The system SHALL store history in the SQLite file `MUKEN_HISTORY_DB` (default `/work/.muken/history.sqlite`), in one table `operations`: `id`, `action`, `args` (JSON), `state`, `job_id`, `started_at`, `finished_at`, `output_tail` (last 200 lines). It SHALL write a row when a job starts and update it when the job ends.

#### Scenario: A deploy
- **WHEN** the user runs a deploy from the web
- **THEN** a row appears with `state` `running`, and becomes `ok` or `failed` at the end

Tests: none yet

### Requirement: The file may be missing
The system SHALL create the directory and the file, and run the migration, when the file is missing, at boot and again on the first access after a deletion while the app runs. The new history SHALL be empty.

#### Scenario: Delete the file
- **WHEN** the file is deleted and the user opens History
- **THEN** the page shows an empty list, with no error

Tests: none yet

### Requirement: A bad file is replaced
The system SHALL rename a file that is not a valid SQLite database to `history.sqlite.bad-<unix time>` and start a new one.

#### Scenario: A corrupt file
- **WHEN** the file holds text that is not SQLite
- **THEN** it is renamed and the history starts empty

Tests: none yet

### Requirement: A DB error never breaks the app
The system SHALL catch every database error in the history code, write a warning to the log, and return an empty result. A failed write SHALL NOT stop, fail or hide an operation.

#### Scenario: The disk is read-only
- **WHEN** the history write fails during a deploy
- **THEN** the deploy runs to the end and the page shows its output

Tests: none yet
