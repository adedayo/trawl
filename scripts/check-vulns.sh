#!/usr/bin/env bash
#
# check-vulns.sh — refuse to publish a known-vulnerable tree.
#
# This is the gate, not the fixer. `./security-fix.sh` applies and verifies
# fixes; this decides whether what is about to leave the machine is allowed to.
# Keeping them apart matters: a gate that silently rewrites lockfiles turns a
# push into an unreviewed dependency change, and the first you would know of it
# is a diff you did not write.
#
# The ranking is the same one security-fix.sh uses, because it is the only one
# that retires risk rather than tidying:
#
#   BLOCKING  reachable from shipped code — a Go symbol this program calls, or
#             an npm package that goes into the bundle a user runs.
#   REPORTED  present only in the build and test toolchain. Real, worth fixing,
#             but it does not reach a user through a released artefact, and
#             blocking a push on it trains people to pass --no-verify, which
#             disables the blocking class too.
#
# That second consequence is the whole argument. A gate that cries wolf is
# worse than no gate, because it is a gate everyone has learned to step around.
#
# Usage:
#   ./scripts/check-vulns.sh            block on shipped, report the rest
#   ./scripts/check-vulns.sh --strict   block on anything at all (releases)
#   ./scripts/check-vulns.sh --quiet    only speak when something is wrong

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

STRICT=false
QUIET=false
for arg in "$@"; do
  case "$arg" in
    --strict) STRICT=true ;;
    --quiet)  QUIET=true ;;
    -h|--help) sed -n '2,27p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) echo "Unknown option: $arg (try --help)" >&2; exit 2 ;;
  esac
done

if [[ -t 1 ]]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'
  GREEN=$'\033[32m'; YELLOW=$'\033[33m'; RESET=$'\033[0m'
else
  BOLD=''; DIM=''; RED=''; GREEN=''; YELLOW=''; RESET=''
fi

ok()   { $QUIET || printf '  %s✔%s %s\n' "$GREEN" "$RESET" "$1"; }
warn() { printf '  %s!%s %s\n' "$YELLOW" "$RESET" "$1"; }
fail() { printf '  %s✘%s %s\n' "$RED" "$RESET" "$1"; }
info() { printf '  %s%s%s\n' "$DIM" "$1" "$RESET"; }

BLOCKING=0
REPORTED=0

# ── npm: what actually ships ─────────────────────────────────────────────────
#
# `--omit=dev` is the distinction the severity count alone cannot make. Trawl's
# release artefacts contain the Angular bundle and nothing from devDependencies,
# so a high in the test runner and a moderate in @angular/core are not the same
# finding however the numbers are coloured.

if command -v npm >/dev/null && command -v jq >/dev/null; then
  for ws in "." "app"; do
    label="$( [[ $ws == "." ]] && echo root || echo "$ws" )"
    [[ -f "$ws/package.json" ]] || continue

    shipped=$( cd "$ws" && { npm audit --omit=dev --json 2>/dev/null || true; } \
      | jq -r '.metadata.vulnerabilities | (.critical + .high + .moderate + .low)' 2>/dev/null )
    total=$( cd "$ws" && { npm audit --json 2>/dev/null || true; } \
      | jq -r '.metadata.vulnerabilities | (.critical + .high + .moderate + .low)' 2>/dev/null )
    shipped=${shipped:-0}; total=${total:-0}
    devonly=$(( total - shipped ))

    if [[ $shipped -gt 0 ]]; then
      fail "npm/$label: $shipped advisory(ies) in code that ships"
      ( cd "$ws" && npm audit --omit=dev 2>/dev/null | sed -n '1,20p' | sed 's/^/      /' ) || true
      BLOCKING=$((BLOCKING + shipped))
    else
      ok "npm/$label: nothing vulnerable ships"
    fi

    if [[ $devonly -gt 0 ]]; then
      if $STRICT; then
        fail "npm/$label: $devonly advisory(ies) in the build toolchain"
        BLOCKING=$((BLOCKING + devonly))
      else
        warn "npm/$label: $devonly advisory(ies) in the build toolchain (not blocking)"
        REPORTED=$((REPORTED + devonly))
      fi
    fi
  done
