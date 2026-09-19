# Tasks: 003-go-sqlite-engine

## Phase 1 — Store Interface & SQLite Core
- [x] Define `pkg/store/store.go` Go interfaces and domain structs (`Asset`, `Finding`, `SecretFinding`, `EmailPosture`, `Regression`) — `Scan` and `Config` structs not yet defined; settings are handled by `pkg/store/sqlite/settings.go`
- [x] Build `pkg/store/sqlite/db.go` initialization with WAL mode pragmas and auto-migrations.
- [x] Implement `pkg/store/sqlite/assets.go` (CRUD, status transitions, review queue filters).
- [x] Implement `pkg/store/sqlite/findings.go` (ingestion, deduplication logic, severity calculations).
- [x] Implement `pkg/store/sqlite/secret_findings.go` (redacted ref hashing, active verification tracking).
- [x] Implement `pkg/store/sqlite/posture.go` (snapshot storage and 2-consecutive confirmation regression logic).
- [x] Write Go unit tests in `pkg/store/sqlite/db_test.go` verifying transaction safety and schema constraints.
- [ ] Define `Scan` and `Config` domain structs, or record the decision that they are not needed

## Phase 2 — Real-time Event Bus
- [x] Implement thread-safe event bus in `pkg/event/bus.go` (`MemoryBus`, covered by `bus_test.go`).
- [x] Wire the Wails IPC event emitter adapter (`app.go`).
- [ ] Wire the WebSocket broadcaster adapter in `cmd/trawl/server.go` — the server currently has no subscriber attached to the bus

## Phase 3 — Convex Cleanup — COMPLETE
- [x] Remove Convex npm packages and `convex/` directory once Go store migration passes all unit tests.
- [x] Remove the stale Convex references from `openspec/project.md` and the vestigial `convex/**/*.ts` include from the root `tsconfig.json`
- [x] Remove the `convex/*` path mapping from `app/tsconfig.json`, the `convex/_generated` entry from `.gitignore`, and the `CONVEX_URL` variable from `.env.example` and both `setup.sh` copies
- [x] Retire the Convex container from the Compose stack (see 005 Phase 5). No reference to Convex remains anywhere outside these historical records.
