# Capability: go-sqlite-engine

The `go-sqlite-engine` capability provides zero-dependency local data storage, transaction management, real-time mutation events, and posture regression evaluation for Trawl.

## Requirements

### Requirement: Universal SQLite Storage
The system MUST persist all asset inventory, scan histories, findings, secret exposure records, email posture evaluations, posture regression snapshots, and system configurations in an embedded SQLite database (`trawl.db`).

#### Requirements
- MUST enable Write-Ahead Logging (`PRAGMA journal_mode=WAL;`) on database initialization.
- MUST apply automatic schema migrations on startup without manual SQL intervention.
- MUST provide sub-50ms query latency for dashboard overview stats and asset lists.

### Requirement: Real-time Mutation Event Dispatching
The system MUST emit structured mutation events whenever state changes occur within the SQLite store.

#### Requirements
- MUST dispatch events matching `asset:updated`, `finding:new`, `scan:progress`, and `regression:new`.
- MUST deliver events to Wails frontend listeners in Desktop mode and WebSocket subscribers in Server mode.

### Requirement: Deterministic Posture Regression & Deduplication
The system MUST evaluate deduplication and posture regression criteria in pure Go code prior to writing records.

#### Requirements
- MUST deduplicate identical findings based on asset ID, vulnerability signature, and target port/path.
- MUST confirm posture regressions only after two consecutive observation checks fail.
