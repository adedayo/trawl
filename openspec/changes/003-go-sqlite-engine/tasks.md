# Tasks: 003-go-sqlite-engine

## Phase 1 — Store Interface & SQLite Core
- [ ] Define `pkg/store/store.go` Go interfaces and domain structs (`Asset`, `Scan`, `Finding`, `SecretFinding`, `EmailPosture`, `Regression`, `Config`).
- [ ] Build `pkg/store/sqlite/db.go` initialization with WAL mode pragmas and auto-migrations.
- [ ] Implement `pkg/store/sqlite/assets.go` (CRUD, status transitions, review queue filters).
- [ ] Implement `pkg/store/sqlite/findings.go` (ingestion, deduplication logic, severity calculations).
- [ ] Implement `pkg/store/sqlite/secret_findings.go` (redacted ref hashing, active verification tracking).
- [ ] Implement `pkg/store/sqlite/posture.go` (snapshot storage and 2-consecutive confirmation regression logic).
- [ ] Write Vitest/Go unit tests in `pkg/store/sqlite/db_test.go` verifying transaction safety and schema constraints.

## Phase 2 — Real-time Event Bus
- [ ] Implement thread-safe event bus in `pkg/event/bus.go`.
- [ ] Wire Wails IPC event emitter and WebSocket broadcaster adapters.

## Phase 3 — Convex Cleanup
- [ ] Remove Convex npm packages and `convex/` directory once Go store migration passes all unit tests.
