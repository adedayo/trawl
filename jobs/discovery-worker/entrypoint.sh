#!/usr/bin/env bash
set -euo pipefail

# discovery-worker entrypoint
#
# Required env vars:
#   CONVEX_INGEST_URL   - Convex HTTP action URL for posting candidate assets
#   SEED_DOMAINS        - Comma-separated list of seed domains
#
# Optional:
#   DRY_RUN=true        - Show what would be discovered without posting results
#   CONVEX_AUTH_TOKEN    - Auth token for the Convex ingest endpoint

DRY_RUN="${DRY_RUN:-false}"

if [[ "$1" == "--dry-run" ]]; then
  DRY_RUN="true"
  shift
fi

if [[ -z "${SEED_DOMAINS:-}" ]]; then
  echo "ERROR: SEED_DOMAINS must be set" >&2
  exit 1
fi

if [[ "${DRY_RUN}" != "true" && -z "${CONVEX_INGEST_URL:-}" ]]; then
  echo "ERROR: CONVEX_INGEST_URL is required for non-dry-run execution" >&2
  exit 1
fi

DOMAINS_FILE=$(mktemp)
trap 'rm -f "${DOMAINS_FILE}"' EXIT
echo "${SEED_DOMAINS}" | tr ',' '\n' > "${DOMAINS_FILE}"

echo "=== Seed domains ==="
cat "${DOMAINS_FILE}"
echo "===================="

if [[ "${DRY_RUN}" == "true" ]]; then
  echo ""
  echo "[DRY RUN] Would run discovery against the above seed domains:"
  echo "[DRY RUN] Tools: subfinder (passive subdomain enum) → amass (OSINT) → CT log queries → ASN/WHOIS pivots"
  echo "[DRY RUN] No queries sent. Exiting."
  exit 0
fi

JOB_RUN_ID="discovery-$(date -u +%Y%m%dT%H%M%SZ)-$$"
RESULTS_DIR="/tmp/results/${JOB_RUN_ID}"
mkdir -p "${RESULTS_DIR}"

echo "[${JOB_RUN_ID}] Starting discovery..."

# Step 1: Subfinder passive subdomain enumeration
echo "[${JOB_RUN_ID}] Running subfinder..."
subfinder -dL "${DOMAINS_FILE}" -json -o "${RESULTS_DIR}/subfinder.json" 2>/dev/null || true

# Step 2: Amass passive enumeration
echo "[${JOB_RUN_ID}] Running amass..."
amass enum -passive -df "${DOMAINS_FILE}" -json "${RESULTS_DIR}/amass.json" 2>/dev/null || true

# Step 3: CT log queries (via crt.sh API)
echo "[${JOB_RUN_ID}] Querying CT logs..."
while IFS= read -r domain; do
  curl -sf "https://crt.sh/?q=%25.${domain}&output=json" \
    >> "${RESULTS_DIR}/ctlogs.json" 2>/dev/null || true
done < "${DOMAINS_FILE}"

# ─── Result ingestion ──────────────────────────────────────────────────────────
echo "[${JOB_RUN_ID}] Posting candidate assets to Convex..."

PAYLOAD=$(jq -n \
  --arg jobRunId "${JOB_RUN_ID}" \
  --slurpfile subfinder "${RESULTS_DIR}/subfinder.json" \
  --slurpfile amass "${RESULTS_DIR}/amass.json" \
  '{
    jobRunId: $jobRunId,
    subfinder: $subfinder,
    amass: $amass
  }' 2>/dev/null || echo '{}')

AUTH_HEADER=""
if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
  AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
fi

curl -sf -X POST "${CONVEX_INGEST_URL}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER} \
  -d "${PAYLOAD}" || {
    echo "ERROR: Failed to post candidates to Convex" >&2
    exit 1
  }

echo "[${JOB_RUN_ID}] Discovery complete."
