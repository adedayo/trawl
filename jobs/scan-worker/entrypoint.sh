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

if [[ $# -gt 0 && "$1" == "--dry-run" ]]; then
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

# ─── Worker Polling Loop ───────────────────────────────────────────────────────
echo "Starting scan-worker polling loop..."

while true; do
  # Poll Convex for next job
  JOB_JSON=$(curl -s -f -X GET "${CONVEX_INGEST_URL/ingest\/scan/jobs\/pop}?type=scan" || echo "")

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

  echo "[${JOB_RUN_ID}] Picked up new scan job"

  # Build the authorised target list
  TARGETS_FILE=$(mktemp)
  echo "$TARGETS" > "${TARGETS_FILE}"

  if [[ "${DRY_RUN}" == "true" ]]; then
    echo "[DRY RUN] Would scan targets:"
    cat "${TARGETS_FILE}"
    # Complete job as success
    curl -s -X POST "${CONVEX_INGEST_URL/ingest\/scan/jobs\/complete}" -H "Content-Type: application/json" -d "{\"jobId\":\"${JOB_RUN_ID}\"}" >/dev/null
    rm -f "${TARGETS_FILE}"
    sleep 5
    continue
  fi

  # ─── Scan execution ────────────────────────────────────────────────────────────
  RESULTS_DIR="/tmp/results/${JOB_RUN_ID}"
  mkdir -p "${RESULTS_DIR}"

  echo "[${JOB_RUN_ID}] Starting scan..."

  # Step 1: Port scan with naabu
  (
    echo "[${JOB_RUN_ID}] Running naabu (port scan)..."
    naabu -list "${TARGETS_FILE}" -json -o "${RESULTS_DIR}/naabu.json" 2>/dev/null || true
    
    AUTH_HEADER=""
    if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
      AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
    fi

    echo "[${JOB_RUN_ID}] Posting partial results (naabu) to Convex..."
    PAYLOAD_NAABU=$(jq -n --arg jobRunId "${JOB_RUN_ID}" --slurpfile naabu "${RESULTS_DIR}/naabu.json" '{jobRunId: $jobRunId, naabu: $naabu}' 2>/dev/null || echo '{}')
    curl -s -X POST "${CONVEX_INGEST_URL}" -H "Content-Type: application/json" ${AUTH_HEADER} -d "${PAYLOAD_NAABU}" >/dev/null || true
  ) &

  # Step 2: HTTP probing with httpx
  (
    echo "[${JOB_RUN_ID}] Running httpx (HTTP probe)..."
    httpx -list "${TARGETS_FILE}" -json -o "${RESULTS_DIR}/httpx.json" \
      -td -title -status-code -tech-detect -tls-grab -cdn \
      2>/dev/null || true

    AUTH_HEADER=""
    if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
      AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
    fi

    echo "[${JOB_RUN_ID}] Posting partial results (httpx) to Convex..."
    PAYLOAD_HTTPX=$(jq -n --arg jobRunId "${JOB_RUN_ID}" --slurpfile httpx "${RESULTS_DIR}/httpx.json" '{jobRunId: $jobRunId, httpx: $httpx}' 2>/dev/null || echo '{}')
    curl -s -X POST "${CONVEX_INGEST_URL}" -H "Content-Type: application/json" ${AUTH_HEADER} -d "${PAYLOAD_HTTPX}" >/dev/null || true
  ) &

  # Step 2.5: Email Posture (DNS Checks)
  (
    echo "[${JOB_RUN_ID}] Running email posture checks..."
    AUTH_HEADER=""
    if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
      AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
    fi

    while IFS= read -r domain; do
      if [[ -n "$domain" && "$domain" =~ [a-zA-Z] ]]; then
        SPF_REC=$(dig +short TXT "$domain" | grep -i "v=spf1" || true)
        SPF_VALID="false"
        if [[ -n "$SPF_REC" ]]; then SPF_VALID="true"; fi

        DMARC_REC=$(dig +short TXT "_dmarc.$domain" | grep -i "v=DMARC1" || true)
        DMARC_POLICY="missing"
        if echo "$DMARC_REC" | grep -qi "p=reject"; then
          DMARC_POLICY="reject"
        elif echo "$DMARC_REC" | grep -qi "p=quarantine"; then
          DMARC_POLICY="quarantine"
        elif echo "$DMARC_REC" | grep -qi "p=none"; then
          DMARC_POLICY="none"
        fi

        DKIM_FOUND="false"
        if dig +short TXT "default._domainkey.$domain" | grep -qi "v=DKIM1"; then
          DKIM_FOUND="true"
        elif dig +short TXT "google._domainkey.$domain" | grep -qi "v=DKIM1"; then
          DKIM_FOUND="true"
        elif dig +short TXT "selector1._domainkey.$domain" | grep -qi "v=DKIM1"; then
          DKIM_FOUND="true"
        fi

        PRIORITY="low"
        if [[ "$DMARC_POLICY" == "missing" || "$DMARC_POLICY" == "none" ]]; then
          PRIORITY="high"
        elif [[ "$SPF_VALID" == "false" ]]; then
          PRIORITY="medium"
        fi

        PAYLOAD=$(jq -n \
          --arg domain "$domain" \
          --arg spf "$SPF_VALID" \
          --arg dkim "$DKIM_FOUND" \
          --arg dmarc "$DMARC_POLICY" \
          --arg priority "$PRIORITY" \
          '{
            domain: $domain,
            spfValid: ($spf == "true"),
            dkimFound: ($dkim == "true"),
            dmarcPolicy: $dmarc,
            priority: $priority
          }')
        
        curl -s -X POST "${CONVEX_INGEST_URL/ingest\/scan/ingest\/email-posture}" \
          -H "Content-Type: application/json" \
          ${AUTH_HEADER} \
          -d "$PAYLOAD" >/dev/null || true
      fi
    done < "${TARGETS_FILE}"
  ) &

  # Step 3: Nuclei scan (KEV-tagged templates first)
  (
    echo "[${JOB_RUN_ID}] Running nuclei (vuln scan)..."
    nuclei -list "${TARGETS_FILE}" -jsonl -o "${RESULTS_DIR}/nuclei.json" \
      -severity critical,high,medium \
      2>/dev/null || true

    AUTH_HEADER=""
    if [[ -n "${CONVEX_AUTH_TOKEN:-}" ]]; then
      AUTH_HEADER="-H \"Authorization: Bearer ${CONVEX_AUTH_TOKEN}\""
    fi

    echo "[${JOB_RUN_ID}] Posting final results (nuclei) to Convex..."
    PAYLOAD_NUCLEI=$(jq -n \
      --arg jobRunId "${JOB_RUN_ID}" \
      --slurpfile nuclei "${RESULTS_DIR}/nuclei.json" \
      '{
        jobRunId: $jobRunId,
        nuclei: $nuclei
      }' 2>/dev/null || echo '{}')

    curl -sf -X POST "${CONVEX_INGEST_URL}" \
      -H "Content-Type: application/json" \
      ${AUTH_HEADER} \
      -d "${PAYLOAD_NUCLEI}" || true
  ) &

  # Wait for all background scan jobs to finish
  echo "[${JOB_RUN_ID}] Waiting for concurrent scans to finish..."
  wait

  # Mark job as complete
  curl -s -X POST "${CONVEX_INGEST_URL/ingest\/scan/jobs\/complete}" -H "Content-Type: application/json" -d "{\"jobId\":\"${JOB_RUN_ID}\",\"status\":\"completed\"}" >/dev/null

  echo "[${JOB_RUN_ID}] Scan complete."
  rm -f "${TARGETS_FILE}"
  rm -rf "${RESULTS_DIR}"
  
  sleep 5
done
