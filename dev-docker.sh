#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "============================================================"
echo "      Trawl — Docker Compose Hot-Reloading Dev Environment  "
echo "============================================================"
echo ""
echo "🚀 Starting Convex Backend + Angular Dashboard (Hot-Reload) + Scan Workers..."
echo ""

docker compose -f "${SCRIPT_DIR}/deploy/compose/docker-compose.dev.yml" up --build
