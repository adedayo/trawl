# Capability: control-portfolio-roi

## Purpose

Price and rank security controls by the expected loss they remove per unit of cost, crediting each across every scenario it covers, with control coverage measured from external observation wherever it is observable — so that the gap between a control purchased and a control delivered becomes a priced, closable finding.

The same pricing serves the remediation queue. Finite remediation capacity makes ordering, not volume, the thing that determines how much loss is retired; so findings are ranked by computed loss reduction per unit cost, everything below the capacity line becomes explicitly accepted and priced risk rather than an unowned backlog, and capacity itself is reported as the constraint it is.

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

### Requirement: Coverage freshness decays and continuous validation sustains it
A measured coverage figure SHALL carry the timestamp of the validation that produced it, and its evidential weight SHALL decay with age on the same freshness terms as any other measured state (Change 008). Successful re-validation SHALL restore full weight without increasing it.

Repeated validation does not make a control more effective and SHALL NOT raise its efficacy estimate. What it buys is that the coverage figure is repeatedly given an opportunity to fall and does not — the assumption is upheld rather than strengthened. See RISK-ARC §5b.

#### Scenario: Coverage evidence ages
- **GIVEN** a control whose coverage was validated once, nine months ago
- **WHEN** its effective mitigation is computed today
- **THEN** the coverage figure is presented with widened uncertainty reflecting the age of the validation

#### Scenario: Continuous validation holds coverage at full weight
- **GIVEN** a control validated daily with an unchanged result
- **WHEN** its effective mitigation is computed
- **THEN** the coverage figure carries full weight, and the repeated validations do not raise the efficacy estimate above the single-observation value

#### Scenario: Validation stops
- **GIVEN** a control previously validated continuously, whose validation has not run for an extended period
- **WHEN** its effective mitigation is computed
- **THEN** the coverage figure decays toward the asserted-coverage treatment and is marked as no longer measured

### Requirement: Remediation is ranked by expected loss reduction per unit cost
The remediation queue SHALL be ordered by the expected reduction in loss per unit of remediation cost, computed through the estimation chain, and SHALL NOT be ordered by severity label, vulnerability score or finding count. Where the model's order differs from the severity order, both SHALL be shown.

#### Scenario: Severity and risk order diverge on access barrier
- **GIVEN** a high-severity weakness on an asset behind a strong access barrier, and a medium-severity weakness on a directly exposed asset
- **WHEN** the queue is ranked
- **THEN** ranking is determined by computed loss reduction, the divergence from severity order is shown, and the access barrier's contribution to the difference is attributable

#### Scenario: Accrued exposure outranks recency
- **GIVEN** a medium-severity weakness open for an extended period and a high-severity weakness detected today
- **WHEN** the queue is ranked
- **THEN** accrued time at risk is reflected in the ranking and may place the older item above the newer

#### Scenario: Instance aggregation saturates
- **GIVEN** a weakness present on many instances
- **WHEN** the value of remediating additional instances is computed
- **THEN** the marginal value declines with each instance, consistent with aggregation over independent contact probabilities

#### Scenario: High severity with negligible contact probability
- **GIVEN** a high-severity weakness whose computed contact probability is negligible
- **WHEN** the queue is ranked
- **THEN** its computed risk reduction is reported as negligible, and any argument for prioritising it regardless is recorded as a stated non-risk justification

### Requirement: Cross-scenario breadth is computed, not assumed from stage position
The credit given to a remediation or control SHALL be summed across every scenario whose estimate it affects, and no stage of the estimation chain SHALL receive a ranking preference on account of its position in the chain.

#### Scenario: Equal proportional reductions at different stages
- **GIVEN** two candidate interventions delivering equal proportional reduction, one at the contact stage and one at the success stage, affecting the same single scenario
- **WHEN** each is priced
- **THEN** their computed loss reductions are equal

#### Scenario: Breadth decides
- **GIVEN** a contact-stage intervention affecting many scenarios and a success-stage intervention affecting one
- **WHEN** each is priced
- **THEN** the contact-stage intervention ranks higher by virtue of the summed credit across scenarios, and the number of scenarios credited is reported

#### Scenario: Late-stage intervention with wide reuse
- **GIVEN** a success-stage intervention shared across more scenarios than a candidate contact-stage intervention
- **WHEN** each is priced
- **THEN** the success-stage intervention may rank higher, and stage position does not override the computed breadth

### Requirement: Detection is valued by queue displacement, not finding volume
The system SHALL report, per detection source, how often its findings changed the composition of the capacity-limited head of the remediation queue, and SHALL NOT present counts of findings discovered or closed as measures of prioritisation effectiveness.

#### Scenario: Volume without displacement
- **GIVEN** a detection source producing many findings, none of which enter the head of the queue
- **WHEN** its value is reported
- **THEN** it is reported as having produced no displacement in the period, and the finding count is not presented as value delivered

#### Scenario: Single displacing finding
- **GIVEN** a detection source producing one finding that displaces the top of the queue
- **WHEN** its value is reported
- **THEN** the displacement and the loss reduction it made available are reported

### Requirement: Deprioritised findings become explicit accepted risk
A finding that is known and not scheduled for remediation SHALL be recorded as accepted risk with its computed expected loss, an owner, a decision date and a stated rationale. The system SHALL NOT permit a known finding to remain in an unowned, unpriced state.

#### Scenario: Finding deprioritised
- **GIVEN** a finding ranked below the capacity threshold
- **WHEN** the period's plan is set
- **THEN** the finding is recorded as accepted risk with its priced expected loss, owner and date, and is distinguishable from a finding awaiting triage

