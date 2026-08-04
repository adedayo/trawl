# The Third Evidence Class

## What a scanner is actually telling you about risk

> **Status of this document.** Working notes behind Trawl changes 006–012, kept as prose because the argument matters more than the schema. Written to feed a post; the specs are the implementation of it. Where it extends published work, that is marked explicitly.

---

## 1. The hole in the framework is shaped like an attack-surface monitor

"The Probability Behind the Finding" is careful about its own boundary:

> Contact probability (the likelihood an attacker reaches the asset at all) is assessed separately from network exposure and attack-surface analysis and is not part of this framework.

The framework yields **P(exploit | contact)** — the product of applicability, attempt and success stages. To get an unconditional probability you need the contact term, and the post says, correctly, that it comes from somewhere else.

That somewhere else has a name and a product category. Continuous external attack-surface monitoring is exactly the practice of measuring what an attacker can reach, from outside, over time. The two halves were built to fit:

```
attack-surface monitoring  →  P(contact)
three-layer framework      →  P(exploit | contact)
                              ─────────────────────
                              P(exploit)
```

The interesting part is not the multiplication. It is what happens to the *character* of the estimate once a continuous sensor is attached to it — because a sensor supplies a kind of evidence the framework's two layers do not quite describe.

---

## 2. Contact is a rate, and time at risk is the only measurement in the chain

Model contact as a rate rather than a probability:

```
λ_contact = λ_ambient(service class) × discoverability × (1 − access barrier)
P(contact | T) = 1 − exp(−λ_contact · T)
```

gated by reachability: no confirmed responding service, no external contact. A DNS record is not exposure, and treating it as such is the most common overstatement in attack-surface tooling.

Four factors are estimates. The fifth, `T`, is not. **Time at risk is measured.** Trawl knows, to the hour, how long a service was reachable, because it looked, repeatedly, and wrote down what it saw.

That deserves emphasis, because in a chain otherwise built from published base rates and bounded judgment, one term is a direct observation of the organisation's own history. "This administrative interface was internet-reachable for forty-one days" is not an estimate anyone can argue with. It is a fact with a timestamp.

Three honesty rules keep it a fact:

- **Censoring is declared.** An asset exposed before you started watching has a left-censored duration. Report "at least forty-one days".
- **Detection latency is included.** Time at risk starts at first *possible* exposure, not first *observed* exposure. Hourly monitoring adds an hour of uncertainty; quarterly assessment adds up to ninety days. The cadence is part of the measurement.
- **Open exposures keep accruing.** An unremediated finding's contact probability rises every day it stays open. Most tools freeze the number at the last scan; the number should be alive.

This is also where continuous monitoring earns a *quantified* advantage over point-in-time assessment, rather than an asserted one. More on that in §5.

---

## 3. The rule that must not be broken: exposure is a prior, not an observation of attackers

The framework is explicit that contact frequency "is never inferred from scanner findings," and it is right. A scanner seeing an open port has not seen an attacker. Conflating the two would let a tool manufacture evidence of attack from evidence of existence, which is precisely the sin that discredits vendor risk scores.

So the distinction has to be structural, not cultural:

- **Exposure model → a prior on contact.** This is the "network exposure and attack-surface analysis" the framework names as its external supplier. Legitimate, labelled `sourced`.
- **Contact evidence → an update to that prior.** Requires observations of actual attacker contact: honeypot and canary hits, WAF and CDN request logs, authentication logs, DMARC aggregate report volumes.

In Trawl these are two separate stored quantities, and no scan-derived code path can write the second. It is a compile-time and CI-enforced separation, not a convention, because conventions of this kind erode in month eight of a delivery under pressure.

---

## 4. The third evidence class

Here is the extension, and it came out of a simple observation: **a DMARC policy of `p=none` is informative in a way the framework's two layers do not quite capture.**

The framework has:

