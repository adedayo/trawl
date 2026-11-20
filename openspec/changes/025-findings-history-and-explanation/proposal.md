# Change: 025-findings-history-and-explanation

## Status

**Proposed, not scheduled.** Raised from a review of the findings path against
three operator needs: seeing change over time, drilling into a finding, and
understanding why a finding matters without being a security specialist.

## Why

Trawl's argument is that continuous monitoring beats periodic assessment
because it notices change. The detection exists in places. What an operator
can *see* of change does not, and three separate properties of the current
code explain why.

**1. Most state is latest-only, so drift cannot be read back.**
`findings`, `signal_observations`, `email_postures` and `assessment_runs` are
all upserts. `assessment_runs` is latest-only on purpose ("a history nothing
reads is a table that only grows"). The only genuine history is
`posture_snapshots` and `regressions`, and those cover DMARC policy and hosting
attribution only. `secret_findings` records `first_seen` and nothing else — no
`last_seen`, no status — so a leaked secret that was rotated looks identical to
one that is still live.

**2. Findings are never resolved, and some have no stable identity.**
- `pkg/core/ingest.go` gives each scan finding `uuid.NewString()`. The same
  issue found by two scans is two rows, so there is nothing to compare and
  nothing that can ever be marked resolved.
- Nothing deletes or resolves a `signal_observations` row. Posture derivation
  treats any assessed observation as *deficient*, so **a fixed SPF or DMARC
  weakness continues to count against the domain until the data is erased.**
  This is a correctness defect independent of history: the product reports a
  control as failing that is working.
- Only `service:` findings resolve, and only when the probe assessed that class
  of finding (`resolveServiceFindings`). That rule — *resolve only what was
  actually re-assessed* — is the right one and is generalised here.

**3. The Findings tab is a flat list and carries no explanation.**
Rows are not clickable and `routes` is empty. The tab shows only CVE, service
and template findings; Vantage findings live in the Email tab. The Vantage
catalogue already supplies a technical `Description`, `Remediation`,
`References` and `Tags`, but nothing answers the question an executive asks:
*why does this matter, and how would someone abuse it?* The `ai_annotation`
column is never written (Change 021 is unbuilt), and the README's promise of
"plain-English explanations" is currently carried by nothing.

The explanation should not wait on 021. A finding explained differently by a
probabilistic layer on each run is a finding whose meaning cannot be audited,
and the executive text is exactly where Trawl's "computed, traceable,
reproducible" claim is most visible. The baseline is therefore **curated,
versioned, deterministic text**, and any AI layer is later, optional and
subordinate.

## What Changes

- **Stable finding identity.** A finding key derived from what it asserts:
  asset, source, rule or signal identifier and a discriminator (host and port,
  CVE and match location, redacted secret reference). Re-running an unchanged
  scan yields the same finding, not a new one.
- **Append-only history.** A run ledger (`scan_runs`), a per-finding event log
  (`finding_events`) and a per-run summary (`scan_summaries`). Existing tables
  remain the *current-state projection*; the event and the projection are
  written together through a single path so they cannot disagree.
- **Resolution that respects coverage.** A finding resolves only when the check
  that raises it reached a conclusion (`ok` or `not_found`) in a later run and
  did not raise it. `not_checked` and `check_failed` leave it open and marked
  *not re-assessed*. Silence is never read as a fix.
- **Drift is attributable.** Each event records library and registry versions,
  and priority changes caused by feed refreshes cite the feed snapshot. "We
  upgraded the scanner" and "KEV added this CVE on a date" are distinguishable
  from "the estate changed".
- **Drill-down.** A unified findings list across all sources, filterable by
  status, severity, source, asset and *changed since*, with a deep-linkable
  detail view: facts, executive narrative, guidance, evidence, timeline and
  provenance.
- **Executive narrative and guidance.** A versioned `config/narratives/`
  registry, keyed by signal identifier with fallback to weakness class, service
  class, secret type and a CVE template. Every finding renders the same fixed
  sections: what we found, why it matters, how it could be abused, what good
  looks like, what to do, how a re-scan confirms the fix, references.
- **Change views.** A "since the last scan" panel (new, resolved, worsened,
  improved), trend lines from run summaries, and a compare-two-scans view.

## Explicitly Out of Scope

- **No AI-composed narrative.** Change 021 may later rephrase text, labelled
  advisory and visually subordinate. The deterministic text is the baseline and
  is complete without it.
- **No monetary values or probabilities of compromise in any narrative.** Change
  012's boundary applies, and its CI test is extended to cover this text.
- **No alert delivery.** Events are recorded here; Change 020 decides which are
  delivered, and builds its dedup identity on the finding key defined here.
- **No retrospective history.** Rows stored before the migration receive one
  labelled *pre-history baseline* event at their recorded `first_seen`. Earlier
  change is not reconstructed or implied. As with Change 017, a domain gains
  full history only from its next assessment.
- **No exploit procedures.** "How it could be abused" describes the attacker's
  goal and the consequence for the organisation, not a method.
- **Upstreaming narratives into the Vantage catalogue** is a separate, later
  step. This change keeps them in Trawl configuration so they can be reviewed
  and corrected without a library release.

## Impact

- **Schema.** Version 8 to 9 through the Change 018 mechanism, with the frozen
  historical-schema migration test extended. New tables are append-only; the
  store exposes no update or delete path for them, and a boundary test says so.
- **Posture derivation changes behaviour.** Resolved observations stop counting
  as deficient. This is intended, but it will move numbers on existing
  installations at their next assessment, and the release note must say so.
- **Transport.** New read endpoints on all three transports; Change 016's
  parity test applies.
- **UI.** The Findings tab is reworked and gains a detail route. Sequence with
  Change 023 (component adoption) and 014's accessibility work so interactive
  rows, focus handling and the detail overlay are written once.
- **Depends on** 018 (migration), 019 (feed snapshots, cited in priority
  events) and 006 (coverage state, which resolution depends on). **Informs**
  020 (dedup identity) and 021 (the annotation boundary).
- **Org-neutral by rule.** Narrative text names no organisation, vendor
  contract or incident, per the project's rule that the engine carries no
  org-specific data.

## Decisions Recorded Here

1. **Event granularity: transitions plus per-run summaries**, not a row per
   finding per run. Transitions answer "what changed and when" compactly;
   summaries answer "what did the estate look like on that day". The cost of
   the alternative is table growth that nothing in the product needs.
2. **Narratives live in Trawl configuration first**, not in Vantage. Reviewable
   by the project, correctable without a library release, and movable upstream
   later without changing any consumer.
3. **Narrative text is rendered at view time and not persisted**, for the same
   reason `catalogue.go` gives for the catalogue: stored prose freezes wording a
   later review is entitled to correct. What is persisted is facts and keys.
4. **Retention: events kept indefinitely; run summaries prunable** through a
   setting. Events are small and are the record; summaries are derived charts.

## Open Questions

- Router (`/findings/:key`, giving deep links and a working back button) or a
  signal-driven panel. Leaning router; to be settled in Phase 0 alongside 023.
- Whether the compare-two-scans view ships in the first iteration or follows
  the trend lines.
