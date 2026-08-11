# Developing Trawl

Everything an end user needs is in the [README](../README.md) and
[docs/distribution.md](distribution.md). This document is for people changing
the code.

---

## Architecture

```
                     ┌─────────────────────────┐   ┌──────────────────────────┐
                     │  in-process scanning:   │   │  trawl-server (Go)       │
                     │  subfinder (embedded)   │   │  SQLite (WAL) store:     │
                     │  vantage (CT, DNS,      │──▶│   assets · scans         │
                     │   email, delegation)    │   │   findings · reference   │
                     │  checkmate (secrets)    │   │   (KEV/NVD/EPSS)         │
                     └─────────────────────────┘   │  job queue (pop/complete)│
                                                   │  ingest endpoints        │
                                                   │  correlation · AI triage │
                                                   │  alerting                │
                                                   └──────────────┬───────────┘
                                                                  │ event bus
                                                                  ▼ (WebSocket)
                                                   ┌──────────────────────────┐
                                                   │   Angular dashboard      │
                                                   │   (nginx container)      │
                                                   └──────────────────────────┘
```

Scanning runs inside the engine rather than in sidecars. The Go ecosystem makes
that the cheaper arrangement: a capability linked as a library is available to
the desktop binary and the server equally, needs no job-queue round trip, and
cannot drift from the version of the engine that interprets its results.

Port, HTTP and vulnerability scanning are the exception, and are currently
absent. `naabu`, `httpx` and `nuclei` were carried by a worker container that
has been retired: it cost fifty minutes a build, almost all of it the cgo and
arm64 emulation cost of one tool, for a capability nothing was consuming.
Linking them into the engine is the intended replacement. Vantage is not that
replacement and is not going to be — its spec `012` requires every check to be
passive or minimally interactive, and a port scanner is neither.

The job queue and ingest endpoints (`GET /api/jobs/pop`,
`POST /api/ingest/*`, `POST /api/jobs/complete`) remain in the server. They
have no client at present; they are kept because in-process scanning still
records through the same ingest path, and because an out-of-process scanner is
the likely shape of any future capability that needs privileges the engine
should not hold.

The server is the only component holding state, and there is no separate
database service. That is a deliberate constraint rather than a simplification:
it means a worker can be killed at any point without corrupting anything, and
it means the entire deployment is one file to back up.

The same engine compiles into two shapes — a Wails desktop binary and a
headless `trawl server` — from one source tree. Anything that would only work
in one of them belongs behind an interface.

## Tech stack

