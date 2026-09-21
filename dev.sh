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

# Hooks are per-clone and cannot be committed into place, so they have to be
# installed by something a developer already runs. This is that something. It
# is idempotent, costs one git config read, and never overrides a hooksPath
# that has been set deliberately to something else.
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -d "${REPO_ROOT}/.githooks" ]] && [[ -z "$(git -C "${REPO_ROOT}" config core.hooksPath || true)" ]]; then
    "${REPO_ROOT}/scripts/install-hooks.sh" >/dev/null 2>&1 \
        && echo "✔ Installed git hooks (pre-push vulnerability gate)" && echo ""
fi

npm run dev
