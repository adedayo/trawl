# Capability: executive-workbench

## Purpose

Present one evidence ledger at three resolutions — board, CISO and engineer — such that every currency figure walks down to a timestamped observation, the board's three decisions are explicit and attributed, uncertainty and coverage travel with every number, and material leaving the tool for an external audience must earn the right to do so.

## ADDED Requirements

### Requirement: Three views over identical underlying figures
The system SHALL provide board, practitioner and engineer views that differ only in aggregation, and the same quantity SHALL be identical across views rather than independently recomputed.

#### Scenario: Cross-view consistency
- **GIVEN** a scenario's expected annual loss shown in the board view and the practitioner view
- **WHEN** both are compared
- **THEN** they are the same stored value, not two separately computed values

### Requirement: Every currency figure drills down to observations
Every monetary figure SHALL be traceable, through the interface, to the scenario, control, asset, finding, ledger rows and finally the timestamped observations that produced it.

#### Scenario: Board figure to raw observation
- **GIVEN** an aggregate expected annual loss in the board view
- **WHEN** an operator drills into it
- **THEN** a bounded sequence of steps reaches individual observations with their timestamp, source, evidence and coverage state

#### Scenario: Drill path is verified
- **GIVEN** the required drill-down check
- **WHEN** it runs against every board-level currency figure
- **THEN** it fails if any figure lacks a complete path to observations

### Requirement: The three board decisions are explicit, attributed and dated
The system SHALL record the loss threshold, the tail-relief price and the review cadence as distinct decisions, each with a rationale, a deciding party and a date, and SHALL distinguish unset from zero.

#### Scenario: Threshold is inherited
- **GIVEN** the loss threshold decision
- **WHEN** it is recorded
- **THEN** it references the existing decision it inherits from, rather than being entered as a new independent figure

#### Scenario: Unset tail price
- **GIVEN** a deployment where the tail-relief price has never been set
- **WHEN** the ranking is displayed
- **THEN** its status is reported as unset, distinctly from a set value of zero, with a notice that the ranking prices the average year only

### Requirement: Board export requires the board's decisions to be set
Export of material for a board audience SHALL be refused while the tail-relief price is unset.

#### Scenario: Export attempted before the decision
- **GIVEN** an unset tail-relief price
- **WHEN** board material is exported
- **THEN** the export is refused, naming the unset decision as the reason

### Requirement: Export gating on verification and calibration
Export for board or external audiences SHALL be refused when any underlying parameter remains unverified or any presented figure is illustrative, and refusal SHALL list each specific reason and its resolving action.

#### Scenario: Unverified parameter
- **GIVEN** a figure resting on a parameter still marked as needing verification
- **WHEN** board material is exported
- **THEN** the export is refused and the specific parameter is named

#### Scenario: Sourced figures carry their statement
- **GIVEN** an export containing figures labelled sourced
- **WHEN** it is produced
- **THEN** each is accompanied by a statement that it propagates assumptions rather than validated probabilities

### Requirement: Exports are recorded
Every export SHALL be recorded with its snapshot reference, audience, gate result and timestamp.

#### Scenario: Traceable export
- **GIVEN** a previously exported board pack
- **WHEN** it is queried
- **THEN** the snapshot, audience and gate result that produced it are retrievable

### Requirement: Immutable periodic snapshots
The system SHALL capture immutable snapshots on the configured cadence, each recording the pack version, tool versions, seed, estimates, rankings, coverage and the board decisions in force at the time.

#### Scenario: Snapshot unaffected by later changes
- **GIVEN** a captured snapshot
- **WHEN** a new pack is subsequently applied
- **THEN** the snapshot's stored figures are unchanged

### Requirement: The two trend reads are produced
The system SHALL report, across snapshots, whether credible intervals are narrowing and whether the control ranking order is stable.

#### Scenario: Intervals not narrowing
- **GIVEN** successive snapshots whose interval widths have not decreased
- **WHEN** the trend read is produced
- **THEN** it reports that uncertainty is not reducing

#### Scenario: Ranking instability
- **GIVEN** successive snapshots in which the top-ranked controls have changed order
- **WHEN** the trend read is produced
- **THEN** the instability is reported as a finding about unresolved judgment rather than presented as a routine change

#### Scenario: Pack change within a series
- **GIVEN** a trend series spanning a pack change
- **WHEN** it is displayed
- **THEN** the change is marked and the series either re-baselines or declares the discontinuity

### Requirement: Movements are attributed to world change, belief correction or parameter change
Every change in a reported figure between snapshots SHALL be attributed to exactly one of: a change in the estate, a change in what is observed of an unchanged estate, or a change in model-pack parameters. The three SHALL be reported as separate components and SHALL NOT be combined into a single trend line.

Improved instrumentation raises reported exposure without the estate having deteriorated. Unless the cause is named, the reporting penalises measurement and the rational response is to measure less. See RISK-ARC §5c.

#### Scenario: New instrumentation reveals existing weaknesses
- **GIVEN** a new sensor that detects weaknesses which the ledger shows were already present
- **WHEN** the next snapshot is compared with the previous one
- **THEN** the increase is attributed to belief correction, is labelled as a correction to the prior report rather than a deterioration in security, and is reported separately from world change

#### Scenario: Genuine deterioration
- **GIVEN** a control observed at full enforcement in one snapshot and observed relaxed in the next
- **WHEN** the snapshots are compared
- **THEN** the increase is attributed to world change

