# Change: 017-email-posture-surface

## Status

**Implemented.** The view now renders the posture record. The coverage figure
is computed in Go by `EmailPosture.Assessed` and exposed additively on the
wire, so there is one definition of "assessed" rather than two — the reasoning
is recorded in Phase 1 of `tasks.md`.

## Why

Change 006 Phase 9 replaced `EmailPosture`'s row of booleans with seven
`EmailControl` values, each carrying a four-state outcome, a reason, and — for
DKIM — whether the result may be read as a fact about the domain at all. The
whole point of that work was to stop an unassessed control reading as a healthy
one.

The backend now makes that distinction. The interface still cannot show it.

`GET /api/v1/email-postures` and the `GetEmailPostures` binding both serve the
widened shape, and `WailsIpcService.emailPostures` holds it — as
`signal<any[]>`, consumed by nothing. The email tab renders from a different
pipeline entirely (`DomainAssessment.controls`, via `assessments`), so the
posture record, its parsed DMARC tags, its deterministic priority and its DKIM
selector evidence are all computed, persisted, transported, and then dropped on
the floor.

That is worse than not having built it. An operator looking at Trawl today
cannot tell whether a domain has no DMARC record or whether the resolver was
down when we asked — which is precisely the confusion the four-state model was
introduced to prevent, and it is now a UI defect rather than an engine one.

There is a second reason to do this now rather than later. `store.EmailPosture`
carries `Assessed() (assessed, total int)`, a Go method, so it does not
serialise. Any view showing "3 of 7 assessed" must derive that figure in
TypeScript from the seven states. That is a second definition of "assessed",
and two definitions of a coverage figure will eventually disagree — at which
point one half of the product reports coverage the other half denies. The
decision about where that figure is computed belongs with the work that first
needs it.

## What Changes

- **The email view reads the posture record.** `EmailPostureUI` is bound to a
  view that renders all seven controls with their state, detail and reason.
  Today's assessment-derived rendering and the posture record are reconciled
  rather than left as two parallel accounts of the same domain.
- **Four states are rendered as four things.** No control is drawn as a tick or
  a cross. `not_checked` and `check_failed` are visually distinct from
  `not_found`, and both display their `reason` — an unexplained "we could not
  tell" is a gap the reader cannot close.
- **DKIM inconclusiveness is honoured.** A `not_found` DKIM control with
  `conclusive: false` is never labelled "DKIM absent". It reports which
  selectors were examined and that the search cannot be exhaustive, because a
  reader told otherwise would commission work that is very likely already done.
- **Coverage accompanies the posture**, per the project-wide rule that an
  assessment figure travels with the coverage of its inputs. Where that figure
  is computed — Go or TypeScript — is decided in this change and enforced by a
  test either way.
- **The DMARC tags are shown as evidence.** Policy, `pct`, `sp`, alignment and
  reporting are rendered next to the derived priority, so the severity a CISO
  is shown can be checked against the record it came from. Partial enforcement
  (`p=reject; pct=40`) reads as partial enforcement.
- **DMARC policy drift is surfaced.** Phase 9 records `dmarc_policy` as a
  posture attribute; a weakened policy is already detected and stored, and
  nothing displays it.

## Explicitly Out of Scope

- **No new assessment.** This change renders what Phase 9 already produces. If
  a field is missing, it is added upstream in vantage and pinned, per 006.
- **No severity computation in TypeScript.** `priority` is computed in Go by a
  pure function and rendered verbatim. The UI never derives, upgrades or
  downgrades a severity.
- **No monetary value and no probability of compromise.** Change 012's
  boundary test applies here as everywhere.
- **No charting.** Consistent with 014's recorded decision.
- **No AI-written prose.** There is no annotation layer yet; when there is, it
  is advisory and visually subordinate, and this view inherits that rule.

## Impact

- **Frontend only, plus a possible additive API field** if the coverage figure
  is decided to belong in Go. No schema change, no assessment change.
- `app/wailsjs/go/models.ts` and `app/src/app/models/types.ts` are already
  regenerated and rewritten against the widened shape, so this change starts
  from types that tell the truth.
- **Closes the last gap in 006's exit criteria** as far as the operator is
  concerned: coverage accompanies every aggregate only if a reader can see it.
