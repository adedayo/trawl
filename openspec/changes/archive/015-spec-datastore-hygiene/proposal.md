# Proposal: 015-spec-datastore-hygiene

## Why

Change 003 replaced Convex with an embedded SQLite store. The code moved; the
specifications did not. Nine files under `openspec/changes/001-initial-build/`
still specify Convex as the datastore, and a requirement that names a component
which no longer exists cannot be verified against anything. It is not merely
out of date — it is unfalsifiable, which is worse, because it goes on looking
like a requirement while asserting nothing.

The urgent one is `scope-authorization`. That spec says where authorisation
state is persisted, and scope authorisation is the guardrail the entire tool
rests on: nothing is assessed until it is signed. A guardrail spec that cannot
be checked against the implementation is a guardrail nobody is auditing.

## What Changes

- Rewrite the datastore paragraphs in the six live Change 001 specs to describe
  the SQLite store as built: `scope-authorization`, `email-authentication`,
  `vulnerability-correlation`, `ai-provider`, `portability-config`,
  `deployment-packaging`, `ci-cd-pipeline`.
- Leave the *historical* references alone. `003-go-sqlite-engine` and
  `002-susceptibility-scoring-integration` mention Convex to explain what was
  migrated away from and why. Editing those would erase the reasoning behind a
  decision that future readers will otherwise have to reconstruct — and a
  record of a superseded choice is not the same defect as a live requirement
  naming a component that does not exist.
- The `dashboard` spec is already handled by Change 014's MODIFIED and REMOVED
  deltas and is out of scope here.

## Impact

No code changes. This is entirely specification repair.

The check worth adding afterwards is a CI grep asserting that no file under
`openspec/specs/` names a datastore other than the one in use. The reason these
drifted is that nothing was watching, and the same thing will happen at the
next storage decision unless something is.

## Deliberately not done

Auditing every Change 001 spec for other drift. That is a larger exercise and
mixing it in would make this change hard to review. This one does a single
mechanical thing across a known list of files.
