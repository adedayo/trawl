# Design: 024-vantage-service-reachability

## Ownership boundary

Vantage owns the observation and protocol probe. Trawl owns authorization
configuration, persistence, exposure-history accounting and risk consumption.
The adapter translates Vantage types into Trawl store types; it does not repeat
probe logic or infer reachability from rendered findings.

```text
operator-declared service
        |
        v
Trawl scope + egress policy
        |
        v
Vantage Change 016 probe
  TCP -> TLS -> HTTP/STARTTLS
        |
        v
structured observation + coverage
        |
        v
Trawl adapter
  reachability history -> contact prior
```

A service observation is exposure evidence. No path in the assessment adapter
may write contact posterior telemetry.

## Probe profiles

The first profile is `declared-services`. Each target carries host, port,
transport and protocol. The profile is bounded by configured concurrency,
timeout and request budget. It is suitable for Trawl's external assessment and
can be run with the same scope-guarded resolver/dialer/HTTP client supplied by
Change 006.

For a host discovered through DNS, Certificate Transparency or another
authorised source, Trawl may use Vantage's ranked common-service catalogue for a
bounded candidate pass. The first pass should use the highest-priority entries
and retain the catalogue service name in each observation; a wider pass is a
separate operator policy decision. This finds accidental exposure without
turning every discovered host into an unbounded port scan.

A future `candidate-discovery` profile is separate. It may inspect a bounded
common-port list or caller-supplied range, but it must disclose its packet and
rate budgets and produce lower-layer observations that are not silently
promoted to application reachability.

## Observation mapping

Trawl stores one observation per asset/service/layer with:

- `responding`, `not_responding` or `unknown` state;
- Vantage check and probe-profile identifiers;
- protocol layer and endpoint;
- observed timestamp;
- bounded structured evidence;
- library and registry versions;
- coverage state and reason.

Only `responding` at the requested service layer can open or sustain an
exposure window. `unknown`, timeout, cancellation, scope refusal and
`check_failed` extend inferred time and never close an exposure. A successful
clean assessment at the relevant layer is the only closure event.

## TLS migration

TLSAudit's useful output is divided into two classes:

1. one-shot evidence from a normal TLS handshake, which belongs in Vantage's
   standard protocol probe;
2. breadth from repeated handshakes across versions/ciphers, which belongs in
   an explicitly bounded deep profile.

The second class is not required to make Change 007 work. It lands only after
Vantage has golden-vector tests for certificates, SNI, protocol versions,
ciphers and STARTTLS. No raw-packet shortcut can replace those handshakes.

## Raw-SYN decision gate

Trawl does not depend on a raw-SYN implementation. Raw-packet and privileged
scanning are outside the foreseeable roadmap because the anticipated use cases
do not justify the platform, privilege and deployment costs. Standard-library
TCP connect probing is the supported candidate-discovery mechanism. A future
proposal may reconsider this only if the use cases materially change.

## Failure and coverage semantics

A failed probe is retained as `check_failed` with its reason. A policy-excluded
probe is `not_checked`. Neither is a statement that the endpoint is closed.
A TCP response with TLS failure is recorded as TCP responding and TLS unknown or
failed; it is not recorded as TCP closed. This layered representation is what
lets the contact model use the strongest fact actually established.
