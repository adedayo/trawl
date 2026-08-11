#!/usr/bin/env bash
set -euo pipefail

# Trawl — local test runner.
#
# This mirrors .github/workflows/ci.yml step for step, so that a green run here
# means a green run there. If you add a gate to CI, add it here too; a local
# check that is weaker than the remote one is worse than no local check,
# because it tells you that you are done when you are not.
#
# Usage:
#   ./test.sh            # everything except the Docker image build
#   ./test.sh --docker   # also build and smoke-test the server image
#   ./test.sh --desktop  # also build the Wails desktop application
#   ./test.sh --quick    # skip the production bundle build (the slow step)

WITH_DOCKER=false
WITH_DESKTOP=false
QUICK=false
for arg in "$@"; do
  case "${arg}" in
    --docker)  WITH_DOCKER=true ;;
    --desktop) WITH_DESKTOP=true ;;
    --quick)   QUICK=true ;;
    -h|--help)
      sed -n '3,15p' "$0" | sed 's/^# \{0,1\}//'
      exit 0
      ;;
    *) echo "Unknown option: ${arg}" >&2; exit 1 ;;
  esac
done

STEP=0
step() {
  STEP=$((STEP + 1))
  echo ""
  echo "──────────────────────────────────────────────────────────────"
  echo "  [${STEP}] $1"
  echo "──────────────────────────────────────────────────────────────"
}

echo "============================================================"
echo "          Trawl — Local Test Runner                        "
echo "============================================================"

# ─── Go engine ────────────────────────────────────────────────────────────────
# ./... is deliberately not used: the root package embeds the built dashboard
# via go:embed, so it cannot compile without a prior Angular build.

step "Go — formatting"
if [ -n "$(gofmt -l ./pkg ./cmd)" ]; then
  echo "The following files are not formatted. Run: gofmt -w ./pkg ./cmd" >&2
  gofmt -l ./pkg ./cmd >&2
  exit 1
fi
echo "✔ gofmt clean"

step "Go — vet"
go vet ./pkg/... ./cmd/...
echo "✔ vet clean"

step "Go — build"
go build ./pkg/... ./cmd/...
echo "✔ engine builds"

# -race is not optional here: the job queue is claimed concurrently by multiple
# workers, and its exclusivity test spawns goroutines to prove it.
step "Go — tests (race detector enabled)"
go test -race -count=1 ./pkg/... ./cmd/...

# ─── Frontend ─────────────────────────────────────────────────────────────────

step "TypeScript — typecheck"
npm run typecheck

step "Frontend — unit tests"
npm test

step "Dependency gate — classifier tests"
npm run test:classifier

# ─── Workers ──────────────────────────────────────────────────────────────────
# The scan worker is a polling loop in normal operation. --dry-run bounds it to
# a single pass; if that guard regresses, this step hangs, which is the
# intended signal.

step "Workers — dry-run validation"
chmod +x jobs/scan-worker/entrypoint.sh
SEED_DOMAINS="example.com" ./jobs/scan-worker/entrypoint.sh --dry-run > /dev/null
echo "✔ worker dry-runs terminate"

# ─── Deployment ───────────────────────────────────────────────────────────────

step "Compose — manifest validation"
docker compose -f deploy/compose/docker-compose.yml config -q
docker compose -f deploy/compose/docker-compose.dev.yml config -q
echo "✔ compose manifests valid"

# Packaging defects are invisible locally, invisible in CI, and discovered by a
# user on a platform nobody here owns, hours after a tag was pushed. The cheap
# subset of those checks is worth running on every commit rather than only at
# release time.
step "Packaging — manifest validation"
./scripts/validate-packaging.sh

if [ "${QUICK}" = false ]; then
  step "Angular — production bundle"
  npm run build > /dev/null
  echo "✔ production bundle builds"
else
  echo ""
  echo "  (skipping production bundle build — --quick)"
fi

if [ "${WITH_DOCKER}" = true ]; then
  step "Server image — build and smoke test"
  docker build -q -f deploy/compose/Dockerfile.server -t trawl-server:local . > /dev/null
  docker rm -f trawl-local > /dev/null 2>&1 || true
  docker run -d --name trawl-local -p 8099:8080 trawl-server:local > /dev/null
  trap 'docker rm -f trawl-local > /dev/null 2>&1 || true' EXIT

  HEALTHY=false
  for _ in $(seq 1 30); do
    if curl -sf http://127.0.0.1:8099/healthz > /dev/null; then HEALTHY=true; break; fi
    sleep 1
  done

  if [ "${HEALTHY}" = false ]; then
    echo "FAIL: server did not become healthy within 30s" >&2
    docker logs trawl-local >&2
    exit 1
  fi

  # Round-trip a job through the queue: enqueue, claim, then confirm the queue
  # reports empty rather than erroring.
  curl -sf -X POST http://127.0.0.1:8099/api/jobs \
    -H 'Content-Type: application/json' \
    -d '{"type":"scan","targets":["example.com"]}' > /dev/null
  CLAIMED=$(curl -sf "http://127.0.0.1:8099/api/jobs/pop?type=scan")
  echo "${CLAIMED}" | grep -q '"status":"running"' \
    || { echo "FAIL: claimed job was not marked running: ${CLAIMED}" >&2; exit 1; }
  EMPTY=$(curl -sf "http://127.0.0.1:8099/api/jobs/pop?type=scan")
  [ "${EMPTY}" = "{}" ] \
    || { echo "FAIL: drained queue returned '${EMPTY}', expected {}" >&2; exit 1; }

  echo "✔ server image healthy; job round-trips through the queue"
else
  echo ""
  echo "  (skipping server image build — pass --docker to include it)"
fi

if [ "${WITH_DESKTOP}" = true ]; then
  step "Wails — desktop application build"
  export PATH="${PATH}:$(go env GOPATH)/bin"
  if ! command -v wails > /dev/null; then
    echo "FAIL: wails CLI not found. Install with:" >&2
    echo "  go install github.com/wailsapp/wails/v2/cmd/wails@latest" >&2
    exit 1
  fi
  wails build > /dev/null
  echo "✔ desktop application builds"

  # Regenerated bindings that differ from the committed ones mean the frontend
  # is calling a backend surface that no longer matches. Catch it here rather
  # than as a runtime "method not found" in the packaged app.
  if ! git diff --quiet -- app/wailsjs; then
    echo "FAIL: Wails bindings in app/wailsjs are stale — commit the regenerated files:" >&2
    git diff --stat -- app/wailsjs >&2
    exit 1
  fi
  echo "✔ committed Wails bindings match the bound Go methods"
else
  echo ""
  echo "  (skipping desktop build — pass --desktop to include it)"
fi

echo ""
echo "============================================================"
echo "  ✔ ALL CHECKS PASSED — READY TO COMMIT"
echo "============================================================"
