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

- [ ] Decide whether `Assessed()` is exposed as an additive API field or
      mirrored in TypeScript. It is a Go method today, so it does not
      serialise. Record the decision here with its reasoning, not only the
      outcome
- [ ] If mirrored: a test asserting the TypeScript and Go definitions agree for
      all four states, so the two cannot drift into reporting different
      coverage for the same posture
- [ ] If exposed: additive JSON fields on the response only — the persistence
      struct does not gain computed columns, and the migration mechanism is
      not involved

## Phase 2 — Reconcile the two accounts of a domain

- [ ] Establish which of `DomainAssessment.controls` and `EmailPosture` is the
      rendering source for each control. Both describe SPF, DKIM and DMARC on
      the same domain, and rendering both would ask the operator to reconcile
      two states for one control
- [ ] Type `WailsIpcService.emailPostures` as `EmailPostureUI[]`. It is
      `signal<any[]>` today, which is how a stale shape survived a phase
- [ ] Remove or rewire whichever source loses, rather than leaving it fetched
      and unread

## Phase 3 — Render the seven controls

- [ ] Per-control rendering of all seven controls in four distinct states, with
      `not_found`, `not_checked` and `check_failed` visually separable without
      opening a row
- [ ] `reason` displayed wherever a control is `not_checked` or `check_failed`
- [ ] DKIM `conclusive: false` rendered as an inconclusive search, never as
      absence; selectors examined and found both disclosed
- [ ] Coverage figure displayed with the posture, per Phase 1's decision
- [ ] Unit tests for the state-to-presentation mapping, including a test that
      no state maps to the same presentation as `ok`

## Phase 4 — Evidence and severity

- [ ] `priority` rendered verbatim; a test asserting the client derives no
      severity of its own
- [ ] Empty `priority` renders as no severity, and does not sort or count as a
      value. An unassessed control has no severity, and displaying one would
      let a resolver outage manufacture a finding
- [ ] DMARC tags rendered as evidence beside the rating: policy, `pct`, `sp`,
      alignment modes, reporting
- [ ] Partial enforcement disclosed as partial. `p=reject; pct=40` is a
      probabilistic control, and a badge reading "reject" would overstate it
- [ ] SPF all-mechanism qualifier and lookup count displayed

## Phase 5 — Drift

- [ ] Surface recorded `dmarc_policy` drift on the domain. Phase 9 detects and
      stores a weakened policy and nothing displays it
- [ ] A failed assessment surfaces no drift, since no fingerprint is written
      for one

## Phase 6 — Guards

- [ ] A test that fails if a control's rendered label asserts a fact the state
      does not support — at minimum, that no `not_checked` control renders
      affirmative text
- [ ] Colour is not the only carrier of state, so the four states remain
      distinguishable without it
- [ ] Change 012's boundary test extended to this view: no monetary value and
      no probability of compromise

## Exit Criteria

An operator opening the email view can say, for every control on every domain,
whether it was assessed; if it was not, why not; if it was, what was observed
and what that is rated; and can check that rating against the record it was
derived from. A DKIM result that establishes nothing says so. No control is
shown as healthy on the strength of a check that did not run.
