# Change: 005-cloud-continuous-easm

## Why

While Desktop mode provides an ideal experience for local analysis and non-technical users, enterprise security teams and organizations require 24/7 continuous attack surface monitoring running on cloud infrastructure. Providing a headless server mode (`trawl server`) allows Trawl to run continuously in cloud environments (Docker, Kubernetes, AWS, GCP), scheduling background asset discovery and vulnerability scans against the same universal SQLite database (`trawl.db`).

## What Changes

This change adds headless server and cloud continuous EASM capabilities:

- `server-cmd` — `trawl server` Go subcommand initializing HTTP/REST API endpoints, WebSocket event streaming, and background cron schedules.
- `universal-sqlite-replication` — Optional Litestream integration for real-time streaming replication of `trawl.db` to cloud object storage (S3/GCS) with sub-second RPO.
- `websocket-broadcaster` — WebSocket server streaming real-time scan progress, asset additions, and posture regression alerts to web clients and remote Wails Desktop apps.
- `remote-desktop-connect` — Allows Wails Desktop App users to connect their local UI to a remote Trawl cloud server instance.

## Impact

- **Operational Flexibility**: The exact same Go binary runs as a local Wails Desktop app or a 24/7 cloud continuous EASM server.
- **Cost & Simplicity**: Uses `trawl.db` (SQLite in WAL mode) for zero-database-infra cloud deployments.

## Explicitly Out of Scope

- Multi-tenant user permission branching (single org / team per deployment).
