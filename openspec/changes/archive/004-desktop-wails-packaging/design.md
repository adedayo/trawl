# Design: 004-desktop-wails-packaging

## Desktop Architecture

```
+-------------------------------------------------------------------------+
|                      Trawl Desktop Application                          |
|                                                                         |
|  +-------------------------------------------------------------------+  |
|  |                 Angular 21+ Frontend (WebView)                    |  |
|  |   Signals | Zoneless | Block Syntax (@if) | Tailwind v4 | Spartan UI  |  |
|  +-------------------------------------------------------------------+  |
|                                    | Wails JS Bridge / IPC              |
|  +-------------------------------------------------------------------+  |
|  |                      Wails Go App (`app.go`)                      |  |
|  |                                                                   |  |
|  |  +-----------------------+  +----------------------------------+  |  |
|  |  |   SQLite Store        |  |     Native Go Scanner Engine     |  |  |
|  |  |   (`pkg/store/sqlite`)|  |  - `github.com/adedayo/checkmate` |  |  |
|  |  |                       |  |  - `subfinder` (Discovery)       |  |  |
|  |  |  Event Bus Proxy      |  |  - `naabu` / `httpx` (Network)   |  |  |
|  |  |  (runtime.EventsEmit) |  |  - `nuclei` (Vulnerability)      |  |  |
|  |  +-----------------------+  +----------------------------------+  |  |
|  +-------------------------------------------------------------------+  |
+-------------------------------------------------------------------------+
```

## Go Native Scanner Integration

### 1. Secret Scanning (`pkg/scanner/secrets.go`)
Import `github.com/adedayo/checkmate` directly:
```go
package scanner

import (
    "context"
    "github.com/adedayo/checkmate/pkg/scanner"
)

type SecretScanner struct {
    // Engine config & options
}

func (s *SecretScanner) ScanRepo(ctx context.Context, repoURL string) ([]scanner.Finding, error) {
    // Call CheckMate engine natively
}
```

### 2. Network & Vulnerability Scanning (`pkg/scanner/network.go`)
Import ProjectDiscovery Go runners (`subfinder`, `naabu`, `httpx`, `nuclei`) as Go modules inside `pkg/scanner/`.

## Wails JS Binding API (`app.go`)

The Wails App struct exposes type-safe methods to Angular:
- `GetAssets(filter string) ([]Asset, error)`
- `AddSeedTarget(target string) error`
- `TriggerScan(assetID string, scanType string) error`
- `GetFindings(filter string) ([]Finding, error)`
- `GetPostureRegressions() ([]Regression, error)`
- `SaveConfig(cfg AppConfig) error`

## Angular 21+ Frontend Architecture (`app/`)

- App config initializes Wails IPC service provider.
- State managed via Angular Signals (`signal()`, `computed()`, `resource()`).
- Components built standalone with `@if`, `@for`, `@switch` block controls.
