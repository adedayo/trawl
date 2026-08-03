# Design: 005-cloud-continuous-easm

## Cloud Continuous Server Architecture

```
+-------------------------------------------------------------------------+
|                        Trawl Cloud Server (`trawl server`)              |
|                                                                         |
|  +-----------------------+     WebSockets /     +--------------------+  |
|  | Remote Desktop App /  | <------------------> |  HTTP / REST API   |  |
|  |   Web Browser (SPA)   |     Server Stream    |      Handlers      |  |
|  +-----------------------+                      +--------------------+  |
|                                                           |             |
|                                                           v             |
|  +-----------------------+                      +--------------------+  |
|  | Continuous Cron Engine| -------------------> |    `trawl.db`      |  |
|  |  (Ofelia / Go Cron)   |    Background Scans  |   (SQLite WAL)     |  |
|  +-----------------------+                      +--------------------+  |
|                                                           |             |
|                                                           v (Replication)
|                                                 +--------------------+  |
|                                                 |  Litestream / S3   |  |
|                                                 +--------------------+  |
+-------------------------------------------------------------------------+
```

## Server Subcommand & API Handlers (`cmd/trawl/server.go`)

- Command: `trawl server --config config.yaml --port 8080`
- REST Routes:
  - `GET /api/v1/assets`
  - `POST /api/v1/assets`
  - `GET /api/v1/findings`
  - `GET /api/v1/regressions`
  - `POST /api/v1/scans/trigger`
  - `GET /ws` (WebSocket connection for live event streaming)

## Litestream Replication Integration

Cloud deployments can run Litestream as a sidecar process or embedded Go library (`github.com/benbjohnson/litestream`):
```yaml
# litestream.yml example
dbs:
  - path: /data/trawl.db
    replicas:
      - type: s3
        bucket: my-trawl-backups
        path: trawl.db
```

## Remote Wails Desktop Client Mode

The Wails Desktop App includes a connection selector in settings:
- **Local Engine (Default)**: Embedded SQLite store + local Go runners.
- **Remote Server Mode**: Points API requests & WebSockets to `https://trawl.example.com`.
