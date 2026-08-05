# Tasks: 005-cloud-continuous-easm

## Phase 1 — Server Subcommand & HTTP/WebSocket Server
- [x] Implement `cmd/trawl/server.go` CLI entrypoint (`trawl server`).
- [x] Build REST API routes matching Wails IPC methods — `/api/v1/{assets,findings,email-postures,jobs}`
- [ ] Build WebSocket server upgrading `/ws` connections and proxying `EventBus` events to connected clients — the route returns 501 and no bus subscriber is attached
- [x] Implement the worker ingest endpoints the job containers post to: `/api/ingest/discovery`, `/api/ingest/scan`, `/api/ingest/secrets`, `/api/ingest/email-posture`
- [x] Implement the job-queue endpoints the workers poll: `GET /api/jobs/pop`, `POST /api/jobs/complete`, plus `POST /api/jobs` to enqueue
- [ ] Parse raw ingest payloads into typed assets and findings — payloads are currently stored verbatim so no evidence is lost, but nothing correlates them yet

## Phase 2 — Continuous Scan Scheduler
- [ ] Implement background Go cron runner for automated daily asset discovery and vulnerability scanning.
- [ ] Wire email authentication posture checks into scheduled cron loop — via vantage (Change 006), not the interim `pkg/scanner/email.go` lookups

## Phase 3 — Remote Desktop Connection Mode
- [ ] Add remote server URL config setting to Wails Desktop app.
- [ ] Build remote API client adapter in Angular UI allowing Desktop app to connect seamlessly to cloud servers.

## Phase 4 — Docker & Cloud Deployment Packaging
- [x] Build multi-stage `Dockerfile` for `trawl server` (`deploy/compose/Dockerfile.server`, CGO-free, non-root).
- [ ] Write Docker Compose cloud setup with optional Litestream replication sidecar.

## Phase 5 — Retire Convex from the deployment stack — COMPLETE
Convex is gone from the entire project. The Compose stack now runs `trawl-server`
(Go + SQLite) as the sole ingest target and job broker, and every dependency in
the tree is OSI-approved.

- [x] Replace the `convex` service in `deploy/compose/docker-compose.yml` and `docker-compose.dev.yml` with the `trawl-server` container
- [x] Rename `CONVEX_INGEST_URL` to `TRAWL_INGEST_URL` across `jobs/*/entrypoint.sh` and both Compose files; the dry-run guard behaviour is unchanged
- [x] Repoint `deploy/compose/nginx.conf` at the Go server (`/api/` and `/ws` proxied to `trawl-server:8080`, single origin, no CORS)
- [x] Remove the FSL-1.1 disclosure from `NOTICE` and `README.md` — done only after the dependency was actually gone
- [x] Replace the brittle `${VAR/ingest\/scan/jobs\/pop}` string substitutions with an explicit `TRAWL_API_BASE`
- [x] Fix the worker auth header, which was built as a quoted string and word-split by the shell, so the bearer token was never actually sent
- [ ] Verify `./test.sh` worker dry-runs still pass under Docker
