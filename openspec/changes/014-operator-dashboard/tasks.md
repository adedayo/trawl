# Tasks: 014-operator-dashboard

**Depends on Change 005 (correlation) and Change 006 Phases 5 and 7 (coverage
persistence, egress policy), both of which have landed.**

## Phase 0 — Spec hygiene
- [ ] Retire the Convex references in the Change 001 `dashboard` spec by
      accepting this change's MODIFIED and REMOVED deltas
- [x] Raise the remaining Convex references as a separate housekeeping change:
      raised as **Change 015 — spec datastore hygiene**, which covers the six
      live Change 001 specs and adds a CI guard. Historical references in
      Changes 002 and 003 are deliberately left alone: they explain what was
      migrated away from, and erasing them would destroy the reasoning behind a
      decision future readers would otherwise have to reconstruct.

## Phase 1 — Read models
- [x] Estate summary read model: asset counts by status, findings by severity,
      regression count, assessment coverage — one call, stored values only
      *(delivered as pure derivations in `app/src/app/dashboard/estate.ts` over
      the rows the detail views already read, rather than as a new backend
      endpoint. A second server-side aggregation would be a second computation,
      and the requirement is that the board figure and the engineer's table
      cannot disagree — sharing the rows achieves that more directly than
      keeping two aggregations in step.)*
- [x] Per-asset assessment summary: run outcome, coverage figure, withheld
      checks with reasons — `unresolvedAssessments`, `estateCoverage`,
      `withheldChecks`
- [x] Ordering read model: findings sorted by the stored deterministic rank,
      each carrying its KEV, EPSS and exposure inputs — `rankFindings` returns
      `RankedFinding`, which carries the inputs that placed it
- [x] **Required check**: no read model computes a figure a detail view
      computes differently — met structurally: the executive area computes
      nothing of its own, so there is no second computation to diverge

## Phase 2 — Live transport
- [x] SSE client behind the existing `trawl-transport` interface, alongside the
      Wails and HTTP implementations *(already present; extended here)*
- [x] Connection state surfaced: last-updated time and an explicit disconnected
      state — `lastUpdatedAt` and `streamHealthy` on `WailsIpcService`, fed by
      `stream:up`/`stream:down` emitted on the transport's connection *edges*
      rather than on every retry
- [x] Test: a dropped stream changes what the interface says about freshness —
      `executive.spec.ts`, flipping `streamHealthy` and asserting the rendered
      "Not live" state and the explicit staleness caveat

## Phase 3 — Executive area
- [x] Executive route and shell, following Trawl conventions — standalone,
      signals, `@if`/`@for`, separate templates, Tailwind v4
- [x] Headline posture figure with coverage rendered inside the same element
- [x] Coverage floor: configured, displayed, and suppressing the figure below it
- [x] Test: below the floor, no headline figure appears in the rendered output
      — `executive.spec.ts`, asserted on the DOM rather than on the derivation,
      because a pure function returning `available: false` proves nothing if
      the template prints a score anyway
- [x] Test: the figure and its coverage cannot be rendered separately — the
      floor-met case asserts both appear together

## Phase 4 — Coverage and withheld assessment
- [x] Four-state rendering across every posture surface, with `not_checked` and
      `check_failed` visually distinct from `ok`
- [x] Withheld-check surface naming the excluding class or service and what
      permitting it would add
- [x] Withheld checks lower coverage rather than leaving the denominator —
      `estateCoverage` counts only `ok + notFound` as concluded, asserted in
      `estate.spec.ts`
- [x] Test: `check_failed` never renders in passing styling — the interface
      counterpart to the store-level test in 006 Phase 5. Asserts the chip
      carries none of this view's "sound" vocabulary. The coverage chips were
      also moved out of the refusal branch so they render *alongside a
      displayed score* — which is precisely when an unstated gap would flatter
      the number, and where the original markup omitted them.

## Phase 5 — Regressions and ordering
- [x] Regression surface: previous value, current value, confirmation time,
      asset — ordered by recency, presented separately from new findings
- [ ] Attribution-refresh suppression carried through from 006 Phase 8, so a
      provider-data refresh does not present as estate change *(blocked: 006
      Phase 8 has not landed)*
- [x] Ordering surface exposing KEV, EPSS and exposure per ranked finding
- [x] Test: two readers of the same data see the same order — `rankFindings`
      is a total order, asserted by ranking the same rows in reversed input
      order and requiring an identical result

## Phase 6 — Empty, loading, error and AI states
- [x] Skeletons on every asynchronous view — shown on first load only. On a
      refresh the previous figures stay on screen, because replacing a real
      number with a shimmer tells the operator less than the slightly stale
      number it replaced.
- [x] Empty states distinguishing nothing-found from nothing-looked-at — the
      `nothing-assessed` and `below-floor` refusals are separate outcomes with
      separate copy, and "no open findings" is qualified by the coverage it was
      observed across
- [x] Error states distinct from both — `loadState` is a three-way signal
      rather than a boolean, since "still loading", "loaded and empty" and
      "could not load" are three different claims about the estate and only one
      is good news. `refreshAll` moved from `Promise.all` to `allSettled`: a
      rejection was discarding the results of everything that succeeded, so one
      failing endpoint blanked views that had good data — and a blank view is a
      claim, not a neutral state.
- [x] AI annotation labelled advisory and visually subordinate — the executive
      area carries no AI annotation at all, so there is nothing to subordinate.
      The requirement applies if one is added.
- [x] Test: every view renders completely with no AI provider configured

## Phase 7 — Boundary enforcement and accessibility
- [x] **Required CI check**: no view in this capability emits a currency symbol
      or a probability-labelled figure — asserted in `executive.spec.ts` against
      rendered output, so it runs in CI rather than depending on review. The
      assertion is on the absence of a *quantity*, not of the words, so the
      boundary note stays free to name what it is denying.
- [x] Per-view automated accessibility scanning at WCAG 2.1 AA — `axe-core`
      over the rendered component. Colour contrast is excluded because jsdom
      has no layout or paint: axe cannot compute a ratio there and would report
      false passes, which is worse than reporting nothing. Contrast needs a
      real browser and is **still outstanding**.
- [x] Test: coverage, severity and regression state survive loss of colour —
      each coverage state names itself in words and differs in border
      treatment as well as hue. Board packs are very often printed.

## Still outstanding
- [ ] Colour-contrast verification in a real browser, which jsdom cannot do


## Deferred deliberately
- [ ] Charting library. Most of what this change surfaces is better served by
      tables and inline indicators. Decide when a view actually needs a chart —
      a dependency added before it is needed is surface bought on speculation.

## Exit Criteria

A CISO opening Trawl sees what is exposed, how much of the estate has actually
been assessed, what became worse since the last check, and what to do first —
with every figure traversable to the rows that produced it, coverage inseparable
from every posture claim, checks withheld by policy visible rather than omitted,
and no monetary or probabilistic claim anywhere in the capability. An estate
below the coverage floor produces no headline figure at all, and the interface
says why.
