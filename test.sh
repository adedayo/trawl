#!/usr/bin/env bash
set -euo pipefail

echo "============================================================"
echo "          Trawl — Rapid Development Test Runner             "
echo "============================================================"
echo ""

# 1. Typecheck
echo "[1/4] Checking TypeScript Types..."
npm run typecheck

# 2. Unit Tests
echo "[2/4] Running Unit Tests (Vitest)..."
npx vitest run convex/dedup.test.ts
npm test

# 3. Worker Allowlist & Script Dry-Runs
echo "[3/4] Validating Worker Entrypoints (--dry-run)..."
chmod +x jobs/scan-worker/entrypoint.sh jobs/discovery-worker/entrypoint.sh jobs/repo-scan-worker/entrypoint.sh
SEED_DOMAINS="example.com" ./jobs/scan-worker/entrypoint.sh --dry-run > /dev/null
SEED_DOMAINS="example.com" ./jobs/discovery-worker/entrypoint.sh --dry-run > /dev/null
SEED_REPOS="https://github.com/example/repo" ./jobs/repo-scan-worker/entrypoint.sh --dry-run > /dev/null
echo "✔ Worker dry-runs verified."

# 4. Production Bundle Build
echo "[4/4] Verifying Angular Production Build..."
npm run build > /dev/null

echo ""
echo "============================================================"
echo "  ✔ ALL CHECKS PASSED — READY TO COMMIT!"
echo "============================================================"
