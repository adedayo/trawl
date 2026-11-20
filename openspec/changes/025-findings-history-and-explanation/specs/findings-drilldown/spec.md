# Capability: findings-drilldown

## Purpose

Let an operator move from an estate-level view to one finding and its history,
and see what changed between scans. Governs presentation and navigation; the
data comes from `findings-history` and the wording from `finding-explanation`.

## ADDED Requirements

### Requirement: All findings are listed together
The system SHALL list findings from every source (CVE and service, Vantage
signals, secrets) in one view, filterable by status, severity, source, asset
and whether they changed since a chosen time.

#### Scenario: Filtering to what changed
- **GIVEN** findings of which three changed since the last scan
- **WHEN** the operator filters to changed since the last scan
- **THEN** exactly those three are listed

#### Scenario: A Vantage finding appears in the main list
- **GIVEN** an email-authentication finding
- **WHEN** the findings list is opened
- **THEN** it is present, without the operator having visited the Email tab

### Requirement: A finding can be opened and linked to
The system SHALL open a finding's detail from its row by pointer or keyboard,
and SHALL give each finding a stable address that reopens the same detail.

#### Scenario: Keyboard access
- **WHEN** a row is focused and activated with the keyboard
- **THEN** the detail opens, and closing it returns focus to the row

#### Scenario: A shared address
- **GIVEN** the address of a finding's detail
- **WHEN** it is opened in a new session
- **THEN** the same finding is shown

### Requirement: The detail shows the explanation, history and provenance
The system SHALL show, for a finding, its status, first-seen and last-seen
times and age, its executive explanation, the evidence, the full timeline of
events, and the scan run, library version and feed snapshot behind it.

#### Scenario: A finding that was fixed and returned
- **GIVEN** a finding resolved and later reopened
- **WHEN** its detail is opened
- **THEN** the timeline shows first seen, resolved and reopened in order, each
  with its time and the run that observed it

### Requirement: Not re-assessed is visible
The system SHALL distinguish a finding that was re-assessed and still present
from one that remains open because its check could not conclude.

#### Scenario: A failed check
- **GIVEN** an open finding whose latest run recorded the check as
  `check_failed`
- **WHEN** it is listed
- **THEN** it is marked not re-assessed, with the reason

### Requirement: Change between scans is shown with its coverage
The system SHALL show the findings new, resolved, worsened and improved since
the previous scan, and SHALL show the coverage of the runs compared.

#### Scenario: Fewer findings on a thinner run
- **GIVEN** a run that assessed fewer checks than the one before
- **WHEN** the change view is shown
- **THEN** the reduced coverage is stated beside the reduced count

### Requirement: Trends do not invent precision
The system SHALL plot history from recorded run summaries only, and SHALL NOT
interpolate across runs that did not happen.

#### Scenario: A gap between scans
- **GIVEN** no scan for ten days
- **WHEN** the trend is drawn
- **THEN** the gap is shown as a gap
