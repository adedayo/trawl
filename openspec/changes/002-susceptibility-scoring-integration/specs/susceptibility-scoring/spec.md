# Capability: susceptibility-scoring

## Purpose

Surface each finding's Applicability/Attempt/Success stage inputs and contact status, computed deterministically from data Trawl already holds, formatted to match the exploit-probability framework's Prior Estimator input shape — so a finding feeds the posture-series estimation and decision tools directly instead of being manually re-derived by a human reading the dashboard.

## ADDED Requirements

### Requirement: Structured stage estimate, separate from priority
The system SHALL compute and store an Applicability/Attempt/Success stage estimate per finding, as a field distinct from the existing deterministic `priority`/`severity` fields; computing or updating the stage estimate SHALL NOT alter `priority`, `severity`, `kev`, `epss`, or `cvss`.

#### Scenario: Stage estimate does not affect priority
- **GIVEN** a finding with an existing deterministic priority score
- **WHEN** its susceptibility stage estimate is computed or recomputed
- **THEN** the finding's `priority`, `severity`, `kev`, `epss`, and `cvss` fields are unchanged

### Requirement: Contact status sourced from asset inventory
The system SHALL surface a `contactStatus` value (`internet-facing` | `internal` | `unknown`) per finding, derived from the associated asset's existing inventory/discovery status, without introducing a new discovery mechanism.

#### Scenario: Internet-facing asset
- **GIVEN** a finding on an asset with status `active` and a confirmed internet-facing discovery source
- **WHEN** the stage estimate is computed
- **THEN** `contactStatus` is `internet-facing`

### Requirement: Deterministic, versioned adjustment bands
The attempt-stage EPSS/KEV-band adjustment SHALL be computed by looking up a versioned `adjustmentBands` config table, not a hardcoded conditional in function code, so the table can be updated to match revisions of the published exploit-probability framework without an engine code change.

#### Scenario: EPSS band lookup
- **GIVEN** a finding with an EPSS score of 0.62
- **WHEN** the attempt-stage adjustment is computed
- **THEN** the system looks up the `> 0.50` band in `adjustmentBands` and applies its stored adjustment value, rather than a value embedded in the computation function

### Requirement: Deduplication between EPSS and raw attention signals
When a finding has a valid EPSS score, the system SHALL NOT additionally apply a separate "exploit code available" or similar attention-proxy adjustment, since EPSS's own score already incorporates that signal.

#### Scenario: EPSS score present
- **GIVEN** a finding with a valid EPSS score and known public exploit code
- **WHEN** the attempt-stage adjustment is computed
- **THEN** only the EPSS-band adjustment is applied; no additional exploit-code-availability adjustment is added on top

### Requirement: Evidence ledger per adjustment
Every adjustment applied to a stage estimate SHALL be recorded with the specific finding/asset field that produced it, so any adjustment can be traced and removed for recomputation.

#### Scenario: Adjustment traceability
- **GIVEN** a computed stage estimate with two applied adjustments
- **WHEN** an operator inspects the evidence ledger for that finding
- **THEN** each adjustment names the exact source field (e.g., `finding.epssScore`, `asset.status`) it was derived from

### Requirement: Export format matches the Prior Estimator's input shape
The dashboard SHALL provide an export/copy action that formats a finding's stage estimate and evidence ledger to match the input shape expected by the exploit-probability framework's Prior Estimator component.

#### Scenario: Copy a finding into the calculator
- **GIVEN** a finding with a computed stage estimate
- **WHEN** an operator uses the export action
- **THEN** the output can be pasted into the Prior Estimator's inputs without manual reformatting

### Requirement: No simulation or Bayesian update inside Trawl
The system SHALL NOT run Monte Carlo simulation or Layer 3 Bayesian conjugate updates on stage estimates; it SHALL emit Layer 1/2 deterministic inputs and the evidence ledger only, leaving simulation and telemetry-based updates to the calculators and the operator's own environment data.

#### Scenario: Stage estimate is an input, not a probability distribution output
- **GIVEN** a finding's computed stage estimate
- **WHEN** it is inspected
- **THEN** it contains base rates, bounds, and named adjustments, never a simulated probability distribution or a Beta-Binomial posterior computed from telemetry Trawl does not itself hold
