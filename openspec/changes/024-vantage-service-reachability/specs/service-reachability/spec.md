# Capability: service-reachability

## Purpose

Consume Vantage's structured service and protocol observations as measured
exposure evidence. This capability does not infer attacker contact or compute
risk by itself.

## Requirements

### Requirement: Layered reachability is preserved

The system SHALL distinguish DNS resolution, TCP response, TLS negotiation,
HTTP response and STARTTLS transition. A lower-layer response SHALL NOT be
promoted to a higher-layer response.

### Requirement: Four-state coverage is carried

Every requested service probe SHALL carry `ok`, `not_found`, `not_checked` or
`check_failed` coverage, with a reason for non-assessed states. `unknown` probe
results SHALL NOT render as closed or clean.

### Requirement: Exposure history uses explicit service evidence

Only a structured Vantage observation proving that the requested service layer
responded MAY open or sustain an exposure window. DNS resolution and provider
attribution alone SHALL NOT create one.

### Requirement: Scope is enforced at the transport

An out-of-scope target, redirect, certificate name or discovered hostname SHALL
receive zero network queries through the Vantage transport used by Trawl.

### Requirement: Contact posterior remains unwritable

Assessment-derived service observations SHALL affect only exposure history and
the exposure-derived contact prior. They SHALL NOT write attacker-contact
posterior telemetry.

### Requirement: Default operation is unprivileged

The default declared-service profile SHALL use the Vantage standard-library
transport path and SHALL require neither cgo nor raw-packet privileges.

### Requirement: Raw-packet scanning is optional and benchmark-gated

A raw-SYN backend SHALL not be enabled unless a reproducible benchmark
establishes material value for a named workload and confirms equivalent
structured observation, scope and coverage semantics.
