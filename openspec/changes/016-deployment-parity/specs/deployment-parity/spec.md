# Capability: deployment-parity

The `deployment-parity` capability guarantees that Trawl is one product in two
shapes. Every operation lives in an application layer; the desktop and container
builds are transports over it. A capability cannot exist in one deployment and
not the other.

## ADDED Requirements

### Requirement: Behaviour Lives In The Application Layer
Every operation the system performs MUST be implemented in the application layer (`pkg/core`). Transport adapters MUST hold no behaviour of their own and MUST only forward to it.

#### Scenario: Desktop transport forwards
- **GIVEN** a Wails IPC binding in `app.go`
- **WHEN** it is invoked
- **THEN** it forwards to an application-layer function and contains no operation logic itself

#### Scenario: HTTP transport forwards
- **GIVEN** an HTTP handler in `cmd/trawl/server.go`
- **WHEN** it is invoked
- **THEN** it forwards to the same application-layer function the desktop binding calls

#### Scenario: No transport-only capability
- **GIVEN** any operation reachable from one transport
- **WHEN** the other transport is inspected
- **THEN** an equivalent entry point exists, because neither transport may hold behaviour the other lacks

#### Scenario: A transport method reaching past the application layer fails the build
- **GIVEN** a transport method that calls a domain package directly rather than forwarding to the application layer
- **WHEN** the required boundary check runs
- **THEN** it fails, naming the method and the package it reached into

#### Scenario: A recorded bypass carries its reason
- **GIVEN** a transport method that reaches past the application layer for a reason that has been accepted
- **WHEN** the boundary check runs
- **THEN** it passes only where that reason is recorded against the method, and fails once the method no longer needs the exemption, so the list cannot outlive what it excuses

### Requirement: Full HTTP API Parity
The HTTP API MUST serve every operation available over Wails IPC, including build identity, assessment reads and runs, email-posture reads and scans, scope read and write, settings, secret findings, regressions and scan triggering. Parity MUST be enforced by an automated check rather than by review.

#### Scenario: Container deployment is fully capable
- **GIVEN** a container deployment with no desktop binary present
- **WHEN** an operator triggers a scan, reads or writes the authorisation record, or runs an assessment
- **THEN** each succeeds over HTTP

#### Scenario: Change 006 data is reachable
- **GIVEN** a container deployment
- **WHEN** measured-state signals, assessment coverage and control postures are requested
- **THEN** they are served, rather than being reachable only from the desktop transport

#### Scenario: Both deployments report the same build identity
- **GIVEN** a container deployment
- **WHEN** its build identity is requested over HTTP
- **THEN** it is served from the same version source the desktop transport reports, so the two cannot disagree about what they are

#### Scenario: A new desktop binding without a route fails the build
- **GIVEN** a method newly bound over Wails IPC with no corresponding HTTP route
- **WHEN** the required parity check runs
- **THEN** it fails, naming the operation the container deployment cannot perform

#### Scenario: A deliberately desktop-only operation is recorded as such
- **GIVEN** an operation that is intentionally unavailable over HTTP
- **WHEN** the parity check runs
- **THEN** it passes only where a written reason is recorded against that operation, so that an omission cannot pass as a decision

### Requirement: Live Event Stream Over SSE
The server MUST expose the event bus to browser clients at `GET /api/v1/events` over Server-Sent Events, restricted to an explicit allowlist of event types.

#### Scenario: Browser receives bus events
- **GIVEN** a browser client subscribed to `/api/v1/events`
- **WHEN** an allowlisted event is published to the bus
- **THEN** it is delivered to that client

#### Scenario: Non-allowlisted events are not broadcast
- **GIVEN** an event type absent from the allowlist
- **WHEN** it is published to the bus
- **THEN** it is not delivered over SSE

#### Scenario: Retired WebSocket route
- **GIVEN** a dashboard build older than this change requesting `/ws`
- **WHEN** the server responds
- **THEN** it returns `410 Gone` naming `/api/v1/events` as the replacement, rather than `501` or an error resembling an outage