- **Layer 2** — instance adjustments. Named, directional, bounded shifts on the log-odds. Explicitly: *"Each adjustment moves only the probability, not the uncertainty width."*
- **Layer 3** — Bayesian updates from your own telemetry. Beta-Binomial conjugate updates from counted observations, weighted by sensor reliability.

Layer 2's rule is right *for what it describes*. A human asserting "this is a production credential, not a test key" adds no precision. It is a claim about the instance, and claims move means.

But `p=none` is not a claim. It is a **verified measurement of the mechanism that determines the outcome**. So is `~all` rather than `-all`. So is an absent MTA-STS policy, an unsigned zone, a wildcard record answering for everything, an administrative port answering from the public internet. Nobody asserted these. A sensor went and looked, and the thing it looked at is the causal machinery itself.

That is a third class:

| | Layer 2 — judgment | **Layer 2.5 — measured state** | Layer 3 — outcome telemetry |
|---|---|---|---|
| Observes | an asserted property | the **mechanism**, directly | **realisations** of the event |
| Example | "this is a production credential" | `p=none`, `~all`, no MTA-STS, port 3389 open | rua counts, canary fire, IR confirmation |
| Statistical role | covariate, asserted | covariate, **verified** | outcome, counted |
| Mechanism | logit shift | logit shift **+ precision gain** | conjugate update |
| Moves the mean | yes | yes | yes |
| Moves the width | **no** | **yes** — capped, decaying | yes, uncapped |
| Availability | always | **always, continuously, free** | rarely, only if instrumented |

The final row is the practical argument. Outcome telemetry is strictly the stronger evidence, and most organisations do not have it. A framework that narrows uncertainty *only* on telemetry leaves nearly every estimate at class width indefinitely — which is an accurate description of the state of the art, and not a happy one.

Measured state is the evidence everybody can have, on every asset, refreshed continuously, at no marginal cost.

### Why it legitimately narrows

Law of total variance:

```
Var(θ) = E[ Var(θ | S) ]  +  Var( E[θ | S] )
         within-component     between-component
```

A class prior's width is partly **heterogeneity of the population the source study drew from**. Some of the domains behind a published spoofing base rate enforce DMARC; some publish nothing at all. That mixture is baked into the spread. Observing which sub-population you are in retires the between-component — it was never your uncertainty in the first place.

The board-legible version:

> "The spread in the industry number partly reflects that some firms enforce DMARC and some don't. We know which we are. That part of the spread isn't our uncertainty — it's theirs."

Nothing was assumed. No outcome was observed. And the estimate is legitimately tighter.

Operationally, a precision-gain factor on effective sample size:

```
g        = Var_class / E[Var | S]
ESS_cond = (ESS_class + 1) · g − 1
```

If observing policy disposition removes 40% of the class variance, `g ≈ 1.67`, and an ESS-6 prior behaves like ESS ≈ 10.7. The interval tightens materially; the mean moves by the ordinary logit adjustment. One line of arithmetic, checkable by hand — the same standard the rest of the method holds itself to.

### Four guardrails, without which this is just confidence laundering

**1. Heterogeneity precondition.** You may claim `g > 1` only if the source population was genuinely mixed in that signal. If the study already conditioned on DMARC policy, observing it tells you nothing new and the variance share is zero. Every claim carries a written argument, so it can be attacked.

**2. Deduplication binds on variance, not only on the mean.** The framework forbids counting EPSS and "a Metasploit module exists" twice, because EPSS already ingests exploit availability. The same discipline must apply to precision. SPF `-all` and DMARC `p=reject` are correlated indicators of one underlying maturity; they are not two independent narrowings. Signals belong to dedup groups, and a group claims its share once — the maximum, not the sum.

**3. A hard cap, `g ≤ 2`.** Measured state may at most halve variance. The reason is not squeamishness: you cannot learn more from a mechanism's configuration than the class study that priced that mechanism contained, and there remains irreducible uncertainty about the class prior itself that no amount of scanning touches. The cap enforces a floor on humility.

