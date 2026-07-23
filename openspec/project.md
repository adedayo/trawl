# Trawl — Project Context

Continuous external attack surface monitoring and continuous external control validation: discover internet-facing assets via passive OSINT, scan them non-destructively, correlate against CISA KEV/NVD/EPSS, check email-authentication posture, scan operator-declared public repositories for exposed secrets (`repository-secrets`), detect when any tracked attribute regresses between checks (`posture-regression`), use AI for triage-only annotation, alert fast. Full narrative design: `../00-design-overview.md`.

This file is read by AI coding agents working in this repo (OpenSpec convention) so they don't need to re-derive project conventions each session.

## Tech Stack

- **Backend/data**: Convex (schema, mutations, queries, HTTP actions, scheduled functions, actions for outbound calls), self-hosted (`get-convex/convex-backend`, Docker Compose, SQLite) — the same open-source engine that also powers Convex Cloud, so managed hosting is a config choice if a self-hoster ever wants it, not a fork
- **Scan/discovery execution**: containers with naabu/httpx/nuclei/subfinder/amass/gitleaks(or trufflehog) — triggered by an Ofelia cron sidecar running `docker compose run`
- **Frontend**: Angular, latest stable at build time (standalone components, `signal()`/`computed()`, `@if`/`@for`/`@switch` control flow — no `NgModule`, no `*ngIf`/`*ngFor`), Tailwind CSS v4 + spartan/ui (1.0-stable, requires Tailwind v4, signals-based/zoneless-ready) for styling/components, Convex's Angular/JS client for real-time data — served via an nginx container in the compose stack
- **Hosting**: an nginx container in the Docker Compose stack; no separate server process needed, since Convex's client SDK handles data directly
- **Testing**: Vitest for unit tests (Angular's own default runner; Karma is deprecated), Playwright for integration/e2e and automated accessibility scanning — both required CI checks, not local-only scripts
- **Vuln/threat intel**: CISA KEV catalog, NVD CVE feed, EPSS scores — all free/public
- **AI**: LLM API called from a Convex action, for annotation only, through a single OpenAI-compatible client (`ai-provider` capability) — BYOK cloud providers or self-hosted Ollama/vLLM/llama.cpp, config-driven `baseUrl`/`apiKey`/`model`, no Anthropic-native adapter for now
- **Dependency hygiene**: Renovate (or equivalent) with a `minimumReleaseAge` cooldown, plus a scheduled agentic triage job (see `ci-cd-pipeline` capability) — "latest stable" is a continuously-enforced policy across this entire stack, not a version pinned once at initial build

## Non-Negotiable Guardrails

These are enforced in code, not just documented as policy:

1. **Non-destructive only.** No exploitation, credential brute-forcing, or DoS-capable technique, ever.
2. **Allowlist enforcement is defense-in-depth.** The scan-worker independently validates every target against the configured authorized scope before touching it — even if upstream data is wrong, the scan job itself refuses out-of-scope targets.
3. **Deterministic severity, AI narrative only.** Priority/severity/KEV/EPSS fields are computed by pure functions. AI output is stored in a separate annotation field and never overwrites them. The same rule governs dependency auto-merge: tests-passing + cooldown-elapsed is the deterministic gate, the AI triage agent narrates and recommends, and it never overrides the gate.
4. **No org-specific data in the engine.** Seed domains/CIDRs, webhook URLs, org name — all external configuration, never hardcoded or committed.

## Conventions

- Convex schema and functions live under `convex/`; validators on every table. Written once, run unchanged whether Convex is self-hosted or, if a self-hoster prefers, pointed at Convex Cloud.
- Job containers live under `jobs/<job-name>/` with their own Dockerfile; each supports a `--dry-run` flag. Each image is triggered by the Ofelia cron sidecar via `docker compose run` — job code has zero awareness of what scheduler invoked it.
- Angular app lives under `app/`. Standalone/signals/`@if`/`@for` only; Tailwind + spartan/ui for all styling — no ad hoc CSS, no `NgModule`.
- Deploy config lives under `deploy/compose/` (Docker Compose file, Ofelia schedule config, nginx config, the guided setup script).
- Tests: pure-logic unit tests (scoring, correlation, dedup) via Vitest require no infra and must exist before any feature is considered done; Convex function tests via `convex-test` against a dev deployment; Playwright covers dashboard integration/e2e flows plus automated accessibility scanning; the allowlist-enforcement test is a required check, not optional. All of the above are required CI checks, not scripts a contributor has to remember to run.
- Config: one `config/<instance-name>.json` (or Docker secrets) per deployment; nothing instance-specific elsewhere.
- Dependency updates: Renovate (or equivalent) with a `minimumReleaseAge` cooldown, plus a scheduled agentic triage job — see `ci-cd-pipeline` capability. Auto-merge requires the deterministic gate (tests pass, cooldown elapsed) and the agent's risk classification to agree; either one's absence routes to human review.

## Spec-Driven Workflow

- `openspec/specs/` — accepted, current capability specs (empty until the first change is archived — this is a 0→1 project, so everything starts as a proposed change).
- `openspec/changes/<id>/` — in-flight or proposed changes: `proposal.md` (why/what/scope), `design.md` (technical approach), `tasks.md` (implementation checklist), `specs/<capability>/spec.md` (delta requirements).
- On completing a change, archive it: merge its spec deltas into `openspec/specs/` and move the change folder to `openspec/archive/`.
- First change: `changes/001-initial-build/` — proposes all fourteen initial capabilities (asset-inventory, asset-discovery, scanning, vulnerability-correlation, email-authentication, posture-regression, repository-secrets, ai-provider, ai-triage, alerting, dashboard, portability-config, deployment-packaging, ci-cd-pipeline) since nothing exists yet.
