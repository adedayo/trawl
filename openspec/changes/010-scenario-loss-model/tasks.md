# Tasks: 010-scenario-loss-model

**Do not start until Changes 008 and 009 are implemented.**

## Phase 0 — Preconditions
- [ ] Packs supply scenario anchors, taxonomy mapping, overlays and judgment parameters
- [ ] Exploitation probabilities available per instance from Change 009
- [ ] Deployment configuration surface for incident history and costed events (organisation data, never in a shipped pack)

## Phase 1 — Scenario entities
- [ ] `scenarios` with taxonomy mapping, counting rule, entry-path definition
- [ ] `incidentHistory` with countability against the scenario's counting rule
- [ ] `costedEvents` with itemised form-level dollars
- [ ] Validation: scenario shares sum to one; entry-path definitions displayed

## Phase 2 — Frequency update
- [ ] Gamma-Poisson update; posterior retained
- [ ] Fractional, automatically extending observation window
- [ ] Shared prior strength with recorded, bounded per-scenario exceptions
- [ ] Test: clean history lowers the rate; near-miss failing the counting rule does not enter the count

## Phase 3 — Magnitude update
- [ ] σ from the anchor pair, held fixed
- [ ] Log-cost blend; Normal posterior on the log-median
- [ ] Expected cost per event via the lognormal mean
- [ ] **Test**: expected annual loss uses the mean, asserting the skew factor explicitly

## Phase 4 — Exposure bridge
- [ ] Entry-path composition per scenario, from asset, weakness and control classes
- [ ] Baseline establishment with a minimum observation period; unavailable rather than defaulted before it
- [ ] Bridge A exposure factor: neutral at baseline, bounded both directions, declared on output
- [ ] Bridge B pseudo-observation: opt-in, weight-capped, withdrawable, refused for unobservable entry paths
- [ ] Tests: neutrality at baseline; remediation lowers the adjusted rate; refusal for insider-type scenarios

## Phase 5 — Overlay and gate
- [ ] Six-form overlay summing to the scenario anchor
- [ ] Four-step gate as required fields; exclusions record the failing leg and reopen trigger
- [ ] Validation failure when a form is neither measured nor excluded
- [ ] Test: excluded forms stay in the attenuation denominator

## Phase 6 — Insurance read
- [ ] Retention, sublimits and limit as programme configuration
- [ ] Per-scenario insurable, below-retention, sublimit-bound and uninsurable shares

## Phase 7 — Simulation
- [ ] Seeded portfolio Monte Carlo; Poisson counts, lognormal severities
- [ ] Exceedance curve with independence and truncation disclosures
- [ ] Credible intervals on every reported figure
- [ ] Judgment-parameter sensitivity sweep with stability reporting

## Phase 8 — Golden vectors
- [ ] **Reproduce the published worked example**: three scenarios, stated priors and events, aggregate expected annual loss and its 90% interval
- [ ] Reproduce the published sensitivity gut-check figures for the shared prior strength

## Exit Criteria

The published worked example reproduces exactly from configuration alone; expected annual loss provably uses the lognormal mean; a quarter of remediation with no incidents moves expected annual loss through a bounded, declared, baseline-neutral exposure factor and through nothing else; every form in every overlay is either measured with a named source or excluded with a failing leg and a reopen trigger; and every figure carries a credible interval with the independence simplification disclosed.
