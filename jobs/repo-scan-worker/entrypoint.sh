#!/usr/bin/env bash
set -euo pipefail

# repo-scan-worker entrypoint
#
# Required env vars:
#   CONVEX_INGEST_URL           - Convex HTTP action URL for posting secret findings
#   SEED_REPOS                  - Comma-separated list of public git repo URLs
#
# Optional:
#   DRY_RUN=true                - Show what would be scanned without cloning or scanning
#   CONVEX_AUTH_TOKEN            - Auth token for the Convex ingest endpoint
#   SECRET_VERIFICATION_ENABLED  - "true" to attempt live verification of found secrets (default: false)
#   MAX_REPO_CLONE_SIZE_MB       - Maximum clone size per repo in MB (default: 500)

DRY_RUN="${DRY_RUN:-false}"

if [[ "$1" == "--dry-run" ]]; then
  DRY_RUN="true"
  shift
fi

if [[ -z "${SEED_REPOS:-}" ]]; then
  echo "ERROR: SEED_REPOS must be set" >&2
  exit 1
fi

if [[ "${DRY_RUN}" != "true" && -z "${CONVEX_INGEST_URL:-}" ]]; then
  echo "ERROR: CONVEX_INGEST_URL is required for non-dry-run execution" >&2
  exit 1
fi

MAX_CLONE_SIZE="${MAX_REPO_CLONE_SIZE_MB:-500}"
VERIFY="${SECRET_VERIFICATION_ENABLED:-false}"

# ─── Allowlist enforcement ──────────────────────────────────────────────────────
# Only scan repos explicitly declared in SEED_REPOS
REPOS_FILE=$(mktemp)
trap 'rm -f "${REPOS_FILE}"' EXIT
echo "${SEED_REPOS}" | tr ',' '\n' > "${REPOS_FILE}"

# Reject any repo URL that looks like it requires auth
while IFS= read -r repo_url; do
  if echo "${repo_url}" | grep -qE '(ssh://|git@|\.git.*@|token=|access_token=)'; then
    echo "ERROR: Repo URL appears to require authentication, which is not supported: ${repo_url}" >&2
    echo "       Only unauthenticated, publicly-reachable repositories are accepted." >&2
    exit 1
  fi
done < "${REPOS_FILE}"

echo "=== Declared public repositories ==="
cat "${REPOS_FILE}"
echo "====================================="

if [[ "${DRY_RUN}" == "true" ]]; then
  echo ""
  echo "[DRY RUN] Would scan the above repositories for secrets:"
  echo "[DRY RUN] Tool: gitleaks (full git history scan)"
  echo "[DRY RUN] Max clone size: ${MAX_CLONE_SIZE}MB"
  echo "[DRY RUN] Live verification: ${VERIFY}"
  echo "[DRY RUN] No repositories cloned. Exiting."
  exit 0
fi

JOB_RUN_ID="reposcan-$(date -u +%Y%m%dT%H%M%SZ)-$$"
RESULTS_DIR="/tmp/results/${JOB_RUN_ID}"
CLONE_DIR="/tmp/clones/${JOB_RUN_ID}"
mkdir -p "${RESULTS_DIR}" "${CLONE_DIR}"

echo "[${JOB_RUN_ID}] Starting repository secret scan..."

while IFS= read -r repo_url; do
  [[ -z "${repo_url}" ]] && continue

  REPO_NAME=$(basename "${repo_url}" .git)
  echo "[${JOB_RUN_ID}] Scanning ${REPO_NAME}..."

  # Clone with size limit
  CLONE_PATH="${CLONE_DIR}/${REPO_NAME}"
  git clone --no-checkout "${repo_url}" "${CLONE_PATH}" 2>/dev/null || {
    echo "WARNING: Failed to clone ${repo_url}, skipping" >&2
    continue
  }

  # Check clone size
  CLONE_SIZE_MB=$(du -sm "${CLONE_PATH}" | cut -f1)
  if [[ "${CLONE_SIZE_MB}" -gt "${MAX_CLONE_SIZE}" ]]; then
    echo "WARNING: ${REPO_NAME} exceeds max clone size (${CLONE_SIZE_MB}MB > ${MAX_CLONE_SIZE}MB), skipping" >&2
    rm -rf "${CLONE_PATH}"
    continue
  fi

  # Full checkout for gitleaks
  (cd "${CLONE_PATH}" && git checkout 2>/dev/null || true)

  # Run gitleaks against full history
  gitleaks detect --source="${CLONE_PATH}" --report-format=json \
    --report-path="${RESULTS_DIR}/${REPO_NAME}.json" \
    2>/dev/null || true

  # Clean up clone immediately
  rm -rf "${CLONE_PATH}"

done < "${REPOS_FILE}"

# ─── Result ingestion (with redaction) ─────────────────────────────────────────
echo "[${JOB_RUN_ID}] Redacting and posting findings to Convex..."

# Redact raw secret values before transmission
for report in "${RESULTS_DIR}"/*.json; do
  [[ -f "${report}" ]] || continue
  # Replace the "Secret" field value with a SHA-256 hash prefix
  jq 'map(if .Secret then .Secret = ("REDACTED:" + (.Secret | @base64 | .[0:16])) else . end)' \
    "${report}" > "${report}.redacted" 2>/dev/null && mv "${report}.redacted" "${report}"
done

PAYLOAD=$(jq -n \
  --arg jobRunId "${JOB_RUN_ID}" \
  --arg verified "${VERIFY}" \
  '{jobRunId: $jobRunId, verified: ($verified == "true"), findings: []}' 2>/dev/null || echo '{}')

# Merge all report files into the payload
for report in "${RESULTS_DIR}"/*.json; do
  [[ -f "${report}" ]] || continue
  REPO_NAME=$(basename "${report}" .json)
  PAYLOAD=$(echo "${PAYLOAD}" | jq --arg repo "${REPO_NAME}" --slurpfile findings "${report}" \
    '.findings += [{ repo: $repo, results: $findings[0] }]' 2>/dev/null || echo "${PAYLOAD}")
done

AUTH_HEADER=""
if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
  AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
fi

curl -sf -X POST "${CONVEX_INGEST_URL}" \
  -H "Content-Type: application/json" \
  ${AUTH_HEADER} \
  -d "${PAYLOAD}" || {
    echo "ERROR: Failed to post findings to Convex" >&2
    exit 1
  }

echo "[${JOB_RUN_ID}] Repository scan complete."
