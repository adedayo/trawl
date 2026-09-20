# Status

What is finished, what is in flight, what has not been started, and what is
real work that no change currently owns.

Regenerate the counts with:

```sh
for f in openspec/changes/*/tasks.md; do
  o=$(grep -cE '^[[:space:]]*- \[ \]' "$f")
  x=$(grep -cE '^[[:space:]]*- \[[xX]\]' "$f")
  printf "%-38s open=%-4s done=%s\n" "$(basename "$(dirname "$f")")" "$o" "$x"
done
```

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

**Alerting.** No webhook delivery, no dedup, nothing fires on a new asset or a
new critical finding. Everything discovered is discovered only by someone
looking at the dashboard. For a tool whose value is noticing change, this is
the largest single gap in the repository.

**KEV/NVD/EPSS feed ingestion.** The fields exist and are populated from
ingested payloads, so ordering works — but nothing pulls the catalogues, so
values are only as fresh as the payload that carried them and no asset is
re-evaluated when CISA adds a CVE. The executive view's "Exploited in the wild"
counter rests on this. It is a figure a CISO will read as current.

**AI provider and triage layer.** None of it is built. Note that Change 014's
"AI annotation labelled advisory and visually subordinate" is satisfied only
vacuously — there is no annotation to subordinate. The requirement binds the
moment one is added.

**Smaller, well-defined:** per-asset posture timeline view (the data exists,
only the view is missing); incremental repository rescanning via a persisted
`lastScannedSha`; the dashboard toggle for `secretVerificationEnabled`; secret
findings wired into posture regression; the same-day redeployment runbook and
its rehearsal; the Playwright e2e suite, which is the same task as finishing
accessibility coverage including colour contrast.

## Open decisions

**spartan/ui was specified from day one and never installed.** The app uses
Tailwind directly. The original item argued the component library must not be
retrofitted later — and retrofitting is exactly what adopting it now would
mean. Either the decision has been made by default and should be recorded, or
it is real debt and should be scheduled. It is deliberately left as an open box
in the 001 ledger rather than ticked, because a tick would claim a decision
nobody made.

**There is no `openspec/specs/` tree.** Every requirement lives under
`changes/`, so *proposed* versus *in force* depends on knowing which change
shipped. The CI guard handles either layout, but the accepted requirements
should eventually be promoted into a live spec tree, so that reading the
requirements does not mean reading the changelog.

## Housekeeping note

Archiving is `git mv <change> openspec/changes/archive/`. If an editor has
unsaved buffers open on files in the moved directory, it will write them back
to the old path and silently recreate the directory — this has happened, and
also produced two zero-byte ledgers. After archiving, confirm with
`ls openspec/changes` and check that no ledger is empty.
