# Design: 008-risk-model-packs

## Pack anatomy

```
config/packs/
  trawl-base-2026.08.json        # shipped: published, citable parameters only
  trawl-base-2026.08.json.sig    # detached signature
  overrides/
    local.json                   # operator layer, never merged into the base
```

Resolution is strictly two-layer: base, then override. No deep merge chains, because a value a reader cannot locate is a value they cannot challenge. Every resolved parameter reports which layer supplied it.

```json
{
  "packVersion": "2026.08",
  "schemaVersion": 1,
  "sections": {
    "weaknessClassPriors": {},
    "attemptBands": {},
    "measuredStateSignals": {},
    "contactBaseRates": {},
    "scenarioAnchors": {},
    "lossFormOverlays": {},
    "judgmentParameters": {}
  }
}
```

## Provenance on every parameter

Every leaf value is an object, never a bare number:

```json
{
  "value": 0.85,
  "ess": 8,
  "source": "GitGuardian State of Secrets Sprawl — majority of exposed secrets remain valid long after disclosure",
  "sourceUrl": "...",
  "retrievedAt": "2026-07-14",
  "verificationStatus": "needs-verification",
  "label": "sourced",
  "notes": "Class mean for secret applicability; discount for private-repo exposure applied via ESS, not the mean."
}
```

`verificationStatus: needs-verification` is the shipped default, mirroring the Prior Estimator's badge. It is not a defect — it is an accurate statement that the author transcribed a figure and the reader has not yet checked it. Any figure destined for a board or external audience must be `verified` or `locally-overridden`, and Change 012 enforces that at export.

## The Layer 2.5 section — the novel content

The published framework has two evidence classes: Layer 2 judgment adjustments (move the mean, never the width) and Layer 3 telemetry updates (move both, via conjugate update). Trawl's continuous assessment produces a third: a **verified measurement of the defensive mechanism itself**. A `p=none` DMARC policy is not an assertion about the instance; it is an observation of the mechanism that determines the outcome.

Such an observation should narrow the interval, and the justification is the law of total variance:

```
Var(θ) = E[ Var(θ | S) ]  +  Var( E[θ | S] )
         within               between
```

Part of a class prior's width is heterogeneity of the population the source study drew from — some of those domains enforced DMARC, some published nothing. Observing which sub-population you are in retires the between-component. Nothing was assumed and no outcome was observed, yet the estimate is legitimately tighter.

Operationalised as a precision gain applied to effective sample size:

```
g          = Var_class / E[Var | S]           precision-gain factor, ≥ 1
ESS_cond   = (ESS_class + 1) · g − 1
```

Each measured-state signal therefore carries:

```json
"SURF-DMARC-001": {
  "stage": "success",
  "deltaLogit": { "value": 1.5, "source": "...", "label": "sourced" },
  "varianceShare": {
    "value": 0.40,
    "heterogeneityArgument": "The source population mixes enforcing and non-enforcing domains; policy disposition is not conditioned on in the published rate.",
    "source": "...",
    "label": "sourced"
  },
  "dedupGroup": "dmarc-enforcement",
  "tauDays": 180,
  "capG": 2.0
}
```

### Four guardrails, encoded as pack schema requirements

**1. Heterogeneity precondition.** `varianceShare` is invalid without a stated `heterogeneityArgument`. If the source study already conditioned on the signal, observing it adds no precision and the share must be zero. The schema makes the claim explicit and therefore challengeable.

**2. Dedup binds on variance, not only on the mean.** The framework forbids counting EPSS and "Metasploit module exists" twice on the mean. The same must hold for precision: SPF `-all` and DMARC `p=reject` are correlated indicators of one underlying maturity. Variance shares are claimed **per dedup group**, once, taking the maximum member share rather than a sum. Two signals in one group cannot each narrow the interval.

**3. Hard cap `g ≤ 2`.** Measured state may at most halve variance. Rationale: you cannot learn more from a mechanism's configuration than the class study that priced that mechanism contained, and there is irreducible model uncertainty about the class prior itself that no amount of scanning touches. The cap is per stage, after dedup.

