# Capability: scenario-loss-model

## Purpose

Turn measured exposure and per-instance exploitation probability into per-scenario expected annual loss in currency, using two closed-form Bayesian updates anchored to published industry data and pulled by the organisation's own history, with a six-form decision overlay and an honest account of what was measured, what was excluded, and why.

## ADDED Requirements

### Requirement: The scenario is the unit of analysis
The system SHALL model named loss-event scenarios, each carrying its own frequency update, magnitude update and loss-form overlay, with judgment parameters shared across scenarios.

#### Scenario: Per-scenario estimates
- **GIVEN** a configured portfolio of scenarios
- **WHEN** estimates are produced
- **THEN** each scenario reports its own updated frequency, updated magnitude and expected annual loss, using the shared judgment parameters

### Requirement: Frequency update from industry prior and own history
Scenario frequency SHALL be updated by blending the industry prior with the organisation's own event count over its observation window, retaining the posterior distribution for interval reporting.

#### Scenario: Clean history lowers the rate
- **GIVEN** a scenario with an industry prior and zero events across the observation window
- **WHEN** the frequency is updated
- **THEN** the updated rate is below the industry prior, and the posterior distribution is retained

#### Scenario: Window extends automatically
- **GIVEN** a re-run one quarter after the previous run with no new events
- **WHEN** the frequency is updated
- **THEN** the observation window has extended by a quarter-year and the prior's share of the denominator has decreased

### Requirement: Event counts match the source population
Each scenario SHALL record the counting rule defining which events are countable against its industry prior, and events failing that rule SHALL NOT contribute to the count.

#### Scenario: Near-miss excluded
- **GIVEN** a scenario whose prior derives from publicly recorded incidents, and an internal near-miss that would not have surfaced publicly
- **WHEN** the frequency is updated
- **THEN** the near-miss does not contribute to the event count, and its exclusion is recorded

### Requirement: Magnitude update in log-cost with fixed spread
Per-event cost SHALL be modelled as lognormal, with the spread derived from the median and upper-percentile anchor pair and held fixed through the update, and the central value blended in log-cost between the industry anchor and the organisation's costed events.

#### Scenario: First costed event
- **GIVEN** a scenario with an industry anchor and one fully costed event
- **WHEN** the magnitude is updated
- **THEN** the anchor moves toward the costed event by the share implied by the prior strength, and the spread is unchanged

### Requirement: Expected loss uses the distribution mean
Expected annual loss SHALL be computed using the mean of the per-event cost distribution, not its median.

#### Scenario: Skewed cost distribution
- **GIVEN** a scenario whose cost distribution has a material spread
- **WHEN** expected annual loss is computed
- **THEN** it exceeds the product of the frequency and the median cost, by the skew factor implied by the spread

### Requirement: Exposure adjustment is relative to the deployment's own baseline
Where measured exposure modifies scenario frequency, the modifier SHALL be neutral when current exposure equals the deployment's established baseline, SHALL be bounded, and SHALL be unavailable rather than defaulted before sufficient observation history exists.

#### Scenario: Exposure at baseline
- **GIVEN** a scenario whose entry-path exposure equals the established baseline
- **WHEN** the frequency is adjusted for exposure
- **THEN** the adjusted frequency equals the unadjusted frequency

#### Scenario: Insufficient history
- **GIVEN** a deployment with less observation history than the configured minimum
- **WHEN** exposure adjustment is requested
- **THEN** no adjustment is applied and the scenario reports that exposure adjustment is not yet available

#### Scenario: Remediation moves the number
- **GIVEN** a scenario whose entry-path exposure has fallen materially below the baseline following remediation
- **WHEN** the frequency is adjusted
- **THEN** the adjusted frequency is below the unadjusted frequency, within the configured bound

### Requirement: Exposure adjustment is declared on every derived figure
Any figure computed with a non-neutral exposure modifier SHALL state that it is exposure-adjusted, report the modifier, and remain readable without the adjustment.

#### Scenario: Adjusted figure presented
- **GIVEN** an expected annual loss computed with a non-neutral exposure modifier
- **WHEN** it is presented
- **THEN** the modifier is shown and the unadjusted figure is available alongside it

