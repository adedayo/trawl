# Capability: posture-regression

## Purpose

Detect when any tracked external-posture attribute — TLS/cipher/protocol support, certificate strength, DMARC/SPF/DKIM policy, open port/service exposure, or externally-discoverable secret exposure — moves in a *worse* direction between consecutive checks on the same asset, and surface it as a distinct, always-alertable finding. A control that quietly degrades is exactly as material as a new weakness appearing, and is easier to miss, since nothing new showed up to draw attention — the asset just got worse. This capability turns Trawl from attack-surface *discovery* alone into continuous external *control validation*: it is the one shared mechanism every check-producing capability (`scanning`, `email-authentication`, and future ones) plugs into, rather than each reinventing its own drift-detection logic.

## ADDED Requirements

### Requirement: Structured, comparable posture snapshots
Every check-producing capability SHALL persist its structured output as a dated posture snapshot per (asset, attribute) pair, so a later check's snapshot can be diffed against the immediately preceding one for that same pair. This capability does not perform checks itself — it consumes snapshots that `scanning` and `email-authentication` (and future capabilities) already emit in structured form.

#### Scenario: Two consecutive scans are comparable
- **GIVEN** an asset was scanned on two consecutive scheduled runs
- **WHEN** the second run's TLS snapshot is compared to the first
- **THEN** the comparison is a structured field diff, not a re-parse of raw scan output

### Requirement: Deterministic direction-of-change classification
For every trackable attribute, a fixed, deterministic better/worse/neutral ordering SHALL be defined in versioned config (e.g., TLS 1.3-only > TLS 1.2-allowed > TLS 1.0/1.1-allowed; DMARC `reject` > `quarantine` > `none` > absent; certificate key length/algorithm strength tiers; open port/service count increasing = worse). Classification SHALL NOT be left to AI judgment, matching the engine-wide deterministic-severity/AI-narrative-only rule.

#### Scenario: TLS downgrade
- **GIVEN** an asset previously supported TLS 1.3 only
- **WHEN** a later scan shows TLS 1.0 has been re-enabled
- **THEN** the change is classified as a regression on the TLS-version attribute, via the versioned ordering table, not an LLM judgment call

#### Scenario: New attack surface on an existing asset
- **GIVEN** an existing, previously-scanned asset had a known, stable open-port set
- **WHEN** a later scan finds a new open port not present in the prior scan
- **THEN** the change is classified as a regression on the exposure-surface attribute (the attack surface grew), using the same mechanism as a TLS or certificate downgrade

### Requirement: Distinct, always-alertable finding category
A confirmed regression SHALL be recorded as a distinct finding category (`regression`), separate from `new-weakness`, `new-asset`, and vulnerability-correlation categories, and SHALL always be eligible for alerting regardless of the resulting state's absolute severity tier — the change itself is the signal, even when the new state isn't yet "critical."

#### Scenario: Downgrade between two adequate states still alerts
- **GIVEN** a cipher-suite change moves an asset from a fully-hardened baseline to a merely-adequate one
- **WHEN** the regression is confirmed
- **THEN** an alert fires, even though the resulting cipher configuration is not itself classified as critical severity

### Requirement: Two-observation confirmation, no noise-driven false regressions
The system SHALL require at least two consecutive observations of a changed state before classifying it as a confirmed regression; a single-observation change SHALL be recorded as a lower-confidence provisional note, not a confirmed regression finding, and SHALL NOT fire an alert.

#### Scenario: Transient change does not alert
- **GIVEN** one scan run shows an unexpected TLS 1.0 result that the next scheduled run does not reproduce
- **WHEN** the second run reverts to the prior state
- **THEN** no confirmed regression or alert was ever created — only a provisional note existed, and it is superseded by the reverting observation

#### Scenario: Repeated change confirms
- **GIVEN** two consecutive scheduled runs both show the same downgraded state
- **WHEN** the second run confirms the first
- **THEN** a confirmed regression finding is created and an alert fires

### Requirement: Append-only regression history, including restoration
Consistent with the `asset-inventory` capability's existing history-retention requirement, every regression and its eventual restoration SHALL be retained as a transition in history, never overwritten, so the operator can see a full posture timeline per asset/attribute.

#### Scenario: Regression later restored
- **GIVEN** a confirmed TLS-version regression exists for an asset
- **WHEN** a later scan confirms TLS 1.0 has been disabled again
- **THEN** a new "restored" transition is recorded, and the original regression entry remains in history rather than being deleted

### Requirement: Shared mechanism, not per-capability reimplementation
Any capability that produces structured, comparable output SHALL use this capability's snapshot/diff/classify/alert contract rather than implementing its own one-off regression logic.

#### Scenario: Email-authentication regression uses the shared mechanism
- **GIVEN** the `email-authentication` capability's DMARC-policy-regression scenario (a domain's policy weakening between checks)
- **WHEN** it is implemented
- **THEN** it is built on this capability's snapshot/diff/classify/alert contract, not a separate, DMARC-specific regression check

### Requirement: Externally-discoverable secret exposure only
Tracking "new secret exposure" as a regression attribute SHALL be scoped to externally-observable exposure discoverable via the same non-destructive techniques `scanning` already uses (e.g., a newly-reachable `.env`/`.git`/config endpoint, a secret newly visible in client-side JS, robots.txt, or a public cloud-storage listing). This capability SHALL NOT introduce authenticated source-repository scanning or any credentialed access — that remains explicitly out of scope per the engine's non-goals (internal/authenticated vulnerability management is a different, already-covered problem).

#### Scenario: Newly-exposed config endpoint
- **GIVEN** a previously-404 `.env` file on an existing, in-scope web asset
- **WHEN** a later scan finds it publicly readable
- **THEN** a new-secret-exposure regression finding is recorded, using the same non-destructive HTTP techniques already in the scan job's template set

#### Scenario: Source-repository scanning stays out of scope
- **GIVEN** this capability's requirements
- **WHEN** they are reviewed for any dependency on GitHub/GitLab App access or authenticated repository cloning
- **THEN** none is found — every regression check here uses only unauthenticated, externally-reachable observations
