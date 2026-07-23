# Capability: deployment-packaging

## Purpose

Package the engine as a single-command, self-hostable deployment (Docker Compose, self-hosted Convex) with config-only portability to any environment — so a self-hoster can go from a clean machine to a running instance without a cloud account, a manual multi-step checklist, or any code changes.

## ADDED Requirements

### Requirement: Engine/deploy separation
Engine code (Convex schema/functions under `convex/`, the Angular app under `app/`, job containers under `jobs/`) SHALL contain zero deployment-specific logic. Anything that differs between installations (which Convex instance to talk to, hosting details, credentials) SHALL be expressed as deploy-time configuration (env vars, compose files), never as a code branch inside the engine.

#### Scenario: No environment-specific branching in engine code
- **GIVEN** a Convex function, job container, or Angular component
- **WHEN** its source is inspected
- **THEN** it contains no conditional logic keyed on "which environment am I running in"

### Requirement: Single-command bring-up
The repo SHALL provide a Docker Compose stack that brings up a fully functional instance — self-hosted Convex backend, Convex dashboard, the Ofelia job scheduler, and the Angular dashboard (served by an nginx container) — with a single `docker compose up -d`, requiring no cloud account and no manual multi-step setup beyond supplying an LLM API key and seed config.

#### Scenario: Clean-machine bring-up
- **GIVEN** a clean machine with Docker and Docker Compose installed and the repo cloned
- **WHEN** the operator runs `docker compose up -d` after populating `.env` with an LLM API key and seed config
- **THEN** the self-hosted Convex backend, dashboard, job scheduler, and Angular dashboard are all running and the dashboard is reachable, with no additional manual steps

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
The repo SHALL be licensed Apache-2.0, and SHALL disclose in its README/NOTICE that the self-hosted Convex backend dependency is licensed FSL-1.1 (source-available, converting to Apache-2.0 two years after each release) rather than an OSI-approved license, so downstream adopters are not misled about the full dependency tree's licensing.

#### Scenario: License disclosure present
- **GIVEN** a new adopter reading the repo's README
- **WHEN** they look for the licensing terms of the self-hosted Convex dependency
- **THEN** the FSL-1.1 terms and its conversion timeline are stated plainly, not omitted or buried

### Requirement: Convex hosting is a config choice, not a fork
Convex schema, functions, and scheduled jobs SHALL run unmodified whether Convex is self-hosted (the documented default) or pointed at Convex Cloud for managed hosting — only the client's target URL and environment-variable values (API keys, webhook URLs) SHALL differ.

#### Scenario: Same schema and functions regardless of Convex hosting choice
- **GIVEN** the `convex/` directory
- **WHEN** it is deployed to a self-hosted Convex instance and, separately, to a Convex Cloud project
- **THEN** both instances expose identical schema, queries, mutations, actions, and scheduled functions with no code changes
