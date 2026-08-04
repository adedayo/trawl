# Design: 012-executive-workbench

## One ledger, three aggregations

```
Board        aggregate EL before/after · exceedance curve · ROSI ranking ·
             three board decisions · sensitivity sweep · quarterly trend
                     │  drill down
CISO         per-scenario EL and boom split · coverage and assessment completeness ·
             susceptibility trend · control drift · instrumentation ranking
                     │  drill down
Engineer     finding · instances · stage estimates · ledger rows ·
             challenge and recompute · remediation scope and coverage
                     │  drill down
Observation  timestamped signal · source · evidence · coverage state ·
             tool version · pack version
```

The views differ only in aggregation. A CISO and a board member looking at the same scenario see the same number at different resolutions, never two numbers reconciled by hand. This is enforceable because every view reads the same stored estimates rather than recomputing its own.

## Drill-down is a required capability, not a convenience

Every currency figure must be walkable to its observations in a bounded number of steps, and the path must be exercised by a test. The argument is the post's:

> When engineering disputes an estimate, remove the challenged modifier and recompute. Nothing else in the chain moves.

If the path from £252K of expected annual loss to "this host answered on port 3389 on 14 July" cannot be walked in the product, the chain of custody is a claim about the architecture rather than a property of it.

## The board's three decisions

```
boardDecision {
  kind: 'loss-threshold' | 'tail-price' | 'review-cadence',
  value, rationale,
  decidedBy, decidedAt, minutesReference,
  status: 'set' | 'unset' | 'stale'
}
```

- **Loss threshold** is *inherited*, not invented: insurance retention and the financial materiality line are decisions the board has already taken. The UI asks which existing decision it is inheriting, and records the reference.
- **Tail price** `w` is the one genuinely new number the board must supply. Unset is a distinct state from zero. A board describing itself as conservative about catastrophic outcomes is asserting `w > 0`; the workbench's job is to ask how much and re-sort the ranking visibly in view of the answer.
- **Review cadence** defaults to quarterly, on the financial reporting rhythm.

While `tail-price` is unset, rankings are shown with a standing notice that they price the average year only, and board-pack export is refused. This is the software equivalent of the post's investment-policy-statement framing: the optimisation cannot be presented as executing a mandate that has not been given.

## Quarterly snapshots and the two trend reads

```
quarterlySnapshot {
  periodEnd, packVersion, toolVersions, seed,
  scenarioEstimates[], controlRankings[], coverageSummary,
  boardDecisionsAtTime[],
  intervalWidths[],          // for the narrowing read
  rankingOrder[]             // for the stability read
}
```

Two reads, as specified:

1. **Are the intervals narrowing?** They should, as history and telemetry accumulate. If they are not, the programme is not learning, and that is the finding.
2. **Is the ranking stable?** If the order flips between quarters or across the judgment-parameter sweep, the purchase depends on an unresolved judgment — which the post identifies as itself the finding, not a defect to be smoothed over.

Snapshots are immutable and store the pack version, tool versions and seed. A pack change between snapshots is marked on the trend, and the series either re-baselines or declares the discontinuity — never silently continues.

## Export gating

```
exportGate(audience) → allowed | refused(reasons[])
```

For `board` and `external` audiences:

- Every parameter behind a presented figure is `verified` or `locally-overridden`; `needs-verification` refuses, listing each offending parameter. This is the Prior Estimator's own rule — *class priors marked needs verification require independent source checks before board or external use* — enforced rather than trusted.
- No figure is `illustrative`.
- Coverage is above the configured floor, or the shortfall is presented.
- The tail price is set.
- Sourced figures are accompanied by their assumption-propagation statement.

Refusal lists specific reasons and the action to resolve each. Every export is recorded with its snapshot, gate result and audience, so an exported figure can always be traced to the state that produced it.

## Engineer challenge flow

1. Open any figure; drill to its ledger rows.
2. Dispute a row: reason, disputing party, date.
3. Withdraw it and see the recomputation — mean and, for variance-bearing measured-state rows, interval.
4. The challenge and its outcome are retained. A withdrawn row is never deleted; its history is part of the record.

This is the mechanism that earns engineering's trust, and the post is explicit about why it matters: *"A model nobody can interrogate earns no trust from the people whose roadmap it is supposed to move."*

## Presentation guardrails, specified

- **Interval as headline.** The rendering component takes an estimate type that cannot be constructed without one, so a bare point estimate is not expressible in the UI layer.
- **No composite score.** Explicitly prohibited. The tool emits probabilities, currency, intervals and rankings — quantities with units and derivations. A 0–1000 index has neither, and every argument the source material makes against black-box scores would apply to one produced here.
- **No mean-time-to-remediate headline.** Same exclusion-bias failure as raw finding counts, one level removed: an item never found never enters the average, so the average improves exactly when coverage drops. Treated identically to finding counts — operational detail only, always paired with the coverage figure for the same class, never promoted to board or CISO level.
- **Freshness as headline, not drill-down.** "How bad" (the value) and "how sure" (the interval) are already headline-mandatory; "how fresh" (age of the newest contributing observation, or the stalest one for an aggregate) gets the same estimate-type treatment so it can't be silently omitted the way MTTR-style averages currently could be.
- **Simplification disclosures** attach to the figures that rely on them: independence on aggregates, ambient-only on contact, external-vantage-only on coverage.
- **AI narrative is attributed and inert.** Drafted prose is labelled as such and cannot alter a figure; the guardrail is tested at this layer because this is where the temptation to let it "summarise" a number is greatest.

## Testing

- Drill-down test: from every board-level currency figure, a bounded path reaches a timestamped observation.
- Consistency test: the same quantity in board, CISO and engineer views is byte-identical, not recomputed.
- Gate tests: unverified parameter refuses; illustrative figure refuses; unset tail price refuses board export.
- Immutability test: a snapshot is unchanged by a later pack update.
- Challenge test: withdrawing a variance-bearing row widens the displayed interval.
- Guardrail tests: no composite score is emitted anywhere; no view renders a point estimate without its interval; AI annotation cannot alter a displayed figure.
- Accessibility: Playwright automated accessibility scanning across all three views, per the existing project testing requirements.
