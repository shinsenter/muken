# Design

## One process model
The base image starts s6. A post-boot hook enables a service `muken-api` (`muken serve`) next to Nginx and PHP-FPM. PHP calls `http://127.0.0.1:7600`. Nginx never proxies to it, so the API is not reachable from outside the container.

## Why an HTTP API and not `exec muken`
- A deploy takes minutes and streams text. PHP-FPM requests time out, and `exec` ties a worker to the run.
- One process can hold the deploy lock and refuse a second run (`409`). Two `exec` calls cannot see each other.
- The API is testable without PHP, with `remote.Fake`.

## The web is an overlay
`web/` holds only the files that differ from a new Laravel project. The image build runs `composer create-project laravel/laravel`, adds Livewire, copies `web/` on top, deletes the default migrations and builds the assets. This keeps the repo small and the framework current. Cost: the app cannot run from the repo alone. `--target web-test` runs the PHP tests.

## Jobs
A change (`deploy`, `rollback`, `scale`, `maintenance`, `up`) starts a job and returns `202` with `job_id` at once. Output is lines with a number. The web polls `GET /v1/jobs/{id}?after=<n>` every second (Livewire `wire:poll`). Server-sent events are in the API too, for other clients. Jobs live in memory, the last 50. The durable record is the history table.

## What stays outside SQLite
Laravel defaults sessions, cache and queue to the database. This app sets `SESSION_DRIVER=file`, `CACHE_STORE=file`, `QUEUE_CONNECTION=sync`. Then SQLite holds one table and nothing else needs it to exist.

## History file
Default path `/work/.muken/history.sqlite`. `.muken/` already holds the keys, has its own `.gitignore`, and is carried by `-v "$PWD":/work`.
Self-heal in `HistoryStore`: missing directory or file, create and migrate. A file that is not a SQLite database, rename to `history.sqlite.bad-<unix time>` and start again. Every method catches `Throwable`, logs a warning and returns an empty value.

## Confirmation without login
Without a login, a click is the only guard. Rollback, deploy, up and maintenance on need a second step: a confirm panel that names the server and the action.

## Open
- The exact `docker run` form that runs `muken` next to the base entrypoint. Unverified, see `openspec/upstream.md`.
- Whether `artisan migrate` creates a missing SQLite file in non-interactive mode. Unverified. The spec does not rely on it: the app creates the file itself.
