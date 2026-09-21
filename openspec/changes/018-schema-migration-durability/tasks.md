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

- [x] `PRAGMA user_version` set to `schemaVersion` (1), inside the same
      transaction as the shape change — `pkg/store/sqlite/db.go`. SQLite's DDL
      is transactional, which is what makes this possible; on an engine where
      it is not, the version would have to be written before the change and
      repaired after. `TestMigrationRecordsTheSchemaVersion`
- [x] A database whose recorded version exceeds the expected one is refused,
      naming both — `ErrNewerSchema`, a distinct type so the caller can tell it
      apart from an ordinary open failure. `TestADatabaseFromTheFutureIsRefused`
      and `TestARefusedDatabaseIsNotWrittenTo`, the latter asserting the
      refusal happens before anything is written, so refusing is not a side
      effect of having already migrated
- [x] The refusal reads as an operator message rather than a failed query:
      it names the store format found and expected, says to upgrade Trawl, and
      says what continuing would cost. Asserted on the message text, because a
      store that will not open is the first thing a user sees

## Phase 2 — Guard the mechanism

- [x] `TestAMigratedDatabaseHasTheSameShapeAsAFreshOne` compares a database
      brought forward from `schemaAsShipped` against one created today, table
      by table. The frozen literal is what makes it work: a test that derived
      the old shape from the current one could only confirm that the migration
      does what the migration does. **Verified by temporarily adding a column
      to the CREATE TABLE block without an `addedColumns` entry — the test
      failed, naming the column.** That omission passed before
- [x] The mechanism's limits stated in `addColumns`'s doc comment: added
      columns only; no retype, rename, drop, backfill, index or constraint
      change; no ordering, because column additions commute and nothing else
      does. Written as a question to the next reader rather than a note,
      because a convention that resembles a framework suppresses the question
- [x] **Decision: the additive registry plus the version marker is the right
      amount of machinery** for a single-file embedded store shipping as one
      binary. A framework buys ordering and down-migrations; ordering is
      unnecessary while every change commutes, and down-migrations on a live
      store are a restore-from-backup operation that should not be made to look
      routine. Recorded in `db.go` next to the mechanism, with the instruction
      to revisit when the first non-additive change arrives — and to revisit
      rather than work around it

## Phase 3 — Disclose what did not carry forward

- [x] `EmailPosture.PredatesAssessment` identifies a posture written before the
      widening, and the view says *assessed before this version of Trawl —
      rescan to see its posture* instead of *0/7 controls assessed*. The
      remedy is a rescan, not a change to the domain, and the badge says so —
      `email-posture.html`, `email-posture.spec.ts` "records that predate the
      four-state posture"
- [x] The first predicate written here was wrong and a test caught it: *no
      control assessed* also describes a domain whose every check failed in a
      resolver outage, and telling that operator their data predates an upgrade
      would send them to rescan a domain just scanned. The signature is an
      unset state, not an unassessed one — the current engine always records
      one of the four, including that a check could not be completed.
      `TestAPostureWithAnyConclusionIsNotLegacy`
- [x] Not added to `app/wailsjs/go/models.ts`: like `assessedControls` before
      it, the flag is produced by `MarshalJSON` rather than by a struct field,
      so the generator does not see it. Consistent with 017; noted here because
      the absence otherwise looks like the drift that file has suffered before
- [ ] Release note for the first release carrying this, recording that
      installations upgraded across 006 Phase 9 show *rescan needed* against
      every domain until their first scan after upgrade

## Exit Criteria

Any released build can open a store written by any earlier released build and
bring it to the current shape, with existing rows intact. A store written by a
later build is refused with a message naming both versions. A schema change
that the mechanism cannot apply is recognised as such before it is written, and
a schema change that is forgotten fails CI rather than a user's database.