| Layer | Choice |
|---|---|
| Engine | Go — single statically linked binary, no runtime dependency |
| Data | SQLite (WAL) via `modernc.org/sqlite` — pure Go, no cgo |
| Desktop | Wails v2, with the Angular dashboard embedded via `go:embed` |
| Real-time | In-process event bus, with Wails IPC and WebSocket adapters |
| Scanning | subfinder (embedded), checkmate for repository secrets |
| DNS / email / delegation | [vantage](https://github.com/adedayo/vantage) (BSD-3) embedded as a library |
| Frontend | Angular (latest stable, signals, standalone, `@if`/`@for`), Tailwind CSS v4 + spartan/ui |
| Testing | Go tests (engine), Vitest (unit), Playwright (e2e + accessibility) |
| Vuln intel | CISA KEV, NVD, EPSS — all free, public |
| AI | OpenAI-compatible client — BYOK cloud or local (Ollama/vLLM/llama.cpp) |
| Dependency hygiene | Renovate with release-age cooldown + agentic triage |

The pure-Go SQLite driver is load-bearing. It is what allows `CGO_ENABLED=0`,
which is what allows the static binary, the `scratch`-adjacent container images
and painless cross-compilation. Swapping in `mattn/go-sqlite3` for a
performance win would cost all three.

## Repository layout

```
trawl/
├── main.go, app.go         # Wails desktop binary
├── cmd/trawl/              # Headless server binary (`trawl server`)
├── pkg/
│   ├── version/            # Build identity, stamped at link time
│   ├── store/              # Store interface + SQLite implementation
│   ├── event/              # In-process event bus
│   ├── scanner/            # Scanning and assessment
│   └── service/            # Orchestration
├── app/                    # Angular dashboard
├── deploy/
│   └── compose/            # Docker Compose stack
├── packaging/              # Homebrew, nfpm, desktop entry
├── scripts/                # release.sh, validate-packaging.sh
├── build/                  # Icons, plists, NSIS assets
├── config/example.json     # Config template (no real values)
└── openspec/               # Spec-driven design docs + RISK-ARC reasoning
```

## Getting set up

```bash
git clone https://github.com/adedayo/trawl.git
cd trawl
npm install
(cd app && npm install)
```

Node version is pinned in `.nvmrc`; `nvm use` will pick it up.

### Desktop development

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.10.2
export PATH="$PATH:$(go env GOPATH)/bin"

wails doctor          # verify toolchain prerequisites
wails dev             # hot-reloading desktop window
wails build           # produces build/bin/Trawl.app (macOS)
```

`wails dev` runs `ng serve` behind the native window, so both Go and Angular
hot-reload. It also regenerates the TypeScript bindings in `app/wailsjs/` from
the methods bound in `main.go` — **commit those when you change `app.go`'s
public surface**, or the frontend will call a method the backend no longer
exposes and the failure will only appear at runtime.

### A build-order trap

`go build ./...` requires a prior `npm run build`. `main.go` embeds the
compiled dashboard with `go:embed`, so the engine cannot compile until
`app/dist/` exists. If you only want to check the Go side:

```bash
go build ./pkg/... ./cmd/...
```

That is also why CI builds the frontend before it builds anything in Go.

### Frontend only

```bash
cd app
npm run dev        # ng serve against a running trawl server
npm test           # Vitest
npx playwright test
```

## Local checks

```bash
./test.sh              # mirrors CI exactly
./test.sh --docker     # also build the server image and round-trip a job
./test.sh --quick      # skip the production bundle
```

`./test.sh` is the contract: if it passes locally and fails in CI, that is a
bug in `test.sh`, not an inconvenience to work around.

## Design principles

- **Non-destructive always; contact deliberate, never incidental** — these are
  two different constraints and conflating them will lead you astray.
  Non-destructive is absolute: no exploitation, no credential brute-forcing,
  nothing DoS-capable, ever. Contact, by contrast, is not something to
  minimise for its own sake — Trawl already opens connections during
  discovery, because reading a certificate means completing a TLS handshake
  with the host presenting it. Once an asset is operator-confirmed, active
  contact (port scanning, service and version detection) is legitimate and
  expected: you cannot give an owner assurance about a service you refuse to
  talk to. What is prohibited is *incidental* contact — a connection nobody
  asked for, that answers no question, or that cannot be traced back to a
  named check against an in-scope target. Every connection should be
  attributable to a check, a target and a reason. Open-source data is
  preferred where it answers the question as well, because it keeps contact
  proportionate, not because contact is itself a cost.
- **Scope enforcement lives in the code path, not the config** — allowlist
  checks sit inside the scan job itself, so an unapproved target cannot be
  reached even by a caller that skips the orchestrator. A scanning tool whose
  safety property lives in a README does not have that safety property. This
  is the invariant that makes active scanning safe to add, so anything new
  that opens a socket must go through it.
- **Deterministic scoring, AI for narrative only** — priority and severity are
  a pure function of KEV/EPSS/CVSS/exposure. The LLM annotates; it never sets
  the number. This keeps scores reproducible and auditable, and means an
  outage or a model change cannot silently reprioritise a queue.
- **Getting worse is as alertable as being new** — every capability writes
  comparable, dated snapshots into one shared regression mechanism, rather
  than each inventing its own notion of change.
- **Config is the only thing that changes between deployments** — zero
  org-specific data in the engine.
- **Cost scales with activity, not time** — scheduled batch jobs, not
  always-on services.
- **Integrity evidence is not optional; identity is** — see
  [RELEASING.md](../RELEASING.md).

## Specs before code

This project uses a spec-driven workflow. Capability requirements are written
and agreed before implementation, under `openspec/`:

- `openspec/project.md` — conventions that apply repository-wide
- `openspec/changes/<nnn>-<name>/` — one folder per change, containing
  `proposal.md`, `design.md`, `tasks.md` and the capability spec deltas
- `openspec/RISK-ARC.md` — the risk-and-architecture reasoning behind the
  scoring model

Read the relevant spec before proposing a change. If your change does not fit
an existing capability, propose a new one first — a pull request that
implements something no spec describes will be asked for the spec.

## Releasing

See [RELEASING.md](../RELEASING.md). In short: releases are cut only via
`./scripts/release.sh`, never by pushing a tag by hand.

## Contributing

Contributions are welcome. Please:

1. Read the relevant capability spec under `openspec/`.
2. Run `./test.sh` before opening a pull request.
3. Keep comments explaining *why*, not *what* — the code already says what.
