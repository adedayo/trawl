# Tasks: 018-schema-migration-durability

**No dependency.** Phase 0 landed inside Change 006 Phase 9, because that phase
needed it. The rest is hardening the mechanism it introduced before a second
change relies on it.

## Phase 0 — What 006 Phase 9 already landed

- [x] `addedColumns` registry and idempotent `addColumns` applied at the end of
      `migrate()` — `pkg/store/sqlite/db.go`. Current columns are read via
      `PRAGMA table_info` first, because SQLite has no
      `ADD COLUMN IF NOT EXISTS` and distinguishing "column exists" from a real
      failure by error text means depending on wording SQLite is free to change
- [x] `TestAnOlderDatabaseGainsTheNewColumns` and
      `TestALegacyRowReadsAsUnassessed` — `pkg/store/sqlite/email_test.go`
- [x] Pre-widening rows read as unassessed rather than promoted from their
      booleans. Preserving the collapse inside the migration would have
      defeated the point of removing it

## Phase 1 — Record the shape

- [ ] Set `PRAGMA user_version` to a constant the build declares, as part of
      the same operation that changes the shape, so a partial migration cannot
      leave a version claiming a shape the database does not have
- [ ] Refuse to open a database whose recorded version exceeds the expected
      one, naming both versions. A newer store opened by an older build is the
      failure most likely to corrupt data quietly, because every individual
      query still succeeds
- [ ] The refusal is surfaced as an operator-legible message, not a failed
      query. A store that will not open is the first thing a user sees

## Phase 2 — Guard the mechanism

- [ ] A test asserting a database brought forward from an earlier shape has the
      same schema as a fresh one. The existing tests assert the new columns
      arrive; they do not assert nothing else differs, so a column added to the
      `CREATE TABLE` block and forgotten in `addedColumns` passes today — which
      is precisely the omission the mechanism exists to prevent
- [ ] State the mechanism's limits in `db.go`: added columns only, no retype,
      rename, drop, backfill, index or constraint change; no ordering, because
      column additions commute and nothing else does. A convention that looks
      like a framework suppresses the question it should provoke
- [ ] Decide, and record here, whether a migration framework is warranted or
      whether the additive registry plus a version marker is the right amount
      of machinery for a single-file embedded store. Either answer is fine; an
      unexamined one is not

## Phase 3 — Disclose what did not carry forward

- [ ] An upgraded installation states that its email postures predate the
      current assessment and require a rescan, rather than showing an empty
      view. An empty view is indistinguishable from a clean one, and a CISO
      reading it concludes there is nothing to fix
- [ ] Release note for the first release carrying 006 Phase 9, recording that
      existing installations show no email posture until the first scan after
      upgrade

## Exit Criteria

Any released build can open a store written by any earlier released build and
bring it to the current shape, with existing rows intact. A store written by a
later build is refused with a message naming both versions. A schema change
that the mechanism cannot apply is recognised as such before it is written, and
a schema change that is forgotten fails CI rather than a user's database.
