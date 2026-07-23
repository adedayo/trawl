# Capability: ci-cd-pipeline

## Purpose

Gate every change — human-authored or dependency-bot-authored — on automated tests, lint, and type-checks before merge, and keep every dependency (npm packages, job-container base images, Convex itself) current through an automated update pipeline with a supply-chain-aware cooldown and an AI-agent triage layer, so the engine stays free of both regressions and known-vulnerable dependencies without demanding constant manual attention from a single-operator project. This capability applies the same discipline the rest of the engine already uses on itself: deterministic gates decide, an agent narrates and recommends, and the agent's opinion never substitutes for the gate.

## ADDED Requirements

### Requirement: Automated test/lint/type-check gate on every change
Every pull request SHALL run, in CI: unit tests (Vitest) for pure-logic modules and Convex functions, integration/end-to-end tests (Playwright) for critical dashboard flows, a full TypeScript type-check, and lint. Merge SHALL be blocked on any failure, with no override path other than fixing the failure.

#### Scenario: Failing test blocks merge
- **GIVEN** a pull request whose changes cause an existing unit or integration test to fail
- **WHEN** CI runs
- **THEN** the merge is blocked until the failure is resolved, regardless of who or what authored the change

### Requirement: Coverage floor on deterministic logic
The priority-scoring function, CPE/CVE correlation logic, ingestion-dedup logic, the `posture-regression` classification ordering, and the `repository-secrets` priority mapping SHALL each carry unit tests covering their documented scenarios (see each capability's spec), enforced as a required CI check — not merely "some tests exist somewhere in the repo."

#### Scenario: New deterministic function ships without scenario coverage
- **GIVEN** a new deterministic scoring function is added without tests for its spec's documented scenarios
- **WHEN** CI runs
- **THEN** the coverage check fails and blocks merge

### Requirement: Automated dependency updates with a minimum-release-age cooldown
The system SHALL use an automated dependency-update tool (e.g., Renovate) configured with a minimum-release-age cooldown before any update — including patch releases — is even proposed: a floor of at least 3 days for routine packages, configurable up to 14 days for higher-risk categories (direct runtime dependencies, anything with install/postinstall scripts). This exists because a compromised-maintainer-token supply-chain attack against a widely-used package (the March 2026 compromise of `axios`, ~100M weekly downloads, is the concrete precedent) can ship a malicious version that looks like a routine patch release for the first hours or days, before it's caught and yanked — the cooldown gives that detection window time to work before the update ever reaches this repo.

#### Scenario: New release is not proposed before its cooldown elapses
- **GIVEN** a dependency publishes a new version today
- **WHEN** the dependency-update tool runs before the configured cooldown period has elapsed
- **THEN** no update PR is opened for that version yet

#### Scenario: Cooldown applies to job-container base images too
- **GIVEN** a new tag is published for a base image used by `scan-worker`, `discovery-worker`, or `repo-scan-worker`
- **WHEN** the dependency-update tool evaluates it
- **THEN** the same cooldown discipline applies as to npm packages — container image tags are not treated as exempt

### Requirement: Agentic triage layer, narrative and recommendation only
A scheduled CI job SHALL invoke an AI coding agent against each open dependency-update pull request once its cooldown has elapsed: the agent reads the changelog/diff, classifies the update's breaking-change and security risk, and runs the full test suite. The agent's output SHALL be a recommendation and a written rationale attached to the PR, never a bypass of the deterministic test gate — an update only auto-merges when **both** the deterministic gate (tests pass, cooldown elapsed) **and** the agent's risk classification (patch/minor, non-security-relevant package) agree; disagreement between the two, or any ambiguity, routes to human review with the agent's rationale attached, not a forced merge.

#### Scenario: Low-risk patch update auto-merges
- **GIVEN** a patch-version update to a non-security-relevant package, past its cooldown, with all tests passing and the agent classifying it low-risk
- **WHEN** the triage job runs
- **THEN** the update is auto-merged, with the agent's rationale recorded on the PR for audit

#### Scenario: Agent and deterministic gate disagree — no auto-merge
- **GIVEN** an update passes all tests but the agent's changelog read flags a likely breaking change, or an update the deterministic classifier marks as major/security-relevant regardless of what the agent concludes
- **WHEN** the triage job runs
- **THEN** the PR is left open for human review with the agent's written rationale attached; it is never auto-merged on the agent's judgment alone

#### Scenario: Major-version and security-relevant packages always require human approval
- **GIVEN** an update is a major-version bump, or touches a package classified as security-relevant (cryptography, authentication, or one of the scanning-tool binaries themselves — naabu/httpx/nuclei/subfinder/amass/gitleaks)
- **WHEN** the triage job evaluates it
- **THEN** it always routes to human review, regardless of the agent's classification or test results

### Requirement: Least-privilege automation credentials
The CI automation's credentials (dependency-update tool, agentic triage job) SHALL be scoped to opening PRs, running CI, and merging only PRs that tool itself proposed and that passed both required gates — never broad write access to the repository, and never authority to modify a PR's diff beyond what the dependency-update tool itself generated.

#### Scenario: Automation cannot alter its own proposed diff
- **GIVEN** the agentic triage job is reviewing a dependency-update PR
- **WHEN** it completes its review
- **THEN** it can comment, label, approve, and merge that PR, but it has no credential path to push additional commits changing the diff
