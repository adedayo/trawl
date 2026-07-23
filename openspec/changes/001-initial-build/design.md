# Design: 001-initial-build

## Architecture

See `../../00-design-overview.md` for the full narrative and diagram. Summary of component responsibilities:

| Component | Runtime | Responsibility |
|---|---|---|
| `discovery-worker` | Docker Compose (job container, Ofelia-triggered) | CT log queries, `subfinder`/`amass`, ASN/WHOIS pivots → POSTs candidate assets to Convex |
| `scan-worker` | Docker Compose (job container, Ofelia-triggered) | `naabu`/`httpx`/`nuclei` (KEV-tagged templates first) against `active` in-scope assets only → POSTs raw + structured results to Convex |
| `repo-scan-worker` | Docker Compose (job container, Ofelia-triggered) | Clones declared public repos (bounded size/depth, incremental after first run), runs Gitleaks/TruffleHog across full history, optionally verifies candidates live (read-only) per `config.secretVerificationEnabled`, redacts values before POSTing findings |
| Convex backend | Self-hosted Convex (`get-convex/convex-backend`, SQLite) | Schema, ingestion HTTP actions, scheduled feed-pull functions, correlation/scoring actions, AI-triage action, alert-dispatch action, real-time queries |
| Angular app | nginx container in the compose stack | Dashboard, pending-asset review queue, auth gate |
| Job scheduler | Ofelia cron sidecar container | Triggers `discovery-worker`, `scan-worker`, and `repo-scan-worker` executions on cron |

**Why Convex can't run the scanners:** Convex actions execute in a sandboxed Node/V8 environment — no arbitrary process exec, no raw sockets. Any capability that shells out to a scanning binary must run in a real container runtime, hence Docker Compose job containers for discovery/scan and Convex for everything else (data, orchestration logic, API-only feed pulls, AI calls). See the `deployment-packaging` capability spec for the FSL-1.1 licensing caveat on self-hosted Convex.

**Email-authentication is cheaper still.** The `email-authentication` capability (SPF/DKIM/DMARC + BIMI/MTA-STS/TLS-RPT/CAA) needs neither a scanning binary nor a raw DNS socket — a DNS-over-HTTPS query is a plain HTTPS call, so it runs as a Convex scheduled action, in the same bucket as the KEV/NVD/EPSS feed-pull functions. No new job container, no new runtime.

