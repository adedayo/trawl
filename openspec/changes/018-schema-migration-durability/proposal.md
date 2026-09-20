# Change: 018-schema-migration-durability

## Status

**Proposed, not started.** Prompted by Change 006 Phase 9, which was the first
change in the project's history to alter an existing table and therefore the
first to discover that no mechanism existed for doing so.

## Why

Trawl's schema is created by a single `CREATE TABLE IF NOT EXISTS` block. That
works exactly once per installation. `IF NOT EXISTS` is a no-op against a
database that already holds the table, so every column added to that block
after the first release reaches new installations only — and every existing
database keeps the old shape and fails on the first write, at run time, on a
user's machine.

Phase 9 needed two new columns on `email_postures` and so added `addColumns`: a
registry of after-the-fact columns, applied idempotently by reading
`PRAGMA table_info` first. That closed the immediate problem and is tested.

It is not a migration mechanism, and the gap between what it is and what it
will be assumed to be is the risk. It handles added columns only. It cannot
retype, rename, drop, backfill, add an index, or change a constraint. It has no
ordering, because column additions commute and nothing else does. And it writes
no version marker, so a database written by a newer build and then opened by an
older one is silently accepted — which is correct for purely additive changes
and wrong the moment a change is not additive.

The next schema change will be made by someone who sees a working mechanism
called `addColumns`, reasonably assumes migrations are handled, and discovers
otherwise only when their change is not a column addition. A convention that
looks like a framework is worse than no framework, because it suppresses the
question.

There is a second, narrower reason. Phase 9 deliberately made pre-existing
`email_postures` rows read as unassessed rather than promoting their booleans,
since preserving the collapse inside the migration would defeat the point of
removing it. That is defensible, and it means every existing installation shows
an empty email view until its first rescan. Nothing currently tells the
operator that, and an empty view is indistinguishable from a clean one.

## What Changes

- **A recorded schema version.** `PRAGMA user_version` is set and checked, so a
  database can state what shape it is in rather than having it inferred.
- **A refusal to open a database from the future.** A store written by a newer
  build is reported and refused rather than silently operated on, which is the
  failure mode most likely to corrupt data quietly.
- **The mechanism's limits are stated where they will be read** — in the
  migration code itself — so the next non-additive change is recognised as one
  before it is written, not after.
- **A test that the migrated shape equals the fresh shape.** An old database
  brought forward and a database created today must be indistinguishable. The
  current tests assert the new columns arrive; they do not assert that nothing
  else differs, and a column added to the `CREATE TABLE` block but forgotten in
  `addedColumns` passes today.
- **Post-migration state is disclosed to the operator.** A domain whose posture
  predates the widening reads as never assessed, because that is true. The view
  says so rather than showing an absence that resembles a clean result.
- **A decision, recorded, on whether a migration framework is warranted** or
  whether the additive registry plus a version marker is the right amount of
  machinery for a single-file embedded store. Either answer is acceptable; an
  unexamined answer is not.

## Explicitly Out of Scope

- **No ORM and no code generation.** The schema is hand-written SQL and stays
  that way.
- **No down-migrations.** Reverting a schema on a live store is a restore-from-
  backup operation, not a routine one, and pretending otherwise invites its use.
- **No backfill of the collapsed email booleans.** Phase 9 decided against
  reconstructing four-state data from two-state data, and this change does not
  reopen that: the fix for an unassessed domain is a rescan, not an inference.
- **No data migration for other capabilities.** This change establishes the
  mechanism and its guards; it does not audit every table for latent drift.

## Impact

- **Engine only.** No API change, no assessment change, no UI change beyond the
  disclosure of pre-widening rows.
- **Touches the open path**, so it affects every start-up. The guard must fail
  loudly and specifically, since a store that will not open is the first thing
  a user sees.
- **Reduces the cost of every later schema change** in 005 and 007–012, all of
  which add tables or columns.