#### Scenario: Ambient contact rate revised upward by a pack update
- **GIVEN** a model-pack update revising the ambient contact rate for a service class, with no change to the estate and no change to observation
- **WHEN** the snapshots are compared
- **THEN** the movement is attributed to parameter change, cites the pack version and source, and is not reported as a change in the organisation's security posture

#### Scenario: Combined movement
- **GIVEN** a period containing an estate change, a new sensor and a pack update
- **WHEN** the movement is reported
- **THEN** the three components are decomposed and separately quantified

### Requirement: Realised value of instrumentation is reported
The system SHALL report, per sensor, the belief corrections it has produced and their magnitude, and SHALL identify sensors that have produced none.

#### Scenario: Sensor has produced no corrections
- **GIVEN** a sensor active across several snapshots that has produced no belief correction
- **WHEN** instrumentation value is reported
- **THEN** the sensor is identified as either observing a stable area or failing to report, and the two are not asserted to be distinguishable without further evidence

#### Scenario: Sensor has produced corrections
- **GIVEN** a sensor whose introduction produced a quantified belief correction
- **WHEN** instrumentation value is reported
- **THEN** the magnitude of the correction is reported as the realised counterpart to the expected value-of-information estimate that justified it

### Requirement: Challenge and recompute is available in the interface
An operator SHALL be able to dispute and withdraw any ledger row from the interface and see the estimate recompute, with the challenge and its outcome retained.

#### Scenario: Withdrawing a measured-state row
- **GIVEN** a displayed estimate resting partly on a measured-state row carrying a variance share
- **WHEN** the row is withdrawn
- **THEN** the displayed figure recomputes and its interval widens, and the withdrawal is retained in the record

#### Scenario: Withdrawn rows are retained
- **GIVEN** a previously withdrawn ledger row
- **WHEN** the ledger is inspected
- **THEN** the row and its withdrawal reason remain visible

### Requirement: The interval is the headline figure
No view or export SHALL present a probability or monetary estimate without its uncertainty interval.

#### Scenario: Rendering without an interval is not expressible
- **GIVEN** the estimate rendering component
- **WHEN** it is supplied a value without an interval
- **THEN** the code does not compile

### Requirement: Labels and coverage travel with every figure
Every presented figure SHALL display its calibration label and the assessment coverage of the inputs behind it.

#### Scenario: Aggregate with partial coverage
- **GIVEN** an aggregate resting on inputs of which a share could not be assessed
- **WHEN** it is displayed
- **THEN** the coverage shortfall is displayed with it

### Requirement: Stated simplifications are disclosed where relied upon
Presentations depending on scenario independence, ambient-only contact modelling or the external-only vantage SHALL disclose those simplifications.

#### Scenario: Aggregate exceedance curve
- **GIVEN** a portfolio exceedance curve
- **WHEN** it is displayed
- **THEN** it discloses that scenarios are treated as independent

### Requirement: The board sees risk retired, risk accepted and capacity together
The board view SHALL present the expected loss being actively retired, the aggregate expected loss held as accepted risk, and the stated remediation capacity, as three figures on the same page.

#### Scenario: Accepted risk is material
- **GIVEN** an aggregate accepted risk comparable to or exceeding the risk being retired
- **WHEN** the board view is rendered
- **THEN** both figures appear together in currency, and the accepted figure is not relegated to a subsidiary view

#### Scenario: Capacity is the binding constraint
- **GIVEN** a queue head reported as capacity-constrained
- **WHEN** the board view is rendered
- **THEN** the constraint is presented as a resourcing decision belonging to the board, distinct from any estimation uncertainty

### Requirement: Finding counts are not presented as a measure of position or progress
No view or export SHALL present counts of findings discovered, open or closed as a headline indicator of security position or of progress. Counts MAY appear as operational detail where accompanied by the loss reduction they represent.

#### Scenario: Count offered as progress
- **GIVEN** a period in which many low-ranked findings were closed and the highest-ranked findings were not
- **WHEN** the board view is rendered
- **THEN** the position is expressed as loss retired rather than as findings closed, and the unworked queue head is visible

### Requirement: The price of deferral is presented without arbitrating the decision
Where a remediation is deferred in favour of other work, the system SHALL present the priced expected loss of the deferral with its interval and calibration label, and SHALL NOT present a recommendation as to which work should proceed.

#### Scenario: Deferral in favour of other work
- **GIVEN** a remediation deferred beyond the capacity line
- **WHEN** the position is presented
- **THEN** the priced expected loss of deferring is shown with its interval and owner, and no recommendation to prioritise or deprioritise the competing work is emitted

#### Scenario: Comparison with a non-risk estimate
- **GIVEN** an externally supplied value estimate for competing work
- **WHEN** it is displayed alongside a risk figure
- **THEN** it is marked as externally supplied and outside the model's calibration, and the two are not combined into a single net figure

### Requirement: No composite risk score
The system SHALL NOT produce a single composite risk score or index. All emitted quantities SHALL be probabilities, rates, monetary amounts, intervals or rankings with a stated derivation.

#### Scenario: Composite score requested
- **GIVEN** any interface or export
- **WHEN** its outputs are inspected
- **THEN** no unitless composite index is present

### Requirement: AI narrative is attributed and cannot alter figures
Any AI-generated prose SHALL be labelled as such, and SHALL NOT change any displayed or exported figure.

#### Scenario: Narrative present
- **GIVEN** a view containing AI-drafted commentary
- **WHEN** figures are compared against the same view with commentary disabled
- **THEN** every figure is identical, and the commentary is visibly attributed
