# Tasks: 001-initial-build

## Phase 0 — Foundations
- [ ] Write self-authorization/scope document (seed CIDRs + domains, explicitly non-destructive) — blocks all scanning work
- [ ] Create the repo (Apache-2.0 license, README, NOTICE disclosing the self-hosted-Convex FSL-1.1 dependency) as the canonical engine location
- [ ] Docker Compose skeleton in `deploy/compose/`: self-hosted Convex + dashboard (SQLite), Ofelia cron sidecar, nginx container for the Angular build
- [ ] Angular CLI app scaffold under `app/`; job container skeleton under `jobs/`
- [ ] Set up a `.env`/Docker-secrets convention (LLM API key, optional Shodan/Censys key, alert webhook URL)
- [ ] Draft and commit Convex schema v1: `assets`, `scans`, `findings`, `config`
- [ ] Scaffold Angular app with latest-stable Angular (standalone, signals, `@if`/`@for`/`@switch` only), Tailwind + spartan/ui installed from day one — not retrofitted later
- [ ] `ci-cd-pipeline` skeleton: GitHub Actions running lint + TypeScript type-check + Vitest on every PR, required to pass before merge, from the very first commit

## Phase 1 — MVP: seed scan → dashboard
- [ ] Build `scan-worker` container: `naabu` + `httpx` + `nuclei` (KEV-tagged templates only)
- [ ] Implement `--dry-run` flag on `scan-worker`; write allowlist-enforcement test
- [ ] Convex HTTP action: ingest scan results, dedup, write `findings`
- [ ] Capture TLS version/cipher/protocol, certificate fields, and open port/service set as structured, comparable fields (not just raw output) — foundation for Phase 3's `posture-regression` capability
- [ ] Vitest unit tests for the ingestion-dedup logic, required in CI per `ci-cd-pipeline`'s coverage-floor requirement
- [ ] Angular v1: read-only asset/findings list via Convex live query; single-user auth gate; explicit loading/empty/error states from the start (not retrofitted)
- [ ] Ofelia → daily `scan-worker` execution via `docker compose run`
- [ ] Staged rollout: personal/sandbox domain → one non-critical subdomain → full seed list
- [ ] **Exit criteria**: live dashboard shows current assets + KEV-flagged findings, refreshed daily, allowlist test passing in CI

## Phase 2 — Discovery automation
- [ ] Build `discovery-worker` container: CT log queries, `subfinder`/`amass`, ASN/WHOIS pivots
- [ ] Confidence-scoring logic; scope-ceiling enforcement independent of confidence
- [ ] Convex logic: diff against inventory, auto-promote high-confidence, queue medium/low for review
- [ ] Angular v2: Pending Assets review queue, asset detail/history view
- [ ] First alerting: Convex action → Slack/Teams webhook on new-asset and new-critical-finding events, with dedup
- [ ] Email-authentication check: Convex scheduled action querying SPF/DKIM/DMARC (+ BIMI/MTA-STS/TLS-RPT/CAA) via DNS-over-HTTPS for each in-scope domain; no new job container
- [ ] Angular v2 addition: email-authentication posture panel per domain (policy, priority, last-checked)
- [ ] **Exit criteria**: new real-world asset (test with a known-but-unlisted subdomain) is discovered, queued, and approvable within one cycle; a seed domain's DMARC/SPF/DKIM posture is checked and surfaced without a job container, and a policy regression on a re-check produces a new finding rather than a silent overwrite

## Phase 3 — Vulnerability intel enrichment
- [ ] Convex scheduled function: pull CISA KEV JSON, NVD API deltas, EPSS scores into reference tables
- [ ] CPE/CVE correlation logic (pure function, unit-tested)
- [ ] Deterministic priority-scoring function (KEV override + CVSS/EPSS/exposure composite for non-KEV)
- [ ] Recompute-on-feed-update job
- [ ] Angular v3: severity-sorted findings view with KEV badge, EPSS score, plain-language "why this matters" line
- [ ] `posture-regression` capability: shared `postureSnapshots`/`regressions` schema, versioned better/worse ordering config per attribute type, two-consecutive-observation confirmation logic, `regression` finding category wired into `alerting`
- [ ] Wire `scanning`'s TLS/cipher/certificate/port-set snapshots and `email-authentication`'s DMARC-policy snapshots into the shared regression mechanism
- [ ] Angular v3 addition: per-asset posture timeline view (regressions and restorations, not just current state)
- [ ] **Exit criteria**: a known KEV-listed CVE against a fingerprinted service in inventory produces a `priority: critical` finding without manual intervention; a TLS/cipher downgrade or DMARC policy weakening confirmed across two consecutive scheduled checks produces a `regression` finding and fires an alert, while a single transient change does not

