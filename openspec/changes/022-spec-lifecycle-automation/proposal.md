# Change: 022-spec-lifecycle-automation

## Status

**In progress.** Phase 1 landed with this change; Phases 2 and 3 are open.

## Why

This project's central argument is *say what you expect, then record what
happened*. The spec tree is where that argument lives, which makes its
correctness a product property rather than housekeeping. It has been wrong
three times, in three different ways, and each was discovered by accident.

**A change lived in two places at once.** 017 was archived with `git mv`. An
editor holding an unsaved buffer on one of the moved files wrote it back to the
old path, recreating the directory. The active change set silently regrew a
change that had been closed. The existing remedy was a note in STATUS.md saying
to run `ls openspec/changes` afterwards. Nobody ran `ls`, which is the expected
outcome for any check whose enforcement mechanism is a sentence.

**A ledger was truncated to nothing** while the proposal beside it went on
citing "Phase 1 of tasks.md" for a decision. A zero-byte ledger is worse than a
missing one: from outside it reads as a change with nothing outstanding. It was
found by opening the file for an unrelated reason.

**The counts in STATUS.md were maintained by a shell loop pasted at the top of
the file**, to be run by whoever remembered. Counts maintained by memory are
counts that are wrong, and wrong counts in a status document are worse than no
counts, because they are quoted.

There is a fourth, structural problem. `openspec/specs/` is empty and every
requirement lives under `changes/`. Whether a requirement is *proposed* or *in
force* is therefore not readable from the tree — it depends on knowing which
change shipped. A reader wanting to know what Trawl guarantees today must read
the changelog and reconstruct it. That is precisely the condition Change 015
was created to fix in a narrower form.

The common thread: every one of these is a claim the tree makes implicitly, and
implicit claims are not checked. The remedy is to make them explicit and let CI
read them.

## What Changes

- **A ledger reconciler**, `cmd/specledger`, following the `cmd/egressdoc`
  precedent: it writes locally and runs `--check` in CI, so the committed
  status document is verifiable rather than merely plausible.
- **The counts in STATUS.md are generated**, and the hand-run shell loop is
  deleted. Only the numeric cells are rewritten; the prose beside them is the
  part a human wrote and the part worth keeping.
- **The structural failures are reported and not corrected.** A change in two
  places, an empty ledger, an archived change with open items, a closed ledger
  still in the active set, a change absent from STATUS. Each has a right answer
  only its author knows, so the command names the problem and stops.
- **Exemptions carry their reasons in data.** 001 holds the live capability
  specs, so its ledger closing does not make it archivable. That is recorded as
  a map entry with the reason attached, not as a branch in the check — an
  unexplained exemption is how a guard degrades into advice.
- **Promotion of accepted requirements into `openspec/specs/`**, so that what
  is in force can be read without reconstructing it from the changelog, and a
  requirement's status is a fact about where it lives.
- **Archiving becomes a checked operation** rather than a remembered one.

## Explicitly Out of Scope

- **No automatic archiving.** Deciding a change is complete is a judgement
  against the code, and `openspec/config.yaml` is explicit that a wrongly
  archived change is harder to notice than an open one. The command will say a
  ledger has closed; a human decides whether that is true.
- **No automatic ticking of boxes.** A tick is a claim that something landed.
  Nothing can infer that from a checklist.
- **No rewriting of prose.** Regenerating the STATUS tables wholesale would
  destroy the reasoning in each row in order to correct two integers.
- **No replacement of `check-spec-datastore.sh`.** It answers a different
  question — whether a live spec names a component that no longer exists — and
  merging the two would produce one command with two unrelated failure modes.
- **No spec linter for requirement shape.** Whether every requirement has a
  scenario is a real check and a separate one; this change is about lifecycle.

## Impact

- **Adds a CI step and a step to `./test.sh`.** Both fail on a tree that
  disagrees with itself.
- **Adds `cmd/specledger`**, a developer tool alongside `egressdoc` and
  `signalgen`. Not shipped, not part of any release artefact.
- **Changes how STATUS.md is edited.** Counts become generated output; anyone
  editing them by hand will have their edit overwritten, which is the intent.
- **Removes an open decision** by making the `openspec/specs/` promotion a
  scheduled task rather than a question.
