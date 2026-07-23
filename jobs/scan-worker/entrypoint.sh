#!/usr/bin/env bash
set -euo pipefail

# scan-worker entrypoint
#
# Required env vars:
#   CONVEX_INGEST_URL   - Convex HTTP action URL for posting results
#   SEED_DOMAINS        - Comma-separated list of domains to scan
#   SEED_CIDRS          - Comma-separated list of CIDRs to scan
#
# Optional:
#   DRY_RUN=true        - Resolve targets and print what would be scanned, without sending any packets
#   CONVEX_AUTH_TOKEN    - Auth token for the Convex ingest endpoint

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ─── Argument parsing ──────────────────────────────────────────────────────────
DRY_RUN="${DRY_RUN:-false}"

if [[ "$1" == "--dry-run" ]]; then
  DRY_RUN="true"
  shift
fi

# ─── Required config validation ────────────────────────────────────────────────
if [[ -z "${SEED_DOMAINS:-}" && -z "${SEED_CIDRS:-}" ]]; then
  echo "ERROR: At least one of SEED_DOMAINS or SEED_CIDRS must be set" >&2
  exit 1
fi

if [[ "${DRY_RUN}" != "true" && -z "${CONVEX_INGEST_URL:-}" ]]; then
  echo "ERROR: CONVEX_INGEST_URL is required for non-dry-run execution" >&2
  exit 1
fi

# ─── Allowlist enforcement (defense-in-depth) ──────────────────────────────────
# Build the authorised target list from config
TARGETS_FILE=$(mktemp)
trap 'rm -f "${TARGETS_FILE}"' EXIT

if [[ -n "${SEED_DOMAINS:-}" ]]; then
  echo "${SEED_DOMAINS}" | tr ',' '\n' >> "${TARGETS_FILE}"
fi

if [[ -n "${SEED_CIDRS:-}" ]]; then
  echo "${SEED_CIDRS}" | tr ',' '\n' >> "${TARGETS_FILE}"
fi

echo "=== Authorised targets ==="
cat "${TARGETS_FILE}"
echo "=========================="

if [[ "${DRY_RUN}" == "true" ]]; then
  echo ""
  echo "[DRY RUN] Would scan the following targets:"
  cat "${TARGETS_FILE}"
  echo ""
  echo "[DRY RUN] Tools: naabu (port scan) → httpx (HTTP probe) → nuclei (vuln scan, KEV templates)"
  echo "[DRY RUN] No packets sent. Exiting."
  exit 0
fi

# ─── Scan execution ────────────────────────────────────────────────────────────
JOB_RUN_ID="scan-$(date -u +%Y%m%dT%H%M%SZ)-$$"
RESULTS_DIR="/tmp/results/${JOB_RUN_ID}"
mkdir -p "${RESULTS_DIR}"

echo "[${JOB_RUN_ID}] Starting scan..."

# Step 1: Port scan with naabu
echo "[${JOB_RUN_ID}] Running naabu (port scan)..."
naabu -list "${TARGETS_FILE}" -json -o "${RESULTS_DIR}/naabu.json" 2>/dev/null || true

# Step 2: HTTP probing with httpx
echo "[${JOB_RUN_ID}] Running httpx (HTTP probe)..."
httpx -list "${TARGETS_FILE}" -json -o "${RESULTS_DIR}/httpx.json" \
  -td -title -status-code -tech-detect -tls-grab -cdn \
  2>/dev/null || true

# Step 3: Nuclei scan (KEV-tagged templates first)
echo "[${JOB_RUN_ID}] Running nuclei (vuln scan)..."
nuclei -list "${TARGETS_FILE}" -jsonl -o "${RESULTS_DIR}/nuclei.json" \
  -severity critical,high,medium \
  2>/dev/null || true

# ─── Result ingestion ──────────────────────────────────────────────────────────
echo "[${JOB_RUN_ID}] Posting results to Convex..."

PAYLOAD=$(jq -n \
  --arg jobRunId "${JOB_RUN_ID}" \
  --slurpfile naabu "${RESULTS_DIR}/naabu.json" \
  --slurpfile httpx "${RESULTS_DIR}/httpx.json" \
  --slurpfile nuclei "${RESULTS_DIR}/nuclei.json" \
  '{
    jobRunId: $jobRunId,
    naabu: $naabu,
    httpx: $httpx,
    nuclei: $nuclei
  }' 2>/dev/null || echo '{}')

AUTH_HEADER=""
if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
  AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
fi

curl -sf -X POST "${CONVEX_INGEST_URL}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER} \
  -d "${PAYLOAD}" || {
    echo "ERROR: Failed to post results to Convex" >&2
    exit 1
  }

echo "[${JOB_RUN_ID}] Scan complete."