**4. Repetition is not accumulation.** Scanning the same domain five hundred times does not yield five hundred observations. Repeated measurement of a static configuration is evidence that the state is *stable*, not new evidence about susceptibility. Persistence buys exactly two things: freshness stays at full weight, and the probability of undetected drift between scans falls. It must never inflate effective sample size. In the implementation, there is simply no field in which such an accumulation could be expressed — the safest place to prevent an error is where it cannot be written down.

---

## 5. Freshness decay, and the value of continuous monitoring

Measured state is a timestamped measurement, so its precision gain should decay:

```
g_effective = 1 + (g − 1) · exp(−Δt / τ)
```

with `τ` per signal class. A DNS policy record is stable for months. An open port, a cloud storage ACL, a certificate is not.

This gives three things at once.

**A quantified argument for continuous monitoring.** The annual penetration test's evidence has decayed to nearly nothing by the third quarter. The continuous sensor's has not. That is now a number in a model rather than a claim in a sales deck — and it is the honest form of the claim, because it also concedes that a fresh annual test in month one is worth exactly as much as a fresh continuous observation in month one.

**Correct behaviour for gaps.** No observation, no gain; the interval widens back toward class width over time. `not_checked` and `check_failed` are thereby structurally incapable of masquerading as reassurance.

**A reason to keep the pack honest.** Parameters carry retrieval dates and participate in the same reasoning. A model quoting a 2023 base rate in 2026 says so.

That last point generalises into a discipline worth stating separately: **"we did not look" must never render as "it is fine."** Vantage draws this distinction at the source — `ok`, `not_found`, `not_checked`, `check_failed` — with the observation that "no record published" and "we could not tell" are different statements. For a *risk* model the inverse error is the dangerous one: a check that never ran must never contribute the reassurance of a check that passed. Every aggregate therefore carries an assessment-coverage figure, and every number is quoted with it.

---

## 6. The selection effect, and why stage attribution saves you

Here is the trap, and it is a good one.

A domain at `p=reject` with reporting enabled shows very few *successfully delivered* forged messages. Naively, that reads as low susceptibility. It is nothing of the kind: it is evidence the control is working. Feed it in carelessly and a CISO concludes "we enforce DMARC, so nobody targets us" — exactly backwards, and the kind of inference that ends up in a post-incident review.

Correct stage attribution dissolves it:

| Signal | Stage | Class |
|---|---|---|
| rua aggregate volumes — messages failing authentication | **Attempt** | outcome (counted; policy-independent) |
| ruf forensic samples | **Attempt**, higher fidelity | outcome |
| DMARC disposition `none` / `quarantine` / `reject` | **Success** | measured state |
| `sp` subdomain policy, `pct` rollout, per-domain coverage | **Success** | measured state |
| SPF `~all` vs `-all`, DKIM key presence and strength | **Success** | measured state |
| MX absent | **Applicability** | measured state |

Attackers attempt spoofing whether or not you reject. Disposition governs what happens *after* the attempt. Report volumes therefore give attempt-stage counts independent of policy, and disposition gives success-stage state. The selection effect vanishes, not by adjustment, but by putting each signal where it belongs.

The general rule: **every signal declares exactly one consuming stage.** A signal claimed by two stages is a build failure. This is the same discipline as the EPSS deduplication rule, applied across stages rather than within one.

### DMARC reports are the most under-used Layer 3 source in existence

Worth saying plainly. Aggregate reports are a daily, free, counted record of attempted abuse of your domains — genuine `k out of n` observations, arriving on their own, at organisations that believe they have no attack telemetry. Most enterprises either do not publish a `rua` address, or publish one into a mailbox nobody parses.

The framework says one canary changes the picture, because it upgrades a stage estimate from a category starting point to an observation from your own environment. Aggregate reports are the same upgrade, already paid for, waiting in a DNS record most domains do not have.

---

## 7. Disagreement is a finding, not an averaging problem

When outcome evidence contradicts measured state — `p=reject` published, yet reports show forged mail being delivered — the wrong move is to average them into a compromise number.

