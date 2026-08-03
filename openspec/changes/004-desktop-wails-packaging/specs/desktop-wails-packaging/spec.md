# Capability: desktop-wails-packaging

The `desktop-wails-packaging` capability delivers Trawl as a cross-platform desktop application using Wails v2/v3, integrating Go native scanner libraries and Angular 21+ frontend assets.

## Requirements

### Requirement: Cross-Platform Native Windowing
The desktop application MUST launch on macOS (Intel and Apple Silicon), Windows (x64/arm64), and Linux desktop environments without requiring Docker or terminal installations.

#### Requirements
- MUST embed production Angular 21+ static assets into the application binary.
- MUST provide native OS window frame controls, menus, and tray notifications.

### Requirement: Native Go Scanner Execution
The desktop application MUST execute asset discovery, vulnerability correlation, network fingerprinting, and secret exposure checks via compiled Go packages.

#### Requirements
- MUST import `github.com/adedayo/checkmate` for secret scanning and cross-repo secret reuse analysis.
- MUST import ProjectDiscovery Go module packages (`subfinder`, `naabu`, `httpx`, `nuclei`) for non-destructive network scanning.
- MUST NOT shell out to external system binaries or Docker CLI commands.

### Requirement: Angular 21+ Signals & Zoneless Reactivity
The desktop user interface MUST leverage modern Angular 21+ conventions.

#### Requirements
- MUST use Angular Signals (`signal()`, `computed()`) for UI state management.
- MUST use `provideExperimentalZonelessChangeDetection()` for change detection.
- MUST use structural control flow blocks (`@if`, `@for`, `@switch`).
