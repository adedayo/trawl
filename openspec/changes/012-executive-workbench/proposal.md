# Change: 012-executive-workbench

## Status

**Proposed, not started.** Depends on 011. Final change in the risk-quantification arc.

## Why

Changes 006–011 build a chain from a DNS record to a funding decision. Without a presentation layer that respects the discipline of that chain, it will be read as a dashboard with impressive numbers on it — which is precisely the failure mode both source posts attack.

Three audiences need three different views of one ledger, and the design constraint is that **every view must be walkable down to the observation beneath it**. A board figure that cannot be traced to a timestamped scan is not a board figure; it is a claim.

The second reason is the division of labour the portfolio post names: the board owns the objective function, the CISO owns the optimisation. That is not a slide — it is three specific decisions (the loss threshold, the tail price, the review cadence) which the software must make explicit, attribute, and refuse to proceed without.

## What Changes

- New capability: `executive-workbench`.
- **Three views over one ledger.** Board, CISO and engineer, differing in aggregation, never in underlying numbers.
- **Drill-down as a hard requirement.** Every currency figure walks down to scenario, control, asset, finding, ledger row, and finally to a timestamped observation with its source and coverage state.
- **The board's three decisions are explicit, attributed and dated**: the loss threshold (inherited from insurance retention and materiality, not invented), the tail-relief price, and the review cadence.
- **Quarterly snapshots and the two trend reads** the post specifies: are the credible intervals narrowing, is the ranking stable. Oversight becomes a trend read rather than a reaction to the week's headlines.
- **Export gating.** Material destined for a board or external audience must satisfy the calibration and verification rules, or the export is refused with the specific reasons listed.
- **Engineer challenge flow.** Open a ledger row, dispute it, withdraw it, see the estimate recompute — the chain-of-custody promise made operational.

## The presentation guardrails

These are requirements, not styling preferences, because the credibility of everything upstream depends on them:

1. **The interval is the headline.** Never a bare point estimate, anywhere, in any view or export.
2. **Labels travel.** `illustrative`, `sourced` and `calibrated` appear beside every figure. A sourced figure is presented as assumption propagation, in those terms.
3. **Coverage travels.** Every aggregate states what share of its inputs were actually assessed. "We did not look" is never rendered as "it is fine".
4. **Simplifications are disclosed** where they are relied upon: scenario independence, ambient-only contact, external vantage only.
5. **No composite score.** The workbench SHALL NOT produce a single proprietary-style risk score. The source material's objection to black-box 0–1000 scores applies with full force to anything this tool might emit, and the prohibition belongs in the spec so it survives the first request for one.
6. **No mean-time-to-remediate as a headline figure.** An item never detected never enters the average, so MTTR improves exactly when detection coverage drops — the same exclusion-bias failure mode as raw finding counts, and it gets the same treatment: detail only, always paired with detection coverage, never a board or CISO headline.
7. **Freshness is a headline dial, not a drill-down fact.** How bad and how sure are already required at headline level (the point estimate and its interval); how fresh — how long ago the newest contributing observation was made — gets the same treatment rather than being left to a drill-down click.

## Explicitly Out of Scope

- **No automated board reporting.** Snapshots are produced on request or on cadence; sending them to anyone is a human act.
- **No multi-tenant permission branching.** Consistent with Change 005's scope.
- **No AI-generated executive narrative presented as analysis.** AI may draft prose, clearly attributed, over figures it cannot alter — the existing guardrail, at its most tempting point of violation.

## Impact

- **Angular dashboard**: three view modes, drill-down navigation, snapshot comparison, export flows.
- **Schema**: `boardDecisions`, `quarterlySnapshots`, `exportRecords`, `challengeRecords`.
- **Completes the arc**: `project.md` is updated to describe Trawl as continuous exposure measurement feeding a quantified risk and investment decision engine.
