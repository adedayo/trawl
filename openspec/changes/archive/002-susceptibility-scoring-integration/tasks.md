# Tasks: 002-susceptibility-scoring-integration

## SUPERSEDED BY CHANGE 009 — DO NOT IMPLEMENT AS WRITTEN

`openspec/project.md` has recorded this supersession for some time, but this
ledger did not, and this is the file someone opens when looking for work. It
listed nine open tasks with no warning attached. That is how superseded work
gets built.

**Why it was superseded.** This change computes susceptibility in one pass — a
band-lookup table mapping EPSS/KEV into an adjustment, applied inside the
triage action. Change 009 separates the same question into three layers with
three distinct evidence classes, because the single-pass version cannot say
*which* evidence moved a number. An estimate whose provenance is not
recoverable cannot be argued with, and an executive figure that cannot be
argued with will eventually be dismissed rather than corrected.

It also assumed the Convex action pipeline that Change 003 removed, and a
`priority` field that `pkg/core/ingest.go` now deliberately leaves unset rather
than populate with an uncalibrated composite.

**What survives.** The band table itself is still useful input, and Change 009
should transcribe it rather than reinvent it — including the requirement,
correct then and still correct, that it live in a versioned config data file
rather than in function bodies.

**What was right and should be carried forward.** Two constraints in this
change are load-bearing and Change 009 must keep them:

- estimates are written *alongside* the deterministic fields and never
  overwrite `priority`/`severity`, with a regression guard proving it
- the surface exposes the evidence ledger and the source field names, not just
  the output number

## Original tasks — not to be implemented

Retained below for provenance so Change 009 can mine them. Deliberately left
unticked: nothing here was built, and ticking them would claim otherwise.

- Confirm Change 001 archived
- Transcribe the EPSS/KEV-band table and named instance-adjustment rows into a versioned `adjustmentBands` config data file
- Add `susceptibilityEstimates` schema table
- Write `computeSusceptibilityEstimate` as a pure, unit-tested function
- Unit tests: one per adjustment band, plus the EPSS/exploit-code deduplication rule
- Extend the triage action to write estimates alongside existing fields
- Regression guard confirming the deterministic fields are unchanged
- Per-finding "susceptibility inputs" panel with evidence-ledger rows
- Copy/export action matching the Prior Estimator component's input shape
