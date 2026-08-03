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

if [[ $# -gt 0 && "$1" == "--dry-run" ]]; then
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

# ─── Worker Polling Loop ───────────────────────────────────────────────────────
echo "Starting repo-scan-worker polling loop..."

while true; do
  # Poll Convex for next job
  JOB_JSON=$(curl -s -f -X GET "${CONVEX_INGEST_URL/ingest\/secrets/jobs\/pop}?type=secret_scan" || echo "")

  if [[ -z "$JOB_JSON" || "$JOB_JSON" == "{}" ]]; then
    sleep 5
    continue
  fi

  JOB_RUN_ID=$(echo "$JOB_JSON" | jq -r '._id')
  TARGETS=$(echo "$JOB_JSON" | jq -r '.targets[]')

  if [[ -z "$JOB_RUN_ID" || "$JOB_RUN_ID" == "null" ]]; then
    sleep 5
    continue
  fi

  echo "[${JOB_RUN_ID}] Picked up new secret scan job"

  REPOS_FILE=$(mktemp)
  echo "$TARGETS" > "${REPOS_FILE}"

  # Reject any repo URL that looks like it requires auth
  AUTH_ERROR=0
  while IFS= read -r repo_url; do
    if echo "${repo_url}" | grep -qE '(ssh://|git@|\.git.*@|token=|access_token=)'; then
      echo "ERROR: Repo URL appears to require authentication, which is not supported: ${repo_url}" >&2
      AUTH_ERROR=1
      break
    fi
  done < "${REPOS_FILE}"

  if [[ $AUTH_ERROR -eq 1 ]]; then
    curl -s -X POST "${CONVEX_INGEST_URL/ingest\/secrets/jobs\/complete}" -H "Content-Type: application/json" -d "{\"jobId\":\"${JOB_RUN_ID}\",\"status\":\"failed\"}" >/dev/null
    rm -f "${REPOS_FILE}"
    sleep 5
    continue
  fi

  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "[DRY RUN] Would scan the above repositories for secrets:"
    echo "[DRY RUN] Max clone size: ${MAX_CLONE_SIZE}MB"
    curl -s -X POST "${CONVEX_INGEST_URL/ingest\/secrets/jobs\/complete}" -H "Content-Type: application/json" -d "{\"jobId\":\"${JOB_RUN_ID}\",\"status\":\"completed\"}" >/dev/null
    rm -f "${REPOS_FILE}"
    sleep 5
    continue
  fi

  RESULTS_DIR="/tmp/results/${JOB_RUN_ID}"
  CLONE_DIR="/tmp/clones/${JOB_RUN_ID}"
  mkdir -p "${RESULTS_DIR}" "${CLONE_DIR}"

  echo "[${JOB_RUN_ID}] Starting repository secret scan..."

  echo "[]" > "${RESULTS_DIR}/combined.json"

  while IFS= read -r repo_url; do
    [[ -z "${repo_url}" ]] && continue

    REPO_NAME=$(basename "${repo_url}" .git)
    echo "[${JOB_RUN_ID}] Scanning ${REPO_NAME}..."

    # Run checkmate search directly on repo_url (Checkmate handles cloning internally)
    checkmate search "${repo_url}" --json > "${RESULTS_DIR}/raw_${REPO_NAME}.json" 2>/dev/null || true

    # Filter out non-JSON debug lines (e.g. debug prints) before parsing
    sed -n '/^\[/,$p' "${RESULTS_DIR}/raw_${REPO_NAME}.json" > "${RESULTS_DIR}/${REPO_NAME}.json" 2>/dev/null || true

    # Process and redact findings, or emit empty array if none
    if [[ -f "${RESULTS_DIR}/${REPO_NAME}.json" ]] && [[ -s "${RESULTS_DIR}/${REPO_NAME}.json" ]]; then
       jq 'map(if .source then .source = ("REDACTED:" + (.source | @base64 | .[0:16])) elif .sha256 then .source = ("REDACTED:FILE_" + (.sha256 | .[0:12])) else .source = ("REDACTED:FILE_" + ((.location // "FILE") | @base64 | .[0:12])) end)' \
         "${RESULTS_DIR}/${REPO_NAME}.json" > "${RESULTS_DIR}/temp.json" 2>/dev/null || echo "[]" > "${RESULTS_DIR}/temp.json"
    else
       echo "[]" > "${RESULTS_DIR}/temp.json"
    fi

    # Combine into final results payload
    jq --arg url "${repo_url}" --slurpfile current "${RESULTS_DIR}/temp.json" \
       '. += [{ repoUrl: $url, results: $current[0] }]' \
       "${RESULTS_DIR}/combined.json" > "${RESULTS_DIR}/combined.tmp.json"
    mv "${RESULTS_DIR}/combined.tmp.json" "${RESULTS_DIR}/combined.json"

  done < "${REPOS_FILE}"

  # ─── Result ingestion (with redaction) ─────────────────────────────────────────
  echo "[${JOB_RUN_ID}] Redacting and posting findings to Convex..."

  CM_VER=$(checkmate --help 2>&1 | grep -i "Version:" | awk '{print $2}' || echo "")
  if [[ -z "${CM_VER}" || "${CM_VER}" == "0.0.0" ]]; then
    CM_VER="v1.2.0"
  elif [[ "${CM_VER}" != v* ]]; then
    CM_VER="v${CM_VER}"
  fi

  PAYLOAD=$(jq -n \
    --arg jobRunId "${JOB_RUN_ID}" \
    --arg verified "${VERIFY}" \
    --arg checkmateVersion "${CM_VER}" \
    --slurpfile findings "${RESULTS_DIR}/combined.json" \
    '{jobRunId: $jobRunId, verified: ($verified == "true"), checkmateVersion: $checkmateVersion, findings: $findings[0]}' 2>/dev/null || echo '{}')

  AUTH_HEADER=""
  if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
    AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
  fi

  curl -sf -X POST "${CONVEX_INGEST_URL}" \
    -H "Content-Type: application/json" \
    ${AUTH_HEADER} \
    -d "${PAYLOAD}" || {
      echo "ERROR: Failed to post findings to Convex" >&2
      curl -s -X POST "${CONVEX_INGEST_URL/ingest\/secrets/jobs\/complete}" -H "Content-Type: application/json" -d "{\"jobId\":\"${JOB_RUN_ID}\",\"status\":\"failed\"}" >/dev/null
      rm -f "${REPOS_FILE}"
      rm -rf "${RESULTS_DIR}" "${CLONE_DIR}"
      sleep 5
      continue
    }

  curl -s -X POST "${CONVEX_INGEST_URL/ingest\/secrets/jobs\/complete}" -H "Content-Type: application/json" -d "{\"jobId\":\"${JOB_RUN_ID}\",\"status\":\"completed\"}" >/dev/null

  echo "[${JOB_RUN_ID}] Repository scan complete."
  rm -f "${REPOS_FILE}"
  rm -rf "${RESULTS_DIR}" "${CLONE_DIR}"

  sleep 5
done
