# Tasks: 005-cloud-continuous-easm

## Phase 1 — Server Subcommand & HTTP/WebSocket Server
- [ ] Implement `cmd/trawl/server.go` CLI entrypoint.
- [ ] Build REST API routes matching Wails IPC methods.
- [ ] Build WebSocket server upgrading `/ws` connections and proxying `EventBus` events to connected clients.

## Phase 2 — Continuous Scan Scheduler
- [ ] Implement background Go cron runner for automated daily asset discovery and vulnerability scanning.
- [ ] Wire email authentication posture checks into scheduled cron loop.

## Phase 3 — Remote Desktop Connection Mode
- [ ] Add remote server URL config setting to Wails Desktop app.
- [ ] Build remote API client adapter in Angular UI allowing Desktop app to connect seamlessly to cloud servers.

## Phase 4 — Docker & Cloud Deployment Packaging
- [ ] Build multi-stage `Dockerfile` for `trawl server`.
- [ ] Write Docker Compose cloud setup with optional Litestream replication sidecar.
