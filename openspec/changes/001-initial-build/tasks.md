# Tasks: 001-initial-build

> **Reconciled 30 August 2026 against the code, not against memory.**
>
> This ledger was written before Change 003 replaced the datastore, and it had
> drifted to reading 68 open / 0 done while most of the product was shipping.
> That is worse than untidy: this is the file someone opens to decide what to
> build next, and it would have sent them either to redo finished work or to
> conclude the record was fiction and stop reading it.
>
> Three outcomes are used, and the distinction matters:
>
> - **[x] Shipped** — the capability exists. Where it shipped in a different
>   shape than planned, the shape is named.
> - **[x] Superseded** — a later change made this unnecessary or wrong, and
>   that change is named. Ticked because it is a conclusion, not pending work.
> - **[ ] Open** — genuinely not built, and someone still intends to.
>
> Where the plan named a Convex function, a job container or an Ofelia
> schedule, the capability usually still exists — it now runs in-process in the
> Go engine. Those are marked shipped with the substitution stated, because
> "we do this differently now" and "we never did this" must not share a
> rendering.

## Phase 0 — Foundations
- [x] Self-authorization/scope document — `docs/scope-authorization.example.md`; the record is enforced fail-closed in `pkg/core/core.go`
- [x] Repo created, Apache-2.0, README, NOTICE. **Superseded in part:** the FSL-1.1 disclosure described the self-hosted Convex dependency, which Change 003 removed. The dependency no longer exists.
- [x] **Superseded (003):** Compose skeleton with a separate datastore service and an Ofelia cron sidecar. `deploy/compose/` brings up the engine and dashboard only — the store is embedded, so there is no datastore service, and scheduling moved in-process.
- [x] Angular CLI app under `app/`; worker skeleton under `jobs/scan-worker/`
- [x] `.env`/secrets convention
- [x] **Superseded (003):** Convex schema v1. The schema is `pkg/store/sqlite` with auto-migrations.
- [ ] **spartan/ui was specified and never installed.** Standalone components, signals, `@if`/`@for` and Tailwind all shipped; the component library did not. The item argued it must not be retrofitted later, and retrofitting is now exactly what adopting it would mean. Left open deliberately: this needs a decision recorded, not a tick.
- [x] CI gating every PR — `.github/workflows/ci.yml`, since extended with gofmt, `go vet`, `-race` tests, a retracted-dependency check and the spec-datastore guard

## Phase 1 — MVP: seed scan to dashboard
- [x] **Withdrawn (004):** `naabu` + `httpx` + `nuclei` runners. Reconsidered, not deferred — each is a template- or probe-driven engine whose blast radius updates independently of this repository, the shape Change 006 Phase 7 refuses. `jobs/scan-worker/` remains as the ingestion path for externally-run tools.
- [x] `--dry-run` and allowlist enforcement — exercised by `test.sh`
- [x] Ingest, dedup, write findings — **shipped as Change 005's `pkg/core/ingest.go`**, correlating payloads behind a scope filter. Raw payloads are written verbatim first, so a parser change cannot lose evidence already received.
- [x] TLS/cipher/certificate and port-set captured as structured comparable fields — via the vantage adapter (006) and `pkg/store/posture.go`
- [x] Unit tests for ingestion/dedup — `pkg/core/ingest_test.go`
- [x] Angular v1: read-only asset and findings views with explicit loading/empty/error states. The three-state requirement was only fully honoured in Change 014; before that, a failed load and an empty estate shared a rendering.
- [x] **Superseded (003/005):** Ofelia-driven daily execution. Scheduling belongs to the engine; the background cron runner is tracked in Change 005 and is still open there.
- [ ] Staged rollout: sandbox domain, then one non-critical subdomain, then the full seed list. Operational rather than code — but not done, and it is the step that catches a scope error while it is still cheap.

## Phase 2 — Discovery automation
- [x] Discovery — `pkg/service/discovery.go`, **in-process rather than a container**. CT-log sourcing is tracked in 006 Phase 8 and remains open there.
- [x] Confidence scoring, with a scope ceiling enforced independently of confidence — proposed domains authorise nothing until moved across deliberately
- [x] Diff against inventory and queue for review — `proposedDomains`/`dismissedDomains`, dismissals remembered so a reviewer is not asked weekly
- [x] Angular v2: review queue, asset detail and history
- [ ] **Alerting is not built.** No webhook delivery, no dedup, nothing fires on a new asset or a new critical finding. Everything discovered is discovered only by someone looking at the dashboard. For a tool whose value is noticing change, this is the largest genuine gap in this ledger.
- [x] Email authentication checks — `pkg/scanner/email.go`, in-process. To be routed through the vantage adapter and the interim path removed (006 Phase 9, gated on its gap analysis).
- [x] Angular v2: email-authentication posture panel per domain

