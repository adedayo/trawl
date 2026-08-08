# Trawl — Project Context

Continuous external attack surface monitoring, continuous external control validation, and — from Change 006 onward — quantified risk decision support. Trawl discovers internet-facing assets from open-source intelligence and deliberate, in-scope checks, assesses them non-destructively, correlates against CISA KEV/NVD/EPSS, assesses email-authentication and DNS delegation posture, scans operator-declared public repositories for exposed secrets, detects when any tracked attribute regresses between checks, and turns that observed exposure into calibrated probability and priced expected loss.

This file is read by AI coding agents working in this repo (OpenSpec convention) so they don't need to re-derive project conventions each session.

**The reasoning behind the risk-quantification arc lives in `openspec/RISK-ARC.md`.** Several specs cite it by section (`See RISK-ARC §5b`) for rationale that would otherwise be asserted without justification. It is the source of truth for *why* those requirements exist and is maintained in this repo alongside the specs.

## Tech Stack

- **Engine**: Go. Single statically linked binary, no runtime dependency, no daemon to install. This property is load-bearing across Changes 003/004/005 and should not be traded away casually.
- **Data**: SQLite via `modernc.org/sqlite` (pure Go, no cgo), WAL mode. Store interface in `pkg/store/store.go`, implementation in `pkg/store/sqlite/`.
- **Desktop**: Wails v2 (`main.go`, `app.go`, `wails.json`). Angular assets are embedded into the binary with `go:embed`; the Go layer is exposed to the frontend over Wails IPC.
- **Server**: the same engine runs headless as `trawl server` (`cmd/trawl/`) for the continuous cloud mode, serving HTTP and broadcasting events over WebSocket.
- **Real-time**: in-process event bus (`pkg/event/bus.go`) with two adapters — Wails IPC for desktop, WebSocket for server. Feature code publishes to the bus and is unaware of which transport is attached.
- **Scanning**: `subfinder` (embedded as a library) for passive discovery, `adedayo/checkmate` for repository secret detection. Preference is for embedding Go libraries over shelling out to CLIs — it makes scope enforceable in the transport, keeps the single-binary property, and removes a class of subprocess failure modes.
- **DNS, email and delegation assessment**: `adedayo/vantage` (BSD-3, Go), embedded as a library. This is the single source of SPF/DKIM/DMARC/MTA-STS/TLS-RPT/BIMI, DNSSEC chain of trust, DANE, CAA, open-resolver and AXFR checks, wildcard and subdomain-takeover assessment, provider attribution and Certificate Transparency enumeration. Scope enforcement is pushed down into an injectable transport, so out-of-scope queries are impossible rather than merely disallowed, and its four-state coverage model (`ok` / `not_found` / `not_checked` / `check_failed`) is the coverage vocabulary the whole risk model depends on.
  - Change 006 **supersedes the DNS-lookup internals of the `email-authentication` capability**: the capability's requirements survive, its implementation is replaced. The interim hand-rolled `miekg/dns` lookups in `pkg/scanner/email.go` are to be removed as part of that change — do not extend them.
  - Vantage is a sibling project, so its API is shaped to Trawl's needs rather than wrapped around. Change 006 Phase 1 is upstream work in the vantage repo.
- **Frontend**: Angular, latest stable at build time — standalone components, `signal()`/`computed()`, `@if`/`@for`/`@switch` only. No `NgModule`, no `*ngIf`/`*ngFor`. Tailwind CSS v4 + spartan/ui for styling and components. Lives under `app/`.
- **Testing**: Go tests for engine logic; Vitest for frontend unit tests; Playwright for e2e and automated accessibility scanning. All are required CI checks, not local-only scripts.
- **Vuln/threat intel**: CISA KEV catalogue, NVD CVE feed, EPSS scores — all free and public.
- **AI**: a single OpenAI-compatible client. BYOK cloud providers or self-hosted Ollama/vLLM/llama.cpp; config-driven `baseUrl`/`apiKey`/`model`. Annotation and narrative only.
- **Node**: pinned in `.nvmrc`; both workflows read `node-version-file` so local and CI cannot drift. Dependency upgrades routinely raise the engine floor, which is how a security fix turns into a broken build.
- **Dependency hygiene**: Renovate with `minimumReleaseAge` cooldowns plus an agentic triage gate — see `renovate.json` and `.github/scripts/`. `./security-fix.sh` resolves known vulnerabilities locally.

