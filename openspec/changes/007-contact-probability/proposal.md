# Change: 007-contact-probability

## Status

**Proposed, not started.** Depends on Change 006. The keystone of the risk-quantification arc: nothing downstream is defensible without it.

## Why

"The Probability Behind the Finding" draws its boundary explicitly:

> Contact probability (the likelihood an attacker reaches the asset at all) is assessed separately from network exposure and attack-surface analysis and is not part of this framework.

That excluded term is exactly what a continuous external attack-surface monitor measures. The framework produces **P(exploit | contact)**; without P(contact), that conditional cannot become an unconditional probability, and every downstream number — susceptibility, expected loss, control ranking — either stays conditional or silently assumes contact is certain.

Today the Prior Estimator asks a human to supply contact from judgment. For internet-facing assets the post notes contact approaches certainty, which is fine; for everything else it is a guess, and it is the guess that dominates the answer.

Trawl can do better, because contact is the one quantity in the whole chain with a genuinely **measured** component: time at risk. Trawl knows, to the hour, how long a service was reachable. "This administrative interface was internet-reachable for 41 days" is not an estimate. It is the strongest sentence available to a CISO in front of a board, and no part of the published framework currently uses it.

## What Changes

- New capability: `contact-probability`. A deterministic exposure model computing a **contact rate** λ_contact per asset-service, and from it P(contact) over a stated window.
- **Contact is modelled as a rate, not a probability.** `P(contact | T) = 1 − exp(−λ_contact · T)`. Rates compose additively across independent contact channels, decompose into named factors, and make the observation window explicit rather than implicit — the post's own insistence that the estimation window reflect the realistic threat timeline per weakness class.
- **Five named factors**, each traceable to a Trawl or vantage observation: reachability (a gate), discoverability, ambient scan attention, access barrier, and time at risk.
- **Time at risk is measured, not estimated**, from Trawl's own first-seen/last-seen posture history, and is labelled as measurement rather than as a sourced prior.
- **Instance aggregation.** The post requires counting instances before scoring; Trawl computes the count from live inventory, and computes remediation coverage against it, so `1 − (1 − P)^N` and the residual estimate for uncovered instances are derived rather than asserted.
- **A hard separation between exposure priors and contact evidence**, described below.

## The separation this change must protect

The framework says contact "is never inferred from scanner findings." That rule is about **Layer 3 outcome updates**, and it is correct: a scanner seeing an open port has not seen an attacker. This change therefore distinguishes two things that are easy to conflate:

- **Exposure model → a prior on contact.** This is precisely the "network exposure and attack-surface analysis" the framework names as its external supplier. Labelled `sourced`. In scope here.
- **Contact evidence → an update to that prior.** Requires observations of actual attacker contact: honeypot and canary hits, WAF/CDN request logs, authentication logs, DMARC aggregate report volumes. Out of scope here; ingestion lands in Change 009.

Trawl's own scan history is never an observation of attacker contact. It is an observation of *reachability*, which is a factor in the prior. This change enforces that in code: the contact prior and the contact posterior are separate stored quantities, and no scanner observation may write the latter.

## Explicitly Out of Scope

- **Layer 3 contact updates from telemetry.** Change 009.
- **Targeted-attacker modelling.** The ambient model prices untargeted, opportunistic contact — mass scanning, credential stuffing, spray. A motivated adversary selecting your organisation deliberately is a different question and is handled as a scenario-level input in Change 010, not by extrapolating a scan-rate model.
- **Internal lateral-movement contact.** Trawl assesses from the external vantage only, matching vantage's own honest limitation.

## Impact

- **Schema additions**: `exposureFactors`, `contactEstimates`, `assetExposureHistory`.
- **Depends on posture history retention**: time-at-risk requires continuous observation, which is the argument for Change 005's server mode. A desktop point-in-time run yields a censored, and therefore explicitly under-stated, time at risk.
- **Every contact estimate carries an interval and a coverage figure**, never a bare point estimate.
