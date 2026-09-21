# Change: 023-spartan-ui-adoption

## Status

**In progress.** Settles an open decision that had been open since Change 001.

## Why

`openspec/project.md` has said since day one that the frontend uses "Tailwind
CSS v4 + spartan/ui for styling and components", and that all styling goes
through them with "no ad hoc CSS". The 001 ledger carries the item as an open
box. `@spartan-ng/cli` is in `app/package.json` as a devDependency.

Nothing else is true. No primitive was ever generated, `@spartan-ng/brain` is
not installed, and every component in `app/src/app/components` is hand-written
Tailwind. Roughly two thousand lines of template assert their own conventions.

The 001 item argued that a component library must not be retrofitted later.
That argument was correct, and it is now the argument against acting, because
retrofitting is precisely what adopting spartan now means. That is why the box
was left open rather than ticked — a tick would have claimed a decision nobody
made. But an open box that stays open for the life of a project is a decision
made by default and not written down, which is the one thing this project's
ledger discipline exists to prevent.

So the decision has to go one way or the other, and there are reasons to take
it rather than record the default.

**Accessibility is the substantive one.** Change 014 has an outstanding
colour-contrast item and an unbuilt accessibility suite. The parts of a
component library that are genuinely hard to hand-write are the parts nobody
hand-writes correctly: focus management, `aria-*` wiring, keyboard interaction,
focus trapping in overlays, live-region announcement. spartan's brain layer is
exactly that, built on the Angular CDK, and separated from presentation. A tool
whose output is read by people making decisions under regulatory obligation
should not be inaccessible because its dialogs were written by hand.

**The retrofit cost is lower than it appears**, because spartan's helm layer is
generated into the repository as source and styled with Tailwind. It is not a
runtime dependency with its own design language to fight. Adopting it converts
hand-written Tailwind into Tailwind organised behind named primitives; it does
not replace the styling system.

**The alternative is worse than it sounds.** Declining means the project keeps
hand-rolling overlays, comboboxes and data tables as the executive workbench
(012) and the operator surfaces grow. That is where accessibility defects are
introduced, and they are introduced silently.

## What Changes

- **The decision is recorded either way.** This change ends with spartan
  adopted and the 001 item ticked with its reasoning, or with the decision
  recorded as declined and the reasoning kept. An open box is no longer an
  acceptable outcome.
- **`@spartan-ng/brain` and `@angular/cdk` are installed**, pinned against
  Angular 22, which brain 1.4.1 supports.
- **Primitives are generated into the repository** under a dedicated directory,
  as source, reviewed like any other code. They are not vendored blindly: a
  generated component that nothing uses is dead code with a licence.
- **Migration is incremental and per-surface**, each surface landing with its
  tests passing. A big-bang conversion of every template at once cannot be
  verified, because most of what would change is visual and the suite is
  jsdom.
- **Accessibility assertions accompany each migrated surface**, so the reason
  for adopting the library is demonstrated rather than assumed. Migrating to an
  accessible primitive and not testing accessibility buys nothing that could
  not have been had by leaving the markup alone.
- **`project.md` is corrected** to describe what is true at each point, rather
  than what was intended in 001.

## Explicitly Out of Scope

- **No redesign.** This is a change of construction, not appearance. A
  migration that also restyles cannot be reviewed, because every diff is both.
- **No new surfaces.** Existing components only.
- **No dependency on spartan for anything the platform provides.** A native
  element that is already accessible stays a native element.
- **No adoption of the whole catalogue.** Generate what is used. A directory of
  unused primitives is dead code that looks like capability.
- **No change to the Wails embedding or the build pipeline** beyond the
  dependency additions.

## Impact

- **Frontend only.** No engine, API or schema change.
- **Adds runtime dependencies** — `@angular/cdk`, `clsx`, and spartan's brain
  layer — to a bundle that is embedded into the desktop binary with `go:embed`.
  Bundle size becomes a thing to watch; it was not before.
- **Raises the Angular floor**, since brain requires `>=21.0.0 <23.0.0`. The
  app is on 22, so this constrains future upgrades to spartan's release cadence
  — a real cost and the main argument on the other side.
- **Touches the accessibility work in 014** and should be sequenced with it:
  migrating a surface and then writing its contrast test twice is wasted.