## Non-Negotiable Guardrails

These are enforced in code, not just documented as policy.

1. **Non-destructive only.** No exploitation, credential brute-forcing, or DoS-capable technique, ever. Note what this does *not* say: no-contact is not an invariant, and neither is passivity. Trawl already opens connections during discovery — reading a certificate requires a TLS handshake with the host presenting it — and once scope is operator-confirmed, port scanning and service detection are legitimate due diligence on your own estate. The invariant is that contact is **deliberate**: every connection must be attributable to a named check, against an in-scope target, for a stated reason, and recorded. Incidental or opportunistic contact is the defect, not contact itself. Do not treat "sends fewer packets" as the safety property; the safety properties are invariants 1 and 2.
2. **Scope enforcement is defence-in-depth.** Every scan path independently validates targets against the configured authorised scope before touching them, even if upstream data is wrong. Where a library is embedded, scope enforcement belongs in the transport: the goal is "we cannot ask", not "we do not ask".
3. **Deterministic severity, AI narrative only.** Priority, severity, KEV and EPSS fields are computed by pure functions. AI output is stored in a separate annotation field and never overwrites them. The same asymmetry governs dependency auto-merge: tests-passing plus cooldown-elapsed is the deterministic gate; the triage agent may withhold auto-merge but can never grant it. An agent that can promote can be prompt-injected by a changelog; an agent that can only veto cannot.
4. **No org-specific data in the engine.** Seed domains, CIDRs, webhook URLs, org names — all external configuration, never hardcoded or committed.
5. **"We did not look" must never render as "it is fine".** Assessment coverage is four-state (`ok` / `not_found` / `not_checked` / `check_failed`) and travels with every aggregate. A check that never ran must never contribute the reassurance of a check that passed.
6. **No unlabelled number.** Every emitted probability or monetary figure carries an uncertainty interval, a calibration label and the coverage of its inputs. Rendering types are constructed so that a bare point estimate does not compile.
7. **Exposure is a prior, not an observation of attackers.** No scan-derived path may write a contact posterior. A scan tells you the door is unlocked; it does not tell you anyone tried the handle.

## Repository Layout

- `main.go`, `app.go`, `wails.json` — the Wails desktop binary; Angular assets embedded via `go:embed`.
- `cmd/trawl/` — the headless server binary (`trawl server`) for continuous cloud mode.
- `pkg/store/` — store interface and domain structs; `pkg/store/sqlite/` is the SQLite implementation.
- `pkg/event/` — the in-process event bus and its transport adapters.
- `pkg/scanner/`, `pkg/service/` — scanning and assessment logic. `pkg/scanner/email.go` is interim: its hand-rolled DNS lookups are superseded by vantage under Change 006 and should not be extended.
- `app/` — the Angular dashboard. Standalone/signals only; Tailwind + spartan/ui for all styling, no ad hoc CSS.
- `jobs/<job-name>/` — containerised workers, each with its own Dockerfile and a mandatory `--dry-run` flag. Job code has no awareness of what scheduler invoked it.
- `deploy/compose/` — Docker Compose files, Ofelia schedule config, nginx config, guided setup script. The stack is `trawl-server` (Go + SQLite, the sole ingest target and job broker), the nginx-served dashboard, Ofelia, and the worker containers. There is no separate database service.
- `config/` — one `config/<instance-name>.json` per deployment; nothing instance-specific lives elsewhere.
- `.github/scripts/` — the dependency gate: a pure classifier plus its triage runner, both unit-tested.
- `packaging/` — release packaging manifests: Homebrew cask and formula, Scoop, nfpm (`.deb`/`.rpm`) and the Linux desktop entry. Validated on every commit by `./scripts/validate-packaging.sh`.
- `scripts/` — `release.sh` (the only supported way to cut a release) and `validate-packaging.sh`.
- `build/` — Wails build inputs: icons, `Info.plist` templates, Windows version resource, manifest and NSIS installer. See `build/README.md` for what is committed and what Wails regenerates.
- `openspec/` — specs, changes and the RISK-ARC reasoning document.

