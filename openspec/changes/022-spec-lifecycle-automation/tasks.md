# Tasks: 022-spec-lifecycle-automation

**No dependency.** Every other change benefits from landing this first, because
each one ends in an archive operation that has twice gone wrong.

## Phase 0 — Repair what was already broken

- [x] Removed `openspec/changes/017-email-posture-surface/`, a duplicate of the
      archived change recreated by an editor buffer after `git mv`. Confirmed
      byte-identical to the archived copy before deleting, so nothing was lost
- [x] Reconciled STATUS.md against the tree: 019, 020 and 021 indexed, the
      resolved *Unclaimed work* entries retired, the spartan/ui open decision
      moved to Change 023 where it can be settled rather than restated

## Phase 1 — Make the tree check itself

- [x] `cmd/specledger`, following the `cmd/egressdoc` precedent: writes
      locally, `--check` in CI. A committed document that CI regenerates is
      verifiable; one maintained by hand is only plausible
- [x] `boxes` counts ticked and unticked separately, because they mean
      different things — a tick records a conclusion, which may be that the
      work landed or that a decision not to do it was taken in place
- [x] Reports a change present under both `changes/` and `changes/archive/`.
      This is the 017 incident, and the previous remedy was a sentence in
      STATUS.md telling the reader to run `ls`. Nobody ran `ls`
- [x] Reports an empty ledger. A zero-byte tasks.md reads from outside as a
      change with nothing left to do, which is why it survived a commit
- [x] Reports an archived change with open items, and a closed ledger still
      sitting in the active set. Both make the active set misdescribe itself
- [x] Reports an active change with no checklist at all — a proposal that never
      became a plan. Archived ledgers are exempt: 002 was superseded and its
      checklist replaced with an explanation of why not to build it, which is
      more use than nine ticked boxes
- [x] Reports a change folder not named in STATUS.md, and a STATUS row naming
      no change folder
- [x] Rewrites only the numeric cells, handling all three table shapes — two
      counts, one count, none — without knowing which table it is in. Learning
      that would mean parsing headings, and would break the first time one was
      reworded
- [x] `notArchivable` carries its reason as data. 001 holds the live capability
      specs, so its ledger closing does not make it archivable. An unexplained
      exemption is how a guard degrades into advice
- [x] Structural problems are reported and never auto-corrected. Each has a
      right answer only its author knows
- [x] Unit tests for every rule above, including the 017 duplicate and the
      prose-only archived ledger — `cmd/specledger/ledger_test.go`
- [x] Wired into `.github/workflows/ci.yml` and `./test.sh`
- [x] Deleted the hand-run count loop from STATUS.md and replaced it with the
      command. Counts maintained by memory are counts that are wrong

## Phase 2 — Promote accepted requirements

- [ ] Decide the shape of `openspec/specs/`: one directory per capability,
      holding the requirements in force, with each stating the change that
      accepted it. Provenance has to survive promotion or the specs become
      assertions with no history
- [ ] Promote the requirements from archived changes — 003, 004, 015, 017 —
      into that tree, one change per commit
- [ ] Promote from substantially delivered changes only what has actually
      landed. A requirement from an open change is *proposed*, and promoting it
      because most of its siblings shipped is how a wish becomes a guarantee
- [ ] Extend `cmd/specledger` to report a capability spec under `changes/` for
      a change that is archived, so a promotion cannot be forgotten
- [ ] Confirm `scripts/check-spec-datastore.sh` still passes over the new tree.
      Its allowlist is anchored on change names, not paths, precisely so that
      moving files does not break it

## Phase 3 — Requirement shape

- [ ] Check that every requirement has at least one scenario, which
      `openspec/config.yaml` already requires and nothing enforces. A
      requirement with no scenario cannot be verified, and an unverifiable
      requirement is a wish
- [ ] Check that requirements use SHALL or MUST and scenarios use
      GIVEN / WHEN / THEN
- [ ] Decide whether this belongs in `specledger` or a separate command. Two
      unrelated failure modes behind one exit code make a failure harder to
      read, which is the argument for keeping `check-spec-datastore.sh` separate

## Exit Criteria

A reader can determine what Trawl guarantees today by reading
`openspec/specs/`, without reconstructing it from the changelog. Archiving a
change and forgetting any part of the operation fails CI rather than being
noticed months later by accident. No number in STATUS.md is maintained by
anyone remembering to update it.
