# Change: 024-vantage-service-reachability

## Status

**Proposed.** Depends on Vantage Change 016 and extends Change 006 without
reintroducing subprocess scanners.

## Why

Trawl's contact-probability model needs a measured answer to a narrow question:
did a declared service respond from the authorised external vantage point?
Current Vantage observations establish DNS resolution and network attribution,
but they do not prove that a service responded. The old `naabu`/`httpx` worker
path can produce raw scan output, but it is not the structured, scope-guarded
observation contract needed by the risk model and its coverage rules.

Vantage Change 016 makes service reachability and protocol evidence a reusable
upstream capability. Trawl should consume that capability in-process and use it
to populate exposure history. The integration must preserve the distinction
between DNS resolution, TCP response, protocol response and attacker contact.

## What Changes

- Consume Vantage's structured service observations through the existing thin
  adapter.
- Persist per-asset/service reachability observations with four-state coverage.
- Update Change 007 exposure history only from explicit service observations;
  DNS resolution and provider attribution alone never open an exposure window.
- Keep observed and inferred time-at-risk separate, including left-censoring and
  failed or unassessed intervals.
- Carry Vantage probe profile, protocol layer, evidence and timestamp into the
  stored observation and read model.
- Add contract tests against the Vantage embedding API and scope-guard tests
  proving an out-of-scope service receives zero network queries.
- Use Vantage's bounded `most-common` profile for recurring sweeps and
  `extended` for explicit escalation. Do not add raw-packet or privileged
  scanning to Trawl; the anticipated use cases do not justify that friction.

## Explicitly Out of Scope

- Implementing TCP or TLS probing inside Trawl.
- Calling `tcpscan`, `tlsaudit`, `naabu`, `httpx` or another scanner as a
  subprocess.
- Treating a responding service as evidence of attacker contact.
- Computing contact probability or exploit probability in the Vantage adapter;
  those remain owned by Changes 007 and 009.
- Automatically probing arbitrary ports or CIDRs. Probes may target hosts
  already admitted to Trawl's authorised inventory through discovery.

## Eventual retirement of TCPScan and TLSAudit

This change is the Trawl-side consumer contract for the intended retirement of
the legacy repositories. Retirement is not complete when Vantage can open a
TCP connection; it is complete when supported consumers no longer depend on
the old tools and the capabilities users relied on are either reproduced or
explicitly retired.

The repositories may be retired from the supported toolchain only after:

- Vantage contract and golden-vector tests cover the supported TCP, TLS,
  STARTTLS and reporting workflows that remain promised;
- Trawl has migrated all supported scans and has no subprocess, module or
  output-parser dependency on TCPScan or TLSAudit;
- Vantage and Trawl documentation identify the replacement profile and any
  deliberately unsupported broad-scan behavior;
- each legacy repository has published a final deprecation release pointing to
  Vantage; and
- one release cycle has passed without an unresolved consumer migration issue.

These gates do not require deleting or archiving the GitHub repositories.
Keeping them public and read-only is a valid portfolio and historical-
preservation choice, provided their README and repository metadata clearly mark
them deprecated and point users to Vantage. The public repositories retain
their accumulated visibility without remaining competing supported
implementations. If they are eventually archived, that is optional
housekeeping and must preserve source history and licensing. Neither choice
implies that Vantage reproduces every raw-SYN implementation detail or
historical CLI format; it means Vantage is the supported capability owner.
