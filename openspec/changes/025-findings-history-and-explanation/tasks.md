# Tasks: 025-findings-history-and-explanation

**Not scheduled.** Phase 1 carries the risk and the value: it fixes a
correctness defect (fixed weaknesses still counted as deficient) and is the
foundation for everything after it. Phases 2 to 5 are straightforward once it
has landed.

**Depends on** 018 (migration mechanism), 019 (feed snapshots, cited in
priority events) and 006 (coverage state). **Informs** 020, whose dedup
identity should be this change's finding key, and 021, whose annotation must
remain subordinate to the narrative defined here.

## Phase 0 — Decide before building

- [x] Write down the finding key per source and what each key deliberately
      ignores. Too broad hides a real change; too narrow makes every rescan
      look new, which is today's behaviour for scan findings
      *(`store.FindingKey` encodes the row identity; signals are (asset, signal), service findings keep their `service:` identifiers, template findings are `scan:` plus a digest of asset, template and match location, secrets are `secret:` plus a digest of repository, rule, file and secret checksum. The line number is deliberately not part of it. Written up in `docs/findings-history.md`.)*
- [x] Settle router versus panel for the detail view, together with Change 023
      *(decided: no router. A small hash address, `#/findings/<key>`, kept in sync with the selected finding, gives a stable shareable address and a working back button without adding routing to an app whose tabs are not routes and whose embedded server has no SPA fallback.)*
- [x] Settle whether compare-two-scans is in the first iteration
      *(in: it is a control over the same changes query that backs the since-last-scan panel.)*
- [x] Confirm the resolution rule per check family: which checks conclude
      (`ok` or `not_found`) strongly enough to resolve what they previously
      raised. DKIM selector probing is the known case where `not_found` is not
      conclusive and must not resolve anything
      *(a check resolves what it raised only when its coverage state is ok or not_found; DKIM additionally requires the posture to call the absence conclusive; template findings do not resolve on absence because a payload does not say which templates ran.)*
- [x] Reconcile the key and event vocabulary with Change 020 before either is
      built, so alerting does not grow a second identity
      *(`store.FindingKey` and the event types are the identity 020 should key dedup on; nothing in 020 is built yet, so the reconciliation is that 020 consumes these rather than defining its own.)*

## Phase 1 — Identity, history and resolution

- [x] Finding key function, pure and deterministic, with table tests
- [x] Replace `uuid.NewString()` in `pkg/core/ingest.go`; a test that ingesting
      the same scan twice yields one finding and one event
- [x] Schema v9 through the 018 mechanism: `scan_runs`, `finding_events`,
      `scan_summaries`; `status`, `resolved_at` and `last_assessed_at` on
      `signal_observations`; `last_seen` and `status` on `secret_findings`.
      Extend the frozen-schema migration test
- [x] One write path that records the event and updates the projection in a
      single transaction. No caller writes either directly
- [x] Transition state machine: first seen, changed, priority changed,
      resolved, reopened. Table-driven tests covering present, absent and
      assessed, absent and `check_failed`, absent and `not_checked`,
      reappearing, severity change, and library-version change
- [x] Resolution only after a concluded check. A `check_failed` or
      `not_checked` run leaves the finding open and marked *not re-assessed*
- [x] Posture derivation ignores resolved observations; test named for the
      defect, `TestAFixedWeaknessStopsCountingAsDeficient`
- [x] Each event carries library and registry versions; a library-driven change
      is labelled as such rather than as a change in the estate
- [x] Feed-driven priority change events cite the feed snapshot identifier
- [x] Pre-history baseline event for existing rows, labelled as such
- [x] Boundary test: the store exposes no update or delete for the append-only
      tables
- [x] UTC throughout and an injected clock; handle the currently ignored
      `time.Parse` errors on read
      *(new tables and the finding, signal and secret writers use UTC and an injectable clock, and their reads fail on an unreadable timestamp instead of returning a zero time. Older writers (assets, jobs, posture snapshots) still use the wall clock and are unchanged.)*
- [x] Retention setting for `scan_summaries`; events are never pruned. Erasing
      discovered data wipes the history with it

## Phase 2 — Read API and transport

- [x] Unified findings list: key, source, status, first and last seen, age
- [x] Finding detail: facts, explanation, guidance, evidence, timeline, related
      regressions, provenance, coverage context
- [x] Scans list, history summary series, and a changes query between two runs
      (new, resolved, worsened, improved)
- [x] HTTP and Wails transports; `parity_test.go` green; read-API authorisation
      per Change 016

## Phase 3 — Executive narrative and guidance

- [x] `config/narratives/` schema and loader, versioned like the signal
      registry. Keys: signal identifier, then weakness class, service class,
      secret type, CVE template
      *(78 entries in six files under `config/narratives/`, loaded with strict decoding. Lookup order is signal, then check, then weakness class, then service class, secret type, CVE template, generic. Check precedes class because the check families (SPF, DMARC, DKIM and so on) are narrower than the ten classes, and thirty email signals share one class.)*
- [x] Fixed sections for every finding: what we found, why it matters, how it
      could be abused, what good looks like, what to do, how a re-scan
      confirms the fix, references
- [x] Instance facts (the observation `Detail`, evidence, service and port) are
      interpolated by a deterministic template; no free generation
- [x] KEV and EPSS expressed in words with the date they were checked, not as a
      probability figure
- [x] Author narratives for the email-authentication, transport, PKI and
      reputation families first, then service exposure classes, secret types
      and the CVE template