## Conventions

- Go code is the engine. Prefer embedding Go libraries over invoking CLIs.
- **DNS, email and delegation assessment goes through vantage.** Do not add hand-rolled resolver calls for anything vantage covers. New assessment capability belongs upstream in vantage, behind an egress profile, rather than as a local DNS lookup here — that is what keeps scope enforceable at the transport and keeps new checks failing closed.
- Pure functions for anything probability-affecting. No probability-affecting constant may be embedded in engine code — parameters live in signed, versioned, source-cited model packs (Change 008).
- Tests: pure-logic unit tests (scoring, correlation, dedup, ranking) require no infrastructure and must exist before a feature is considered done. The scope-enforcement test and the dependency-gate tests are required checks, not optional.
- `./test.sh` runs the full local suite — typecheck, Go tests, frontend tests, worker dry-runs, packaging validation, production build.
- `./security-fix.sh` applies safe dependency fixes and reports what needs a decision. It refuses to run on a dirty tree.
- **Releases are cut with `./scripts/release.sh vX.Y.Z` and nothing else.** The script gates and tags; the workflows build and publish. Do not build or upload release artefacts by hand, and do not add publishing steps to the script — a release path with two entry points has a state nobody can reason about.
- **Every binary reports one version, from `pkg/version`.** Stamped at link time with `-X github.com/adedayo/trawl/pkg/version.Version`. Never add a second `var Version` to a `main` package; the desktop app, the server and the workers must not be able to disagree.
- **Release builds build the tree as committed.** No `go mod edit` during a build, no dependency resolved to `@main` or `@latest`, every base image and scanner tool pinned. This is not tidiness — an SBOM or provenance statement over a build that mutated its own dependency graph describes something that existed only during that CI run.
- **Integrity evidence is not optional; identity is.** Checksums and keyless cosign signatures need no secret and always run — they are the trust anchor, and they are stronger than a vendor certificate because they are publicly logged and bound to the release workflow. Apple and Authenticode signing are guarded on secrets. Trawl holds no Apple Developer Program membership and will not buy one, so macOS builds are ad-hoc signed and not notarised. Never publish an artefact labelled notarised when it is not, and never tell a user to disable Gatekeeper system-wide — per-artefact quarantine removal on a verified download is the remedy we document.

## Spec-Driven Workflow

- `openspec/specs/` — accepted, current capability specs (empty until the first change is archived; this remains a 0→1 project).
- `openspec/changes/<id>/` — proposed or in-flight changes: `proposal.md` (why/what/scope), `design.md` (technical approach), `tasks.md` (implementation checklist), `specs/<capability>/spec.md` (delta requirements).
- On completing a change, archive it: merge its spec deltas into `openspec/specs/` and move the change folder to `openspec/archive/`.
- Keep `tasks.md` current as work lands. A project whose central argument is *say what you expect, then record what happened* cannot run an unmaintained ledger.

### Change map

| Change | Capability | State |
|---|---|---|
| 001 | Initial build — fourteen capabilities | Partly superseded by 003/004/005 |
| 002 | Susceptibility scoring | **Superseded by 009.** Do not implement as written. |
| 003 | Go + SQLite engine | Phases 1–3 largely implemented |
| 004 | Wails desktop packaging | In progress |
| 005 | Cloud continuous EASM | In progress |
| 006 | Vantage integration — measured-state signals, coverage model | Proposed |
| 007 | Contact probability — the keystone; supplies P(contact) | Proposed |
| 008 | Risk model packs — versioned, signed, source-cited parameters | Proposed |
| 009 | Exploit probability engine — three layers, three evidence classes | Proposed |
| 010 | Scenario loss model — frequency, magnitude, FAIR overlay | Proposed |
| 011 | Control portfolio ROI, remediation queue, capacity model | Proposed |
| 012 | Executive workbench — three views over one ledger | Proposed |

Suggested build order: **006 Phase 1** (the vantage embedding API, upstream work and the long pole) and **008 precision primitives** in parallel, then **007**, then 009 → 010 → 011 → 012.
