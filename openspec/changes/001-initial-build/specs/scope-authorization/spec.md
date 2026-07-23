# Capability: scope-authorization

## Purpose

Provide in-app onboarding and persistent authorization management allowing operators to review, sign, export, and enforce target scope bounds directly within the user interface before any scanning job is permitted to run.

## ADDED Requirements

### Requirement: In-app authorization wizard & sign-off
The Angular dashboard SHALL present an interactive scope authorization wizard on first launch (and whenever authorization is uninitialized), prompting the operator to confirm authorized seed domains/CIDRs, review the non-destructive rules of engagement, and record explicit digital authorization before scanning operations can execute.

#### Scenario: First-run authorization prompt
- **GIVEN** an instance where scope authorization has not been signed
- **WHEN** an operator accesses the dashboard
- **THEN** the application displays the authorization wizard blocking scan execution until signed

### Requirement: Authorization state persisted in Convex
Convex schema SHALL store authorization state (signer identity, authorized targets, signed timestamp, rules of engagement acknowledgment version) in `config`, and ALL scan ingestion/trigger actions SHALL verify `authorizationSignedAt` is present before processing scan operations.

#### Scenario: Unsigned backend blocks scan execution
- **GIVEN** a scan job attempting to execute or ingest findings
- **WHEN** Convex checks instance configuration and `authorizationSignedAt` is missing
- **THEN** Convex rejects the operation with an authorization required error

### Requirement: Exportable compliance scope contract
The Angular dashboard SHALL provide an export mechanism allowing the operator to download a formatted Markdown/PDF authorization audit contract populated with the instance's active scope boundaries and signature metadata.

#### Scenario: Operator exports audit record
- **GIVEN** a signed and active authorization configuration
- **WHEN** the operator clicks "Export Scope Authorization Contract" in the UI
- **THEN** the dashboard generates and downloads a formatted Markdown contract reflecting the active scope and sign-off timestamp