The right move is two moves:

1. **Telemetry governs the estimate.** It observes outcomes; state observes only mechanism.
2. **The divergence is raised as a control-efficacy finding.** Usually the cause is an enforcement gap: a missing subdomain policy, a rollout percentage below 100, an unenrolled sending domain, a lookalike outside the assessed set.

This falls out for free once both evidence classes live in one ledger, and it converts the risk model into a detector. A model that only produces numbers is a reporting tool. A model that notices when its own inputs disagree is an instrument.

---

## 8. Uncertainty as a first-class object, and the value of information

Once uncertainty is a stored quantity rather than a caveat, a new question becomes askable: **which missing sensor would most narrow the interval currently driving the decision?**

The portfolio-ROI method already brushes against this. When the top two controls' return intervals overlap, it observes that the overlap is itself decision information — collect more cost data, or fund both. Generalise that and you get a ranking of *instrumentation* alongside the ranking of controls, priced the same way: expected narrowing per pound, with a flag for whether the narrowing could actually change the funding order.

The output reads like this:

> "Three of your scenarios rest on class priors because you have no attempt telemetry. Publishing report addresses on nineteen domains — no licence cost, one DNS change — would begin outcome-updating the business-email-compromise attempt stage within thirty days."

Uncertainty reduction that cannot change a decision ranks below uncertainty reduction that can. That distinction is what keeps this from becoming measurement for its own sake.

---

## 9. Measured coverage: the control you bought versus the control you have

The portfolio method takes each control's effectiveness as an estimate. Fair — efficacy research is not something a scanner can do.

But efficacy is not the whole story. **A control only delivers where it is actually deployed**, and deployment is often externally observable: DMARC at enforcement across *n* of *m* sending domains, MTA-STS in enforce mode, zones signed, TLS configured, patch latency on internet-reachable services.

So: `effective TMP = TMP × measured coverage`, and one sentence becomes available that no purely top-down model can produce:

> "You are paying for this control and receiving 68% of it. Closing the gap costs nothing in licence terms and is worth £X a year."

Two corollaries follow immediately.

**Control drift is an ROI event.** A policy relaxed from enforcement on one sending domain is not a medium-severity ticket in a queue. It is a measurable increase in expected annual loss, and presenting it that way changes how a team triages posture regressions.

**Asserted coverage must be marked as such.** Endpoint detection, backups and awareness training are not externally observable. Those carry attested coverage with a source and a date, visually distinguished from measured coverage. The distinction is the point: a model that quietly treats an attestation and a measurement as the same kind of thing has given up the only advantage it had.

---

## 10. Crossing from findings to loss events, without cheating

The hardest honest problem in the whole chain. Bottom-up gives per-instance exploitation probabilities. Top-down gives per-scenario loss frequencies anchored to industry data and pulled by your own incident history. **A compromised asset is not a loss event**, and substituting one for the other would be indefensible.

Two bridges, both declared:

**Bridge A — exposure as a mitigation-like modifier (default).** Attack surface sits left of boom. The aggregate susceptibility of a scenario's entry-path assets acts as an exposure factor on the event rate, structurally identical to a control's threat-mitigation potential, with a **neutral baseline**: at your own established exposure level, the rate is unchanged. Only deviation moves it. The industry anchor already prices a firm operating at typical exposure; multiplying it by an absolute susceptibility figure would double-count.

This keeps the anchor doing what anchors are good at, keeps the update to one line, and makes a quarter of remediation visible in expected annual loss — which is the thing every CISO has wanted from a risk model and never got.

**Bridge B — entry-path pseudo-observation (opt-in).** Aggregate exploitation probability over the window enters the frequency update as a low-weight pseudo-observation, at or below the weight the framework assigns to vendor and lab evidence. Powerful, easy to abuse, therefore off by default, weight-capped, withdrawable, and refused entirely for scenarios whose entry path is not externally observable. An insider-misuse scenario has no external entry path, and pretending otherwise would be the worst available failure.