else
  warn "npm or jq missing — dependency audit skipped"
fi

# ── go: what is actually called ──────────────────────────────────────────────
#
# govulncheck distinguishes a vulnerable symbol this program calls from a module
# that merely sits in the graph. Only the first is exposure. Blocking on the
# second would fail a push over a package nothing imports.

if command -v go >/dev/null; then
  if ! command -v govulncheck >/dev/null; then
    if [[ -x "$(go env GOPATH)/bin/govulncheck" ]]; then
      export PATH="$PATH:$(go env GOPATH)/bin"
    fi
  fi

  if command -v govulncheck >/dev/null && command -v jq >/dev/null; then
    GOVULN_JSON="$(mktemp)"
    trap 'rm -f "$GOVULN_JSON"' EXIT
    govulncheck -format json ./... > "$GOVULN_JSON" 2>/dev/null || true

    called_fixable=$(jq -rs '
      [ .[] | select(.finding) | .finding
        | select(.trace != null and .trace[0].function != null)
        | select(.fixed_version != null and .fixed_version != "")
        | "\(.trace[0].module) \(.osv) — fixed in \(.fixed_version)" ] | unique | .[]' \
      "$GOVULN_JSON" 2>/dev/null || true)

    called_unfixed=$(jq -rs '
      [ .[] | select(.finding) | .finding
        | select(.trace != null and .trace[0].function != null)
        | select(.fixed_version == null or .fixed_version == "")
        | "\(.trace[0].module) \(.osv)" ] | unique | .[]' \
      "$GOVULN_JSON" 2>/dev/null || true)

    if [[ -n "$called_fixable" ]]; then
      fail "Go: reachable vulnerabilities with a published fix"
      while IFS= read -r l; do [[ -n "$l" ]] && info "    $l"; done <<< "$called_fixable"
      BLOCKING=$((BLOCKING + $(grep -c . <<< "$called_fixable")))
    fi

    # Reachable with no fix published cannot be a blocking condition: there is
    # nothing to do about it, and a gate that cannot be satisfied is one that
    # gets bypassed. It is named on every run so that carrying it stays a
    # decision rather than a habit.
    if [[ -n "$called_unfixed" ]]; then
      warn "Go: reachable, NO fix published — carried as accepted risk"
      while IFS= read -r l; do [[ -n "$l" ]] && info "    $l"; done <<< "$called_unfixed"
      REPORTED=$((REPORTED + $(grep -c . <<< "$called_unfixed")))
    fi

    [[ -z "$called_fixable" && -z "$called_unfixed" ]] && ok "Go: no reachable vulnerabilities"
  else
    warn "govulncheck not installed — Go reachability not checked"
    info "  go install golang.org/x/vuln/cmd/govulncheck@latest"
  fi
fi

# ── verdict ──────────────────────────────────────────────────────────────────

if [[ $BLOCKING -gt 0 ]]; then
  printf '\n%s✘ %s vulnerability(ies) reach shipped code.%s\n\n' "$BOLD$RED" "$BLOCKING" "$RESET"
  info "Fix them before this leaves the machine:"
  info ""
  info "    ./security-fix.sh"
  info ""
  info "If one cannot be fixed, that is an acceptance and needs a stated reason."
  info "Record it, then push with --no-verify so the bypass is visible in the log."
  printf '\n'
  exit 1
fi

if [[ $REPORTED -gt 0 ]]; then
  $QUIET || printf '\n  %s%s issue(s) reported and not blocking. Run ./security-fix.sh when convenient.%s\n' \
    "$DIM" "$REPORTED" "$RESET"
fi

$QUIET || printf '\n%s✔ Nothing vulnerable reaches shipped code.%s\n' "$GREEN" "$RESET"
exit 0
