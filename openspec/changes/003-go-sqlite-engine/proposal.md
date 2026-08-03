# Change: 003-go-sqlite-engine

## Why

Trawl currently relies on a self-hosted Convex backend (Node.js/Rust runtime in Docker) for data persistence, schema validation, and real-time state. While functional for web server setups, Convex introduces external server dependencies that prevent Trawl from running natively as a zero-dependency desktop app or a lightweight single-binary CLI/server.

Moving backend storage to a native Go store backed by an embedded SQLite database (`trawl.db` in WAL mode) eliminates external runtime dependencies, reduces memory footprint, delivers sub-50ms application startup, and provides a unified, portable storage layer across Desktop, Docker, and Cloud server deployments.

## What Changes

This change replaces Convex with a native Go storage implementation (`pkg/store/sqlite`):

- `store-interface` — Go `store.Store` interface defining type-safe methods for assets, scans, findings, secret findings, email posture, posture snapshots, regressions, and configuration.
- `sqlite-store` — High-performance SQLite store provider (`trawl.db`) operating in Write-Ahead Logging (WAL) mode with auto-migrations.
- `event-bus` — Thread-safe Go event dispatcher (`pkg/event`) emitting real-time mutation events (`asset:updated`, `finding:new`, `scan:progress`) to Wails IPC in Desktop mode and WebSockets in Server mode.
- `native-dedup` — Pure Go deduplication and regression confirmation logic replacing Convex JavaScript actions.

## Impact

- **Storage**: All application data resides in a single, portable SQLite database file (`trawl.db`).
- **Dependencies**: Removes Convex Node.js/Rust container requirements entirely.
- **Portability**: Enables Trawl to run as a single compiled Go binary on macOS, Windows, Linux, and Docker containers.

## Explicitly Out of Scope

- Remote database engines (e.g. PostgreSQL, MySQL) — SQLite is the single universal storage engine across all deployment models.
- External server dependencies for local execution.