### Requirement: Entry-path pseudo-observation is opt-in and restricted
Treating aggregated exploitation probability as a pseudo-observation in the frequency update SHALL be disabled by default, weight-capped, recorded as a withdrawable ledger row, and unavailable for scenarios whose entry path is not observable from the external vantage.

#### Scenario: Unobservable entry path
- **GIVEN** a scenario whose entry path cannot be observed externally
- **WHEN** the pseudo-observation mode is requested for it
- **THEN** it is refused, with the reason stated

#### Scenario: Withdrawable
- **GIVEN** a scenario using a pseudo-observation
- **WHEN** the pseudo-observation is withdrawn
- **THEN** the frequency recomputes from the remaining evidence

### Requirement: Six-form overlay with a measure-or-exclude gate
Each scenario SHALL carry a loss-form overlay, and each form SHALL be either measured with a named data source or consciously excluded with the failing materiality, obtainability or decision-relevance test recorded together with a reopen trigger.

#### Scenario: Unmeasurable form
- **GIVEN** a form that applies to a scenario but cannot be obtained
- **WHEN** the overlay is completed
- **THEN** the form is recorded as excluded, naming obtainability as the failing test, with a stated trigger that would reopen it

#### Scenario: Overlay completeness
- **GIVEN** a scenario overlay with a form that is neither measured nor explicitly excluded
- **WHEN** the scenario is validated
- **THEN** validation fails

### Requirement: Excluded forms remain in the attenuation denominator
Excluded loss-form dollars SHALL be omitted from the numerator when computing a control's effective attenuation, and SHALL remain in the denominator.

#### Scenario: Control touching an excluded form
- **GIVEN** a scenario with an excluded form and a control that would have attenuated it
- **WHEN** the control's effective attenuation is computed
- **THEN** the excluded dollars do not raise the attenuation, and the scenario's total overlay dollars are unchanged

### Requirement: Insurance read per scenario
The overlay SHALL report, per scenario, the share of expected cost reachable by the insurance programme, the share falling below retention, the share where a sublimit binds before the headline limit, and the structurally uninsurable share.

#### Scenario: Uninsurable share identified
- **GIVEN** a scenario whose overlay includes reputation and competitive-advantage dollars
- **WHEN** the insurance read is produced
- **THEN** those dollars are reported as structurally uninsurable

### Requirement: Portfolio simulation with disclosed simplifications
Annual-loss exceedance SHALL be produced by seeded simulation across scenarios, and any presentation SHALL disclose that scenarios are treated as independent and disclose any axis truncation.

#### Scenario: Exceedance curve presented
- **GIVEN** a portfolio exceedance curve
- **WHEN** it is displayed
- **THEN** it discloses the independence simplification and any truncation point

#### Scenario: Reproducibility
- **GIVEN** a stored simulation result with its seed, inputs and pack version
- **WHEN** it is re-run
- **THEN** the result is identical

### Requirement: Credible intervals on every reported figure
Every frequency, magnitude and expected-loss figure SHALL carry a credible interval, and no such figure SHALL be presented as a bare point estimate.

#### Scenario: Data-poor scenario
- **GIVEN** a scenario with no costed events
- **WHEN** its expected annual loss is presented
- **THEN** it carries a wide credible interval reflecting the state of the evidence

### Requirement: Judgment-parameter sensitivity sweep is produced
The system SHALL produce results at the published sweep points for the shared judgment parameters, and SHALL report whether conclusions are stable across the sweep.

#### Scenario: Sweep reported
- **GIVEN** a configured portfolio
- **WHEN** the sensitivity sweep is run
- **THEN** results are produced at each sweep point and the stability of the ordering across the sweep is reported

### Requirement: Prior-strength exceptions are recorded and bounded
Per-scenario deviations from the shared prior strength SHALL each record the test they fail and a rationale, and the system SHALL warn when the number of exceptions exceeds the configured expectation.

#### Scenario: Excessive exceptions
- **GIVEN** a portfolio in which most scenarios carry a prior-strength exception
- **WHEN** the portfolio is validated
- **THEN** a warning is raised that the shared-parameter discipline is not being observed
