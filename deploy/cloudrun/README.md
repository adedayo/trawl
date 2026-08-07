# Trawl on Cloud Run

Cloud Run scales instances up and down on demand. That is a different execution
model from the Compose deployment, and two of its properties change what is
correct rather than merely what is fast. Both are handled by the binary, but
one of them constrains how you must configure the service.

## Build and deploy

```sh
gcloud run deploy trawl \
  --source . \
  --dockerfile deploy/cloudrun/Dockerfile \
  --region europe-west2 \
  --max-instances 1 \
  --no-allow-unauthenticated \
  --set-env-vars TRAWL_AUTH_TOKEN=$(openssl rand -hex 32)
```

The image contains both the API and the dashboard, so there is no second
service, no proxy configuration and no possibility of the UI being a different
version from the API it calls.

## The two constraints

### 1. CPU is throttled between requests — scans run inline

Under Cloud Run's default CPU allocation, an instance is throttled to
near-zero CPU once it has written its final response byte. A scan started on a
goroutine and left to finish after a `202 Accepted` does not fail; it *stalls*,
until some unrelated request happens to wake the instance, or is destroyed when
the instance scales to zero. The caller has already been told the scan started.

The binary detects Cloud Run via `K_SERVICE` and switches `TRAWL_SCAN_MODE` to
`inline`: `POST /api/v1/scans` holds the request until the scan completes and
returns `200` with the outcome. Set `--timeout` above your longest expected
scan (the default 300s is usually enough for a single domain; raise it for
large portfolios).

The alternative, if you prefer the asynchronous behaviour, is to deploy with
`--no-cpu-throttling`, which keeps CPU allocated for the instance's lifetime,
and set `TRAWL_SCAN_MODE=background` explicitly. That costs more, because you
are billed for allocated CPU rather than for request time.

### 2. The filesystem is instance-local — pin to one instance, mount a volume

Trawl holds state in SQLite. On Cloud Run each instance gets a private
filesystem, and by default that filesystem is in-memory: it counts against the
instance's memory limit and is discarded when the instance is reclaimed.

Two instances therefore mean **two databases**, and a dashboard whose answer
depends on which instance served the request. Findings recorded by one are
invisible to the other. Scaling to zero discards whichever it was.

So a Cloud Run deployment of Trawl must either:

- **Pin to a single instance and mount durable storage**, which is the
  supported configuration today:

  ```sh
  gcloud run deploy trawl \
    --max-instances 1 \
    --add-volume name=trawl-data,type=cloud-storage,bucket=YOUR_BUCKET \
    --add-volume-mount volume=trawl-data,mount-path=/data
  ```

  Note that GCS FUSE is not a POSIX filesystem and SQLite's locking on it is
  not sound for concurrent writers — which is precisely why `--max-instances 1`
  is not optional here. For a stricter guarantee, mount Filestore (NFS)
  instead.

- **Or treat the deployment as ephemeral** — acceptable for CI-style one-shot
  assessments that read their result from the response, not from the database.

The server logs this constraint at startup whenever it detects an autoscaling
platform, so it is stated rather than discovered.

## What is not yet supported

Horizontal scaling. It needs a networked store behind `store.Store` — the
interface exists and nothing above it assumes SQLite, so this is a matter of
writing an implementation rather than of restructuring. Until then,
`--max-instances 1` is a correctness requirement and not a cost preference.

The in-memory event bus has the same boundary: with more than one instance,
a dashboard streaming events from instance A would not see work happening on
instance B. The frontend refreshes on scan completion regardless of the event
stream, so the UI stays correct — but "live" would become partial.

## Environment

| Variable            | Default                    | Notes |
| ------------------- | -------------------------- | ----- |
| `PORT`              | injected by Cloud Run      | Takes precedence over `TRAWL_LISTEN_ADDR`. |
| `TRAWL_DB_DSN`      | `/data/trawl.db`           | Selects the storage backend by scheme. A bare path or `sqlite:` means SQLite — point it at a mounted volume. |
| `TRAWL_DB_PATH`     | unset                      | Deprecated SQLite-only spelling, still honoured when `TRAWL_DB_DSN` is unset. |
| `TRAWL_AUTH_TOKEN`  | unset                      | Required in any deployment reachable beyond loopback. |
| `TRAWL_SCAN_MODE`   | `inline` on Cloud Run      | `background` only with `--no-cpu-throttling`. |

### Choosing a storage backend

`TRAWL_DB_DSN` is dispatched on its scheme by a registry in `pkg/store`. A
backend becomes available by being linked into the binary, where it registers
itself; nothing in the entrypoints names a concrete store. This is what allows
the SQLite constraint below to be lifted by configuration rather than by a code
change — an unrecognised scheme fails at startup rather than falling back to a
local file, because a process quietly holding a private, empty copy of the
estate would still pass its health check.