#### Scenario: Undecided finding
- **GIVEN** a finding neither scheduled nor accepted
- **WHEN** the period's plan is validated
- **THEN** validation reports it as an undecided finding requiring a decision, and it is not silently carried

### Requirement: Accepted risk is aggregated and reported as a single priced figure
The system SHALL compute and report the total expected loss represented by all accepted risk, and SHALL present it alongside the expected loss being actively retired.

#### Scenario: Individually small, collectively material
- **GIVEN** a register of individually low-ranked accepted risks whose aggregate expected loss is material
- **WHEN** the position is reported
- **THEN** the aggregate is reported as currency and is not obscured by the individual rankings

### Requirement: Acceptance decisions expire and are re-raised on material rank change
An acceptance SHALL carry a review date, and the system SHALL re-raise an accepted risk for decision when its computed rank or expected loss changes materially, including through the passage of time at risk alone.

#### Scenario: Rank changes through elapsed time alone
- **GIVEN** a risk accepted when its computed expected loss was below the threshold, with no change to the estate
- **WHEN** accrued time at risk raises its computed expected loss above the threshold
- **THEN** the acceptance is re-raised for decision and the cause is attributed to elapsed exposure rather than to a change in the estate

#### Scenario: Acceptance lapses
- **GIVEN** an accepted risk whose review date has passed
- **WHEN** the position is reported
- **THEN** the acceptance is reported as lapsed and is not counted as a current decision

### Requirement: Capacity is modelled as an explicit constraint
The remediation plan SHALL be computed against a stated remediation capacity, and where the ranked head of the queue is unchanged and unworked across consecutive periods the system SHALL report capacity as the binding constraint.

#### Scenario: Stable unworked head
- **GIVEN** a queue whose highest-ranked items are unchanged and unremediated across consecutive periods
- **WHEN** the position is reported
- **THEN** capacity is reported as the binding constraint, and the report states that further detection or re-ranking will not reduce loss

#### Scenario: Capacity not stated
- **GIVEN** no stated remediation capacity
- **WHEN** the remediation plan is computed
- **THEN** the plan reports that the capacity constraint is unstated and the queue head is advisory only

### Requirement: The marginal value of additional capacity is computed
The system SHALL compute the expected loss reduction obtainable from each additional increment of remediation capacity beyond the stated capacity, SHALL report the schedule of these marginal values, and SHALL identify the increment at which the marginal value falls below a stated cost per increment.

#### Scenario: Funding decision
- **GIVEN** a stated capacity, a ranked queue and a cost per increment of capacity
- **WHEN** the marginal value schedule is computed
- **THEN** it reports the loss reduction available from each successive increment and the point at which further increments no longer exceed their cost

#### Scenario: Diminishing returns
- **GIVEN** a marginal value schedule
- **WHEN** it is inspected
- **THEN** successive increments are non-increasing in value, reflecting that higher-ranked items are consumed first

### Requirement: Cost-effectiveness ranks independently of severity magnitude
Ranking SHALL use loss reduction per unit of remediation cost, such that a low-cost item of modest reduction MAY rank above a high-cost item of larger reduction.

#### Scenario: Cheap fix outranks expensive one
- **GIVEN** a low-cost remediation with modest loss reduction and a high-cost remediation with larger absolute loss reduction
- **WHEN** the queue is ranked under a binding capacity constraint
- **THEN** the ratio determines the order, and the absolute reduction alone does not

### Requirement: The queue head is committed for a window
The ranked head of the queue SHALL be committed for a stated window during which re-ranking does not displace work in progress, and SHALL be re-ranked at each window boundary. An item whose computed expected loss rises above the committed head within a window SHALL be raised as an interrupt rather than silently reordering the queue.

#### Scenario: Re-ranking within a window
- **GIVEN** committed work in progress and a re-ranking that changes the order below the committed head
- **WHEN** the queue is recomputed mid-window
- **THEN** the committed head is unchanged and the new order takes effect at the next window boundary

#### Scenario: Material breach interrupts
- **GIVEN** a newly detected finding whose computed expected loss exceeds that of a committed item
- **WHEN** the queue is recomputed mid-window
- **THEN** the finding is raised as an explicit interrupt with the comparison that justifies it, rather than reordering the queue silently

#### Scenario: Ranking stability is reported in both directions
- **GIVEN** successive windows
- **WHEN** ranking stability is reported
- **THEN** both an unchanging ranking and a ranking changing at every window are reported as findings, the first about detection and the second about commitment

### Requirement: Remediation credit is valued at commitment, not at completion
The expected loss reduction credited to a completed remediation SHALL be the value computed from the ranking parameters and pack version in force when the item entered the committed queue head, recorded at that time. Completed work SHALL NOT be revalued using later parameters.

#### Scenario: Parameters change after commitment
- **GIVEN** an item committed at one pack version and completed after a pack update that raises its computed value
- **WHEN** loss retired is reported
- **THEN** the credit is the value recorded at commitment, and the parameter change is reported separately as a parameter change

#### Scenario: Retrospective revaluation attempted
- **GIVEN** a completed remediation
- **WHEN** a revaluation using current parameters is attempted
- **THEN** it is refused, on the basis that the price is fixed before the work is selected

### Requirement: Risk-weighted backlog is reported in preference to backlog count
The system SHALL report the aggregate priced expected loss of open findings as the primary backlog measure, SHALL report the count separately, and SHALL surface periods in which the two move in opposite directions.

#### Scenario: Count rises while priced total falls
- **GIVEN** a period in which the highest-priced findings were remediated and a larger number of low-priced findings were detected
- **WHEN** the backlog is reported
- **THEN** the priced total is reported as improved, the count is reported as increased, and the divergence is stated explicitly

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
