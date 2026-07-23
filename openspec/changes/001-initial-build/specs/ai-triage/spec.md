# Capability: ai-triage

## Purpose

Use an LLM to produce human-readable summaries, remediation suggestions, and duplicate/noise flags as an annotation layer on top of deterministic findings — narrative only, never the source of severity.

## ADDED Requirements

### Requirement: Annotation generated above threshold
The system SHALL generate an AI annotation (summary, suggested remediation, duplicate-likelihood) for every new finding above a configurable priority threshold.

#### Scenario: Critical finding triggers annotation
- **GIVEN** a new finding is created with `priority: critical`
- **WHEN** it exceeds the configured triage threshold
- **THEN** an AI annotation is attached within one processing cycle summarizing the risk in plain language and suggesting a remediation

### Requirement: Annotation never overwrites deterministic fields
AI annotations SHALL be stored as a field separate from the deterministic finding record and SHALL NEVER overwrite priority, severity, or KEV/EPSS fields.

#### Scenario: AI disagrees with computed priority
- **GIVEN** the AI-generated summary suggests a finding seems lower-risk than its computed priority
- **WHEN** the annotation is stored
- **THEN** the finding's `priority`, `kev`, and `epss` fields remain unchanged; only the annotation text reflects the AI's narrative

### Requirement: Grounded context, no fact invention
The system SHALL include structured context (scan evidence, CVE/KEV/EPSS data, asset metadata) in the AI prompt; the AI SHALL NOT be asked to infer facts not present in that context.

#### Scenario: Prompt construction
- **GIVEN** a finding is queued for triage
- **WHEN** the triage prompt is built
- **THEN** it includes only the finding's own scan evidence, matched CVE/KEV/EPSS data, and asset metadata — no external facts requiring the model's own knowledge of the target

### Requirement: Duplicate flags collapse display only
AI-flagged "likely duplicate" findings SHALL still remain visible in the underlying data model (hidden/collapsed in UI only), not deleted.

#### Scenario: Near-duplicate flagged
- **GIVEN** the AI suggests a finding is a near-duplicate of an existing open finding
- **WHEN** a human reviews the dashboard
- **THEN** both records remain queryable in the database, with only the display collapsed
