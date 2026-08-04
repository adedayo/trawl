# Tasks: 007-contact-probability

**Do not start until Change 006 is implemented.**

## Phase 0 — Preconditions
- [ ] Change 006 signal registry populated (stage declarations present on every mapped signal)
- [ ] Posture history retention confirmed sufficient to derive first-seen/last-seen per asset-service
- [ ] Source review: published mass-scan, internet-background-radiation and honeypot contact-rate research, per service class

## Phase 1 — Exposure history
- [ ] `assetExposureHistory`: first-observed-exposed, last-observed-exposed, still-exposed flag, per asset-service
- [ ] Left-censoring detection and flagging
- [ ] Detection-latency derivation from assessment cadence
- [ ] Live time-at-risk computation (grows with wall clock, not frozen at last scan)

## Phase 2 — Factor computation
- [ ] Reachability gate from confirmed responding services; unknown suppresses the estimate
- [ ] Discoverability multiplier from CT, passive DNS, wildcard zone, zone-transfer exposure, index presence — with cap
- [ ] Access-barrier attenuator from auth requirement, allowlist, mTLS, WAF/CDN fronting — bounded strictly below 1
- [ ] Ambient attention lookup by service class (values sourced from Change 008 packs; a stub table until 008 lands)
- [ ] Pure-function unit tests per factor

## Phase 3 — Contact estimate
- [ ] `computeContactEstimate` pure function: λ_contact, P(contact | T), intervals
- [ ] Window selection by weakness class with recorded rationale
- [ ] Interval propagation from factor uncertainty
- [ ] Calibration label assignment
- [ ] Coverage figure attached from Change 006 states

## Phase 4 — Instance aggregation
- [ ] Instance enumeration from inventory per weakness
- [ ] Heterogeneous aggregation `1 − Π(1 − P_i)`, reducing to `1 − (1 − P)^N` when homogeneous
- [ ] Remediation coverage from confirmed-absent re-observation
- [ ] Residual estimate over uncovered instances

## Phase 5 — Guardrails as required checks
- [ ] **Required CI check**: single-stage signal consumption
- [ ] **Required CI check**: no assessment-derived path can write a contact posterior
- [ ] Test: unremediated exposure's P(contact) strictly increases over time
- [ ] Test: reachability unknown never defaults to zero or to a nominal value

## Phase 6 — Surface
- [ ] Per-asset contact panel: factors, sources, time at risk with censoring, interval, label, coverage
- [ ] Explicit statement that the figure prices ambient contact only
- [ ] Factor-removal recompute action

## Exit Criteria

An internet-reachable service with a certificate-transparency-published hostname and no access barrier produces a contact estimate with a decomposed, source-traced factor set, a measured and honestly censored time at risk, a 90% interval, a calibration label and an assessment coverage figure — and removing any single factor recomputes the estimate in one step while leaving every other factor untouched. Contact posteriors remain provably unwritable by any scan-derived path.
