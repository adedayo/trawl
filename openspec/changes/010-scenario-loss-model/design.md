# Design: 010-scenario-loss-model

## The two updates

**Frequency (left of boom).**

```
λ_updated = (β₀ · λ_industry + k) / (β₀ + T)
posterior ~ Gamma(shape = β₀ · λ_industry + k, rate = β₀ + T)
```

`k` is the count of material events of that class; `T` is the observation window in years, fractional and extending automatically with each quarterly re-run. Two disciplines from the post are enforced rather than documented:

- `k` must be counted against the *same population* the source counts. Sources built on publicly recorded incidents do not include internal near-misses; counting them mixes populations and inflates the posterior. Each scenario records its counting rule and events record whether they meet it.
- `β₀` is shared across scenarios and anchored to the rarest scenario in the portfolio. Per-scenario exceptions are permitted but must carry a stated test failure and a one-sentence rationale — the post expects one or two per portfolio, not one per scenario. The engine warns above that threshold.

**Magnitude (right of boom).**

```
σ         = (ln P95 − ln P50) / 1.6449        fixed, held through the update
µ_updated = (m₀ · ln(anchor) + Σ ln(costed_i)) / (m₀ + n)
posterior ~ Normal(µ_updated, σ² / (m₀ + n))
E[cost per event] = exp(µ + σ²/2)
```

σ is held fixed because one or two observations cannot support learning a spread. The **lognormal mean, not the median**, drives expected annual loss; the post gives the worked figure ($176.6K versus $252.0K on its ransomware row) precisely because the omission is common and material. A required test asserts the mean is used.

`m₀` has no automatic decay — it cedes weight only when an event is actually costed, which may be years apart — so where uncertain the pack guidance is to set it low rather than high.

## Expected annual loss

```
EL_scenario = λ_updated × exp(µ + σ²/2)
```

with the 90% credible interval produced by sampling the two posteriors jointly, seeded, as in the published method.

## The exposure bridge

### Bridge A — exposure as a frequency modifier (default)

```
EF = exposureFactor(scenario, window)          neutral at the deployment's own baseline
λ_effective = λ_updated × EF
```

Four constraints make this defensible:

1. **Neutral baseline.** `EF = 1` when current entry-path exposure equals the deployment's established baseline. The industry anchor already prices a firm operating at typical exposure; multiplying it by an absolute susceptibility figure would double-count. Only *deviation* moves λ.
2. **Baseline requires history.** `EF` is unavailable — not defaulted to 1 with false confidence — until the deployment has accumulated the configured minimum observation period. Before that, the scenario reports λ unmodified and states that exposure adjustment is not yet available.
3. **Bounded.** `EF` is capped in pack data, both directions. Attack-surface change is one input among many to loss-event frequency; it cannot plausibly move the rate by an order of magnitude on its own.
4. **Declared.** Every figure derived with `EF ≠ 1` states that it is exposure-adjusted, reports `EF`, and can be re-read with `EF = 1`.

Entry-path composition is scenario configuration: which asset classes, weakness classes and controls constitute the path. For business email compromise that is the email-authentication posture across sending domains; for ransomware, internet-reachable remote-access and administrative services; for data breach, exposed data stores and application surfaces. The mapping is data, and it is displayed, because an unexamined entry-path definition is where this bridge would quietly break.

### Bridge B — entry-path pseudo-observation (opt-in)

```
k_effective = k + w_exposure · P_aggregate(exploit over window)
```

with `w_exposure` capped low (scanner-derived evidence, in the region of the published 0.3 lab/vendor weight or below). Disabled by default. Any scenario using it is labelled `illustrative` unless the deployment has calibration records supporting the link, and the pseudo-observation appears as a distinct, withdrawable ledger row.

The engine refuses to enable Bridge B for scenarios whose entry path is not fully observable from the external vantage — an insider-misuse scenario, for instance, has no externally measurable entry path, and pretending otherwise would be the worst failure mode available here.

## Six-form overlay and the gate

Each scenario's P50 anchor is split across the FAIR forms: response, replacement, productivity, fines and judgments, reputation, competitive advantage. The overlay serves two decisions only — control targeting and the insurance read — and the post is clear that form-level data is far scarcer than event-level data.

The four-step gate is enforced as required fields per form:

1. Does the form apply here?
2. What would concretely happen? (loss mechanism, in specific terms)
3. Measure it or consciously exclude it — material / obtainable / decision-changing; if a leg fails, record which one.
4. Name the data source for anything committed to measure.

Excluded forms are stored with the failing leg and a **reopen trigger**, and remain visible. The canonical case is reputation damage: applicable almost everywhere, measurable almost nowhere. The record of *why* a form was excluded is itself output an auditor can inspect.

Crucially, excluded forms **stay in the denominator** when a control's effective attenuation is computed in Change 011 — the post's rule — so exclusion never flatters a control's return.

## Insurance read

Retention, sublimits and limit are scenario or programme configuration. The overlay then reports, per scenario: insurable dollars, dollars below retention (self-insured in practice), dollars where a sublimit binds before the headline limit, and structurally uninsurable dollars — chiefly reputation and competitive position. That uninsurable share is what resilience investment defends, and naming it is one of the more useful things this model does for a board.

## Portfolio simulation

Per the published calculator: 10,000 trial years; each trial draws a Poisson event count per scenario and lognormal per-event losses; sum per year; read the exceedance curve, and the P50/P95 of annual loss. Seeded and reproducible.

**Scenario independence is a stated simplification**, disclosed on the curve itself as the published calculator discloses it. Real scenarios share kill chains — the phished credential that begins a ransomware event also begins a data breach — and correlation raises the value of upstream shared controls. The independence figure is therefore a floor for such controls, and the disclosure says so rather than leaving the reader to assume precision that is not there.

Axis truncation follows the published convention: truncate where the pre-control curve falls below one in a thousand, and disclose the truncation on the chart.

## Testing

- Golden-vector tests reproducing the published worked example: three scenarios, the stated priors and events, aggregate EL of $423.1K with a 90% interval of $166K–$888K.
- Lognormal-mean test: expected annual loss uses the mean, not the median; the skew factor is asserted explicitly.
- Population-mixing test: an event failing the scenario's counting rule does not enter `k`.
- Baseline test: `EF` is exactly 1 at baseline and unavailable before the minimum observation period.
- Bridge B refusal test: a scenario without an externally observable entry path cannot enable it.
- Exclusion test: excluded forms remain in the attenuation denominator.
- Sweep test: β₀ and m₀ sweeps produce the published sensitivity bands.