## Phase 4 — Repository secrets scanning
- [ ] Add `repository` asset type; `config.seedRepos[]` (operator-declared, public repos only — reject/flag any URL requiring auth)
- [ ] Build `repo-scan-worker` container: clone (bounded by `config.maxRepoCloneSizeMb`), run Gitleaks/TruffleHog across full history
- [ ] Incremental rescanning: persist `lastScannedSha` per repository, scan only new commits after first run
- [ ] Live verification: read-only calls to issuing providers (e.g. AWS STS `get-caller-identity`), gated entirely by `config.secretVerificationEnabled` (default true)
- [ ] Redaction: mask/hash raw secret values into `secretFindings.redactedRef` at ingestion; verify no raw value reaches storage, dashboard, or alert payloads
- [ ] Deterministic priority function: verified-active > unverified pattern match, by scope/provider
- [ ] Wire new/removed secret findings into the shared `posture-regression` mechanism (Phase 3)
- [ ] Angular v4: dashboard secret-verification toggle (writes `config.secretVerificationEnabled` live, no redeploy) and per-repository findings view
- [ ] **Exit criteria**: a seeded public test repo with a planted, revoked test credential in its history is scanned end-to-end and produces a redacted, correctly-prioritized finding; toggling verification off in the dashboard UI stops outbound provider calls on the next scheduled run without a code change

## Phase 5 — AI provider & triage layer
- [ ] Build the `ai-provider` client: single OpenAI-compatible implementation, config-driven (`config.aiProvider.baseUrl`/`.apiKey`/`.model`/`.timeoutMs`); no per-provider branching, no Anthropic-native adapter
- [ ] Timeout/retry policy; on failure, mark annotation unavailable for the cycle without blocking the deterministic pipeline
- [ ] Convex action: build grounded prompt (scan evidence + CVE/KEV/EPSS + asset metadata only) for findings above `triageThreshold`
- [ ] Store AI annotation as a separate field; verify it never mutates priority/severity/KEV/EPSS
- [ ] Surface AI summary + suggested remediation in dashboard
- [ ] Duplicate-flag logic: collapse display only, never delete underlying record
- [ ] **Exit criteria**: synthetic critical finding gets an AI annotation within one processing cycle against a BYOK cloud provider; annotation accuracy spot-checked against evidence; a simulated LLM timeout leaves priority/severity intact and does not fail the pipeline

## Phase 6 — Portability, deployment packaging & software-quality automation
- [ ] Audit codebase for any hardcoded organization- or instance-specific value; move all to `config`
- [ ] Write same-day redeployment runbook (new Convex project, new config, new secrets)
- [ ] Rehearse a real redeployment against a second (test) instance to validate the runbook
- [ ] Repo audit: confirm zero employer-identifying or instance-identifying strings outside gitignored/local config
- [ ] Build the guided first-run setup command (`./setup.sh` or equivalent): prompts for seed domains/repos, LLM API key **or** local-model choice, alert webhook, admin credential; validates Docker/Compose; brings the stack up; waits on health checks; prints the dashboard URL
- [ ] Add the optional, profile-gated `ollama` Compose service; guided setup enables it (and points `config.aiProvider.baseUrl` at it) only when the operator chooses the local-model path, and validates reachability instead of accepting an unreachable local-only address
- [ ] **Rehearsal**: on a clean machine, clone the repo and complete setup via the guided command alone — self-hosted Convex, dashboard, Ofelia-scheduled jobs, Angular dashboard, no manual file editing required
- [ ] Playwright e2e suite covering the dashboard's critical flows (review-queue approval, operational-toggle write, live-finding update) wired into `ci-cd-pipeline`
- [ ] Automated accessibility scan (e.g. axe-core via Playwright) wired into `ci-cd-pipeline`, gating on WCAG 2.1 AA
- [ ] Configure Renovate (or equivalent) with `minimumReleaseAge` cooldown (3-day floor, 14-day for direct runtime deps and anything with install scripts), covering npm packages, Convex, and all job-container base images
- [ ] Build the agentic triage job: scheduled CI workflow invoking an AI coding agent against cooldown-cleared dependency-update PRs, running the full test suite, classifying risk, and auto-merging only when both the deterministic gate and the agent's classification agree; everything else routes to human review with the agent's rationale attached
- [ ] Least-privilege scoping for the dependency-update and agentic-triage automation credentials (open/comment/approve/merge only, no push access beyond the tool's own proposed diff)
- [ ] Craft-extraction pass: capture design lessons back into `PKM/Craft/` separate from any running instance's data
- [ ] **Exit criteria**: a second instance stands up from the runbook alone (no code edits, under one business day); a third party can bring up the stack via the guided setup command alone; a seeded low-risk dependency-update PR clears cooldown, passes tests, is classified low-risk by the agent, and auto-merges with its rationale recorded; a seeded major-version or security-relevant-package update never auto-merges regardless of test results or agent classification

## Phase 7 — Stretch
- [ ] Historical trend charts (asset count over time, mean-time-to-detection/resolution)
- [ ] Ticketing integration (e.g. Zendesk) as an alternate/additional alert channel
- [ ] Attack-path chaining view (AI-suggested multi-finding chains)
- [ ] Multi-user RBAC on the dashboard
