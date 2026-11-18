#!/usr/bin/env bash
set -euo pipefail

# feed-ingest worker entrypoint
#
# Triggers a vulnerability catalogue refresh against the Trawl server, or loads
# local air-gapped catalogue files when supplied.
#
# Required env vars (unless DRY_RUN=true):
#   TRAWL_API_BASE      - Base URL of the Trawl server (e.g. http://trawl-server:8080)
#
# Optional:
#   DRY_RUN=true        - Print the endpoints or local files that would be refreshed without network calls
#   TRAWL_AUTH_TOKEN    - Bearer token for the Trawl server
#   EPSS_THRESHOLD      - EPSS score crossing to record as a regression (default 0)
#   KEV_URL             - Override CISA KEV endpoint
#   EPSS_URL            - Override FIRST EPSS endpoint
#   NVD_URL             - Override NIST NVD 2.0 endpoint
#   KEV_FILE            - Local KEV JSON file (offline mode)
#   EPSS_FILE           - Local EPSS CSV/CSV.GZ file (offline mode)

DRY_RUN="${DRY_RUN:-false}"
if [[ $# -gt 0 && "$1" == "--dry-run" ]]; then
  DRY_RUN="true"
  shift
fi

KEV_URL="${KEV_URL:-https://www.cisa.gov/sites/default/files/feeds/known_exploited_vulnerabilities.json}"
EPSS_URL="${EPSS_URL:-https://epss.empiricalsecurity.com/epss_scores-current.csv.gz}"
NVD_URL="${NVD_URL:-https://services.nvd.nist.gov/rest/json/cves/2.0}"
EPSS_THRESHOLD="${EPSS_THRESHOLD:-0}"

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "[DRY RUN] Vulnerability feed ingestion plan:"
  if [[ -n "${KEV_FILE:-}" || -n "${EPSS_FILE:-}" ]]; then
    echo "  mode: offline"
    [[ -n "${KEV_FILE:-}" ]] && echo "  cisa-kev file: ${KEV_FILE}"
    [[ -n "${EPSS_FILE:-}" ]] && echo "  epss file: ${EPSS_FILE}"
  else
    echo "  mode: network"
    echo "  cisa-kev endpoint: ${KEV_URL}"
    echo "  epss endpoint: ${EPSS_URL}"
    echo "  nvd endpoint: ${NVD_URL} (referenced CVEs only)"
  fi
  echo "  epss threshold: ${EPSS_THRESHOLD}"
  exit 0
fi

if [[ -z "${TRAWL_API_BASE:-}" ]]; then
  echo "ERROR: TRAWL_API_BASE is required for non-dry-run execution" >&2
  exit 1
fi

AUTH_ARGS=()
if [[ -n "${TRAWL_AUTH_TOKEN:-}" ]]; then
  AUTH_ARGS=(-H "Authorization: Bearer ${TRAWL_AUTH_TOKEN}")
fi

if [[ -n "${KEV_FILE:-}" || -n "${EPSS_FILE:-}" ]]; then
  if [[ -n "${KEV_FILE:-}" ]]; then
    PAYLOAD=$(jq -n --arg feed "cisa-kev" --arg sourceUrl "file://${KEV_FILE}" --rawfile body "${KEV_FILE}" --argjson threshold "${EPSS_THRESHOLD}" \
      '{feed: $feed, sourceUrl: $sourceUrl, body: $body, epssThreshold: $threshold}')
    curl -sf -X POST "${TRAWL_API_BASE%/}/api/v1/feeds/snapshot" \
      -H "Content-Type: application/json" "${AUTH_ARGS[@]}" -d "${PAYLOAD}" >/dev/null
  fi
  if [[ -n "${EPSS_FILE:-}" ]]; then
    PAYLOAD=$(jq -n --arg feed "epss" --arg sourceUrl "file://${EPSS_FILE}" --rawfile body "${EPSS_FILE}" --argjson threshold "${EPSS_THRESHOLD}" \
      '{feed: $feed, sourceUrl: $sourceUrl, body: $body, epssThreshold: $threshold}')
    curl -sf -X POST "${TRAWL_API_BASE%/}/api/v1/feeds/snapshot" \
      -H "Content-Type: application/json" "${AUTH_ARGS[@]}" -d "${PAYLOAD}" >/dev/null
  fi
  echo "Offline feed snapshots applied."
  exit 0
fi

PAYLOAD=$(jq -n \
  --arg kev "${KEV_URL}" \
  --arg epss "${EPSS_URL}" \
  --arg nvd "${NVD_URL}" \
  --argjson threshold "${EPSS_THRESHOLD}" \
  '{epssThreshold: $threshold, endpoints: {"cisa-kev": $kev, "epss": $epss, "nvd": $nvd}}')

curl -sf -X POST "${TRAWL_API_BASE%/}/api/v1/feeds/refresh" \
  -H "Content-Type: application/json" "${AUTH_ARGS[@]}" -d "${PAYLOAD}" >/dev/null

echo "Vulnerability feeds refreshed."