**4. Repetition is not accumulation.** Scanning a static configuration 500 times is not 500 observations. Persistence buys exactly two things: freshness stays at full weight, and the probability of unobserved drift between assessments falls. It must never inflate ESS. Encoded by making precision gain a function of the *current* observation and its age, with no accumulation term available in the schema at all — the safest place to prevent an error is where it cannot be expressed.

Read this guardrail precisely, because it is easy to over-read. It says repetition is not evidence about *susceptibility* — the conditional stage probabilities, which are properties of how the world responds to an attack and which no amount of looking at your own configuration can inform. It does not say repetition is worthless. Repetition is evidence about *state*, and state includes time at risk, which is the one directly measured term in the contact model (007). Re-observation resets `Δt` in the decay term to zero; it does not raise `g`, which remains bounded by `capG`. The premise is sustained, the estimate is not sharpened. The two flows are kept apart structurally: precision gain reads only the current observation and its age, while time-at-risk accrual is a separate quantity owned by 007 and never routed through a pack parameter. See RISK-ARC §5b.

## Freshness decay

Measured state is a timestamped measurement, so its precision gain decays:

```
g_effective = 1 + (g − 1) · exp(−Δt / τ)
```

`τ` is per signal class, in pack data: a DNS policy record is stable for months; an open port or a storage ACL is not. Three consequences worth stating:

- Continuous monitoring has a **quantified** advantage over point-in-time assessment. An annual penetration test's evidence has decayed to near-nothing by Q3; Trawl's has not.
- `not_checked` and `check_failed` behave correctly by construction: no observation, no gain, interval widens back toward class width over time.
- A stale pack is visibly stale, because `retrievedAt` participates in the same reasoning.

## Scenario anchors and the taxonomy mapping

The portfolio-ROI post's six-step construction from sector × size to per-scenario priors is stored as data, including the step most likely to be fudged:

```json
"scenarioAnchors": {
  "taxonomyMapping": {
    "data-breach": ["system-intrusion", "accidental-disclosure"],
    "business-email-compromise": ["fraud-or-scam"],
    "ransomware": ["ransomware"]
  },
  "shares": { "...": "must sum to 1.0 across the scenario list" },
  "expertEstimatedResiduals": [
    { "scenario": "ai-security", "share": 0.05,
      "rationale": "No published share for this class; taken from the residual.",
      "priorStrengthException": true }
  ]
}
```

A schema validation requires shares to sum to 1.0 and requires every expert-estimated residual to carry a rationale and an exception flag — the post's rule-2 exception discipline, enforced rather than remembered.

## Judgment parameters

β₀, m₀ and w, each with its sweep points, because the post is explicit that *"the sweep is the deliverable"*:

```json
"judgmentParameters": {
  "beta0":  { "value": 30, "sweep": [10, 30, 100], "rationale": "...", "sourceEdition": "IRIS 2025" },
  "m0":     { "value": 5,  "sweep": [3, 5, 10],    "rationale": "..." },
  "tailWeight": { "value": 0.0, "sweep": [0, 0.05, 0.10],
                  "rationale": "Board has not yet set a tail price; default zero.",
                  "owner": "board" }
}
```

`owner: board` is meaningful: Change 012 refuses to present a ranking as board-mandated while `w` remains at its unset default.

## Signing and update discipline

Packs are signed; an unsigned or signature-mismatched pack refuses to load rather than loading with a warning. A risk model whose parameters can be edited undetectably is worse than no risk model, because it carries unearned authority.

Updates are operator-initiated and produce a **diff report**: which parameters moved, by how much, and which stored estimates change as a result. Estimates are never silently recomputed under a new pack; the previous computation is retained with its original pack version so a quarterly trend read compares like with like or explicitly declares a re-baseline.

## Required check

A CI check greps the engine for probability-affecting numeric literals outside pack loading and test fixtures. Crude, and effective: it is the mechanical enforcement of "if a parameter is not in a pack, it does not exist."

## Testing

- Schema validation: shares sum to 1.0; `varianceShare` requires a heterogeneity argument; residuals require rationale and exception flag.
- Dedup test: two signals in one group produce one variance share, not two.
- Cap test: stacked signals cannot drive `g` above 2.
- Decay test: precision gain approaches zero as `Δt` grows; repeated identical observations do not accumulate ESS.
- Signature test: tampered pack refuses to load.
- Override test: an override is reported as `locally-overridden` and never mutates the base file.
