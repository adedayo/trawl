# Capability: asset-discovery

## Purpose

Continuously expand the known asset inventory using passive, non-intrusive OSINT sources, so unknown/forgotten exposure is found before an attacker finds it — without ever expanding scan scope beyond what's authorized.

## ADDED Requirements

### Requirement: Certificate transparency monitoring
The system SHALL query certificate transparency logs for SANs matching configured root domains on a scheduled interval.

#### Scenario: New SAN appears
- **GIVEN** a new SAN `vpn.example.com` appears in a CT log for authorized root domain `example.com`
- **WHEN** the discovery job runs
- **THEN** it is added to the inventory as a candidate asset with source `ct-log`

### Requirement: Passive subdomain enumeration
The system SHALL run passive subdomain enumeration (e.g. `subfinder`) against configured root domains without active brute-force DNS flooding.

#### Scenario: Passive enumeration run
- **GIVEN** a scheduled discovery cycle starts
- **WHEN** subdomain enumeration executes against an authorized root domain
- **THEN** it uses passive data sources only, generating no more than a configured minimal DNS query volume against the target's own nameservers

### Requirement: Confidence scoring
The system SHALL score every discovered candidate asset with a confidence level (`high` | `medium` | `low`) based on source reliability.

#### Scenario: Source reliability drives confidence
- **GIVEN** a candidate asset was found via CT log SAN match on an authorized domain
- **WHEN** it is scored
- **THEN** it receives `high` confidence

### Requirement: Confidence-gated promotion
High-confidence candidates SHALL be auto-promoted into the active inventory; medium/low-confidence candidates SHALL be queued for human review before promotion.

#### Scenario: Low-confidence match queued
- **GIVEN** a Shodan org-name match returns an IP with no clear ownership link to the authorized scope
- **WHEN** discovery evaluates it
- **THEN** it is queued as low-confidence for manual review, not auto-scanned

### Requirement: Scope ceiling independent of confidence
The system SHALL NEVER promote a candidate asset outside the configured authorized scope (seed domains/CIDRs and their direct subdomains) into scan-eligible status, regardless of confidence score.

#### Scenario: Out-of-scope high-confidence match
- **GIVEN** a candidate asset matches with high confidence but its domain is not a subdomain of any authorized root domain and not within an authorized CIDR
- **WHEN** promotion logic evaluates it
- **THEN** it is rejected from scan-eligible status regardless of its confidence score
