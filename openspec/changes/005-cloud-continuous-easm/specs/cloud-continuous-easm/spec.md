# Capability: cloud-continuous-easm

The `cloud-continuous-easm` capability enables Trawl to run as a 24/7 continuous attack surface monitoring server on cloud infrastructure using the universal SQLite database (`trawl.db`).

## Requirements

### Requirement: Headless Server Mode
The system MUST provide a `trawl server` subcommand to run as a headless daemon on cloud infrastructure.

#### Requirements
- MUST serve HTTP/REST API endpoints for assets, scans, findings, secret exposure, and posture regressions.
- MUST stream real-time mutation events over WebSocket connections.
- MUST execute scheduled background discovery and vulnerability scans.

### Requirement: Remote Desktop Client Connection
The Wails Desktop application MUST support connecting to remote `trawl server` deployments.

#### Requirements
- MUST allow configuration of remote server host URL and API authorization tokens.
- MUST maintain UI state parity between local desktop engine and remote cloud engine modes.

### Requirement: Universal SQLite Storage Parity
The cloud server MUST use the exact same SQLite schema (`trawl.db`) and `pkg/store/sqlite` implementation as the Desktop application.