- [x] **Coverage test:** CI fails when any catalogue identifier, registry
      weakness class or service class has no narrative or fallback
- [x] **Lint test:** no currency or probability claim, no "secure" or "fine"
      wording attached to a non-assessed state, no organisation-specific names.
      Extends the 012 boundary test
      *(enforces currency, percentage, probability, em-dash, assurance-word, non-US-spelling and sentence-length rules over all content and over rendered output for every catalogue identifier in every coverage state. It cannot check person names or acronym expansion, which were authored to but are not enforced.)*
- [x] Narrative version recorded in the view, so a reader can tell which
      wording they were shown
- [x] Reword the README line about the AI layer: the baseline explanation is
      deterministic and the AI layer is optional

## Phase 4 — Findings UI

- [x] Unified list across all sources with filters for status, severity,
      source, asset and *changed since*
      *(filtering is client-side over the full index, which is fetched once; the server filters exist on the endpoint and transport.)*
- [x] Rows are real links or buttons with keyboard support; focus returns to
      the row on close
      *(focus returns to the row when closing a detail opened from the list; a detail opened from the Email or Secrets tab returns to that tab without restoring focus to the originating link.)*
- [x] Detail view: header with status, first seen, last seen and age; executive
      card; guidance; evidence; timeline; provenance (scan run, library
      version, feed snapshot)
- [x] Email and Secrets tabs link to the same detail
      *(matched on the source row's own id (`signalId`, `secretId`), which the summary carries because the key is opaque.)*
- [x] Remove or populate the CVSS figure, which has no backing column
      *(removed; there was no backing column.)*
- [x] Not-re-assessed findings are visibly distinct from open ones
- [x] Specs for list, detail and timeline; the boundary spec extended to the
      narrative text

- [x] **Verify the UI in a browser against a running server.** The specs and
      the production build pass, and the API was exercised end to end against a
      seeded database, but no one has looked at the rendered list, detail,
      change panel and trend chart. jsdom cannot judge layout, focus rings or
      colour contrast
      *(verified in Chromium against a running `trawl server` seeded with a synthetic estate; caught and fixed a startup hydration bug in `app/src/app/app.ts` where `ngOnInit` omitted `refreshFindings`, `refreshFeedStatuses`, and `refreshVersion` instead of calling `refreshAll()`.)*
- [ ] Decide the initial-bundle budget. The production build now reports
      533 kB against the 500 kB warning (494.56 kB before this change); the
      error budget is 1 MB. About 65 kB is already deferred. Either raise the
      warning budget or trim the eager service and transport code

## Phase 5 — Change views

- [x] "Since the last scan" panel on Overview
      *(its default comparison is against the nearest earlier run of the same kind on the same asset, so a feed refresh is not compared with an assessment.)*
- [x] Trend lines from `scan_summaries`, each point carrying its coverage so a
      thin run does not read as an improvement
      *(gaps drawn as gaps; runs with lower coverage than the one before are drawn hollow.)*
- [x] Compare-two-scans view, if settled into this iteration
- [x] Documentation: the finding key, the resolution rule, what history does
      and does not cover, and the retention setting

## Phase 6 — Feed catalogue persistence and freshness

Raised after Phase 5: the feed snapshots (Change 019) recorded metadata and
per-finding results but not the catalogue entries, and "retrieved" stopped
moving while a catalogue was unchanged, so a feed confirmed this morning could
read as days old.

- [x] Store the entries of the latest CISA KEV snapshot with the snapshot, in one
      transaction; keep every snapshot's metadata; backfill a snapshot stored
      before entries were kept
      *(schema v10: `feed_records`. EPSS and NVD are deliberately not stored entry by entry: EPSS is hundreds of thousands of rows a day, NVD is referenced CVEs only. The snapshot row describes them.)*
- [x] Record the last successful consultation per feed, separate from the
      snapshot's arrival time; a failure does not move it
      *(schema v10: `feed_checks`.)*
- [x] Judge staleness on the last consultation, falling back to the snapshot's
      time for databases with no recorded check
- [x] A refresh of unchanged content evaluates findings recorded since, and adds
      no run when there is nothing to evaluate
- [x] Refresh a single named feed over HTTP and Wails; refuse an unknown name;
      serialise refreshes
- [x] Executive view: when the catalogue was last refreshed, when its content
      arrived, entry count, last failure, a refresh per feed
- [x] EPSS refresh fetches only the CVEs on stored findings, from FIRST's
      per-CVE API, in batches; a failed batch fails the whole fetch; a snapshot
      records which CVEs it asked about so a later finding is left unchecked
      rather than recorded as unscored; the daily file remains for offline use
      and mirrors
      *(`epss-api` endpoint key; `feed.Snapshot.Queried`. NVD gained the same scope record.)*
- [x] Documentation in `docs/feeds.md`
- [ ] Verify the refresh controls in a real browser, and the KEV and EPSS
      refreshes against the live CISA and FIRST endpoints. The FIRST API's
      response shape and query-length limit are taken from its documentation
      and not observed. The tests use a local stand-in for the source, and no one has
      looked at the rendered catalogue list

## Exit Criteria

Running the same scan twice produces one finding with an unchanged timeline.
Fixing a weakness and re-scanning resolves it, and the domain's posture stops
counting it. A re-scan in which the check could not conclude leaves it open and
says so. Any finding can be opened from the list, shows when it first appeared
and every change since, and explains in plain English why it matters and what
to do, with no figure the engine cannot support. A reader can tell a change in
the estate from a change in the scanner or a feed.
