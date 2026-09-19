# Tasks: 003-go-sqlite-engine

## Phase 1 — Store Interface & SQLite Core
- [x] Define `pkg/store/store.go` Go interfaces and domain structs (`Asset`, `Finding`, `SecretFinding`, `EmailPosture`, `Regression`) — `Scan` and `Config` structs not yet defined; settings are handled by `pkg/store/sqlite/settings.go`
- [x] Build `pkg/store/sqlite/db.go` initialization with WAL mode pragmas and auto-migrations.
- [x] Implement `pkg/store/sqlite/assets.go` (CRUD, status transitions, review queue filters).
- [x] Implement `pkg/store/sqlite/findings.go` (ingestion, deduplication logic, severity calculations).
- [x] Implement `pkg/store/sqlite/secret_findings.go` (redacted ref hashing, active verification tracking).
- [x] Implement `pkg/store/sqlite/posture.go` (snapshot storage and 2-consecutive confirmation regression logic).
- [x] Write Go unit tests in `pkg/store/sqlite/db_test.go` verifying transaction safety and schema constraints.
- [x] **Decision: `Scan` and `Config` domain structs are not needed.** A scan
      is not a stored entity — it is a run that produces assets, findings and
      posture observations, each already modelled, and its progress is carried
      on the event bus rather than persisted. Configuration is a small set of
      keyed values, handled by `pkg/store/sqlite/settings.go`; giving it a
      struct in the store interface would push every future setting through a
      schema migration for no gain. Both are recorded here rather than left
      unticked, because an open box implies work that nobody intends to do.

## Phase 2 — Real-time Event Bus
- [x] Implement thread-safe event bus in `pkg/event/bus.go` (`MemoryBus`, covered by `bus_test.go`).
- [x] Wire the Wails IPC event emitter adapter (`app.go`).
- [x] **Superseded: the transport is Server-Sent Events, not WebSocket.** The
      bus does have a subscriber attached — `newBroadcaster(bus)` in
      `cmd/trawl/server.go`, streaming at `GET /api/v1/events`. `/ws` is
      retained only to return 410 naming its replacement, so an older client
      fails with an explanation rather than a connection refused. SSE was
      chosen because the traffic is one-way server-to-client: a bidirectional
      protocol would have been surface bought for a direction nothing uses.
      This item claimed the server had no subscriber, which had not been true
      for some time.

## Phase 3 — Convex Cleanup — COMPLETE
- [x] Remove Convex npm packages and `convex/` directory once Go store migration passes all unit tests.
- [x] Remove the stale Convex references from `openspec/project.md` and the vestigial `convex/**/*.ts` include from the root `tsconfig.json`
- [x] Remove the `convex/*` path mapping from `app/tsconfig.json`, the `convex/_generated` entry from `.gitignore`, and the `CONVEX_URL` variable from `.env.example` and both `setup.sh` copies
- [x] Retire the Convex container from the Compose stack (see 005 Phase 5). No reference to Convex remains anywhere outside these historical records.
