# Tasks: 005-cloud-continuous-easm

## Phase 1 — Server Subcommand & HTTP/WebSocket Server
- [x] Implement `cmd/trawl/server.go` CLI entrypoint (`trawl server`).
- [x] Build REST API routes matching Wails IPC methods — `/api/v1/{assets,findings,email-postures,jobs}`
- [x] ~~Build WebSocket server upgrading `/ws` connections~~ — **superseded by Server-Sent Events at `GET /api/v1/events`.** The traffic is one-way, SSE survives proxies that mangle upgrade headers, and browsers reconnect unaided; a bidirectional protocol would have been surface bought for a capability nothing needs. The bus subscriber is `broadcaster` in `cmd/trawl/stream.go`, which subscribes to an allowlist of event types rather than to everything, so a new event type does not widen what the dashboard discloses. `/ws` is retained and answers `410 Gone` naming its replacement, so an older dashboard build fails with an explanation instead of a bare connection error that reads as the server being down.
- [x] Implement the worker ingest endpoints the job containers post to: `/api/ingest/discovery`, `/api/ingest/scan`, `/api/ingest/secrets`, `/api/ingest/email-posture`
- [x] Implement the job-queue endpoints the workers poll: `GET /api/jobs/pop`, `POST /api/jobs/complete`, plus `POST /api/jobs` to enqueue
- [x] Parse raw ingest payloads into typed assets and findings *(`pkg/core/ingest.go`; naabu, httpx and nuclei output correlated into assets, findings and posture observations)*
  - The verbatim write still happens first and unconditionally — correlation is interpretation, and the raw payload is the evidence it derives from, so a parser fixed later can be re-run against what actually arrived.
  - A correlation failure is not an ingest failure: the response reports `correlated: false` with the reason rather than returning an error the worker would answer by retrying a payload that is already stored.
  - Scope is re-checked at correlation, and refusals are **reported** in the summary rather than silently dropped — a worker returning out-of-scope results means something upstream is wrong.
  - Open ports are recorded as a posture attribute, not as a finding per port. An open port is not a defect; a port that was not open last week is, and that is the regression path's question to answer.
  - `Priority` is deliberately left unset. It is a deterministic function of KEV and EPSS enrichment, and a transport-side guess would be a second, contradictory source for a number the model requires to be reproducible.

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
