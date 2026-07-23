# Change: 001-initial-build

## Why

Manual or periodic external attack surface review misses the gap between "an asset becomes exposed / a CVE goes KEV" and "we notice." That gap is exactly the attacker's window of opportunity. The same review cadence also misses a quieter gap: a control that was fine at the last review can degrade before the next one, and nothing about a periodic check draws attention to a downgrade the way it would to something brand new. This change builds the whole system from zero: continuous OSINT-driven asset discovery, non-destructive scanning, deterministic vulnerability correlation against CISA KEV/NVD/EPSS, a shared posture-regression mechanism that catches degradation on any tracked attribute, AI-assisted triage, and fast alerting — so the operator finds out before a threat actor exploits it, not after, and knows when something that used to be fine quietly stopped being fine.

## What Changes

This is a 0→1 build. Nothing exists yet, so this change proposes adding all fourteen initial capabilities as net-new:

- `asset-inventory` — canonical asset record with lifecycle state
- `asset-discovery` — passive OSINT expansion (CT logs, passive DNS, ASN/WHOIS)
- `scanning` — non-destructive port/service/TLS/version scanning
- `vulnerability-correlation` — deterministic CVE/KEV/EPSS matching and priority scoring
- `email-authentication` — deterministic SPF/DKIM/DMARC (+ BIMI/MTA-STS/TLS-RPT/CAA) posture checks via DNS-over-HTTPS, no job container required
- `posture-regression` — shared snapshot/diff/classify/alert mechanism detecting when any tracked external-posture attribute (TLS/cipher, certificate strength, DMARC policy, attack-surface size, externally-discoverable secret exposure) degrades between checks — continuous external control validation, not just discovery
- `repository-secrets` — full-history secret scanning of operator-declared public repositories, with operator-toggleable live verification against issuing providers
- `ai-provider` — single OpenAI-compatible client (BYOK cloud providers + self-hosted Ollama/vLLM/llama.cpp), no distinct Anthropic-native adapter for now
- `ai-triage` — LLM-generated summary/remediation annotation layer
- `alerting` — Slack/webhook notification with dedup and category routing
- `dashboard` — Angular real-time dashboard (latest-stable Angular, signals, `@if`/`@for`, Tailwind + spartan/ui), asset-approval queue, and operational-toggle controls
- `portability-config` — enforced separation of engine code from instance-specific data
- `deployment-packaging` — single-command, self-hostable Docker Compose packaging (self-hosted Convex) with config-only portability, plus a guided first-run setup command for turnkey self-hosting
- `ci-cd-pipeline` — automated test/lint/type-check gate (Vitest + Playwright) on every change, plus dependency automation with a supply-chain cooldown and an AI-agent triage layer that recommends but never overrides the deterministic gate

## Impact

- **New infrastructure only.** A self-hosted Convex deployment (Docker Compose) and an Angular app — no cloud account required. No changes to any other existing infrastructure, tooling, or process.
- **New public repository.** The engine (`convex/`, `app/`, `jobs/`, Docker Compose deploy) is published as a standalone, Apache-2.0-licensed public repo — a thought-leadership/community artifact.
- **Read-only/passive relationship to scanned assets.** The system observes; it does not modify anything on the assets it scans.
- **Cost:** zero cloud cost by design — see `design.md` for the cost model.

## Explicitly Out of Scope

- Active exploitation, credential brute-forcing, or any DoS-capable technique — non-destructive detection only, enforced in code (see `scanning` capability).
- Internal/authenticated vulnerability management — this covers the external, unauthenticated attacker's view only.
- Private-repository secret scanning of any kind — no credential (PAT, SSH key, GitHub/GitLab App) is ever accepted or stored; `repository-secrets` only clones repositories reachable via unauthenticated, anonymous git access.
- Multi-user access control beyond a single-operator credential gate (deferred; the design doesn't preclude adding it later).
- Automatic full-scope scanning of anything discovered — medium/low-confidence assets always go through human review before promotion (see `asset-discovery`).

## Rollout

See `tasks.md` for the phased implementation order (CI/CD skeleton from Phase 0 onward, MVP scan+dashboard first, discovery automation second, vuln enrichment and posture-regression third, repository-secrets scanning fourth, AI triage fifth, portability packaging and full dependency automation sixth) and `design.md` for the technical architecture and validation strategy each phase must satisfy before moving to the next.
