# Capability: repository-secrets

## Purpose

Passively clone and scan operator-declared **public** code repositories for exposed secrets (API keys, credentials, tokens), across full git history, using the same non-destructive, deterministic-scoring discipline as every other capability — with optional live verification against the issuing provider to distinguish a confirmed-active credential from a pattern match, toggleable by the operator when they'd rather trade false positives for zero outbound calls to third-party providers.

A public repository is unauthenticated, externally-reachable content — anyone, including an attacker, can clone it with no credentials — so this capability extends the "external, unauthenticated attacker's view" the engine already covers, rather than crossing into the authenticated/internal scanning the engine's non-goals explicitly exclude.

## ADDED Requirements

### Requirement: Operator-declared scope, never auto-discovered
The system SHALL only scan repositories explicitly declared in `config.seedRepos[]`. It SHALL NOT attempt to discover an operator's repositories via GitHub/GitLab org enumeration, code search, or any other passive or active technique.

#### Scenario: Undeclared repository excluded
- **GIVEN** a repository is not present in `config.seedRepos[]`
- **WHEN** the repository-scanning job builds its target list
- **THEN** that repository is excluded, regardless of any other signal suggesting it might belong to the operator

### Requirement: Public repositories only
The system SHALL only clone repositories reachable via unauthenticated, anonymous git access. It SHALL NOT accept, store, or use any credential (personal access token, SSH key, GitHub/GitLab App installation) for private-repository access. Authenticated/private repository scanning is explicitly out of scope, consistent with the engine's existing non-goal on internal/authenticated vulnerability management.

#### Scenario: Private repository rejected at config time
- **GIVEN** an operator adds a repository URL that requires authentication to clone
- **WHEN** the configuration is validated
- **THEN** the entry is rejected as unsupported, with no credential-entry path offered

### Requirement: Full git-history scanning, non-destructive
The system SHALL scan the full commit history of each declared repository, not only the current branch tip, since a secret committed and later removed remains recoverable from history. Only clone and read-only static analysis operations are performed — no write access, no force-push, nothing capable of altering the repository.

#### Scenario: Secret found in history despite later removal
- **GIVEN** a secret was committed and then removed in a later commit on the default branch
- **WHEN** the repository is scanned
- **THEN** the finding is still detected, sourced from the historical commit

### Requirement: Bounded clone size and depth
The system SHALL enforce a configurable maximum clone size and/or history depth per repository, and SHALL skip any repository exceeding the configured bound, logging a visible, specific reason rather than allowing an unbounded job run.

#### Scenario: Oversized repository skipped
- **GIVEN** a declared repository exceeds the configured size bound
- **WHEN** the scan job runs
- **THEN** that repository is skipped with a logged reason, and the job completes normally for the remaining in-bound repositories

### Requirement: Incremental rescanning
On repeat scheduled scans, the system SHALL scan only commits added since the last successful scan of that repository (using the last-scanned commit SHA as the boundary), rather than rescanning full history every cycle — except on first scan or when a full rescan is explicitly requested via config.

#### Scenario: Second scan is incremental
- **GIVEN** a repository was fully scanned on a prior run and its last-scanned commit SHA was recorded
- **WHEN** the next scheduled scan runs
- **THEN** only commits added since that SHA are examined

### Requirement: Deterministic pattern-match detection as the baseline
The system SHALL detect candidate secrets via deterministic pattern/entropy matching, always active regardless of the live-verification toggle below.

#### Scenario: Baseline detection always runs
- **GIVEN** live verification is disabled via configuration
- **WHEN** a repository is scanned
- **THEN** pattern-match detection still runs and still produces findings

### Requirement: Optional live verification, operator-toggleable from the dashboard
The system SHALL support live verification of candidate secrets — a read-only call to the issuing provider's API (e.g., AWS STS `get-caller-identity`) to confirm whether a matched credential is currently active. This behavior SHALL be controlled by a single explicit flag, `config.secretVerificationEnabled` (default `true`), which the operator can toggle from the dashboard UI (see `dashboard` capability). When disabled: no outbound calls are made to any credential-issuing provider, only pattern-match findings are produced, and those findings are capped at a lower deterministic priority ceiling reflecting the higher false-positive rate of unverified matches.

#### Scenario: Verification enabled, credential confirmed active
- **GIVEN** `secretVerificationEnabled` is `true` and a candidate secret matches a known provider's credential format
- **WHEN** the scan job verifies it
- **THEN** a read-only check against the issuing provider confirms whether it is active, and the finding's priority reflects that confirmation

#### Scenario: Verification disabled via UI toggle
- **GIVEN** the operator has turned `secretVerificationEnabled` off in the dashboard
- **WHEN** the next scheduled scan runs
- **THEN** zero outbound verification calls are made to any provider, findings are pattern-match only, and their priority is capped below the ceiling available to verified findings

#### Scenario: Toggle takes effect without a code change
- **GIVEN** the operator changes `secretVerificationEnabled`
- **WHEN** the next scheduled scan executes
- **THEN** the new behavior applies without any redeploy or code modification, since this is a config value, not a build-time flag

### Requirement: Deterministic priority mapping
Priority SHALL be a pure function of verification status and secret type/scope (e.g., verified-active credential with broad/production scope ranks above verified-active with narrow scope, which ranks above any unverified pattern match). No secret finding's priority is ever set or adjusted by the AI-triage layer, matching the engine-wide deterministic-severity/AI-narrative-only rule.

#### Scenario: Verified outranks unverified regardless of pattern confidence
- **GIVEN** one finding is a verified-active credential and another is a high-confidence but unverified pattern match
- **WHEN** priorities are computed
- **THEN** the verified finding ranks higher, independent of the two detectors' internal confidence scores

### Requirement: Secret value redaction everywhere
The system SHALL NEVER store, display, or transmit the full raw secret value, including in Slack/webhook alert payloads. Findings and alerts SHALL reference provider, best-effort scope/permissions (if determinable via a read-only check), file path, commit SHA, and verification status only; the raw value SHALL be redacted before any persistence.

#### Scenario: Alert payload contains no raw credential
- **GIVEN** a verified-active secret finding fires an alert
- **WHEN** the alert payload is inspected
- **THEN** it contains provider, scope, file/commit reference, and verification status, and no reconstructable form of the raw secret value

### Requirement: Feeds the shared posture-regression mechanism
New secret findings, and previously-found secrets that are subsequently removed or rotated such that they no longer verify, SHALL be tracked as posture snapshots per repository through the existing `posture-regression` capability's shared snapshot/diff/classify/alert contract, not a bespoke check.

#### Scenario: New secret in a previously-clean repository is a confirmed regression
- **GIVEN** a repository had zero live secret findings on its last two scans
- **WHEN** a new verified-active secret appears and is confirmed on the next scheduled scan
- **THEN** it is surfaced through `posture-regression` as a confirmed regression on that repository's secret-exposure attribute

### Requirement: Same job-container pattern as other scan jobs
The repository-scanning job SHALL run as a standard job container (e.g., `repo-scan-worker`), triggered identically to `discovery-worker`/`scan-worker` — an Ofelia cron sidecar running `docker compose run` — with no scheduler-specific code in the job itself.

#### Scenario: Same triggering mechanism as other job containers
- **GIVEN** the `repo-scan-worker` image
- **WHEN** it runs on its configured schedule
- **THEN** it is invoked by the same Ofelia/`docker compose run` mechanism as `discovery-worker` and `scan-worker`, with no job-specific scheduling logic
