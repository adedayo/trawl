# Capability: deployment-packaging

## Purpose

Package the engine as a single-command, self-hostable deployment (Docker
Compose over an embedded SQLite store) and as a single statically linked binary,
with config-only portability to any environment — so a self-hoster can go from a
clean machine to a running instance without a cloud account, a manual
multi-step checklist, or any code changes.

## ADDED Requirements

### Requirement: Engine/deploy separation
Engine code (the Go engine under `pkg/` and `cmd/`, and the Angular app under
`app/`) SHALL contain zero deployment-specific logic. Anything that differs
between installations (database location, listen address, credentials) SHALL be
expressed as deploy-time configuration (env vars, config file, compose files),
never as a code branch inside the engine.

#### Scenario: No environment-specific branching in engine code
- **GIVEN** a Go package or Angular component
- **WHEN** its source is inspected
- **THEN** it contains no conditional logic keyed on "which environment am I running in"

### Requirement: Single-command bring-up
The repo SHALL provide a Docker Compose stack that brings up a fully functional
instance — the engine server and the Angular dashboard served by nginx — with a
single `docker compose up -d`, requiring no cloud account and no manual
multi-step setup beyond supplying seed config.

There SHALL be no separate database service. The engine holds its state in an
embedded SQLite database on a named volume, so the stack has no datastore to
provision, back up separately, or keep version-compatible with the engine.

Every service SHALL declare both `image:` and `build:`, so that the default path
pulls a published image while a local build remains available without editing
the compose file.

#### Scenario: Clean-machine bring-up
- **GIVEN** a clean machine with Docker and Docker Compose installed and the repo cloned
- **WHEN** the operator runs `docker compose up -d` after populating `.env` with seed config
- **THEN** the engine server and dashboard are running and the dashboard is
  reachable, with no additional manual steps

### Requirement: Guided first-run setup, not hand-edited files
The repo SHALL provide a single guided setup command (e.g., `./setup.sh` or an equivalent npm/make target) that interactively collects the minimal required values (seed domains, LLM API key or local-model choice, alert webhook, admin credential), generates `.env` and the initial `config/<instance>.json` from that input, validates that Docker and Docker Compose are installed and running before proceeding, brings the stack up, waits for each service's health check to pass, and prints the dashboard URL on success. Hand-editing `.env`/config files SHALL remain possible for advanced users but SHALL NOT be the only documented path.

#### Scenario: First-time self-hoster completes setup without reading source
- **GIVEN** a clean machine with Docker installed and the repo cloned
- **WHEN** the operator runs the guided setup command and answers its prompts
- **THEN** the stack comes up, health checks pass, and the dashboard URL is printed, with no manual file editing required

#### Scenario: Missing prerequisite fails loudly with remediation
- **GIVEN** Docker is not installed or the Docker daemon is not running
- **WHEN** the guided setup command is run
- **THEN** it stops immediately with a specific, actionable message (what's missing and how to fix it), never a raw stack trace or a silent hang

### Requirement: License and dependency disclosure
The repo SHALL be licensed Apache-2.0, and SHALL disclose in its README/NOTICE
the licensing of any dependency whose terms are not OSI-approved, so downstream
adopters are not misled about the full dependency tree's licensing.

#### Scenario: License disclosure present
- **GIVEN** a new adopter reading the repo's README
- **WHEN** they look for the licensing terms of the dependency tree
- **THEN** any non-OSI-approved terms are stated plainly, not omitted or buried

### Requirement: The same engine runs desktop and headless
The engine SHALL run unmodified as the desktop application and as the headless
server. Only the transport in front of it SHALL differ, and both SHALL read the
same database and the same authorization record.

Deployment choice SHALL NOT be a fork. An operator who starts on the desktop and
later moves to a server SHALL carry their database across without migration, and
SHALL find the same authorization in force.

#### Scenario: Same engine, two transports
- **GIVEN** a database written by the desktop application
- **WHEN** the headless server is run against it
- **THEN** the estate, findings and authorization are identical, with no
  conversion step and no re-signing
