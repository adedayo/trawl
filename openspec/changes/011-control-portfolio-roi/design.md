# Design: 011-control-portfolio-roi

## Control register

```
control {
  id, name,
  type: 'shield' | 'shock-absorber' | 'hybrid',
  tmp,                        // threat mitigation potential, with source and interval
  lmap,                       // loss magnitude attenuation potential, with source
  formsTouched: [...],        // which FAIR forms the LMAP applies to
  annualisedCost,
  scenarios: [...],           // every scenario it covers
  coverageSignals: [...],     // Change 006 signal ids proving deployment, where observable
  coverageMode: 'measured' | 'asserted' | 'unknown'
}
```

`coverageMode` is not cosmetic. A control whose deployment Trawl can observe is in a different epistemic category from one an operator asserts is fully deployed, and the ranking must show which is which.

## Effective TMP

```
effectiveTMP = tmp × coverage
coverage     = assets or domains with the control observed present ÷ assets or domains in scope for it
```

Three rules:

1. **Four-state coverage propagates.** Domains where the control could not be assessed are reported as unknown, excluded from the numerator, and disclosed as a coverage-confidence figure. They are never counted as covered.
2. **Asserted coverage is permitted but marked.** Not every control is externally observable — endpoint detection, backups, awareness training. Those carry asserted coverage with an attestation source and date, and the ranking distinguishes them visually and in export.
3. **The gap is priced.** For every control with measured coverage below 1, the engine computes the expected-loss reduction available from closing the gap alone, at no change in licence cost. This is the headline output of the capability.

## Effective attenuation, per the published rule

```
effectiveAttenuation(control, scenario)
  = lmap × (measured overlay dollars in the forms it touches ÷ scenario's total overlay dollars)
```

Excluded forms drop out of the numerator but stay in the denominator (Change 010). Hence one control carries a different price in each scenario it covers — backups against a fines-heavy breach overlay attenuate a smaller share than against a productivity-heavy ransomware overlay. The engine computes this per scenario and never reuses a single global figure.

## Marginal pricing

```
ΔEL(control)  = EL(portfolio without control) − EL(portfolio with control)
ΔP95(control) = P95(portfolio without control) − P95(portfolio with control)
ROSI          = (ΔEL + w · ΔP95) / annualisedCost
```

Toggle-out with everything else bought, because control effects compound — two 40% controls do not give 80% — and standalone valuation double-counts shared headroom.

**The reconciliation line is mandatory.** Marginal contributions deliberately do not sum to the portfolio total; the difference is reduction that overlapping controls would each deliver alone, and is credited to none of them. The published calculator shows `marginal contributions + shared reduction = total removed`, and so must this. Omitting it invites the reasonable objection that the numbers do not add up.

**Ranking is on the aggregate**, with a per-scenario breakdown showing where the credit comes from. The post's demonstration is decisive: scored inside ransomware alone, backups beat MFA; scored across the portfolio, the order reverses. A control scored inside a single scenario is understated by exactly the scenarios the scoring left out.

## Tail-relief weight

`w` is the price the board puts on a pound of reduction in the one-in-twenty annual loss. Default zero, at which the formula reduces to a pure expected-loss return.

`w` is owned by the board (Change 008 records the ownership). Change 012 refuses to present a ranking as board-mandated while `w` is unset — a board that describes itself as conservative about catastrophic outcomes is asserting `w > 0`, and the method's contribution is to ask how much and re-sort visibly in view of the answer.

## Left/right-of-boom split

Each control's marginal reduction splits into dollars from fewer events (its effective TMP) and dollars from smaller events (its effective attenuation), with the interaction divided evenly by averaging the two orders in which they could apply.

The per-scenario split is a **flag, not a decoration**. A scenario whose reduction is overwhelmingly left of boom has thin damage limitation, and no quantity of additional shields fixes that. The portfolio question the post poses — *per threat, do I have adequate strength on both sides of the boom, and am I buying the next unit at the best marginal rate?* — is answerable only with this split present.

## Control drift as a priced event

`posture-regression` already detects a control weakening. Here it is priced:

```
driftEvent {
  controlId, signalId, previousState, currentState, detectedAt,
  coverageBefore, coverageAfter,
  elImpactPerYear,        // recomputed portfolio delta
  affectedScenarios
}
```

A DMARC policy relaxed from `p=reject` to `p=none` on a sending domain becomes "expected annual loss increased by £X" rather than a medium-severity ticket in a queue. This is the single change most likely to alter how a security team triages posture regressions.

## Value of information

For each candidate instrumentation action, estimate the expected reduction in the width of the intervals that currently drive the ranking:

```
informationValue {
  action,                         // e.g. publish rua on 19 domains
  cost,                           // often zero or near-zero
  targetStage, targetScenarios,
  currentIntervalWidth,
  expectedIntervalWidthAfter,     // from expected outcome-observation volume and evidence weight
  expectedTimeToEffect,
  rankingStabilityImpact          // would the ranking order change?
}
```

Ranked by expected narrowing per pound. Two classes dominate in practice: publishing DMARC report addresses (free, high volume, fast), and deploying one canary per weakness category — the post's own recommendation, since a single dummy credential upgrades an attempt-stage estimate from a category starting point to an observation from your own environment.

The `rankingStabilityImpact` field is what makes this decision-relevant rather than merely interesting: instrumentation that narrows an interval without any prospect of changing the funding order is lower value than instrumentation that could flip it.

## Testing

- Golden-vector tests reproducing the published worked example: four controls, the stated ranking and returns, and the reconciliation of marginal contributions plus shared reduction to the total removed.
- Tail-weight test: at the published non-zero weight, returns move to the published figures.
- Coverage test: unassessed domains never count as covered; coverage confidence is reported.
- Gap-pricing test: a control at partial measured coverage reports the value of closing the gap.
- Per-scenario attenuation test: one control produces different attenuation in two scenarios with different overlays.
- Aggregate-versus-single-scenario test: reproduce the published order reversal between the two scoring bases.
- Drift test: a relaxed policy produces a priced expected-loss increase.
