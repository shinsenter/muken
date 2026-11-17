# Keys Specification

## Purpose

Keep the keys of a project in one folder next to `muken.yaml`, out of git, and never lose a key by accident.

## Requirements

### Requirement: Keep keys in `.muken`
The system SHALL keep the keys of a project in `.muken/` next to `muken.yaml`, with a `.gitignore` that ignores everything in it.

#### Scenario: The folder is created
- **WHEN** a command creates a key
- **THEN** `.muken/` exists with mode 0700
- **AND** `.muken/.gitignore` holds `*`

#### Scenario: The user has a `.gitignore` already
- **WHEN** `.muken/.gitignore` exists
- **THEN** the system does not rewrite it

Tests: `TestEnsureDirKeepsTheFolderOutOfGit`, `TestEnsureDirKeepsAnExistingGitignore`

### Requirement: Create the age key in the project
The system SHALL create the secrets key at `.muken/age.key` with mode 0600 when the user runs `secrets init`.

#### Scenario: A new project
- **WHEN** the user runs `secrets init` in a project with no key
- **THEN** the key is at `.muken/age.key`
- **AND** `secrets set` and `secrets list` work with it

Tests: `TestInitCreatesTheKeyInTheProject`, `TestRoundTripAndFileIsEncrypted`

### Requirement: Never overwrite a key
The system SHALL stop when `secrets init` would overwrite a key, and when a new key would hide the key that an existing secrets file needs.

#### Scenario: The project key exists
- **WHEN** `.muken/age.key` exists and the user runs `secrets init`
- **THEN** the command fails and says it does not overwrite the key
- **AND** the key file is unchanged

#### Scenario: A new key would hide the old one
- **WHEN** the secrets file exists, the only key is in `~/.config/muken/age.key` and the user runs `secrets init`
- **THEN** the command fails, names the old key and says to copy it to `.muken/age.key`
- **AND** no new key is created

#### Scenario: A new project next to an old key
- **WHEN** no secrets file exists and the user runs `secrets init`
- **THEN** the system creates a project key

Tests: `TestInitWarnsInsteadOfOverwritingTheProjectKey`, `TestInitRefusesToShadowTheKeyOfAnExistingSecretsFile`, `TestInitWithoutSecretsFileMayCreateAProjectKeyNextToAnOldOne`

### Requirement: Find the key in a fixed order
The system SHALL look for the age key in this order: `MUKEN_AGE_KEY` (the key text), the file in `MUKEN_AGE_KEY_FILE`, `.muken/age.key`, then `~/.config/muken/age.key`.

#### Scenario: The file variable wins
- **WHEN** `MUKEN_AGE_KEY_FILE` is set and a project key exists
- **THEN** the system uses the file from the variable

#### Scenario: The old key is the fallback
- **WHEN** the project has no key and `~/.config/muken/age.key` exists
- **THEN** the system uses the old key

#### Scenario: CI passes the key text
- **WHEN** `MUKEN_AGE_KEY` holds the key
- **THEN** the system decrypts the secrets file without a key file

#### Scenario: No secrets file
- **WHEN** the secrets file does not exist
- **THEN** the system returns no secrets and no error

Tests: `TestEnvironmentFileWinsOverTheProjectKey`, `TestUserKeyIsTheFallback`, `TestKeyFromEnvironmentForCI`, `TestLoadWithoutFileIsEmpty`

### Requirement: Create an SSH key for the server
The system SHALL create an ed25519 key pair in `.muken/.ssh/` with `server key [name]`, and never overwrite one.

#### Scenario: A named key
- **WHEN** the user runs `server key acme`
- **THEN** `.muken/.ssh/acme` has mode 0600 and `acme.pub` holds a matching public key with the comment `muken-acme`
- **AND** the command asks no question and calls no cloud API

#### Scenario: The key exists
- **WHEN** the key file exists
- **THEN** the command fails and the file is unchanged

#### Scenario: A bad name
- **WHEN** the name has a slash, starts with a dot, or goes up a folder
- **THEN** the command fails with "bad key name"

Tests: `TestKeyCreate`, `TestKeyCreateKeepsAnExistingKey`, `TestKeyCreateRejectsBadNames`, `TestServerKeyAsksNothing`
