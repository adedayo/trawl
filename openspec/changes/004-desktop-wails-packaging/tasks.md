# Tasks: 004-desktop-wails-packaging

**Ledger reconciled 2026-08-27.** This file recorded nothing as done while most
of it had shipped, which is the one failure mode a project arguing for *say what
you expect, then record what happened* cannot afford. Where a task landed under
a later change, that change is named rather than the task being ticked silently
— a task marked done in the wrong place is only a quieter kind of wrong.

## Phase 1 — Wails Core Setup
- [x] Initialize `wails.json` with frontend build path (`app/dist/app/browser`) and product metadata (`Trawl`).
- [x] Build Wails entrypoint `main.go` and application bridge `app.go`.
- [x] Add app branding assets (`build/appicon.png`, `.icns`, `.ico`). *(`build/` holds the PNG and SVG masters plus the `darwin/` and `windows/` inputs; Wails derives `.icns`/`.ico` at build time — see `build/README.md`.)*

## Phase 2 — Native Go Scanner Imports
- [x] Add `github.com/adedayo/checkmate` to `go.mod` and build secret scan runner in `pkg/scanner/secrets.go`. *(pinned `v1.3.3`)*
- [x] Add `subfinder` Go runner under `pkg/scanner/`. *(`pkg/scanner/network.go`, embedded as a library)*
- [ ] **Withdrawn: `naabu`, `httpx` and `nuclei` runners.** Not deferred — reconsidered. Each is a template- or probe-driven engine whose blast radius is a property of content that updates independently of this repository, which is exactly the shape Change 006 Phase 7 now refuses: a dependency that can widen what Trawl contacts without an operator having agreed. Port and service assessment remain in scope for the project, but they need the declared-egress treatment vantage's checks have, and that is a change of its own rather than three imports here.
- [x] Enforce non-destructive scope checks in Go runner logic. *(Strengthened past what this task asked: Change 006 Phase 3 moved enforcement into the transport, so an out-of-scope name cannot produce a packet rather than merely not being requested.)*

## Phase 3 — Angular 21+ Frontend Upgrade & Wails IPC
- [x] Upgrade `app/` dependencies to Angular 21+ (standalone components, Signals, zoneless detection). *(now on Angular 22)*
- [x] Build Wails IPC frontend service mapping Wails Go bindings to Angular Signals stores. *(`app/wailsjs/`)*
- [x] Verify UI responsive layouts with Tailwind CSS v4 and spartan/ui components.

## Phase 4 — Cross-Platform Desktop Packaging & CI
- [x] Configure `wails build` for macOS, Windows, and Linux. *(delivered by Change 013; `.github/workflows/release.yml` builds darwin/universal, windows NSIS and linux)*
- [x] Build GitHub Actions release matrix generating `.dmg`, `.exe` installer, and `.tar.gz` packages. *(delivered by Change 013, which also added the `.deb`/`.rpm`/AppImage paths and the checksum and cosign evidence this task did not anticipate)*

## Remaining

Nothing, other than the withdrawn item above. This change is ready to archive:
merge its spec deltas into `openspec/specs/` and move the folder to
`openspec/archive/`, carrying the port- and service-assessment question forward
as a new proposal rather than leaving it as an unticked box here.
