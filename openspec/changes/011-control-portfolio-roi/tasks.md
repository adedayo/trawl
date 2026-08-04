# Tasks: 011-control-portfolio-roi

**Do not start until Change 010 is implemented.**

## Phase 0 — Preconditions
- [ ] Scenario expected losses and overlays available from Change 010
- [ ] Signal observations with four-state coverage available from Change 006
- [ ] Dual-score sources agreed: which controls take pack values, which take operator values with attestation

## Phase 1 — Control register
- [ ] `controls`, `controlScenarioLinks`, `controlCoverage`
- [ ] Validation: both scores, forms touched, annualised cost, at least one covered scenario
- [ ] Coverage-signal binding for externally observable controls
- [ ] Asserted-coverage path with attestation source and date

## Phase 2 — Measured coverage
- [ ] Coverage computation from signal observations
- [ ] Four-state propagation: unassessed excluded from the numerator, reported as coverage confidence
- [ ] `effectiveTMP = tmp × coverage`
- [ ] Coverage-gap valuation: expected-loss reduction from closing the gap at no licence cost
- [ ] Tests: unassessed never counts as covered; gap value reported for partial coverage

## Phase 3 — Attenuation
- [ ] Per-scenario effective attenuation from the overlay, excluded forms in the denominator only
- [ ] Test: one control, two overlays, two different attenuations

## Phase 4 — Marginal pricing and ranking
- [ ] Toggle-out ΔEL and ΔP95 per control
- [ ] Shared-reduction reconciliation line
- [ ] Aggregate ranking with per-scenario breakdown
- [ ] Tail-relief weight in the return; default zero
- [ ] Credible intervals; overlap identification among top ranks
- [ ] **Golden-vector tests**: reproduce the published worked example's ranking, returns, reconciliation and the order reversal between single-scenario and aggregate scoring

## Phase 5 — Boom split
- [ ] Left/right split per control, interaction apportioned by averaging both orders
- [ ] Per-scenario split with thin-damage-limitation flag

## Phase 6 — Control drift pricing
- [ ] Bind `posture-regression` events to controls
- [ ] Recompute portfolio delta on drift; report affected scenarios
- [ ] Test: relaxed policy produces a priced expected-loss increase

## Phase 7 — Value of information
- [ ] Candidate instrumentation catalogue (report addresses, canaries per weakness class, log ingestion)
- [ ] Expected interval-width reduction from expected observation volume and evidence weight
- [ ] Ranking by expected narrowing per unit cost, with ranking-stability impact
- [ ] Test: uncertainty reduction that cannot change the order ranks below one that can

## Phase 8 — Remediation queue, acceptance and capacity
- [ ] `remediationQueue`: rank by ΔE[loss] per unit remediation cost through the estimation chain; store severity order alongside and surface divergences
- [ ] Ranking tests: access barrier inverts severity order; accrued time at risk outranks recency; instance aggregation saturates; high severity with negligible contact probability retires negligible risk
- [ ] Cross-scenario credit summed per intervention; test that equal proportional reductions at contact and success stages price equally on a single scenario, and that breadth alone decides between them
- [ ] `queueDisplacement` per detection source, per period; prohibit finding counts as a prioritisation metric
- [ ] `acceptedRisks`: priced expected loss, owner, decision date, rationale, review date; validation reports undecided findings
- [ ] Aggregate accepted-risk figure, computed and exported alongside risk actively retired
- [ ] Automatic re-raise on material rank or expected-loss change, including change driven by elapsed time at risk alone; lapsed acceptances reported as lapsed
- [ ] `remediationCapacity` as a stated input; detect a stable unworked queue head across periods and report capacity as the binding constraint
- [ ] Test: with capacity unstated, the plan declares the queue head advisory only
- [ ] Marginal value schedule for additional capacity increments; identify the increment at which value falls below stated cost; test that the schedule is non-increasing
- [ ] Ratio ranking test: cheap-modest outranks expensive-large under a binding constraint
- [ ] Commitment window: freeze the queue head, re-rank at boundaries, raise material breaches as explicit interrupts with the justifying comparison
- [ ] Ranking-stability reporting in both directions — never-changes and always-changes are both findings
- [ ] `committedValue` recorded at commitment with pack version; loss-retired credit reads it; retrospective revaluation refused
- [ ] Risk-weighted backlog total as primary measure, count reported separately, divergence surfaced

## Exit Criteria

The published worked example's control ranking, returns and reconciliation reproduce exactly; a control at partial measured coverage is ranked on its effective rather than nominal mitigation and reports the priced value of closing its gap; a relaxed policy on one sending domain produces a currency figure rather than a ticket; and the workbench can name the cheapest instrumentation action that would most narrow the uncertainty currently driving the funding order.
