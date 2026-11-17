# Image Specification

## Purpose

Ship muken as a small Docker image for people who do not want to install a binary.

## Requirements

### Requirement: A small image with SSH
The system SHALL build the image `cli` on `alpine` with `openssh-client` and `ca-certificates`, with `muken` as the entry point. The image SHALL NOT contain the Docker CLI or a Docker daemon.

#### Scenario: The image runs muken
- **WHEN** the user runs the image with `version`
- **THEN** it prints the muken version

#### Scenario: The image can use SSH and HTTPS
- **WHEN** a command in the image runs `ssh -V`
- **THEN** it prints the OpenSSH version
- **AND** `/etc/ssl/certs/ca-certificates.crt` exists, so API calls over HTTPS verify certificates

Tests: smoke test in `.github/workflows/image.yml`

### Requirement: Use the newest base
The system SHALL build on `alpine:latest` and run `apk upgrade` at build time.

#### Scenario: A build
- **WHEN** CI builds the image
- **THEN** the base and its packages are the newest at that time

Tests: none yet

### Requirement: Scan before publish
The system SHALL fail the build when Trivy finds a HIGH or CRITICAL vulnerability that has a fix.

#### Scenario: A fixable vulnerability
- **WHEN** the scan finds a fixable HIGH or CRITICAL vulnerability
- **THEN** the `image` workflow fails and nothing is pushed

Tests: Trivy step in `.github/workflows/image.yml`

### Requirement: Publish signed images for a tag
The system SHALL push the image for `linux/amd64` and `linux/arm64` on a `v*` tag, with the tags `<version>`, `<minor>` and `latest`, and sign it with cosign. A tag with a dash, such as `v0.1.0-rc1`, SHALL get only its own tag.

#### Scenario: A release tag
- **WHEN** the tag `v0.1.0` is pushed
- **THEN** the image gets the tags `0.1.0`, `0.1` and `latest`

#### Scenario: A pre-release tag
- **WHEN** the tag `v0.1.0-rc1` is pushed
- **THEN** the image gets only the tag `0.1.0-rc1`

Tests: publish job in `.github/workflows/image.yml`. Not run yet: no tag has been pushed.
