# Copilot instructions — Trawl

Project context, guardrails and conventions live in `openspec/project.md`.
Read it. This file covers only how to *work* here.

## Ask, or do not ask

Approval is expensive, so spend it where it buys something. It buys something
when there is a real choice to make, or when the action cannot be undone.

**Do without asking:**

- Reading anything — files, git history, test output, the ledger.
- Building, testing, formatting, linting, typechecking. These are how a claim
  becomes checkable; making them cheap is the point.
- Editing files and staging and committing the result. A commit is a proposal
  in durable form, and `git reset --soft` undoes it.
- Mechanical steps that follow from a decision already taken. If the decision
  was "add a column", do not ask again before writing the migration entry.

**Ask first:**

- Anything that publishes: `git push`, tags, releases, `./scripts/release.sh`.
- Anything that destroys work not yet committed: `rm -rf`, `git reset --hard`,
  `git clean`, rebases, force flags.
- A schema change, a new dependency, or a change to a public contract.
- A genuine fork in the design where either branch is defensible. Say what the
  options are and which one you would take, rather than asking an open
  question. An open question hands the work back.

Do not ask for confirmation of the obvious next step, and do not narrate a
plan and then wait. Do the work and report what happened.

## Reporting

- State what changed and what now passes. Prefer the name of a test over the
  word "verified".
- Say plainly when something is unverified, partial, or was skipped, and why.
  An unstated gap is worse than an admitted one.
- Do not restate a file's contents back after editing it. The diff is there.

## Verification

Run these before claiming work is done:

```sh
go build ./... && go test ./...
gofmt -l pkg cmd jobs
go vet ./...
npm --prefix app run typecheck
npm --prefix app run test:ci
```

`./test.sh` runs the whole thing including packaging validation and the
production build. Use it before proposing a release.

## The ledger

`openspec/` is the project's central argument, not paperwork.

- Keep `tasks.md` current *as work lands*, not afterwards.
- A ticked box means the work landed **or** that a decision not to do it was
  recorded in place. Both are conclusions. Tick withdrawn and superseded items
  with their reasoning; do not leave them as phantom backlog.
- Never tick a box against the ledger. Tick it against the code, naming the
  test or the file.
- Archive with `git mv <change> openspec/changes/archive/`, one change per
  commit, then run `./scripts/check-specs.sh`. An editor with unsaved buffers
  on a moved directory will recreate it and leave a zero-byte ledger. This has
  happened more than once.
- Update `openspec/STATUS.md` in the same commit that changes what it
  describes.

## Style

- Prose in specs, proposals and commit messages is written for a reader who
  will arrive in two years without context. Say why, not just what.
- Commit subjects are sentences in the imperative, describing the change's
  intent rather than its mechanics: "Name what each advisory faults", not
  "update spf_eval.go".
- Comments explain the reasoning that the code cannot. Do not comment what the
  next line plainly says.
- Go: standard library, then third-party, then own-module imports, each in its
  own block. goimports enforces this and the release gate fails on it.
- Angular: standalone components and signals only. `@if`/`@for`/`@switch`,
  never `*ngIf`/`*ngFor`. No `NgModule`.
- Never hand-edit `app/wailsjs/go/models.ts`; regenerate it. It has silently
  drifted for an entire phase before.