## Phase 3 — Vulnerability intel enrichment
- [ ] **KEV/NVD/EPSS feed ingestion is not built.** `kev` and `epssScore` exist as fields and are populated from ingested payloads, so the dashboard can rank by them — but nothing pulls the catalogues. The values are only as fresh as the payload that carried them, and no asset is re-evaluated when CISA adds a CVE. The executive view's "Exploited in the wild" counter rests on this.
- [ ] CPE/CVE correlation as a unit-tested pure function
- [x] **Superseded (002 to 009):** the deterministic composite priority score. `pkg/core/ingest.go` deliberately leaves `Priority` unset rather than inventing a composite; ordering is supplied by Change 014's `rankFindings` (KEV, then EPSS, then severity, then id — a total order). A calibrated score arrives with 009.
- [ ] Recompute-on-feed-update job — blocked on feed ingestion above
- [x] Angular v3: severity-sorted findings with KEV badge and EPSS
- [x] `posture-regression`: snapshots, per-attribute better/worse ordering, two-consecutive-observation confirmation — `pkg/store/posture.go`
- [x] TLS/certificate/port-set and DMARC-policy snapshots wired into the shared regression mechanism
- [ ] Per-asset posture timeline view. The data exists; only the view is missing. Change 014 surfaces current regressions but not history.

## Phase 4 — Repository secrets scanning
- [x] `repository` asset type and `config.seedRepos[]`
- [x] Secret scanning across full history — **in-process via the checkmate SDK (`pkg/scanner/secrets.go`)**, not a separate container. One fewer container to schedule, sign and keep current.
- [ ] Incremental rescanning with a persisted `lastScannedSha`. Every run rescans full history: correct, and increasingly slow.
- [x] Live verification against issuing providers, with a `verified` flag persisted
- [x] Redaction — only `RedactedRef` (a checksum) is stored; no raw secret value reaches storage, dashboard or any payload
- [x] Verified-active ranked above unverified pattern match
- [ ] Secret findings wired into the posture-regression mechanism, so a newly appearing secret registers as a change and not only as a finding
- [ ] Dashboard toggle writing `secretVerificationEnabled` live. Verification is configuration-only; there is no way to stop outbound provider calls from the UI.

## Phase 5 — AI provider and triage layer
**None of this is built.** There is no AI provider client, no annotation and no
triage layer anywhere in `pkg/` or `app/`.

Worth stating plainly because Change 014's ledger records "AI annotation
labelled advisory and visually subordinate" as satisfied — true only in the
vacuous sense that there is no annotation to subordinate. The requirement binds
if one is added.

- [ ] `ai-provider` client: one OpenAI-compatible implementation, config-driven, no per-provider branching
- [ ] Timeout and retry; on failure the annotation is unavailable for the cycle and the deterministic pipeline is unaffected
- [ ] Grounded prompt built from evidence only, for findings above `triageThreshold`
- [ ] Annotation stored in a separate field, proven never to mutate priority, severity, KEV or EPSS
- [ ] Surface the summary and suggested remediation, labelled advisory
- [ ] Duplicate-flag logic collapses display only, never deletes the record

## Phase 6 — Portability, packaging and software-quality automation
- [x] No hardcoded organisation- or instance-specific values; all externalised to config
- [x] Guided first-run setup — `setup.sh`
- [x] Renovate with `minimumReleaseAge` cooldown (3 days, 14 for direct runtime dependencies)
- [x] Agentic dependency triage — `.github/workflows/dependency-triage.yml` with `.github/scripts/`; deterministic gate first, agent classification second, both required to agree before auto-merge
- [ ] Same-day redeployment runbook, and a rehearsal against a second instance. `docs/distribution.md` covers artefacts, not standing up a fresh instance.
- [ ] Repo audit confirming zero employer- or instance-identifying strings outside gitignored config
- [ ] **Reconsider alongside Phase 5:** the optional profile-gated `ollama` Compose service. Not built. A local-model service with no provider client to talk to would be scaffolding for nothing.
- [ ] Playwright e2e suite covering review-queue approval, operational-toggle write and live-finding update
- [ ] Accessibility scanning gating on WCAG 2.1 AA. **Partially shipped (014):** axe-core runs against the executive view in unit tests, but it is not wired across all views and **colour contrast is excluded**, because jsdom cannot compute a ratio. Completing this is the same task as the Playwright suite above.
- [ ] Least-privilege scoping for the automation credentials (open, comment, approve and merge only; no push beyond the tool's own diff)
- [x] **Out of scope:** craft-extraction pass into `PKM/Craft/`. A personal knowledge-management practice, not a deliverable of this repository.

---

## What this reconciliation surfaced

**Alerting (Phase 2) and feed ingestion (Phase 3) are the two gaps that most
affect what the tool claims.** Without alerting, nothing reaches an operator who
is not already looking. Without feed ingestion, the "Exploited in the wild"
counter reflects whatever a payload happened to carry rather than the current
KEV catalogue — a figure a CISO will read as current. The interface labels its
coverage honestly, but the underlying freshness gap is real and is not visible
from the dashboard.

**The remaining open items here are not a backlog for this change.** 001 is the
original build plan; several of its gaps belong to later changes that already
track them (005 for cron, 006 for CT logs and email routing, 009 for scoring).
What remains genuinely unclaimed is listed in `openspec/STATUS.md` under
*Unclaimed work* so it is visible somewhere other than a heading that reads
like history.
