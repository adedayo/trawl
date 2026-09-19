# Capability: cloud-continuous-easm

The `cloud-continuous-easm` capability enables Trawl to run as a 24/7 continuous attack surface monitoring server on cloud infrastructure using the universal SQLite database (`trawl.db`).

## ADDED Requirements

### Requirement: Headless Server Mode
The system MUST provide a `trawl server` subcommand to run as a headless daemon on cloud infrastructure, serving HTTP/REST endpoints, streaming mutation events over WebSocket, and executing scheduled background scans.

#### Scenario: REST surface is served
- **GIVEN** a running `trawl server`
- **WHEN** a client requests assets, scans, findings, secret exposure or posture regressions
- **THEN** the server responds over HTTP/REST without a desktop session being present

#### Scenario: Mutations stream in real time
- **GIVEN** a client holding an open WebSocket connection
- **WHEN** any tracked entity is created, updated or removed
- **THEN** the corresponding event is broadcast over that connection

#### Scenario: Scheduled scans run unattended
- **GIVEN** a server configured with a discovery and vulnerability schedule
- **WHEN** a scheduled interval elapses with no operator connected
- **THEN** the scan executes and its results are persisted

### Requirement: Remote Desktop Client Connection
The Wails desktop application MUST support connecting to a remote `trawl server` deployment, with configurable host URL and API authorisation token, and MUST present identical UI state whether backed by the local engine or a remote one.

#### Scenario: Remote server configured
- **GIVEN** an operator with a reachable `trawl server` URL and a valid API token
- **WHEN** they enter both in the desktop application's configuration
- **THEN** the application connects to that server and operates against its data

#### Scenario: Transport is not visible in the interface
- **GIVEN** the same underlying data
- **WHEN** it is viewed through the local engine and through a remote server
- **THEN** the rendered state is identical, because the transport is not a UI concern

### Requirement: Universal SQLite Storage Parity
The cloud server MUST use the same SQLite schema (`trawl.db`) and the same `pkg/store/sqlite` implementation as the desktop application, with no server-only divergence.

#### Scenario: A database moves between deployments
- **GIVEN** a `trawl.db` written by the desktop application
- **WHEN** it is opened by `trawl server`
- **THEN** it is read without migration or conversion, and the reverse also holds

#### Scenario: No second store implementation
- **GIVEN** the server binary
- **WHEN** its storage layer is inspected
- **THEN** it uses `pkg/store/sqlite` directly, with no parallel server-specific implementation
