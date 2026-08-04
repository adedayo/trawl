# Change: 010-scenario-loss-model

## Status

**Proposed, not started.** Depends on 008 and 009.

## Why

Change 009 produces per-instance exploitation probabilities. Boards do not fund instances; they fund protection against **loss events**. "Security Investment as Board Strategy" supplies the missing layer: the scenario as the unit of analysis, two closed-form Bayesian updates (one for how often loss events happen, one for what they cost), and a six-form decision overlay on the dollars.

The value of running that engine inside Trawl rather than in a spreadsheet is that one of its inputs stops being static. In the published method, a scenario's frequency posterior moves only when you have an incident. That means a CISO who spent a quarter closing exposed administrative interfaces sees **no movement whatsoever** in expected annual loss — the model cannot see the work. Trawl can, because it measures the entry-path exposure that precedes the event.

## What Changes

- New capability: `scenario-loss-model`.
- **Scenarios as first-class entities**, with the taxonomy mapping to published incident patterns carried from Change 008's packs.
- **Frequency update**: `λ_updated = (β₀ · λ_industry + k) / (β₀ + T)`, with the Gamma posterior retained for intervals.
- **Magnitude update**: blended in log-cost, `updated anchor = (m₀ · industry anchor + Σ your costed events) / (m₀ + n)`, with σ fixed from the P50/P95 anchor pair and held through the update.
- **Expected annual loss uses the lognormal mean**, not the median. The post is emphatic: skipping this understates expected loss by exactly `e^(σ²/2)`.
- **Six-form FAIR overlay** per scenario, with the four-step measure/exclude gate, and exclusions recorded with the failing leg and a reopen trigger.
- **Exposure as a frequency modifier** — the bridge from Trawl's bottom-up measurement to the top-down scenario model. Described below and specified carefully, because it is the step most open to abuse.
- **Monte Carlo** over the portfolio for the annual-loss exceedance curve.

## The bridge, and its honesty constraints

A compromised asset is not a loss event, and substituting one for the other would be indefensible. Two bridges, both explicit, both labelled.

**Bridge A — exposure as a TMP-like modifier (default).** Attack surface sits left of boom. The aggregate susceptibility of the assets forming a scenario's entry path acts as an *exposure factor* on λ, structurally identical to a control's threat-mitigation potential, with a **neutral baseline**: when exposure equals the deployment's own historical baseline, λ is unchanged. It moves only as the surface degrades or improves relative to that baseline. This keeps the industry anchor doing the work it is good at, keeps the update to one line of arithmetic, and makes "we closed twelve exposed KEV-listed services this quarter" appear as movement in expected annual loss.

**Bridge B — entry-path pseudo-observation (opt-in, `illustrative` by default).** For scenarios with a directly observable entry path, the aggregated unconditional exploitation probability over the window enters the Gamma update as a low-weight pseudo-observation under the same evidence-weighting discipline that governs Layer 3. Powerful and easy to abuse, hence opt-in, weight-capped, and label-gated.

Both bridges are declared on every figure they touch. A board reading an expected-loss number is entitled to know whether the estimate moved because of an incident, an industry revision, or a change in measured exposure.

## Explicitly Out of Scope

- **No control pricing or ranking.** Change 011.
- **No scenario correlation modelling.** Scenarios are treated as independent, as in the published method, with the direction of the resulting error disclosed on every aggregate. Shared kill chains raise the value of upstream shared controls; that is portfolio context, not a model term here.
- **No automatic reconciliation with the enterprise risk register.** Remains a human, cadenced step.

## Impact

- **Schema**: `scenarios`, `incidentHistory`, `costedEvents`, `lossFormOverlays`, `exclusionRecords`, `scenarioEstimates`.
- **New deployment configuration**: incident history and costed events are organisation-specific data under the existing `portability-config` discipline, never shipped in a pack.
- **Compute**: portfolio Monte Carlo, seeded and reproducible.
