# Capability: portability-config

## Purpose

Ensure the entire system can be redeployed for a new organization by changing configuration only, with zero code changes — the requirement that makes this a portable craft item rather than a single-organization-specific artifact.

## ADDED Requirements

### Requirement: Externalized organization-specific values
All organization-specific values (seed domains/CIDRs, seed repository URLs, alert webhook URLs, org display name) SHALL be externalized to a single configuration source, never hardcoded in application or job code.

#### Scenario: Config-only redeployment
- **GIVEN** the operator moves to a new environment or organization
- **WHEN** they redeploy using a new config file and a fresh Convex project
- **THEN** no source file requires editing to remove the previous instance's identifiers

### Requirement: Secrets never committed
All credentials/API keys SHALL be loaded from a secrets manager (or local env for dev), never committed to the repository.

#### Scenario: Repository audit
- **GIVEN** the repository's full commit history
- **WHEN** it is searched for API keys or credentials
- **THEN** none are found in any commit

### Requirement: Engine/instance-data separation
The repository SHALL be structured so that "engine" code (schema, jobs, correlation logic, UI) contains no data specific to any one deployment instance; instance data (scan results, findings, seed lists) SHALL live outside the portable codebase or in a gitignored/instance-specific location.

#### Scenario: Portable repo contains no employer identifiers
- **GIVEN** someone reads the public/portable repo
- **WHEN** they search for the current employer's name
- **THEN** zero matches are found outside of local, gitignored config

### Requirement: Same-day redeployment runbook
A documented runbook SHALL exist describing the steps to stand up a new instance (new Convex project, new config, new secrets) in under one business day.

#### Scenario: Fresh deployment following the runbook
- **GIVEN** a fresh Convex project has been created
- **WHEN** the operator follows the redeployment runbook
- **THEN** the system is fully operational for the new environment without any code modification
