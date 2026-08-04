# Design: 007-contact-probability

## The model

Contact is a **rate**, not a probability:

```
λ_contact = λ_ambient(service class) × D × (1 − B)          [contacts per day]
P(contact | T) = 1 − exp(−λ_contact · T)                    [T = time at risk, days]
```

gated by reachability:

```
R = 0  →  λ_contact = 0   (external contact not possible from this vantage)
R = 1  →  as above
```

Rates rather than probabilities, for four reasons:

1. Rates from independent contact channels add. Probabilities do not.
2. The observation window becomes explicit. The post insists the window reflect the realistic threat timeline per weakness class — hours for internet-facing secrets, weeks for isolated assets — and a rate model forces that choice into the open instead of burying it.
3. Time at risk enters linearly and legibly. Halve exposure duration, halve expected contacts.
4. It composes with Change 010's Poisson frequency model without a change of representation.

## The five factors

| Factor | Symbol | Source | Type |
|---|---|---|---|
| Reachability | R ∈ {0,1} | Confirmed open port/service (naabu/httpx), not a DNS record alone | Gate, measured |
| Ambient attention | λ_ambient | Service/port class base rate from published mass-scan and honeypot research; KEV membership for the service | Prior, `sourced` |
| Discoverability | D ≥ 1 | CT issuance (vantage `--enumerate`), passive DNS, wildcard zone, search-index presence | Multiplier, measured |
| Access barrier | B ∈ [0,1) | Authentication required, IP allowlist, mTLS, WAF/CDN fronting | Attenuator, measured |
| Time at risk | T | Trawl's own first-seen → last-seen history | **Measurement** |

### Reachability is a gate, not a modifier

A DNS record is not exposure. Only a confirmed responding service sets `R = 1`. An asset that resolves but refuses connections has no external contact rate — the correct answer is zero, not a small number. This is the single most common overstatement in attack-surface tooling and the model should not reproduce it.

Where reachability could not be determined, `R` is **unknown**, not zero: the estimate is suppressed and reported as uncovered, per the coverage discipline inherited from Change 006.

### Ambient attention is the only estimated factor

λ_ambient is a class-level prior from published research on internet background radiation, honeypot contact rates and mass-scan telemetry — the same kind of source, with the same honesty constraints, as the weakness-class priors in the exploit-probability framework. It carries a source, an effective sample size, a verification status and a calibration label, and lives in Change 008's packs as data.

KEV membership for the *service* raises λ_ambient: a KEV-listed service class is under documented mass exploitation, so contact attempts against it are empirically more frequent. This is a contact-stage use of KEV and must not be double-counted against the attempt-stage KEV band from the exploit-probability framework — see dedup below.

### Discoverability multiplies

An attacker contacts what they find. A host named in a Certificate Transparency log is enumerable by anyone, for free, permanently. `D` is a bounded multiplier (capped, in pack data) over: CT issuance, passive DNS presence, wildcard DNS answering for everything (vantage `wild`), zone transfer exposure (`axfr`), and search-index presence.

Obscurity is a weak defence and the cap enforces that: `D` may raise the rate substantially but its floor is meaningfully above zero. An undiscovered host is still contacted by indiscriminate address-space scanning, which does not need to find it by name.

### Access barrier attenuates, never gates

`B` never reaches 1. An authenticated endpoint is still contacted; contact is what precedes an authentication attempt. Conflating "requires credentials" with "unreachable" would erase credential stuffing and spray from the model entirely — which are among the most common real contact events.

### Time at risk is the measurement

```
T = last_observed_exposed − first_observed_exposed
```

Three honesty rules:

1. **Censoring is declared.** An asset exposed before Trawl began observing has a left-censored T. Report it as "at least 41 days", never as 41 days.
2. **Detection latency is included.** Time at risk starts at first *possible* exposure, not first *observed* exposure. Continuous monitoring at a one-hour cadence adds at most an hour of uncertainty; a quarterly assessment adds up to 90 days, and the model must say so.
3. **Still-exposed assets have a growing T.** Live, not frozen at last scan. An open finding's contact probability rises every day it stays open — the behaviour a CISO expects and most tools fail to show.

This is where continuous monitoring visibly outperforms point-in-time assessment, and it is a quantified argument rather than a marketing one.

## Instance aggregation

The post: *"Instance count is itself an input… Count instances before setting stage estimates."* Trawl computes it.

```
P_any(T) = 1 − Π over instances i of (1 − P_i(T))
```

Per-instance rather than the estimator's `1 − (1 − P)^N`, because Trawl's instances genuinely differ — one host behind a WAF, another not. When instances are homogeneous the formula reduces to the published one exactly.

Remediation coverage is derived, not asserted:

```
covered = instances with the weakness confirmed absent on re-observation
residual P_any = computed over the uncovered instances only
```

This makes "removing three of fifteen leaves the route open through the remaining twelve" a live computed figure rather than a discipline someone has to remember.

## Dedup: one signal, one consumer

The framework forbids double-counting EPSS against separate exploit-code adjustments. The same discipline must span stages, because contact and attempt both plausibly consume attention signals.

| Signal | Consumed by | Not consumed by |
|---|---|---|
| KEV membership of the **service class** | contact (λ_ambient) | — |
| KEV membership of the **specific CVE** | attempt (EPSS/KEV band) | contact |
| EPSS score | attempt | contact |
| CT issuance | contact (D) | attempt |
| WAF / CDN fronting | contact (B) | success |
| Authentication required | contact (B) | success |

Every signal declares exactly one consuming stage in the registry, and the engine enforces single consumption. A signal claimed by two stages is a build failure, not a runtime warning.

## Output

```
contactEstimate {
  assetId, serviceId, weaknessClass,
  lambdaContact:   { point, low, high },       // per day
  windowDays:      T,
  windowRationale: "class threat timeline: internet-facing service",
  pContact:        { point, low, high },       // 90% interval
  timeAtRisk:      { days, censored: bool, detectionLatencyDays },
  factors: [ { name, value, source, evidenceClass, observedAt, state } ],
  coverage:        { checksConcluded, checksApplicable },
  label:           "illustrative" | "sourced" | "calibrated",
  isPrior:         true            // never a posterior; see below
}
```

`isPrior` is structural. Contact **posteriors** are a separate stored quantity written only by Change 009's telemetry ingestion. No scanner-derived code path may write one. This is the enforcement point for the framework's "contact is never inferred from scanner findings" rule, and it is a test, not a convention.

## What this deliberately does not model

- **Targeted adversaries.** This prices ambient, opportunistic contact. A deliberate adversary selecting your organisation is a scenario-level input in Change 010. Extrapolating a mass-scan rate to a targeted attacker would be a category error, and the UI must say which is being shown.
- **Internal contact.** External vantage only, matching vantage's own stated limitation.
- **Contact correlation across assets.** One actor sweeping a range contacts many assets at once. Independence is a stated simplification, as scenario independence is in the portfolio post; it understates aggregate contact and the direction of the error is disclosed.

## Testing

- Pure-function unit tests per factor, including R = 0 collapsing λ to zero and R = unknown suppressing the estimate.
- Censoring test: an asset first observed on Trawl's first run reports a lower-bounded T.
- Growth test: an unremediated exposure's P(contact) strictly increases with wall-clock time.
- Dedup test (required check): no signal is consumed by two stages.
- Prior/posterior separation test (required check): no scanner-derived path can write a contact posterior.
