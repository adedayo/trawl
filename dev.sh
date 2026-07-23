#!/usr/bin/env bash
set -euo pipefail

echo "============================================================"
echo "      Trawl — Rapid Hot-Reloading Development Environment   "
echo "============================================================"
echo ""
echo "  [CONVEX]  Backend & Schema Watcher (Hot-Reloads convex/)"
echo "  [ANGULAR] Frontend Dev Server    (Hot-Reloads app/ at http://localhost:4200)"
echo ""
echo "Press Ctrl+C to stop both servers."
echo "============================================================"
echo ""

npm run dev
