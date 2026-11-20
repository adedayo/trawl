# Capability: findings-history

## Purpose

Record what the engine has observed about each finding over time, so that
change and drift can be read back. Governs identity, the event record and
resolution. It does not govern how a finding is explained (see
`finding-explanation`) or presented (see `findings-drilldown`).

## ADDED Requirements

### Requirement: A finding has a stable identity
The system SHALL derive a finding's key from what the finding asserts, so that
the same condition observed by different runs is the same finding.

#### Scenario: The same scan is ingested twice
- **GIVEN** a scan result already ingested
- **WHEN** an identical result is ingested again
- **THEN** one finding exists, with one first-seen event and an updated
  last-seen time

#### Scenario: A different instance is a different finding
- **GIVEN** the same weakness observed on two hosts of one domain
- **WHEN** both are recorded
- **THEN** two findings exist, each with its own key

### Requirement: Changes are recorded as append-only events
The system SHALL record each transition of a finding (first seen, changed,
priority changed, resolved, reopened) as an event carrying its UTC timestamp,
the run that observed it, and the prior and new values. Events SHALL NOT be
updated or deleted by any engine code path.

#### Scenario: Severity changes between runs
- **GIVEN** a finding recorded as medium
- **WHEN** a later run observes it as high
- **THEN** a change event records both values and the run that observed it, and
  the earlier event is unaltered

#### Scenario: The store offers no mutation of history
- **WHEN** the store interface is inspected
- **THEN** no operation updates or deletes an individual event

#### Scenario: The operator removes an asset
- **GIVEN** an asset with recorded events
- **WHEN** the operator removes the asset
- **THEN** its events are discarded with it, as is everything else recorded
  against it

### Requirement: The event and the current state agree
The system SHALL write a finding's event and its current-state record in one
transaction.

#### Scenario: A write fails part-way
- **GIVEN** an event and a projection update in one write
- **WHEN** either fails
- **THEN** neither is persisted

### Requirement: A finding resolves only when it was re-assessed
The system SHALL mark a finding resolved only when a later run concluded the
check that raises it (`ok` or `not_found`) and the check did not raise it.

#### Scenario: A weakness is fixed
- **GIVEN** an open finding from an SPF check
- **WHEN** a later run concludes the SPF check and does not raise it
- **THEN** the finding is resolved, a resolved event is recorded, and the
  domain's posture no longer counts it as deficient

#### Scenario: The check could not conclude
- **GIVEN** an open finding
- **WHEN** a later run records the check as `check_failed` or `not_checked`
- **THEN** the finding remains open and is marked as not re-assessed

#### Scenario: An inconclusive absence does not resolve
- **GIVEN** a finding raised by a check whose `not_found` is not conclusive,
  such as DKIM selector probing
- **WHEN** a later run finds nothing
- **THEN** the finding is not resolved

#### Scenario: A resolved finding returns
- **GIVEN** a resolved finding
- **WHEN** a later run raises it again
- **THEN** it is reopened, a reopened event is recorded, and its first-seen
  time is unchanged

### Requirement: Drift is attributable to its cause
The system SHALL record the library and registry versions on every event, and
SHALL cite the feed snapshot on every event caused by a feed refresh.

#### Scenario: The scanner library is upgraded
- **GIVEN** a finding whose severity differs after a library upgrade
- **WHEN** the change event is read
- **THEN** it shows the two library versions, and is distinguishable from a
  change in the estate

#### Scenario: A CVE enters the exploited-vulnerability catalogue
- **WHEN** a feed refresh raises a finding's priority
- **THEN** the event names the feed and snapshot that caused it

### Requirement: Existing data is not given invented history
The system SHALL give each finding that predates the migration one event,
labelled as a pre-history baseline, at its recorded first-seen time, and SHALL
NOT reconstruct earlier change.

#### Scenario: An existing installation is upgraded
- **GIVEN** findings stored before the migration
- **WHEN** the migration completes
- **THEN** each has a single baseline event, and no earlier transition is
  implied

### Requirement: Run summaries carry their coverage
The system SHALL record, for each run, counts by severity and status together
with the coverage of the checks the run attempted.

#### Scenario: A thin run
- **GIVEN** a run in which half the checks were `not_checked`
- **WHEN** its summary is read
- **THEN** it states that coverage, so a lower finding count is not read as an
  improvement