**Repository-secrets needs a job container, same reason as scan-worker.** Cloning a git repository and running Gitleaks/TruffleHog is a real process exec against a real filesystem — this cannot run inside a Convex action's sandbox, so `repo-scan-worker` is a third job container alongside `discovery-worker`/`scan-worker`, triggered the same way. The one operation it performs beyond a normal scan job — live-verifying a candidate secret — is a read-only call to the credential's *issuing provider* (e.g., AWS, not the target repository's own infrastructure), gated entirely by `config.secretVerificationEnabled` and toggleable from the dashboard.

**AI provider (`ai-provider` capability): one OpenAI-compatible client, no per-provider branching.** `ai-triage`'s LLM calls go through a single OpenAI-compatible client (`config.aiProvider.baseUrl`/`.apiKey`/`.model`) — this shape is spoken natively by OpenAI, Azure OpenAI, Groq, OpenRouter, and self-hosted Ollama/vLLM/llama.cpp, so one client covers cloud BYOK and self-hosted alike. Anthropic's Claude API does **not** speak this shape (it's the native Messages API only, on every platform including Bedrock/Vertex/Foundry) — a Claude adapter is deliberately out of scope for this pass, not silently folded in. This repo's Docker Compose stack gets an optional, profile-gated `ollama` service so self-hosting is one guided-setup choice away; reachability depends on Convex and the model running in the same network (true when both are self-hosted together) — if Convex is hosted elsewhere, `aiProvider.baseUrl` must be a publicly-reachable endpoint, and guided setup validates that rather than letting it fail silently at triage time.

## Convex Schema (sketch)

```
assets:            { type (ip|domain|repository), value, source, confidence, status, firstSeen, lastSeen }
scans:             { assetId, jobRunId, rawOutputRef, startedAt, completedAt, partial }
findings:          { assetId, scanId, cpe, cveIds, kev, epss, cvss, priority, status, aiAnnotation? }
referenceKev:       { cveId, dateAdded, vendorProject, product, requiredAction }
referenceCve:       { cveId, cvss, cpeMatches, publishedAt }
referenceEpss:      { cveId, score, updatedAt }
emailAuthPosture:  { domainAssetId, spf: {...}, dkim: {...}, dmarc: {...}, bimi, mtaSts, tlsRpt, caa, priority, checkedAt }
secretFindings:    { repoAssetId, provider, verified, scopeGuess, filePath, commitSha, redactedRef, priority, status, lastScannedSha, firstSeen, lastSeen }
postureSnapshots:  { assetId, attribute, value, capturedAt }
regressions:       { assetId, attribute, previousValue, newValue, direction, category, status(provisional|confirmed|restored), firstObservedAt, confirmedAt }
alerts:            { targetId, targetType(asset|finding), category, sentAt, dedupKey }
config:            { instanceName, seedDomains[], seedCidrs[], seedRepos[], webhookUrl, staleAfterDays, triageThreshold, secretVerificationEnabled, maxRepoCloneSizeMb, aiProvider: { baseUrl, apiKey, model, timeoutMs } }
```

`emailAuthPosture` is kept separate from `findings` rather than shoehorned into the CVE/KEV/EPSS-shaped table — it has no CPE/CVE, and its priority is a pure function of DMARC policy/alignment/pct instead of KEV/EPSS/CVSS. It still feeds the same `alerting` capability on regression (see the `email-authentication` spec's scheduled re-check requirement).

`secretFindings` stores `redactedRef` (a masked/hashed reference), never the raw secret value — see the `repository-secrets` spec's redaction requirement. `lastScannedSha` per repository backs incremental rescanning. `verified` is only ever populated when `config.secretVerificationEnabled` is true; when false, findings are pattern-match only and capped at a lower priority tier.

`postureSnapshots` and `regressions` back the `posture-regression` capability: every check-producing capability writes dated, structured snapshots per (asset, attribute); the regression logic diffs consecutive snapshots for the same pair using a versioned better/worse ordering table (data, not code, same discipline as `config`), and only promotes a `provisional` regression to `confirmed` after a second consecutive observation. `email-authentication`'s DMARC-policy-regression scenario and `scanning`'s TLS/cipher/certificate/port-set tracking both write into this same pair of tables rather than each inventing their own diff logic.

`config` holds only this instance's values (see `portability-config` spec) — it is data, never code.

## Frontend Tech Stack & UX

- **Angular, latest stable at build time.** As of this writing that's Angular 22 (the "signal-first" era) — standalone components throughout, `signal()`/`computed()` for state, and the built-in `@if`/`@for`/`@switch` control-flow syntax exclusively (no `NgModule`, no `*ngIf`/`*ngFor`). No version is pinned in this document beyond that policy — the `ci-cd-pipeline` capability's dependency automation is what keeps the actual installed version current going forward.
- **Tailwind CSS v4 + spartan/ui** for styling and components. Verified current as of this writing: Tailwind is on its v4 line (v4.3.x — a from-scratch rewrite, not an incremental bump on v3: CSS-native config via `@property`/cascade layers, full builds up to 5x faster); spartan/ui is 1.0-stable (semantically versioned, 55+ components, CLI at v1.1.1) and **requires Tailwind v4**, so the two are locked together by the library itself, not an independent pairing to reconcile. spartan/ui is the closest Angular equivalent to shadcn/ui: unstyled, accessible "brain" primitives plus a copy-in "helm" component layer, built specifically for Tailwind v4 and Angular signals (signals-based, SSR-compatible, zoneless-ready — matching the Angular 22 signal-first direction rather than sitting awkwardly next to it). This is the mechanism behind the `dashboard` capability's "premium, no ad hoc styling" requirement: one shared token source, not per-component invention.
- **Vitest for unit tests, Playwright for integration/e2e.** Vitest is now Angular's own default test runner (Karma is deprecated/EOL); Playwright drives the dashboard's critical flows (review-queue approval, toggle changes, live-finding updates) end-to-end, including an automated accessibility scan per the `dashboard` capability's a11y requirement. Both run in `ci-cd-pipeline`, not just locally.
- **"Latest stable" as an ongoing policy, not a one-time choice.** Every layer of the stack (Convex, Node, TypeScript, the job-container base images, naabu/httpx/nuclei/subfinder/amass/gitleaks) tracks latest-stable continuously through `ci-cd-pipeline`'s dependency automation rather than being pinned once at initial build and left to drift — see that capability's spec for the cooldown and agentic-triage mechanics that make continuous updates safe rather than reckless.

## Software Quality & Supply-Chain Hygiene

See the `ci-cd-pipeline` capability spec for the full requirements; the design rationale:

- **Deterministic gate decides, agent narrates — same rule the rest of the engine already follows, applied to its own maintenance.** Every other capability keeps AI to narrative/annotation while a pure function sets the number a human trusts (priority, regression classification, secret-finding tiering). Dependency auto-merge follows the identical shape: tests-passing and cooldown-elapsed are the deterministic gate; the AI agent's changelog read is the narrative/recommendation layer; auto-merge requires both, and either one's absence routes to a human, never a forced merge on the agent's opinion alone.
- **The cooldown is not theoretical.** March 2026's compromise of `axios` (a stolen maintainer token pushing a malicious release of a package with ~100M weekly downloads) is exactly the failure mode a same-day auto-merge policy would have shipped straight into this repo. A minimum-release-age window (3–14 days, configurable, per Renovate's `minimumReleaseAge`/cooldown mechanism) buys the time for that kind of compromise to be caught and yanked upstream before it ever reaches Trawl.
- **The agent reviews Renovate's diff; it doesn't produce its own.** The agentic triage job's write access is scoped to commenting, labeling, approving, and merging a PR the dependency-update tool already opened — never to pushing additional changes of its own. This keeps the automation's blast radius equal to "what Renovate proposed and tests already validated," not "whatever the agent decided to change."

## Security & Authorization Design

- **Self-authorization first.** Before the first scan runs against any real target, a written scope-authorization document (seed CIDRs/domains, explicitly non-destructive) must exist — this is a project precondition, not a system feature, but `tasks.md` Phase 0 blocks on it.
- **Secrets** in Docker secrets or an untracked `.env` (LLM API key, optional Shodan/Censys key, alert webhook URL). Never committed.
- **Least-privilege job containers**: `discovery-worker`, `scan-worker`, and `repo-scan-worker` each run with no more access than they need (egress + the Convex ingestion endpoint only) — no shared "do everything" credential.
- **Defense-in-depth allowlisting**: the scan job independently validates every target against `config.seedDomains`/`seedCidrs` before scanning, regardless of what status the asset record claims (see `scanning` spec, Requirement: Scope enforcement); `repo-scan-worker` applies the same independent-revalidation discipline against `config.seedRepos`.
- **Own-SOC awareness**: scanner source IP(s) should be whitelisted in the operator's own EDR/SOC alerting (e.g. CrowdStrike) so the monitoring system doesn't trigger the operator's own incident response against itself.
- **Live-verification calls are read-only and provider-scoped**: when `secretVerificationEnabled` is true, `repo-scan-worker` calls only the credential's issuing provider (e.g., AWS STS, never a third party unrelated to the credential), using the least-privileged read-only verb each provider offers (e.g., `get-caller-identity`, never an action that could consume quota, modify state, or trigger billing).

## Cost Design

- Everything runs on infrastructure the self-hoster already owns (laptop, home server, or their own VM) — self-hosted Convex, Ofelia scheduling, and the nginx-served Angular build all run on whatever infrastructure the self-hoster already owns, with zero idle cloud cost by design.
- The one real variable cost is the LLM API — controlled by the `triageThreshold` config value so only priority ≥ threshold findings get annotated, not every raw scan result; a self-hoster using a self-hosted open-weight model instead of a cloud API pays nothing per call.
- Job container timeout caps keep any single run bounded, regardless of how the self-hoster's own infrastructure is billed.
- **`repo-scan-worker`'s cost variable is clone size, not call volume**: `config.maxRepoCloneSizeMb` bounds disk and job duration per repository; incremental rescanning after the first full-history scan keeps steady-state cost low regardless of repository count.

## Testing & Validation Strategy

(See `00-design-overview.md` for the full narrative; concretely, per phase:)

- **Unit tests (Vitest)**: priority-scoring function, CPE/CVE matcher, ingestion-dedup logic, `posture-regression`'s ordering tables, `repository-secrets`' priority mapping — pure functions, no infra dependency, run in CI on every change.
- **Convex function tests**: `convex-test` against a dev deployment for schema validators, the ingestion HTTP action, and scheduled functions.
- **Integration/e2e tests (Playwright)**: the dashboard's critical flows — review-queue approval, an operational toggle write (e.g., `secretVerificationEnabled`), and a finding appearing live without a page reload — plus an automated accessibility scan per the `dashboard` capability's WCAG 2.1 AA requirement.
- **Scan-job `--dry-run`**: resolves and prints the target list without sending a single packet — this is how allowlist-enforcement is verified in CI.
- **Required check**: an explicit test asserting `scan-worker` refuses any out-of-scope target, gating every deployment.
- **Staged rollout**: personal/sandbox domain → one non-critical subdomain → full seed list, in that order, before Phase 1 is considered "done."
- **Idempotency tests**: re-ingest identical scan output twice, assert no duplicate finding and no duplicate alert.
- **Synthetic alert injection**: manually insert a `priority: critical` test finding, confirm exactly one alert fires through the configured channel.
- **All of the above are required CI checks** (see `ci-cd-pipeline` capability), not scripts a contributor has to remember to run locally.

## Architectural Best Practices Applied

- Idempotent ingestion keyed on stable asset/finding identity (not insert-only).
- Immutable, append-only status-transition history on findings (audit trail, no overwrites).
- Human-in-the-loop gate on all medium/low-confidence asset promotion (no silent scope growth).
- Config/secrets fully externalized from day one, not retrofitted in Phase 6 (Phase 6 validates it end-to-end via a real redeployment rehearsal, it doesn't introduce it).
- Raw secret values are never persisted or transmitted — `secretFindings.redactedRef` is written at ingestion time, before the record ever reaches a database index, a dashboard render, or an alert payload.

## Open Questions / Risks

- Shodan/Censys API costs if used for discovery pivots — treat as an optional Phase 2 enhancement, not a Phase 1 dependency, so the MVP has no paid OSINT API in its critical path.
- LLM cost drift if `triageThreshold` is set too low — monitor spend against whatever budget alert mechanism the self-hoster's chosen LLM provider offers.
- CT log query volume/rate limits on free public sources — build in backoff, since this runs daily, not continuously.
- Very large monorepos could still be expensive within `maxRepoCloneSizeMb` — a shallow-but-wide history (many commits, small blobs) can take longer to walk than to store; monitor job duration, not just clone size, and consider a separate time-based cap if this proves to be the binding constraint in practice.
- Disabling `secretVerificationEnabled` trades a real cost (more false positives, lower-priority-capped findings) for a real benefit (zero outbound calls to credential providers) — this is an explicit operator choice, not a default the engine should second-guess or override.
- The agentic triage job itself is an LLM API cost and a trust surface — bound its run frequency (e.g., once per day against the queue of cooldown-cleared PRs, not per-commit) and revisit the least-privilege scoping if a future credential-leak class of supply-chain attack specifically targets CI bot tokens (a documented 2026 concern in the wider ecosystem, not unique to this project).
- "Latest stable" language in this document (Angular 22, Vitest, etc.) will be stale by the time Phase 0 actually starts — treat those as illustrations of the policy, not the version to hardcode; `ci-cd-pipeline`'s automation is what keeps the real number current.
