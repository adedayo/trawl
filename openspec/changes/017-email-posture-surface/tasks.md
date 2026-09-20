# Tasks: 017-email-posture-surface

**Depends on Change 006 Phase 9, which has landed.** The store, the adapter,
both transports and the generated bindings already carry the widened shape.
Everything below is Angular, plus one deliberate decision about where a
coverage figure is computed.

## Phase 0 — Preconditions

- [x] `store.EmailPosture` widened to seven four-state controls — 006 Phase 9,
      `pkg/store/email.go`
- [x] Served over both transports — `GetEmailPostures` binding and
      `GET /api/v1/email-postures`, asserted by `cmd/trawl/parity_test.go`
- [x] `app/wailsjs/go/models.ts` regenerated from the widened structs, so the
      generated `EmailPosture` carries `EmailControl` values rather than the
      booleans it declared until now. Regenerated rather than hand-edited: a
      hand-maintained copy of a generated file drifts silently, and this one
      had already drifted through an entire phase without anyone noticing
- [x] `EmailPostureUI` in `app/src/app/models/types.ts` rewritten against the
      same shape, with `EmailControlUI` carrying `state`, `detail`, `reason`
      and `conclusive`. The old interface declared `spfValid: boolean` and
      `dmarcPolicy: 'reject' | 'quarantine' | 'none' | 'missing'` — a type that
      cannot represent an unassessed control, so any view built against it
      would have been forced to invent one

## Phase 1 — Decide where coverage is computed

- [x] **Decided: exposed additively from Go, not mirrored in TypeScript.**
      `EmailPosture.MarshalJSON` computes `Assessed()` at the serialisation
      boundary and emits `assessedControls` and `totalControls` alongside the
      record — `pkg/store/email.go`, tested by `pkg/store/email_json_test.go`.

      Mirroring was rejected because it makes the definition of "assessed"
      exist twice, in two languages, with nothing but a test holding them
      together. A test can only compare the two definitions that exist today;
      it cannot stop the next state being added to one side. The failure that
      would follow is the worst kind for this product: one half reporting
      coverage the other half denies, with neither visibly wrong.

      Computed at the boundary rather than stored as a column because a stored
      count can go stale against the states it counts. Derived on the way out,
      it cannot. The persistence struct therefore gains nothing, and the
      migration mechanism is not involved
- [x] Mirroring test — **not applicable**, per the decision above. Nothing is
      mirrored, so there are no two definitions to hold together
- [x] Additive JSON fields on the response only. `MarshalJSON` wraps an alias
      that sheds the method, so no persisted shape changed and no column was
      added

## Phase 2 — Reconcile the two accounts of a domain

- [x] Rendering source established: the **posture record is the single source
      of control state**, and the assessment contributes only the advisories
      raised against it. Both pipelines describe SPF, DKIM and DMARC on the
      same domain; rendering both would ask the operator to reconcile two
      states for one control — asserted by *takes each control state from the
      posture record, not the assessment*
- [x] `WailsIpcService.emailPostures` typed as `EmailPostureUI[]`. It was
      `signal<any[]>`, which is how a stale shape survived an entire phase
- [x] The assessment's own account of the seven posture controls is not
      re-rendered beneath them — *does not repeat a posture control under the
      remaining surface*. A domain with a posture record and no assessment
      still appears, rather than vanishing from the only view that shows it

## Phase 3 — Render the seven controls

- [x] All seven controls rendered in four distinct states — *renders every
      control, including those never assessed*
- [x] `reason` displayed wherever a control is `not_checked` or `check_failed`,
      and no reason invented where none was recorded
- [x] DKIM `conclusive: false` rendered as an inconclusive search, never as
      absence, disclosing both the selectors found and those examined
- [x] Coverage figure displayed with the posture, read from the record rather
      than derived in the client, per Phase 1. A record predating the figure
      says so instead of reporting zero coverage
- [x] State-to-presentation tests, including *gives each state its own
      presentation, none shared with `ok`*

## Phase 4 — Evidence and severity

- [x] `priority` rendered verbatim — *renders the stored priority verbatim*
- [x] Empty `priority` renders as no severity and is not counted as a rated
      domain. An unassessed control has no severity, and displaying one would
      let a resolver outage manufacture a finding
- [x] DMARC tags rendered as evidence beside the rating: policy, `pct`, `sp`,
      alignment modes and reporting; no evidence offered for a policy that was
      never observed
- [x] Partial enforcement disclosed as partial. `p=reject; pct=40` is a
      probabilistic control, and a badge reading "reject" would overstate it
- [x] SPF all-mechanism qualifier and lookup count displayed, including what
      `+all` authorises and a warning past the lookup limit

## Phase 5 — Drift

- [x] Recorded `dmarc_policy` drift surfaced against its domain, and drift
      recorded against another attribute ignored
- [x] A domain whose assessment did not conclude reports no drift, since no
      fingerprint is written for one

## Phase 6 — Guards

- [x] *never renders affirmative text for an unassessed control* — a rendered
      label may not assert a fact the state does not support
- [x] *remains legible without colour* — colour is not the only carrier of
      state
- [x] Change 012's boundary test extended to this view — *states no monetary
      value and no probability of compromise*

## Phase 7 — Advisories name what is at fault

Added during implementation. Rendering the advisories made a defect in them
visible that was invisible while nothing displayed them.

- [x] vantage upgraded to v1.6.0, which names the offending item in the
      description of a finding drawn from a set — which `include` term is
      broken, which exchanger does not resolve, which DKIM selector carries
      the weak key. Before it, `SURF-SPF-009` printed an entire SPF record and
      appended the broken term after it, identifying a problem without
      pointing at it
- [x] The occurrence-specific half of the description carried through to the
      view as `SignalObservation.Detail`, via `detail()` in
      `pkg/scanner/vantage/adapter.go`. The catalogue half is still read from
      the installed library at view-build time: it is identical for every
      occurrence of an identifier and a library upgrade should be free to
      reword it. The appended half cannot be recovered that way, because the
      catalogue has never seen this domain
- [x] `detail TEXT` added to `signal_observations` through the additive
      migration mechanism, not the schema alone. `CREATE TABLE IF NOT EXISTS`
      is a no-op against an existing database, so schema-only would have
      reached new installations and broken every installed one on its first
      write. Covered by a test that builds the pre-`detail` shape, seeds a row
      and opens it with the current store
- [x] Rendered as prose between the general explanation and the remediation —
      *names the offending item, not only the identifier*. Asserted against
      the rendered DOM, because the failure being guarded is a template that
      drops a field, with the no-detail case asserted alongside it

## Exit Criteria

- [x] An operator opening the email view can say, for every control on every
      domain, whether it was assessed; if it was not, why not; if it was, what
      was observed and what that is rated; and can check that rating against
      the record it was derived from. A DKIM result that establishes nothing
      says so. No control is shown as healthy on the strength of a check that
      did not run

**Outstanding after this change:** observations stored before Phase 7 carry no
`detail` until their domain is re-scanned, so an advisory raised earlier still
renders without naming its item. The column is nullable and the view treats
absence as "this signal names no particular item", so nothing is broken by it —
but the improvement is not retrospective, and a reader comparing two domains
may see one advisory pinpointed and another not.
