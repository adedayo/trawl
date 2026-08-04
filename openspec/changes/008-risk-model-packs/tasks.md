# Tasks: 008-risk-model-packs

**May proceed in parallel with 006/007. Pack *content* for contact and email domains depends on both.**

## Phase 0 — Preconditions
- [ ] Decide the signing mechanism and key distribution for shipped packs
- [ ] Source review and transcription plan: which published sources supply which sections

## Phase 1 — Format and loader
- [ ] Pack JSON schema (sections, leaf-object provenance shape)
- [ ] Two-layer resolution: base then override, with layer attribution on every resolved value
- [ ] Signature verification; refuse to load on absence or mismatch
- [ ] Pack version recorded on every estimate written by any consumer

## Phase 2 — Validation
- [ ] Leaf-object validation: no bare values
- [ ] Measured-state signal completeness (variance share, dedup group, decay constant, cap)
- [ ] Variance share requires heterogeneity argument; zero where the source conditioned on the signal
- [ ] Scenario shares sum to 1.0; residuals carry rationale and exception marker
- [ ] Judgment parameters carry sweep points and owner

## Phase 3 — Precision-gain primitives
- [ ] `precisionGain(signals, observedAt, now, pack) -> g` — dedup by group (max, not sum), cap, decay
- [ ] `conditionedESS(essClass, g) -> ess` 
- [ ] Unit tests: dedup, cap, decay to zero, no accumulation from repeated observation, unassessed yields no gain

## Phase 4 — Section content
- [ ] `attemptBands` — migrate Change 002's EPSS/KEV band table into pack form
- [ ] `weaknessClassPriors` — secret, NHI, dependency, cloud misconfiguration, email spoofing, delegation hijack, subdomain takeover
- [ ] `measuredStateSignals` — keyed by Change 006 signal ids
- [ ] `contactBaseRates` — ambient contact rate by service class (consumed by Change 007)
- [ ] `scenarioAnchors` — frequency priors, magnitude percentiles, taxonomy mapping
- [ ] `lossFormOverlays` — six-form default splits per scenario
- [ ] `judgmentParameters` — β₀, m₀, tail weight, with sweeps

## Phase 5 — Update discipline
- [ ] Pack diff report: parameters moved, estimates affected
- [ ] Retention of prior estimates with their original pack version
- [ ] Operator-initiated update only; no automatic fetch

## Phase 6 — Guardrail
- [ ] **Required CI check**: no probability-affecting numeric literal in engine code outside pack loading and fixtures

## Exit Criteria

Every number the engine can produce traces to a pack entry carrying a source, a retrieval date, a verification status and a calibration label; a tampered pack refuses to load; correlated signals narrow an interval once rather than twice; precision gain is capped, decays with age, and does not grow with repeated observation of an unchanged configuration; and applying a new pack produces a diff without silently re-pricing history.
