# Tasks: 015-spec-datastore-hygiene

## Phase 1 — The guardrail spec
- [x] `specs/scope-authorization/spec.md`: restated persistence against the
      SQLite store — the `scope_settings` JSON document, the fail-closed read
      path, the requirement for at least one seed domain, and consented
      third-party endpoints held apart from the target scope
- [x] Confirmed each restated requirement against `pkg/core/core.go` and
      `pkg/store/sqlite` rather than against memory

Findings from that verification:

- **The old spec required a "rules of engagement acknowledgment version"
  field. No such field exists.** Rather than restate a requirement the code
  does not meet, it was dropped. A requirement sitting in a spec as though it
  were already true stops anyone from noticing the gap. If the acknowledgment
  version is wanted, it should be raised as work.
- **The record is not tamper-evident, and the old spec implied it was.** It is
  a plain JSON document in a local SQLite file; anything with write access can
  alter it, including the signer's name. An explicit requirement now says so,
  because the field is called a signature and a signature implies a property
  this record does not have.

## Phase 2 — The remaining live specs
- [x] `specs/email-authentication/spec.md` — checks run in-process against the
      resolver the egress policy permits, not as a scheduled cloud action
- [x] `specs/vulnerability-correlation/spec.md`
- [x] `specs/ai-provider/spec.md` — reachability restated around the engine's
      own process; the container `localhost` trap named explicitly
- [x] `specs/portability-config/spec.md`
- [x] `specs/deployment-packaging/spec.md` — the largest rewrite: no separate
      datastore service, `image:` plus `build:` on every service, and the
      hosting-choice requirement replaced by *the same engine runs desktop and
      headless against the same database and the same authorization record*
- [x] `specs/ci-cd-pipeline/spec.md` — gates restated as Go tests under `-race`
      plus `go vet`; added the requirement that retracted dependencies fail CI
      and are exempt from the update cooldown
- [x] `specs/dashboard/spec.md` — the loading/empty/error requirement was still
      phrased around a live-query subscription. Rewritten, and strengthened to
      say why the three states may not be collapsed.

## Phase 3 — Stop it recurring
- [x] CI check: `scripts/check-spec-datastore.sh`, wired into the `go` job.
      Historical references under `changes/` are allowlisted individually.
- [x] **Required check**: the guard fails on a reintroduced reference —
      verified with a probe file.

Finding from that verification:

- **The first version of the guard passed while looking in the wrong
  directory.** It `cd`-ed two levels up instead of one, so `grep` matched
  nothing and it reported success — against a deliberately planted violation
  *and* against a real one already present in `dashboard/spec.md`. A guard that
  passes because it looked in the wrong place is precisely the failure it
  exists to prevent, and it would have been indistinguishable from a working
  guard for as long as nobody tested it. It now exits non-zero if `openspec/`
  is not where it expects.

## Also added while here
- [x] Retracted-dependency check in CI. The `modernc.org/libc` retraction found
      earlier had been sitting in the tree unnoticed, and nothing would have
      caught the next one.

## Out of scope
- `003-go-sqlite-engine` and `002-susceptibility-scoring-integration`: the
  references there are historical and explain the migration.

## Exit Criteria — met

No live specification names a datastore the tool does not use, the scope
authorisation record's persistence is described accurately enough to audit
against the code, the record's *limits* are stated rather than implied away,
and a reintroduced reference fails CI.
