# Tasks: 023-spartan-ui-adoption

**Sequence with 014.** Its colour-contrast item and its unbuilt accessibility
suite are the substantive reason for adopting the library. Migrating a surface
and then writing its accessibility test is one job; doing them separately means
writing each surface's assertions twice.

**This change must not end with an open box.** It ends adopted, or it ends
declined with the reasoning recorded. The 001 item has been open since day one,
and an open box that stays open for the life of a project is a decision made by
default and never written down.

## Phase 0 — Settle the decision

- [x] Establish what was actually true. `@spartan-ng/cli` was present as a
      devDependency; `@spartan-ng/brain` was not installed, no primitive had
      ever been generated, and every component was hand-written Tailwind.
      `project.md` had described the intended state as the current one since
      001
- [x] Confirm spartan supports the Angular the app is on. brain 1.4.1 declares
      `>=21.0.0 <23.0.0`; the app is on 22
- [x] **Decision: adopt.** The retrofit argument in 001 was correct and is now
      the argument against acting — which is exactly why it cannot be left to
      decide itself. The deciding factor is accessibility: focus management,
      `aria-*` wiring, keyboard interaction and focus trapping are the parts
      nobody hand-writes correctly, and 012's workbench will need overlays and
      data tables. Declining means hand-rolling those, silently, later
- [x] Record the cost accepted alongside it: brain's Angular peer range
      constrains future Angular upgrades to spartan's release cadence

## Phase 1 — Install

- [x] `@spartan-ng/brain`, `@angular/cdk`, `clsx` and `tailwind-merge` added to
      `app/package.json`
- [x] Typecheck and the existing suite pass unchanged with the dependencies
      installed — 116 tests across 13 files, `npm --prefix app run test:ci`.
      Establishes that the install alone changes no behaviour, so anything that
      breaks later is attributable to a migration rather than to the library
- [ ] Record the bundle size before any migration. The bundle is embedded into
      the desktop binary with `go:embed`, so its size was previously nobody's
      concern and is now a thing that can regress unnoticed
- [ ] Decide where generated primitives live and record it in `project.md`.
      They are source, reviewed like any other code, not vendored

## Phase 2 — First surface, as proof

- [ ] Choose one surface with real interaction — not a static panel. A
      migration that proves nothing about focus or keyboard behaviour proves
      nothing about the reason for the change
- [ ] Generate only the primitives that surface uses. A directory of unused
      primitives is dead code that looks like capability
- [ ] Migrate it with no visual redesign. A diff that is both a restyle and a
      restructure cannot be reviewed, because every hunk is both
- [ ] Its existing tests pass unmodified, or each modification is justified in
      this ledger. A test changed to accommodate a refactor is a test that
      stopped checking something
- [ ] Add the keyboard and focus assertions the migration is supposed to earn.
      Migrating to an accessible primitive without testing accessibility buys
      nothing that leaving the markup alone would not also have bought
- [ ] Re-measure the bundle and record the delta here

## Phase 3 — Remaining surfaces

- [ ] One surface per commit, tests passing at each. A big-bang conversion
      cannot be verified: most of what changes is visual and the suite is jsdom
- [ ] Native elements that are already accessible stay native. A `<button>`
      does not need a primitive
- [ ] Stop when the remaining surfaces have no interaction worth the
      dependency, and record where the line was drawn. Partial adoption is a
      legitimate outcome; an unexamined partial adoption is not

## Phase 4 — Close the record

- [ ] Tick the 001 item with the reasoning, replacing the open box that has
      stood since the project began
- [ ] Correct `project.md` to describe what is true rather than what was
      intended. It currently asserts spartan is in use for all components
- [ ] Confirm the accessibility suite in 014 covers the migrated surfaces, so
      the justification for this change is demonstrated rather than asserted

## Exit Criteria

No component asserts its own overlay, focus or keyboard conventions by hand
where a primitive exists for it. Every migrated surface has an assertion about
the behaviour the primitive was adopted for. The 001 ledger records a decision
rather than an open question, and `project.md` describes the codebase as it is.
