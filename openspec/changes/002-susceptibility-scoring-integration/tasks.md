# Tasks: 002-susceptibility-scoring-integration

**Do not start until Change 001 is implemented and archived.**

## Phase 0 — Preconditions
- [ ] Confirm Change 001 archived (`openspec/specs/` populated, `changes/001-initial-build/` moved to `archive/`)
- [ ] Transcribe the exploit-probability framework's EPSS/KEV-band table and named instance-adjustment rows into a versioned `adjustmentBands` config data file (not hardcoded in function bodies)

## Phase 1 — Deterministic computation
- [ ] Add `susceptibilityEstimates` schema table
- [ ] Write `computeSusceptibilityEstimate` as a pure, unit-tested function (contact status, applicability from scan-confirmation vs. version-match, attempt from EPSS/KEV band lookup, success from category base rate)
- [ ] Unit tests: one per adjustment band, plus the EPSS/exploit-code deduplication rule

## Phase 2 — Wiring into ai-triage
- [ ] Extend the `ai-triage` Convex action to call the computation and write `susceptibilityEstimates` rows alongside (never overwriting) existing `priority`/`severity`/`aiAnnotation` fields
- [ ] Convex function test: regression guard confirming the deterministic fields are unchanged by this addition

## Phase 3 — Dashboard surface
- [ ] Add a per-finding "susceptibility inputs" panel: stage estimates, evidence-ledger rows with source field names, contact status
- [ ] Add a copy/export action formatting the panel's data to match the Prior Estimator component's input shape

## Exit Criteria
A KEV-listed finding on a confirmed internet-facing asset produces a structured stage-estimate ledger (contact, applicability, attempt, success, each with named sources) that a human can paste into the Prior Estimator with zero hand-transcription of the underlying KEV/EPSS/CVSS/contact facts — and the existing deterministic priority/severity fields are provably unchanged by the addition.
