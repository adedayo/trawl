# Capability: email-authentication

## Purpose

Passively assess each in-scope domain's email-authentication posture (SPF, DKIM, DMARC, and the adjacent BIMI/MTA-STS/TLS-RPT/CAA records) via DNS-over-HTTPS, surfacing domain-spoofing and business-email-compromise preconditions the same way `vulnerability-correlation` surfaces CVE-based weaknesses — deterministic, non-destructive, and cheaper than any other capability in the system: a DNS TXT record read, not a scan.

## ADDED Requirements

### Requirement: SPF record check
The system SHALL query each in-scope domain's SPF (Sender Policy Framework) TXT record and record its presence, syntactic validity, and whether its all-mechanism is overly permissive (`+all`).

#### Scenario: Missing SPF record
- **GIVEN** an in-scope domain has no SPF TXT record
- **WHEN** the email-authentication check runs
- **THEN** a finding is recorded noting SPF absence, with priority reflecting that any sender can claim to send as this domain

#### Scenario: Overly permissive SPF
- **GIVEN** an in-scope domain's SPF record ends in `+all`
- **WHEN** the check runs
- **THEN** a finding is recorded noting the permissive all-mechanism, since it authorizes any host to send as the domain

### Requirement: DKIM selector check
The system SHALL query a configurable list of common/known DKIM selectors for each in-scope domain and record which selectors resolve to a valid DKIM public-key record, while explicitly not claiming exhaustive selector coverage.

#### Scenario: No configured selector resolves
- **GIVEN** none of the configured selectors resolve to a valid DKIM record for a domain
- **WHEN** the check runs
- **THEN** a finding is recorded as "no DKIM selector found among checked selectors," never as "domain has no DKIM" — selector discovery is inherently incomplete since selectors aren't advertised in DNS

### Requirement: DMARC policy check and deterministic priority
The system SHALL query each in-scope domain's DMARC TXT record and classify its policy (`none` | `quarantine` | `reject`), percentage (`pct`), and alignment mode (`strict` | `relaxed` for both `aspf`/`adkim`). Priority SHALL be a deterministic function of these fields alone — no DMARC/SPF/DKIM finding's priority is ever set or adjusted by the AI-triage layer, matching the engine-wide deterministic-severity/AI-narrative-only rule.

#### Scenario: No DMARC record
- **GIVEN** an in-scope domain has no DMARC TXT record at all
- **WHEN** the check runs
- **THEN** a finding is recorded at the highest priority this capability assigns, since no reporting or enforcement policy exists at all

#### Scenario: Monitor-only policy
- **GIVEN** an in-scope domain's DMARC record has `p=none`
- **WHEN** the check runs
- **THEN** a finding is recorded at a priority reflecting that spoofed mail is observed but not blocked or quarantined

#### Scenario: Enforced policy
- **GIVEN** an in-scope domain's DMARC record has `p=reject` at `pct=100`
- **WHEN** the check runs
- **THEN** no open finding is recorded for DMARC enforcement on that domain

### Requirement: Adjacent passive record checks, informational priority
The system SHALL also check BIMI, MTA-STS, TLS-RPT, and CAA records for each in-scope domain and surface their absence as informational-priority findings, distinct from and never elevated above SPF/DKIM/DMARC findings.

#### Scenario: Missing MTA-STS
- **GIVEN** an in-scope domain has no MTA-STS policy
- **WHEN** the check runs
- **THEN** an informational finding is recorded, at a priority tier below any open SPF/DKIM/DMARC finding on the same domain

### Requirement: No new job container; runs as a scheduled Convex action
The system SHALL implement all checks in this capability as DNS-over-HTTPS queries (a plain HTTPS call to a public DoH resolver) executed from a Convex scheduled action, identically to the existing KEV/NVD/EPSS feed-pull functions. This capability SHALL NOT require a Docker Compose job container, since it involves no raw DNS sockets and no scanning binaries.

#### Scenario: No job container needed
- **GIVEN** Convex is either self-hosted or pointed at Convex Cloud
- **WHEN** the email-authentication check runs
- **THEN** it executes as a Convex action making an outbound HTTPS call, with no job container or scheduler-triggered container execution involved either way

### Requirement: Scope-limited, scheduled re-check
The system SHALL only check domains within the configured authorized scope (seed domains, or a configured mail-domain subset), and SHALL re-run checks on a configurable schedule so that policy drift (a domain's DMARC policy weakening after initial check) is caught, not just recorded once.

#### Scenario: Policy regression detected
- **GIVEN** a domain's DMARC policy was `p=reject` at last check and is now `p=none`
- **WHEN** the next scheduled check runs
- **THEN** a new finding is recorded reflecting the regressed policy, and the domain's finding history shows the transition rather than silently overwriting the prior state