Both bridges are declared on every figure they touch. A board reading an expected-loss number is entitled to know whether it moved because of an incident, an industry revision, or a change in measured exposure.

---

## 11. What the ledger has to become

The framework's two-part evidence ledger — priors and adjustments in part one, updates in part two — is usually read as a reporting requirement. It should be read as a **storage model**.

If the ledger is a view rendered over a database of scores, it rots: the numbers and their justifications drift apart, and within a year the justification is archaeology. If the ledger *is* the database, then auditability is free and the chain of custody is a property of the system rather than a claim about it.

The middle evidence class adds fields, and they earn their place. Each row carries its class, its signal identifier, its observation state, its observation time, its decay constant, its log-odds delta, its claimed variance share, its dedup group, and whether a cap bound it. Outcome rows carry the counts, the sensor quality, the evidence weight and — importantly — the distribution parameters *before and after*, so any update can be re-checked in a single line without re-running the engine.

Then "remove the challenged modifier and recompute" still works in one step, and now does the right thing in both dimensions: withdrawing a measured-state row moves the mean **and widens the interval back**. In the published framework, removal only moves the mean, because Layer 2 adjustments never touched the width. Once the third class exists, withdrawal has to restore the uncertainty it removed. That is a small consequence of the extension, and a satisfying one — it means the ledger is genuinely reversible.

---

## 12. The three dials

If there is one framing to take to a board, it is this. A risk estimate has three dials, and most tools show only the first:

- **How bad** — the mean.
- **How sure** — the interval.
- **How fresh** — when anyone last actually looked.

Scanning moves the second and third continuously, for free, on every asset. Telemetry moves all three but only where you have instrumented. Judgment moves the first alone.

Which turns the quarterly board read into something more defensible than a heat map:

> The ranking is stable. The intervals are narrowing. Here is what narrowed them — measurement, not assumption.

---

## Open questions

Genuinely open, and worth resolving before or in public.

1. **Calibrating `g` empirically.** Variance shares are currently reasoned arguments about source-population heterogeneity. With enough instances and outcomes they could be estimated. The chicken-and-egg is obvious, and the honest answer today is that these are `sourced`, not `calibrated`.
2. **Is `g ≤ 2` the right cap?** It is a defensible working heuristic, chosen for humility rather than derived. It should be published as such and revised against evidence.
3. **Contact-model calibration.** Ambient contact rates by service class rest on published mass-scan and honeypot research. Honeypot deployment alongside a monitored estate would calibrate them directly, and is the single most valuable experiment available here.
4. **Cross-instance calibration.** The framework identifies a structural obstacle: non-CVE weaknesses are instances, not categories, so there is no shared label and no common dataset. A fleet of tools recording predictions and outcomes against a shared *class* taxonomy is at least a partial route — not a solution, but an instrument that makes the problem approachable. Whether that can be done without pooling data anyone would object to pooling is unresolved.
5. **Correlated contact.** One actor sweeping an address range contacts many assets at once. Independence understates aggregate contact. The direction of the error is known and disclosed; the magnitude is not.

---

## Provenance

Extends, and does not replace:

- *The Probability Behind the Finding: Calibrated Exploit Estimates Beyond CVEs* — three-layer estimate, evidence ledger, calibration labels, EPSS at the attempt stage, deduplication.
- *Security Investment as Board Strategy: Pricing Protection on Both Sides of the Boom* — scenario as unit of analysis, the two closed-form updates, six-form overlay, marginal control pricing, the board's three decisions.

New here: the contact-probability model as a measured rate with time at risk; **Layer 2.5, measured state as an evidence class that narrows uncertainty**, with its four guardrails and freshness decay; stage attribution as the resolution of the enforcement selection effect; divergence between state and outcome as a control-efficacy finding; measured control coverage and effective mitigation; value-of-information ranking for instrumentation; and the exposure bridge with a neutral baseline.

Implementation: Trawl changes 006–012.
