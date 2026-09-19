# Design: 002-susceptibility-scoring-integration

## Schema Addition (sketch)

```
findings:                (existing table, unchanged fields)
susceptibilityEstimates: { findingId, stage: 'applicability'|'attempt'|'success',
                            baseRateSource, baseRateRange,
                            adjustments: [{ label, direction, weight, sourceField }],
                            contactStatus: 'internet-facing'|'internal'|'unknown',
                            computedAt }
adjustmentBands:         { category, stage, signal, adjustment, sourceVersion }
```

`adjustmentBands` holds the exploit-probability framework's published EPSS/KEV-band table (and the other named instance-adjustment rows) as versioned config data — updating it when the published framework revises its bands is a data change, not a code change, consistent with the `portability-config` capability's existing engine/config separation.

## Computation (pure function, unit-tested)

`computeSusceptibilityEstimate(finding, asset, adjustmentBands) -> stageEstimates[]`

- **Contact status**: read directly from the asset record's discovery/inventory status (`assets.status`, `assets.type`) — Trawl already knows whether an asset is in the authorized, internet-facing scope. No new signal required; this is a projection of existing data.
- **Applicability stage**: `high` if a scan (nuclei/httpx) directly confirmed the weakness on the live asset; `medium` if only a version/CPE match without direct confirmation. This distinction already exists in the `vulnerability-correlation` capability's matching logic — it is surfaced, not computed anew.
- **Attempt stage**: EPSS score / KEV membership looked up against `adjustmentBands`, applying the exact band table from "The Probability Behind the Finding" (§Where EPSS Fits In): EPSS > 0.50 or KEV-listed → +2.0, EPSS 0.10–0.50 → +1.0, EPSS 0.01–0.10 → 0, EPSS < 0.01 → −1.0, with the no-CVE bands applying when no CVE/EPSS score exists.
- **Success stage**: base rate only, no adjustment beyond what the published framework already assigns per weakness category, unless a future signal (e.g., confirmed external reachability class) justifies one; Trawl does not invent new adjustment categories beyond the published table.

This function is deliberately a straight lookup-and-format layer, not a new scoring model — the arithmetic and the adjustment values are the exploit-probability framework's, not a reinterpretation.

## Evidence Ledger

Each `susceptibilityEstimates.adjustments[]` entry records `sourceField` (e.g., `finding.epssScore`, `asset.status`, `finding.kev`) so a reader can trace any adjustment back to the exact Trawl field that produced it, matching the framework's "hand them the exact ledger row, remove the challenged modifier, and recompute in one line" requirement.

## Export Format

The dashboard's export action serializes a finding's `susceptibilityEstimates` rows into the same shape the Prior Estimator component's inputs expect (category, stage, base rate range, applied adjustments), so a human can paste the block directly into the calculator rather than re-reading the finding and re-entering values by hand. No API integration between the two Angular apps is in scope for this change — export is copy/paste, not a live link.

## Why Not Automate the Full Pipeline

Wiring Trawl directly into the calculator's simulation (skipping the copy/paste step) was considered and rejected for this change: the calculators intentionally keep a human in the loop to apply judgment adjustments the sensor cannot see (e.g., "is this specific credential long-lived and reusable" — an NHI-category fact Trawl's external scanning cannot observe from outside). Automating the hand-off would blur that boundary. A tighter integration remains a plausible future change once enough Trawl instances exist to justify it, not a default.

## Testing

- Unit tests on `computeSusceptibilityEstimate`: pure function, no infra, covers each adjustment band and the KEV/EPSS deduplication rule (an EPSS-derived adjustment and a raw "exploit code available" adjustment must never both apply to the same finding — mirrors the framework's own deduplication rule).
- Convex function test: the `ai-triage` action's extended output includes `susceptibilityEstimates` rows without altering the existing `priority`/`severity`/`aiAnnotation` fields (regression guard on the deterministic-severity/AI-narrative-only boundary).
