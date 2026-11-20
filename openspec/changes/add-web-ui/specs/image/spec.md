# Image Specification (delta)

## MODIFIED Requirements

### Requirement: A small image with SSH
The system SHALL keep the target `cli`: `alpine` with `openssh-client` and `ca-certificates`, `muken` as the entry point, no Docker CLI and no Docker daemon. It SHALL be published with `-cli` tags.

#### Scenario: The cli image runs muken
- **WHEN** the user runs the `cli` image with `version`
- **THEN** it prints the muken version

Tests: smoke test in `.github/workflows/image.yml`

### Requirement: Use the newest base
The system SHALL build the `cli` target on `alpine:latest` and the `web` target on the alpine variant of the newest `shinsenter/laravel` tag, and run `apk upgrade` in both.

#### Scenario: A build
- **WHEN** CI builds the image
- **THEN** the bases and their packages are the newest at that time

Tests: none yet

## ADDED Requirements

### Requirement: Tags of the two images
The system SHALL push `web` for `linux/amd64` and `linux/arm64` on a `v*` tag with `<version>`, `<minor>` and `latest`, and `cli` with `<version>-cli`, `<minor>-cli` and `cli`. A tag with a dash gets only its own tag (`<version>` or `<version>-cli`). It SHALL sign both with cosign.

#### Scenario: A release tag
- **WHEN** the tag `v0.1.0` is pushed
- **THEN** `web` gets `0.1.0`, `0.1`, `latest` and `cli` gets `0.1.0-cli`, `0.1-cli`, `cli`

Tests: publish job in `.github/workflows/image.yml`. Not run yet.

### Requirement: A web image
The system SHALL build a target `web` on `shinsenter/laravel` (alpine variant) that holds `/usr/local/bin/muken`, `openssh-client`, `ca-certificates` and the Laravel app of `web/` in `/var/www/html`. It SHALL set `ENABLE_SSHD=1`. It SHALL NOT contain the Docker CLI or a Docker daemon. It SHALL NOT publish a port by itself.

#### Scenario: The image serves the site and runs muken
- **WHEN** the container starts with `-v "$PWD":/work`
- **THEN** Nginx and PHP-FPM serve the app on port 80
- **AND** `muken serve` listens on `127.0.0.1:7600`
- **AND** `docker exec <container> muken version` prints the version

#### Scenario: SSH is on and needs a key
- **WHEN** the container starts without `SSHD_AUTHORIZED_KEYS`
- **THEN** sshd runs and nobody can log in, because password login is off

Tests: smoke test in `.github/workflows/image.yml`. By hand on 2026-11-20: the image builds, Nginx, PHP-FPM, sshd and `muken serve` start, key login over SSH works. The workflow itself has not run.

### Requirement: Build the web assets in the image
The system SHALL build the Tailwind CSS and JavaScript with Vite in a build stage and ship only the result. The image SHALL load no asset from a CDN.

#### Scenario: A page loads offline
- **WHEN** the browser has no internet access
- **THEN** the page still shows its styles and scripts

Tests: none yet
