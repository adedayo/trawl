# Tasks: 005-cloud-continuous-easm

## Phase 1 — Server Subcommand & HTTP/WebSocket Server
- [x] Implement `cmd/trawl/server.go` CLI entrypoint (`trawl server`).
- [ ] Build REST API routes matching Wails IPC methods — only `/api/v1/assets` exists so far
- [ ] Build WebSocket server upgrading `/ws` connections and proxying `EventBus` events to connected clients — the route exists but no bus subscriber is attached
- [ ] Implement the worker ingest endpoints the job containers post to: `/api/ingest/discovery`, `/api/ingest/scan`, `/api/ingest/secrets`
- [ ] Implement the job-queue endpoints the repo-scan worker polls: `GET /api/jobs/pop`, `POST /api/jobs/complete`

## Phase 2 — Continuous Scan Scheduler
- [ ] Implement background Go cron runner for automated daily asset discovery and vulnerability scanning.
- [ ] Wire email authentication posture checks into scheduled cron loop — via vantage (Change 006), not the interim `pkg/scanner/email.go` lookups

## Phase 3 — Remote Desktop Connection Mode
- [ ] Add remote server URL config setting to Wails Desktop app.
- [ ] Build remote API client adapter in Angular UI allowing Desktop app to connect seamlessly to cloud servers.

## Phase 4 — Docker & Cloud Deployment Packaging
- [ ] Build multi-stage `Dockerfile` for `trawl server`.
- [ ] Write Docker Compose cloud setup with optional Litestream replication sidecar.

## Phase 5 — Retire Convex from the deployment stack
The engine and desktop app no longer depend on Convex; the Compose stack still does. This is the last remaining Convex dependency and it blocks an honest licensing story, since `get-convex/convex-backend` is FSL-1.1 rather than OSI-approved.

- [ ] Replace the `convex` service in `deploy/compose/docker-compose.yml` and `docker-compose.dev.yml` with the `trawl server` container
- [ ] Rename `CONVEX_INGEST_URL` to `TRAWL_INGEST_URL` across `jobs/*/entrypoint.sh`, `ofelia.conf` and both Compose files; keep the dry-run guard behaviour identical
- [ ] Repoint `deploy/compose/nginx.conf` and `Dockerfile.dashboard` at the Go server
- [ ] Remove the FSL-1.1 disclosure from `NOTICE` and the corresponding `README.md` section once the dependency is actually gone — not before
- [ ] Verify `./test.sh` worker dry-runs still pass after the rename
