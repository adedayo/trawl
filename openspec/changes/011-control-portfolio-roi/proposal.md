# Change: 011-control-portfolio-roi

## Status

**Proposed, not started.** Depends on 010.

## Why

With Change 010, Trawl knows what each scenario costs per year. The remaining question is the one a board actually asks: **which of these priced options do we fund first?**

"Security Investment as Board Strategy" answers it — dual-score every control (threat-mitigation potential left of boom, loss-magnitude-attenuation potential right of boom), price each marginally by toggling it out with everything else bought, and rank on aggregate expected-loss reduction per dollar, with a stated price on tail-risk reduction.

Trawl adds something the published method has to take on trust. In the post, a control's effectiveness is an estimate. But a control only delivers where it is actually deployed, and Trawl **measures the coverage denominator** for a meaningful subset of controls: DMARC at enforcement across *n* of *m* sending domains, MTA-STS in enforce mode, DNSSEC signed, TLS configuration, patch latency on internet-reachable services.

That yields the single most useful sentence this whole arc can produce for a CISO:

> "You are paying for this control and receiving 68% of it. Closing the gap costs nothing in licence terms and is worth £X a year."

No commercial platform does this, because none owns both the control register and the external measurement of its deployment.

## What Changes

- New capability: `control-portfolio-roi`.
- **Control register** with dual scores, annualised cost, and the scenarios each control covers.
- **Measured coverage.** Where a control's deployment is externally observable, coverage is computed from Change 006's signal observations rather than asserted. **Effective TMP = TMP × measured coverage.**
- **Marginal pricing.** Toggle a control out with everything else bought; the difference is its contribution. Marginal contributions deliberately do not sum to the portfolio total, and the shared-reduction reconciliation line is reported explicitly.
- **Return calculation** including the tail-relief term: `ROSI = (ΔEL + w · ΔP95) / annualised cost`, with `w` defaulting to zero and owned by the board.
- **Left/right-of-boom split** per control and per scenario, with the interaction divided evenly by averaging the two orders of application — the flag that reveals a scenario with strong prevention and thin damage limitation.
- **Control drift as an ROI event.** The existing `posture-regression` capability already detects a control regressing. Here, that regression is priced: a DMARC policy relaxed from enforcement is not a medium-severity ticket, it is a measurable increase in expected annual loss.
- **Value-of-information ranking.** Rank *instrumentation* alongside controls, by expected interval reduction per pound.

## Value of information — why it belongs here

The published method notes that when the top two controls' intervals overlap, the overlap is itself decision information: collect more data, or fund both. Once uncertainty is first-class, that observation generalises into a ranking.

Publishing a DMARC aggregate report address costs nothing and converts a domain from measured-state-only evidence to outcome evidence, beginning to update the attempt stage within a reporting cycle. Change 006 already detects its absence. So the workbench can say:

> "Three scenarios rest on class priors because you have no attempt telemetry. Publishing report addresses on nineteen domains — no licence cost, one DNS change — would begin outcome-updating the business-email-compromise attempt stage within thirty days."

Instrumentation, ranked beside controls, by expected narrowing per pound. This falls naturally out of the design rather than being bolted on, and it is a capability the source material gestures at without operationalising.

## Explicitly Out of Scope

- **No automated procurement or budget integration.** The output is a ranking with intervals; funding decisions stay with people.
- **No control-efficacy research.** Dual scores are operator or pack inputs with sources, not values Trawl derives. Trawl measures *coverage*, not *efficacy*.
- **No scenario correlation.** Carried forward from Change 010; the independence figure is a floor for shared upstream controls and is disclosed as such.

## Impact

- **Schema**: `controls`, `controlCoverage`, `controlScenarioLinks`, `rosiResults`, `informationValueResults`.
- **Depends on measured coverage** from Change 006's signal observations, including the four-state model — a control whose deployment could not be assessed reports unknown coverage, never full coverage.
