# Tasks: 004-desktop-wails-packaging

## Phase 1 — Wails Core Setup
- [ ] Initialize `wails.json` with frontend build path (`app/dist/app/browser`) and product metadata (`Trawl`).
- [ ] Build Wails entrypoint `main.go` and application bridge `app.go`.
- [ ] Add app branding assets (`build/appicon.png`, `.icns`, `.ico`).

## Phase 2 — Native Go Scanner Imports
- [ ] Add `github.com/adedayo/checkmate` to `go.mod` and build secret scan runner in `pkg/scanner/secrets.go`.
- [ ] Add `subfinder`, `naabu`, `httpx`, and `nuclei` Go runners under `pkg/scanner/`.
- [ ] Enforce non-destructive scope checks in Go runner logic.

## Phase 3 — Angular 21+ Frontend Upgrade & Wails IPC
- [ ] Upgrade `app/` dependencies to Angular 21+ (standalone components, Signals, zoneless detection).
- [ ] Build Wails IPC frontend service mapping Wails Go bindings to Angular Signals stores.
- [ ] Verify UI responsive layouts with Tailwind CSS v4 and spartan/ui components.

## Phase 4 — Cross-Platform Desktop Packaging & CI
- [ ] Configure `wails build` for macOS, Windows, and Linux.
- [ ] Build GitHub Actions release matrix generating `.dmg`, `.exe` installer, and `.tar.gz` packages.
