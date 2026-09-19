# Change: 007-deployment-parity

## Why

Trawl ships in two shapes. The desktop build is a Wails binary embedding an Angular frontend and driving Go directly over IPC. The container build is `cmd/trawl server` behind nginx, serving the same frontend to a browser.

Until this change they were not the same product.

Everything the application could do lived in `app.go` — the Wails bindings. Scope reading, scan orchestration, assessment triggering and the whole of the vantage integration were methods on the `App` struct, reachable only through the desktop transport. `cmd/trawl/server.go` held a `*sqlite.SQLiteStore` and served four read endpoints over it. So the container deployment could list assets, findings, email postures and jobs, and could do nothing else: it could not run a scan, could not read or write the authorisation record, could not assess a domain, and had no access to the measured-state signals, coverage or control postures that Change 006 spent its entire scope building.

The dashboard container was the sharpest expression of the problem. It built the Angular bundle and served it from nginx, and nginx proxied `/api/` to the server — but the frontend spoke only `window.go.main.App`, which does not exist in a browser. Every call silently short-circuited on the `if (window.go && ...)` guard that wrapped it. The dashboard rendered its chrome and no data, and did so without erroring, because a missing binding was indistinguishable from an empty result.

The failure mode this invites is worse than a missing feature. An operator running the container reads the README, sees a capability described, finds the tab empty, and concludes their estate is clean.

## What Changes

- **New capability: `deployment-parity`.** An application layer, `pkg/core`, holds every operation Trawl performs. Both transports become adapters over it: `app.go` forwards Wails IPC calls, `cmd/trawl/server.go` forwards HTTP requests. Neither holds behaviour, so a capability cannot exist in one deployment and not the other.

- **The HTTP API gains full parity.** Assessment reads and runs, scope read and write, settings, secret findings, regressions and scan triggering are all served. Every Wails binding has an HTTP equivalent, and both call the same function.

- **A live event stream over Server-Sent Events** at `GET /api/v1/events`, replacing a `/ws` route that returned 501. The bus that feeds the desktop webview now also feeds browser clients, through an explicit allowlist of event types.

- **A transport seam in the frontend.** `TrawlTransport` is implemented by `WailsTransport` and `HttpTransport`, selected at runtime by detecting the Wails runtime. The same bundle drives both deployments; no component knows which one it is in.

- **The signal registry moves to `config/signals` as a Go package**, embedded rather than read from disk, so every entrypoint — desktop, server, and any future worker — carries the same mapping inside the binary.

- **Scope enforcement moves into the application layer** and fails closed. An absent, unparseable or unsigned authorisation record authorises nothing, and the assessment is refused at the transport rather than run against a portfolio nobody consented to. The HTTP transport cannot supply its own scope, only read the stored one.

- **Autoscaled deployment is supported explicitly.** The server detects a managed platform, honours the injected `PORT`, drains work on `SIGTERM`, and — critically — runs scans inline rather than detached. Under default CPU allocation an instance is throttled once it has written its response, so a scan left on a goroutine stalls indefinitely or dies at scale-down, having already returned `202 Accepted`. Inline execution makes the response mean what it says. A single-container image serving both the API and the dashboard removes the second service and the proxy configuration with it.

## Impact

- **Affected code:** new `pkg/core` and `config/signals`; `app.go` reduced to a forwarding adapter; `cmd/trawl/server.go` rebuilt over the core with new routes and an SSE broadcaster; new `app/src/app/transport/`; `wails-ipc.service.ts` rewritten to delegate.

- **A refusal is now a result, not an error.** `AssessDomain` returns a `refused` outcome with a stated reason rather than failing. A caller that treated an error as "nothing to show" would erase the distinction between "we were not allowed to look" and "we looked and found nothing" — the same distinction the four-state coverage model exists to preserve.

- **`/ws` now returns 410 Gone** naming its replacement, rather than 501. A dashboard build older than this change fails with a message identifying the new endpoint instead of an error resembling a server outage.

- **nginx needs `proxy_buffering off` on the event stream.** Buffering is the default for proxied responses, and a buffered stream delivers an entire assessment's events at once when it finishes, which is precisely when they stop being useful.

## Non-Goals

- **No authentication model beyond the existing bearer token.** Mutating endpoints are behind `authed`; the read API is not. A deployment exposed beyond loopback needs an authenticating proxy in front of it, and that remains true after this change.

- **No second frontend.** The parity is achieved by making one bundle work in both places, not by maintaining a browser-specific dashboard. Two frontends would drift for exactly the reason two backends did.

- **Phase 9 of Change 006 is not closed here.** The legacy hand-rolled email scanner still runs alongside the vantage assessment. Removing it requires the gap analysis that change specifies, and doing it as a side effect of a deployment change would hide a behavioural regression inside an architectural one.
