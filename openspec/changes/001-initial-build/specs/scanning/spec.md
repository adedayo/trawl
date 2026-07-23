# Capability: scanning

## Purpose

Perform non-destructive, rate-limited scans of in-scope assets to identify open services, versions, and misconfigurations without impacting availability — the recon layer that feeds vulnerability correlation.

## ADDED Requirements

### Requirement: Scope enforcement, defense-in-depth
The system SHALL only scan assets with status `active` that fall within the configured authorized scope; the scan job SHALL independently re-validate this allowlist rather than trusting upstream data.

#### Scenario: Pending asset excluded
- **GIVEN** an asset is in status `pending` (not yet approved)
- **WHEN** the scan job builds its target list
- **THEN** that asset is excluded from the scan

#### Scenario: Upstream data corruption
- **GIVEN** an asset record was somehow marked `active` outside the normal promotion workflow and falls outside the configured authorized scope
- **WHEN** the scan job builds its target list
- **THEN** the job's own scope-validation logic rejects it independent of the asset's stored status

### Requirement: Non-destructive techniques only
The system SHALL use non-destructive techniques only: port/service discovery, banner/version fingerprinting, TLS inspection, and non-intrusive vulnerability-detection templates. No exploitation, brute-force credential attempts, or denial-of-service-capable techniques are permitted.

#### Scenario: Template selection
- **GIVEN** a nuclei template is being considered for the scan job's template set
- **WHEN** it is classified as intrusive, exploit-executing, or DoS-capable
- **THEN** it is excluded from the template set used by this system

### Requirement: Rate limiting and identification
The system SHALL rate-limit requests per target and SHALL identify itself via a recognizable User-Agent/reverse-DNS pointing to an abuse-contact page.

#### Scenario: Scan traffic identification
- **GIVEN** a scan job sends an HTTP request to a target
- **WHEN** the target inspects the request
- **THEN** the User-Agent identifies the scanner and its source IP resolves via PTR record to a host with an abuse-contact page

### Requirement: Scheduled execution, not always-on
The system SHALL run on a configurable schedule (default daily) via the Ofelia cron sidecar triggering a `docker compose run` execution of the scan-worker container, not as an always-on process.

#### Scenario: Scheduled trigger
- **GIVEN** the configured schedule is daily at 02:00 UTC
- **WHEN** that time arrives
- **THEN** the Ofelia sidecar runs the scan-worker image via `docker compose run`, which exits on completion

### Requirement: Partial results preserved
The system SHALL emit raw scan output and structured findings to the ingestion endpoint even on partial failure; partial results SHALL NOT be discarded.

#### Scenario: Job timeout mid-scan
- **GIVEN** a scan job's container execution exceeds its configured timeout
- **WHEN** it is terminated
- **THEN** results gathered up to that point are still submitted for ingestion before the job exits

### Requirement: Structured, comparable hardening attributes
In addition to raw scan output, the system SHALL emit TLS version/cipher/protocol support, certificate fields (issuer, algorithm, key length, expiry), and the open port/service set as structured, per-asset fields comparable across scan runs — not only as unstructured findings — so the `posture-regression` capability can diff a run against the immediately preceding one for the same asset.

#### Scenario: Structured TLS snapshot emitted
- **GIVEN** a scan job completes TLS inspection against an in-scope asset
- **WHEN** it emits results for ingestion
- **THEN** the supported TLS versions and cipher suites are included as structured fields, not only as free-text banner output
