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

### Requirement: Authorization state persisted in the embedded store
The engine SHALL persist the authorization record (signer identity, signer
title, authorized seed domains/CIDRs/repositories, consented third-party
endpoints, and the authorization date) as a single JSON document in the
embedded SQLite settings table under the key `scope_settings`, and every
operation that assesses, discovers, or ingests SHALL confirm the record is
signed before proceeding.

Both the desktop and headless transports SHALL read the same key against the
same database, so an operator who authorizes a portfolio in the desktop
application and later runs the container against that database finds the same
authorization in force. Authorization is a property of the estate record, not
of the process that happens to be reading it.

Reading the record SHALL fail closed. An absent row, an unreadable row, a
malformed document, and a document whose `isAuthorized` flag is false SHALL all
yield an empty scope authorizing nothing. A scope that lists domains but was
never signed is a draft, and running against a draft is running without
authorization.

An authorization SHALL additionally require at least one seed domain. A signed
record naming no targets authorizes nothing, since there is nothing it could be
said to authorize.

Consented third-party endpoints SHALL be stored in the same record but held as
a separate field, and SHALL NOT be inferred from the target scope. Consent to
assess a domain is not consent to disclose that domain to a certificate
transparency log or any other third party.

#### Scenario: Unsigned record blocks assessment
- **GIVEN** a `scope_settings` document whose `isAuthorized` is false
- **WHEN** any operation that assesses, discovers, or ingests reads the scope
- **THEN** it receives an empty scope and refuses the operation

#### Scenario: Corrupted record authorizes nothing
- **GIVEN** a `scope_settings` row containing a malformed document
- **WHEN** the scope is read
- **THEN** an empty scope is returned rather than an error the caller might
  ignore, because treating an unreadable record as permissive would turn a
  corrupted settings row into an unbounded scan

#### Scenario: Signed record naming no targets authorizes nothing
- **GIVEN** a record with `isAuthorized` true and an empty seed domain list
- **WHEN** authorization is checked
- **THEN** the scope is not authorized

#### Scenario: The same authorization governs both transports
- **GIVEN** an authorization signed in the desktop application
- **WHEN** the headless server is run against the same database
- **THEN** the same authorization is in force, without re-signing

### Requirement: The authorization record's integrity is not asserted
The engine SHALL NOT claim that the stored authorization record is
tamper-evident. It is a plain JSON document in a local SQLite database, and any
process with write access to that file can alter it, including altering the
signer's name. The record answers "who said this was in scope, and when" for an
operator acting in good faith and for an auditor reading a cooperative system;
it is not evidence against someone with access to the machine.

This is stated as a requirement rather than left unsaid because the field is
called a signature, and a signature implies a property this record does not
have. An operator who believes the record is cryptographically bound would
draw a stronger conclusion from it than it can support.

#### Scenario: Documentation does not overstate the record
- **GIVEN** any document or interface describing scope authorization
- **WHEN** it refers to the signature
- **THEN** it does not describe the record as tamper-proof, tamper-evident, or
  cryptographically signed

### Requirement: Exportable compliance scope contract
The Angular dashboard SHALL provide an export mechanism allowing the operator to download a formatted Markdown/PDF authorization audit contract populated with the instance's active scope boundaries and signature metadata.

#### Scenario: Operator exports audit record
- **GIVEN** a signed and active authorization configuration
- **WHEN** the operator clicks "Export Scope Authorization Contract" in the UI
- **THEN** the dashboard generates and downloads a formatted Markdown contract reflecting the active scope and sign-off timestamp
