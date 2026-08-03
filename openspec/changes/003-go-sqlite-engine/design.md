# Design: 003-go-sqlite-engine

## Architecture & Data Flow

```
+-------------------------------------------------------------------------+
|                              Go App Process                             |
|                                                                         |
|  +-----------------------+     Wails IPC /     +---------------------+  |
|  |  Angular 21+ Frontend | <-----------------> |  Wails / HTTP API   |  |
|  |     (Signals UI)      |   WebSocket Stream  |      Handlers       |  |
|  +-----------------------+                     +---------------------+  |
|                                                           |             |
|                                                           v             |
|                                                +---------------------+  |
|                                                |   `store.Store`     |  |
|                                                |      Interface      |  |
|                                                +---------------------+  |
|                                                           |             |
|                                                           v             |
|                                                +---------------------+  |
|                                                |    `SQLiteStore`    |  |
|                                                | (`trawl.db` in WAL) |  |
|                                                +---------------------+  |
+-------------------------------------------------------------------------+
```

## Storage Interface Schema (`pkg/store/store.go`)

The `Store` interface exposes domain operations:
- `Assets`: `GetAssets`, `GetAssetByID`, `SaveAsset`, `UpdateAssetStatus`
- `Scans`: `CreateScan`, `UpdateScanProgress`, `CompleteScan`
- `Findings`: `GetFindings`, `SaveFinding`, `DeduplicateFinding`
- `SecretFindings`: `GetSecretFindings`, `SaveSecretFinding`
- `EmailPosture`: `GetEmailPosture`, `SaveEmailPosture`
- `PostureSnapshots`: `GetSnapshots`, `RecordSnapshot`, `ConfirmRegression`
- `Config`: `GetConfig`, `SaveConfig`

## SQLite Configuration (`pkg/store/sqlite/db.go`)

- **Driver**: `modernc.org/sqlite` (pure Go, CGO-free cross-compilation) or `github.com/mattn/go-sqlite3`.
- **Pragmas**:
  - `PRAGMA journal_mode=WAL;` (High concurrency background writes)
  - `PRAGMA busy_timeout=5000;` (Prevents lock contention)
  - `PRAGMA foreign_keys=ON;` (Data integrity)
- **Migrations**: Auto-applied schema migration scripts executing on engine initialization.

## Real-Time Event Dispatcher (`pkg/event/bus.go`)

Thread-safe pub-sub bus dispatching structured mutation events:
```go
type EventType string

const (
    EventAssetUpdated   EventType = "asset:updated"
    EventFindingNew     EventType = "finding:new"
    EventScanProgress   EventType = "scan:progress"
    EventRegressionNew  EventType = "regression:new"
)

type Event struct {
    Type      EventType   `json:"type"`
    Payload   interface{} `json:"payload"`
    Timestamp time.Time   `json:"timestamp"`
}
```

In Wails Desktop mode, `EventBus` proxies directly to `runtime.EventsEmit(ctx, string(event.Type), event.Payload)`.
In Server mode, `EventBus` proxies to WebSocket client connections.
