# Status

What is finished, what is in flight, what has not been started, and what is
real work that no change currently owns.

The per-change counts in the tables below are maintained by
`go run ./cmd/specledger`, which reads the ledgers and rewrites the numeric
cells. CI runs it with `--check`. Do not edit the counts by hand: counts
maintained by remembering to run something are counts that are wrong, which is
why the shell loop that used to live here was replaced.

The same command reports the structural failures this tree has actually
suffered — a change present under both `changes/` and `changes/archive/`, an
empty ledger, an archived change with open items, a closed ledger still sitting
in the active set, a change nobody indexed here. Those it reports and does not
correct, because each has a right answer only its author knows.

A ticked box means the work landed **or** that a decision not to do it was
recorded in place. Both are conclusions. An open box means work someone still
intends to do — which is why withdrawn and superseded items are ticked with
their reasoning rather than left to accumulate as phantom backlog.

---

## Archived — closed

| Change | Outcome |
|---|---|
| `002-susceptibility-scoring-integration` | **Superseded by 009.** Single-pass band lookup cannot say which evidence moved a number. Two constraints it got right are carried forward. |
| `003-go-sqlite-engine` | Complete. Store, event bus, removal of the previous datastore. |
| `004-desktop-wails-packaging` | Complete. The three scanner runners were **withdrawn**, not deferred. |
| `015-spec-datastore-hygiene` | Complete. Specification repair after 003, plus the CI guard that stops it recurring. |
| `017-email-posture-surface` | Complete. The four-state posture is rendered, and coverage is computed once, in Go, at the serialisation boundary. Phase 7 was added during implementation: displaying the advisories exposed a defect in them, in that a finding drawn from a set named the set and not the member. **Not retrospective** — observations stored before it carry no `detail` until their domain is re-scanned. |

## Active — substantially delivered

| Change | Open | Done | What is left |
|---|---:|---:|---|
| `022-spec-lifecycle-automation` | 8 | 15 | Phase 1 landed: `cmd/specledger` generates these counts and reports the structural failures this tree has actually suffered. What remains is promoting accepted requirements into `openspec/specs/`, so that what is in force can be read without reconstructing it from the changelog, and a requirement-shape check for the scenario rule `config.yaml` states and nothing enforces. |
| `006-vantage-integration` | 2 | 69 | Phases 0–9 complete. What remains is close-out: demonstrating the exit criteria in a single run rather than as separately passing tests, and publishing the commits. Phase 9's gap analysis inverted the phase — the superseded code was not working code but a placeholder that reported its own failures as answers. **Rendering the four-state email posture was Change 017, which has landed and is archived**; the engine distinguishes a gap from an outage, and the view now shows the difference. |
| `016-deployment-parity` | 6 | 44 | Transport-parity assertion, the read-API authorisation model, cross-distribution documentation. Two items — a networked store and a cross-instance bus — are honestly labelled as what horizontal scaling would require, not as work in progress. |
| `013-distribution-and-release` | 3 | 33 | Three verification steps, all needing a **clean machine and a real workflow run**. Nothing here can be verified from a development machine, which is why it is still open. |
| `014-operator-dashboard` | 4 | 28 | Colour contrast in a real browser (jsdom cannot compute a ratio); attribution-refresh suppression, **now unblocked** — 006 Phase 8 landed it as `network_attribution`; two read-model checks. |
| `005-cloud-continuous-easm` | 6 | 13 | Background cron runner, remote-server configuration for the desktop app, Compose/Litestream setup. Its 006 Phase 9 gate has lifted. |
| `001-initial-build` | 22 | 34 | Reconciled 30 Aug 2026 against the code. Its remaining items are mostly tracked by later changes; what nothing tracks is listed under *Unclaimed work* below. Holds the **live capability specs**, so it is not archivable when its ledger closes. |

## Active — not started

| Change | Open | Notes |
|---|---:|---|
| `018-schema-migration-durability` | 8 | 006 Phase 9 was the first change to alter an existing table, and found no mechanism for it. It added one. This hardens that mechanism — a version marker, a refusal to open a newer store, a test that a migrated shape equals a fresh one — before a second change assumes more of it than it does. **017 has since been the second change to use it**, adding `signal_observations.detail`, so this is no longer hypothetical. |
| `019-vulnerability-feed-ingestion` | 24 | **Build next.** KEV, NVD and EPSS are consumed but never fetched, so every exploitation figure is as fresh as the payload that happened to carry it. Supplies 009 with a citable evidence class and gives 014's "Exploited in the wild" counter a date. |
| `020-alerting-and-delivery` | 19 | Specified now, **deliberately unscheduled** — it earns its keep approaching production use. Written early because dedup identity and coverage-in-the-alert are cheap to honour while surfaces are being built and expensive to retrofit once several have grown their own notification paths. |
| `021-ai-annotation-and-triage` | 20 | Not scheduled; the product is complete without it. Phase 0 is worth doing early regardless: **014's "annotation labelled advisory and subordinate" currently passes because there is no annotation**, which is indistinguishable in CI from passing because it holds. |
| `007-contact-probability` | 35 | The keystone: supplies P(contact). |
| `008-risk-model-packs` | 25 | Versioned, signed, source-cited parameters. |
| `009-exploit-probability-engine` | 44 | Absorbs the archived 002. |
| `010-scenario-loss-model` | 32 | |
| `011-control-portfolio-roi` | 44 | |
| `012-executive-workbench` | 37 | Depends on 008–011. Until those land, **no view may state a monetary value or a probability of compromise** — enforced by the boundary test in `executive.spec.ts`, in CI rather than by review. |

---

## Unclaimed work

Real gaps that no change currently owns. Listed here because they were
invisible inside a ledger whose heading reads like history, and an invisible
gap is indistinguishable from a decision not to build something.

The three largest entries that stood here — alerting, feed ingestion, and the
AI layer — now have changes: **019**, **020** and **021**. They are no longer
unclaimed; they are scheduled, deferred and unscheduled respectively, and the
difference between those three is recorded rather than inferred.

**Smaller, well-defined:** incremental repository rescanning via a persisted
`lastScannedSha`; the dashboard toggle for `secretVerificationEnabled`; secret
findings wired into posture regression; the same-day redeployment runbook and
its rehearsal; the Playwright e2e suite, which is the same task as finishing
accessibility coverage including colour contrast.

## Open decisions

**There is no `openspec/specs/` tree.** Every requirement lives under
`changes/`, so *proposed* versus *in force* depends on knowing which change
shipped. The CI guard handles either layout, but the accepted requirements
should eventually be promoted into a live spec tree, so that reading the
requirements does not mean reading the changelog.

## Housekeeping note

Archiving is `git mv <change> openspec/changes/archive/`, one change per
commit. If an editor has unsaved buffers open on files in the moved directory,
it will write them back to the old path and silently recreate the directory —
this has happened twice, and also produced two zero-byte ledgers. **After
archiving, run `go run ./cmd/specledger`.** It catches both failures, which is
why it exists; `ls openspec/changes` caught neither, because nobody ran it.
