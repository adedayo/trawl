# Change: 006-vantage-integration

## Status

**Proposed, not started.** Depends on Change 003 (Go/SQLite engine) being implemented. First change in the risk-quantification arc (006 → 012); see `openspec/RISK-ARC.md`.

## Why

Trawl's `email-authentication` capability (Change 001) checks SPF/DKIM/DMARC by hand-rolled DNS lookups. `vantage` (github.com/adedayo/vantage, BSD-3, Go) already does that work and considerably more: MTA-STS with policy retrieval, TLS-RPT, BIMI, DNSSEC chain of trust, NSEC/NSEC3, DANE at three ports, CAA with tree climbing, open-resolver and AXFR checks, wildcard detection, subdomain-takeover assessment, provider attribution with jurisdiction expectations, and Certificate Transparency enumeration. Every check is external, credential-free and non-destructive — the same guardrail Trawl enforces.

Reimplementing that inside Trawl would be duplicated effort against a moving target (DNS standards, provider range files, CT log endpoints).

Both projects are under common ownership, so Trawl **embeds vantage as a Go library** rather than invoking it as a subprocess. Vantage's README currently states that `pkg/` is an implementation detail excluded from its version guarantees; that caveat exists to protect vantage from unknown third-party consumers, and it does not describe this relationship. Trawl becomes vantage's first-class embedding consumer, and vantage grows a deliberate, documented Go API shaped by that requirement. The work of defining that API is part of this change, upstream as well as here.

The strategic reason is larger than convenience. The later changes in this arc need a **registry of measured-state signals**: externally observable facts about the configuration of a defensive mechanism, each with a stable identifier, a severity, an evidence string, and — critically — an explicit statement of whether it was actually assessed. Vantage's finding catalogue (`SURF-*` identifiers) and its four-state coverage model (`ok` / `not_found` / `not_checked` / `check_failed`) *are* that registry. This change adopts them.

## What Changes

- New capability: `vantage-integration`. Trawl imports vantage's assessment packages directly, running checks in-process and persisting findings, coverage states and discovered hostnames into the existing SQLite store.
- **An embedding API is defined upstream in vantage**, driven by Trawl's requirements: a context-first assessment entry point, injectable resolver and network transport, structured results with no rendering or CLI concerns, typed sentinel errors, and a declared egress profile per check. Trawl pins it by module version and guards it with contract tests.
- **Measured-state signal registry.** Every `SURF-*` finding identifier is mapped, in versioned data, to a signal record: which weakness class or scenario it bears on, which exploitation stage it informs, and its dedup group. This registry is consumed by Changes 007–009; this change only establishes and populates it.
- **Four-state coverage is preserved end to end.** `not_checked` and `check_failed` are stored and surfaced distinctly from `not_found`. An unassessed control is never rendered, scored or aggregated as a passing one.
- **Scope enforcement moves into the transport.** Because assessment runs in-process, Trawl supplies the resolver and dialer vantage uses, and an out-of-scope target becomes unreachable at the network layer rather than merely un-requested. This is a stronger guarantee than a subprocess boundary could offer.
- **Discovery feed.** Certificate Transparency enumeration yields hostnames fed into `asset-discovery` as a new discovery source, and provider attribution enriches `asset-inventory` with hosting provider and inferred jurisdiction.
- **Supersedes** the DNS-lookup internals of the `email-authentication` capability. The capability's requirements survive; its implementation is replaced.
- **New weakness classes** enter the inventory: domain/delegation hijack (DNSSEC, NSEC3 walking, CAA, AXFR, open resolver, wildcard) and subdomain takeover — neither previously modelled by Trawl.

## Explicitly Out of Scope

- **No subprocess execution, and no dependency on the CLI.** Trawl does not shell out, does not parse vantage's rendered output, and does not require a `vantage` binary to be installed. The CLI remains vantage's contract with the wider world; it is not Trawl's.
- **No fork.** Changes vantage needs in order to be embeddable are made upstream in vantage and consumed by version. If a requirement is genuinely Trawl-specific rather than generally useful, it belongs in Trawl's adapter layer, not in vantage.
- **No probability computation.** This change stores signals and coverage. Their effect on priors, uncertainty and scenario rates lands in 008 and 009.
- **No `--from internal` vantage point.** Not implemented upstream; Trawl does not pretend otherwise.
- **No telemetry ingestion.** Detecting that a domain publishes no `rua` address is in scope (it is a measured-state observation); receiving and parsing DMARC aggregate reports is Change 009.

## Impact

- **No new runtime dependency.** Assessment compiles into the Trawl binary, which preserves the single-binary property that Changes 003, 004 and 005 depend on — no bundled executable in the Docker image, nothing to discover on `PATH`, and desktop packaging is unaffected.
- **A new upstream dependency relationship.** Trawl pins a vantage module version; upgrades are a `go.mod` bump plus a contract-test run. API drift surfaces as a compile failure, which is the earliest and cheapest place for it to surface.
- **Scope enforcement extended.** Checks that query the target's own nameservers directly (`axfr`, `ns`, `tko`, `wild`) are excluded by a policy stated over declared egress classes, and are additionally unreachable through the scope-guarded transport. Configuration names egress classes and consented third-party endpoints, never individual checks — so an upgrade introducing new egress fails closed.
- **Schema additions**: `signalObservations`, `signalRegistry`, `assessmentCoverage`.
