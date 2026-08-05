#!/usr/bin/env bash
set -euo pipefail

echo "============================================================"
echo "      Trawl — Rapid Hot-Reloading Development Environment   "
echo "============================================================"
echo ""
echo "  [ANGULAR] Frontend Dev Server   (Hot-Reloads app/ at http://localhost:4200)"
echo ""
echo "  For the Go server, run in a second shell:"
echo "      go run ./cmd/trawl server"
echo "  Or use ./dev-docker.sh to run both under Docker Compose."
echo ""
echo "Press Ctrl+C to stop."
echo "============================================================"
echo ""

npm run dev
