# Capability: control-portfolio-roi

## Purpose

Price and rank security controls by the expected loss they remove per unit of cost, crediting each across every scenario it covers, with control coverage measured from external observation wherever it is observable — so that the gap between a control purchased and a control delivered becomes a priced, closable finding.

## ADDED Requirements

### Requirement: Controls are dual-scored and scenario-linked
Each control SHALL record a threat-mitigation score, a loss-magnitude-attenuation score, the loss forms its attenuation touches, its annualised cost, and every scenario it covers.

#### Scenario: Control definition completeness
- **GIVEN** a control in the register
- **WHEN** it is validated
- **THEN** validation fails unless both scores, the forms touched, the annualised cost and at least one covered scenario are present

### Requirement: Coverage is measured where observable and marked where asserted
Control coverage SHALL be computed from external observations where the control's deployment is externally observable, SHALL be marked as asserted with an attestation source and date otherwise, and the mode SHALL be reported wherever the control appears.

#### Scenario: Externally observable control
- **GIVEN** a control whose deployment is observable across a set of domains
- **WHEN** its coverage is computed
- **THEN** coverage is derived from those observations and reported as measured

#### Scenario: Asserted coverage
- **GIVEN** a control that cannot be observed externally
- **WHEN** its coverage is recorded
- **THEN** it is marked as asserted, carries an attestation source and date, and is distinguished from measured coverage in rankings and exports

### Requirement: Unassessed deployment never counts as covered
Assets or domains where a control's presence could not be determined SHALL be excluded from the covered count and reported as a coverage-confidence figure.

#### Scenario: Assessment failure
- **GIVEN** a control assessed across twenty domains where three assessments could not conclude
- **WHEN** coverage is computed
- **THEN** those three are not counted as covered, and the coverage figure is accompanied by its confidence

### Requirement: Effective mitigation is scaled by coverage
The mitigation applied in loss computation SHALL be the control's score scaled by its coverage, and SHALL NOT use the unscaled score.

#### Scenario: Partial deployment
- **GIVEN** a control deployed on a minority of the assets in its scope
- **WHEN** expected loss is computed
- **THEN** the mitigation applied is proportionately reduced

### Requirement: The coverage gap is priced
For every control with coverage below full, the system SHALL compute the expected-loss reduction obtainable by closing the coverage gap alone, without additional licence cost.

#### Scenario: Gap valuation
- **GIVEN** a purchased control at partial measured coverage
- **WHEN** the portfolio is priced
- **THEN** the value of raising that control to full coverage is reported as a distinct, costed opportunity

### Requirement: Effective attenuation is computed per scenario from the overlay
A control's attenuation in a scenario SHALL be its attenuation score scaled by the measured overlay dollars in the forms it touches divided by that scenario's total overlay dollars, and SHALL be computed separately for each scenario.

#### Scenario: One control, two overlays
- **GIVEN** a control covering two scenarios with materially different loss-form compositions
- **WHEN** its effective attenuation is computed
- **THEN** it differs between the two scenarios

### Requirement: Controls are priced marginally
A control's contribution SHALL be computed by removing it from the portfolio while all other controls remain, and SHALL NOT be computed standalone.

#### Scenario: Overlapping controls
- **GIVEN** two controls whose effects overlap
- **WHEN** each is priced
- **THEN** each is priced with the other retained, and neither is credited with reduction the other would also deliver

### Requirement: Shared reduction is reconciled explicitly
The system SHALL report the reconciliation of marginal contributions plus shared reduction to the total reduction delivered by the portfolio.

#### Scenario: Marginals do not sum
- **GIVEN** a portfolio whose marginal contributions total less than its overall reduction
- **WHEN** results are presented
- **THEN** the shared reduction is reported as a distinct line reconciling the two figures

### Requirement: Ranking is on the aggregate with a per-scenario breakdown
Controls SHALL be ranked by aggregate expected-loss reduction per unit cost across every scenario they cover, with a per-scenario breakdown showing the source of each control's credit.

#### Scenario: Broad versus narrow control
- **GIVEN** a control covering several scenarios and a control covering one
- **WHEN** they are ranked
- **THEN** the ranking reflects credit earned across all covered scenarios, and the breakdown shows the per-scenario contribution of each

### Requirement: Tail-relief weight enters the return and defaults to zero
The return calculation SHALL include a tail-relief term priced by a published weight applied to the reduction in the upper-percentile annual loss, defaulting to zero.

#### Scenario: Default weight
- **GIVEN** a portfolio with the tail-relief weight unset
- **WHEN** returns are computed
- **THEN** the return reduces to expected-loss reduction per unit cost

#### Scenario: Non-zero weight re-sorts visibly
- **GIVEN** a portfolio ranked at a zero tail-relief weight
- **WHEN** a positive weight is applied
- **THEN** returns are recomputed including the tail term and any change in ordering is shown

### Requirement: Left and right of boom split reported per control and scenario
Each control's marginal reduction SHALL be split into reduction from fewer events and reduction from smaller events, with the interaction apportioned evenly, and the split SHALL be reported per scenario.

#### Scenario: Prevention-heavy scenario
- **GIVEN** a scenario whose reduction derives overwhelmingly from preventing events
- **WHEN** the split is reported
- **THEN** the imbalance is surfaced as an indication of thin damage limitation for that scenario

### Requirement: Every return carries a credible interval and overlaps are surfaced
Every reduction and return figure SHALL carry a credible interval, and overlapping intervals among top-ranked controls SHALL be identified as decision information.

#### Scenario: Top two overlap
- **GIVEN** the two highest-ranked controls whose return intervals overlap
- **WHEN** the ranking is presented
- **THEN** the overlap is stated, together with the implication that the ordering is a best estimate under uncertainty

### Requirement: Control drift is priced
A detected weakening of a deployed control SHALL produce a recomputed expected-loss impact together with the scenarios affected.

#### Scenario: Policy relaxed
- **GIVEN** a control observed at full enforcement that is subsequently observed relaxed on some assets
- **WHEN** the drift is detected
- **THEN** the resulting increase in expected annual loss is computed and reported alongside the affected scenarios

### Requirement: Instrumentation is ranked alongside controls
The system SHALL rank candidate instrumentation actions by their expected reduction in the uncertainty currently driving the ranking, per unit cost, and SHALL indicate whether each could change the funding order.

#### Scenario: Missing attempt telemetry
- **GIVEN** scenarios resting on class priors because no outcome telemetry is available for them
- **WHEN** instrumentation is ranked
- **THEN** actions that would begin producing outcome observations for those stages are listed with their cost, expected effect on interval width, expected time to effect, and whether the ranking order could change

#### Scenario: Uncertainty reduction that changes nothing
- **GIVEN** a candidate action that would narrow an interval without any prospect of altering the ordering
- **WHEN** instrumentation is ranked
- **THEN** it is ranked below actions of comparable cost that could alter the ordering
