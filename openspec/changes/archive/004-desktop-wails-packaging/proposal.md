# Change: 004-desktop-wails-packaging

## Why

Currently, running Trawl requires Docker Compose, container builds, and terminal setup. This restricts usage to technical operators with container environments. Building a cross-platform desktop application using Wails v2/v3 enables any user, security lead, or analyst to install and run Trawl on their native operating system (macOS, Windows, Linux) via a single executable or installer.

## What Changes

This change adds cross-platform desktop application packaging and native Go scanner integration:

- `wails-runtime` — Wails v2/v3 app configuration (`wails.json`, `main.go`, `app.go`) embedding Angular 21+ static assets (`app/dist/app/browser`).
- `native-checkmate-scanner` — Direct Go import of `github.com/adedayo/checkmate` for secret exposure scanning and cross-project credential reuse correlation.
- `native-projectdiscovery-scanners` — Direct Go package integration of `subfinder`, `naabu`, `httpx`, and `nuclei` runners into the Go application binary.
- `angular-21-signals-ui` — Upgraded Angular SPA using Angular 21+ Signals (`signal()`, `computed()`), experimental zoneless change detection (`provideExperimentalZonelessChangeDetection()`), block syntax (`@if`, `@for`, `@switch`), and spartan/ui + Tailwind CSS v4 styling.
- `desktop-release-packaging` — Cross-platform CI release pipeline producing `.dmg` (macOS Apple Silicon & Intel), native Windows installer (`.exe`), and Linux binaries (`.tar.gz`).

## Impact

- **Single Binary**: Users download a single self-contained application binary with zero external dependencies.
- **UX**: Unlocks seamless desktop installation and native OS windowing.

## Explicitly Out of Scope

- Shelling out to external CLI binaries or containerized tools in Desktop mode.
