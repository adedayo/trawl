#!/usr/bin/env bash
#
# install-hooks.sh — point git at the hooks this repository tracks.
#
# Hooks live in .githooks/ and are version-controlled. .git/hooks is not: it is
# per-clone, invisible in review, and cannot be changed for everyone at once. A
# gate that only exists on the machine of whoever set it up is not a gate.
#
# This sets core.hooksPath, which is a single local config line and is undone
# with `git config --unset core.hooksPath`.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

chmod +x .githooks/* 2>/dev/null || true
git config core.hooksPath .githooks

echo "✔ Hooks installed — git will use .githooks/"
echo "    pre-push: refuses a push carrying vulnerabilities that reach shipped code"
echo ""
echo "  Bypass a single push with --no-verify. Remove entirely with:"
echo "      git config --unset core.hooksPath"