#### Scenario: Proxy does not buffer the stream
- **GIVEN** the committed nginx configuration
- **WHEN** the event-stream location is inspected
- **THEN** `proxy_buffering` is off, because a buffered stream delivers an assessment's events only once it has finished

### Requirement: One Frontend Bundle Across Both Transports
The frontend MUST select its transport at runtime behind a `TrawlTransport` seam implemented by `WailsTransport` and `HttpTransport`. No component may know which deployment it is running in.

#### Scenario: Runtime selection
- **GIVEN** the same Angular bundle
- **WHEN** it loads with the Wails runtime present, and again with it absent
- **THEN** it selects `WailsTransport` in the first case and `HttpTransport` in the second

#### Scenario: No silent empty dashboard
- **GIVEN** a browser deployment
- **WHEN** the frontend requests data
- **THEN** it issues HTTP calls, and a transport failure surfaces as an error rather than rendering as an empty result

#### Scenario: Components are transport-agnostic
- **GIVEN** any component
- **WHEN** its dependencies are inspected
- **THEN** it depends on `TrawlTransport` and never on a concrete implementation

#### Scenario: Naming a concrete transport outside the seam fails the build
- **GIVEN** a file outside the transport seam that names `WailsTransport` or `HttpTransport`
- **WHEN** the required check runs
- **THEN** it fails, because such a file compiles and renders normally and would fail only in the deployment it was not developed against

### Requirement: Embedded Signal Registry
The signal registry MUST be an embedded Go package under `config/signals`, compiled into every entrypoint rather than read from disk.

#### Scenario: Every entrypoint carries the same mapping
- **GIVEN** the desktop binary, `trawl server` and any worker
- **WHEN** each resolves a signal
- **THEN** all resolve identically, with no filesystem dependency

### Requirement: Scope Enforcement In The Application Layer Fails Closed
Scope enforcement MUST live in the application layer, MUST authorise nothing when the authorisation record is absent, unparseable or unsigned, and MUST NOT accept scope supplied by a transport.

#### Scenario: Absent or invalid authorisation authorises nothing
- **GIVEN** an absent, unparseable or unsigned authorisation record
- **WHEN** an assessment is requested
- **THEN** it is refused before any target is contacted

#### Scenario: Transport cannot supply its own scope
- **GIVEN** an HTTP request attempting to carry its own scope
- **WHEN** it is handled
- **THEN** the supplied scope is ignored and only the stored authorisation record is consulted

#### Scenario: Refusal is a result, not an error
- **GIVEN** an assessment refused for want of authorisation
- **WHEN** `AssessDomain` returns
- **THEN** it returns a `refused` outcome carrying a stated reason, so that "we were not allowed to look" is never collapsed into "we looked and found nothing"

### Requirement: Autoscaled Deployment Support
The server MUST run correctly on a managed autoscaling platform: honouring the injected `PORT`, draining on `SIGTERM`, and executing scans inline rather than on a detached goroutine.

#### Scenario: Injected port honoured
- **GIVEN** a managed platform injecting `PORT`
- **WHEN** the server starts
- **THEN** it listens on that port

#### Scenario: Graceful drain
- **GIVEN** a running instance receiving `SIGTERM`
- **WHEN** it shuts down
- **THEN** it drains in-flight work before exiting

#### Scenario: Scans run inline
- **GIVEN** an instance under default CPU allocation, which is throttled once it has written its response
- **WHEN** a scan is triggered
- **THEN** it executes inline so that the response reflects real progress, rather than returning `202 Accepted` over a goroutine that stalls or dies at scale-down

#### Scenario: Single container serves API and dashboard
- **GIVEN** the autoscaled deployment
- **WHEN** its services are inspected
- **THEN** one image serves both the API and the dashboard, with no separate proxy service required
