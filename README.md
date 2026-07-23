# Trawl

Continuous external attack surface monitoring **and** continuous external control validation — discover internet-facing assets through passive OSINT, scan them non-destructively, correlate findings against CISA KEV / NVD / EPSS, check email-authentication posture, scan public repositories for exposed secrets, detect when any tracked attribute quietly regresses between checks, use AI strictly as a triage/annotation layer, and alert before an attacker's window opens.

Discovery answers *"what's out there."*
Posture-regression detection answers *"did something that used to be fine stop being fine."*

## What This Does

| Capability | How |
|---|---|
| **Asset discovery** | Passive OSINT — CT logs, subfinder, amass, ASN/WHOIS pivots. No port scanning until an asset is approved. |
| **Non-destructive scanning** | naabu + httpx + nuclei (KEV-tagged templates first), with defense-in-depth allowlist enforcement. |
| **Vulnerability correlation** | Deterministic CPE/CVE matching against CISA KEV, NVD, and EPSS — KEV is the highest-signal feed. |
| **Email-auth posture** | SPF / DKIM / DMARC (+ BIMI / MTA-STS / TLS-RPT / CAA) via DNS-over-HTTPS. No scanning binary needed. |
| **Repository secret scanning** | Full git-history scanning of operator-declared public repos with Gitleaks/TruffleHog. Optional live verification. |
| **Posture regression** | Shared snapshot/diff mechanism catches TLS downgrades, DMARC weakening, new open ports, re-exposed secrets — anything that gets worse between checks. |
| **AI triage** | LLM-generated plain-language summary and remediation guidance. Advisory only — never sets priority scores. |
| **Fast alerting** | Slack/webhook with dedup and category routing. |

## Architecture

```
                     ┌─────────────────────────┐
   Ofelia (cron)  ──▶│  discovery-worker        │──▶ POST candidates
                     │  (CT logs, subfinder,    │
                     │   ASN/WHOIS pivots)      │
                     └─────────────────────────┘
                                                        │
                     ┌─────────────────────────┐        ▼
   Ofelia (cron)  ──▶│  scan-worker             │   ┌──────────────────────────┐
                     │  (naabu/httpx/nuclei,    │──▶│    Convex (self-hosted)   │
                     │   KEV-tagged templates)  │   │  assets · scans           │
                     └─────────────────────────┘   │  findings · reference     │
                                                     │  (KEV/NVD/EPSS) · alerts  │
   Ofelia (cron)  ──▶┌─────────────────────────┐   │  HTTP actions (ingest)    │
                     │  repo-scan-worker        │──▶│  scheduled fns (feeds)    │
                     │  (gitleaks/trufflehog)   │   │  actions (correlation,    │
                     └─────────────────────────┘   │   AI triage, alerting)    │
                                                     └──────────────┬───────────┘
                                                                    │ real-time
                                                                    ▼ subscription
                                                     ┌──────────────────────────┐
                                                     │   Angular dashboard      │
                                                     │   (nginx container)      │
                                                     └──────────────────────────┘
```

**Self-hosted, single-command deployment** — Convex backend, job scheduling, and the dashboard all run in Docker Compose. No cloud account required.

## Tech Stack

| Layer | Choice |
|---|---|
| Backend / data | [Convex](https://convex.dev) (self-hosted via `get-convex/convex-backend`) — real-time subscriptions, scheduled functions, HTTP actions |
| Scanning | Container-based — naabu, httpx, nuclei, subfinder, amass, gitleaks |
| Frontend | Angular (latest stable, signals, standalone, `@if`/`@for`), Tailwind CSS v4 + spartan/ui |
| Testing | Vitest (unit), Playwright (e2e + accessibility) |
| Vuln intel | CISA KEV, NVD, EPSS — all free, public |
| AI | OpenAI-compatible client — BYOK cloud or local (Ollama/vLLM/llama.cpp) |
| Job scheduling | Ofelia cron sidecar |
| Dependency hygiene | Renovate with release-age cooldown + agentic triage |

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) and Docker Compose v2+
- An LLM API key (any OpenAI-compatible provider) **or** a local model (Ollama, etc.)

### Setup

```bash
git clone https://github.com/adedayo/trawl.git
cd trawl
./setup.sh
```

The guided setup collects your seed domains, LLM API key (or local-model choice), and alert webhook, then brings the full stack up and prints your dashboard URL.

**Advanced users** can skip the guided setup:

```bash
cp .env.example .env
# Edit .env with your values
docker compose -f deploy/compose/docker-compose.yml up -d
```

## Project Structure

```
trawl/
├── app/                    # Angular dashboard
├── convex/                 # Convex schema, functions, actions
├── jobs/
│   ├── discovery-worker/   # OSINT asset discovery
│   ├── scan-worker/        # Port/service/vuln scanning
│   └── repo-scan-worker/   # Public repo secret scanning
├── deploy/
│   └── compose/            # Docker Compose stack
│       ├── docker-compose.yml
│       ├── ofelia.conf     # Cron schedule
│       ├── nginx.conf      # Serves Angular build
│       └── setup.sh        # Guided first-run setup
├── config/
│   └── example.json        # Config template (no real values)
└── openspec/               # Spec-driven design docs
```

## Design Principles

- **Non-destructive by default** — enforced in code (allowlist checks in the scan job itself), not just policy.
- **Deterministic scoring, AI for narrative only** — priority/severity is a pure function of KEV/EPSS/CVSS/exposure; the LLM annotates, it never sets the number.
- **Getting worse is as alertable as being new** — every capability writes comparable, dated snapshots into one shared regression mechanism.
- **Config is the only thing that changes between deployments** — zero org-specific data in the engine.
- **Cost scales with activity, not time** — scheduled batch jobs, not always-on services.

## Non-Goals

- **Not an exploitation tool.** No active exploitation, credential brute-forcing, or DoS-capable technique.
- **Not internal vulnerability management.** External, unauthenticated attacker's view only.
- **Not a SIEM.** Produces findings, not a log pipeline.
- **Not fully autonomous.** Medium/low-confidence discovered assets require human approval before scanning.

## Development

```bash
# Install dependencies
npm install

# Run the Angular dev server
cd app && npm run dev

# Run unit tests
npm test

# Run e2e tests
npx playwright test
```

See `openspec/` for the full spec-driven design — capability specs, implementation phases, and architectural decisions.

## License

This project is licensed under the [Apache License 2.0](LICENSE).

### Dependency License Notice

> [!IMPORTANT]
> The self-hosted Convex backend (`get-convex/convex-backend`) is licensed under **FSL-1.1** (Functional Source License), which converts to Apache-2.0 two years after each release. FSL-1.1 is source-available and free to self-host and modify, but it is **not** an OSI-approved open-source license. This is disclosed here so adopters can make an informed decision about the full dependency tree's licensing terms.
>
> All other dependencies use standard open-source licenses. See [NOTICE](NOTICE) for details.

## Contributing

Contributions are welcome. Please read the spec under `openspec/` before proposing changes — this project uses a spec-driven development workflow where capability requirements are defined before implementation.
