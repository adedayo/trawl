# Tasks: 007-deployment-parity

## Phase 1 — Application layer
- [x] `pkg/core`: the single home for every operation Trawl performs
- [x] Scope reading and writing, failing closed on absent, malformed or unsigned records
- [x] Scan orchestration that records every capability's failure by name rather than collapsing to one error
- [x] Assessment read and run paths
- [x] Tests: scope fail-closed across every way the record can be wrong

## Phase 2 — Embedded signal registry
- [x] `config/signals` becomes a Go package exposing the registry as bytes
- [x] Every entrypoint carries it in the binary; nothing reads it from disk at run time
- [x] Test: a fresh deployment seeds the registry and resolves identifiers

## Phase 3 — Wails transport
- [x] `app.go` reduced to forwarding; no decisions remain in it
- [x] Assessment bindings exposed: list, read, run
- [x] Registry seeded on startup

## Phase 4 — HTTP transport
- [x] Server rebuilt over `pkg/core` rather than the store
- [x] Assessment endpoints: `GET /api/v1/assessments`, `GET|POST /api/v1/assessments/{domain}`
- [x] Scope endpoints: `GET|PUT /api/v1/scope`
- [x] Settings endpoints: `GET|PUT /api/v1/settings/{key}`
- [x] Scan trigger: `POST /api/v1/scans`, detached from the request context
- [x] Secret findings and regressions exposed
- [x] Mutating endpoints authed; read endpoints unchanged
- [ ] **Required check**: an automated assertion that every `App` method has an HTTP counterpart, so a capability added to one transport cannot ship without the other

## Phase 5 — Live events
- [x] SSE broadcaster over the event bus, with an allowlist of streamed types
- [x] Slow clients skipped rather than waited for, so a dashboard cannot stall an assessment
- [x] Keepalive comments, so a quiet estate does not have its stream reaped
- [x] `/ws` returns 410 naming the replacement
- [x] nginx configured with buffering disabled on the stream
- [ ] Test: an event published on the bus reaches a connected SSE client

## Phase 6 — Frontend transport seam
- [x] `TrawlTransport` interface covering every backend operation
- [x] `WailsTransport`, probing each binding and degrading rather than crashing
- [x] `HttpTransport` over REST and `EventSource`
- [x] Runtime detection; one bundle, both deployments
- [x] `WailsIpcService` delegates and holds no transport knowledge
- [ ] Test: the service drives a fake transport, with no `window.go` present

## Phase 7 — Autoscaled deployment
- [x] Managed-platform detection via `K_SERVICE`, with the assumptions logged at startup
- [x] Injected `PORT` takes precedence over `TRAWL_LISTEN_ADDR`
- [x] Graceful shutdown on `SIGTERM`, draining connections and work in flight within the platform's grace period
- [x] Scans run inline where CPU is not guaranteed after a response; `TRAWL_SCAN_MODE` overrides
- [x] Single-container image serving the API and the embedded dashboard from one origin
- [x] Tests: scan-mode defaults per platform, explicit override, port precedence
- [x] Operator documentation stating the SQLite/scaling constraint rather than leaving it to be discovered
- [x] Storage backend selected by DSN scheme through a registry in `pkg/store`, so no entrypoint names a concrete store
- [x] SQLite registers itself on import; the composition roots call `store.Open` and are backend-agnostic
- [x] An unrecognised scheme fails at startup rather than falling back to a local file
- [ ] A networked `store.Store` implementation, which is what horizontal scaling actually requires
- [ ] A bus implementation that fans out across instances, so "live" is not per-instance

## Phase 8 — Outstanding
- [ ] Worker containers reach the assessment path; today they post raw payloads that nothing parses
- [ ] Authorisation model for the read API when exposed beyond loopback
- [ ] Operator documentation covering all three distributions and where they differ (they should not, except where the platform forces it)

## Exit Criteria

Every capability Trawl offers is reachable from all three distributions, because all three call the same function. A capability added to the application layer is exposed by every transport or by none. The container dashboard renders the same data as the desktop application, live, from the same database. An authorisation record that is absent, malformed or unsigned authorises nothing in any shape. A deployment whose platform cannot support a behaviour adopts the safe alternative automatically and says so, rather than appearing to work.
