#!/usr/bin/env bash
#
# security-fix.sh — resolve known vulnerabilities in dependencies, locally.
#
# The operating principle is the same one the dependency gate uses: apply what
# is safe automatically, refuse to apply what needs a decision, and never let a
# fix land without verification. A script that silently upgrades a major version
# and leaves you to discover the breakage is not a security tool.
#
# Three classes of outcome, and they are kept apart deliberately:
#
#   APPLIED    a fix within the declared semver range, verified by the test suite
#   DECISION   a fix requiring a breaking change — reported, never applied
#   RESIDUAL   no fix published — reported, so it can be accepted explicitly
#
# The third class is the one most tools omit, and it is the one that matters: an
# unfixable vulnerability you know about is an accepted risk. One you have not
# been told about is priced at zero.
#
# Usage:
#   ./security-fix.sh              audit and apply safe fixes, then verify
#   ./security-fix.sh --dry-run    report only, change nothing
#   ./security-fix.sh --no-verify  skip the test suite (not recommended)
#   ./security-fix.sh --go-only    Go modules only
#   ./security-fix.sh --npm-only   npm workspaces only

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$REPO_ROOT"

DRY_RUN=false
VERIFY=true
DO_GO=true
DO_NPM=true

for arg in "$@"; do
  case "$arg" in
    --dry-run)    DRY_RUN=true ;;
    --no-verify)  VERIFY=false ;;
    --go-only)    DO_NPM=false ;;
    --npm-only)   DO_GO=false ;;
    -h|--help)    sed -n '2,26p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *)            echo "Unknown option: $arg (try --help)" >&2; exit 2 ;;
  esac
done

# ── output ───────────────────────────────────────────────────────────────────

if [[ -t 1 ]]; then
  BOLD=$'\033[1m'; DIM=$'\033[2m'; RED=$'\033[31m'; GREEN=$'\033[32m'
  YELLOW=$'\033[33m'; BLUE=$'\033[34m'; RESET=$'\033[0m'
else
  BOLD=''; DIM=''; RED=''; GREEN=''; YELLOW=''; BLUE=''; RESET=''
fi

section() { printf '\n%s┌─ %s %s\n' "$BOLD$BLUE" "$1" "$RESET"; }
ok()      { printf '  %s✔%s %s\n' "$GREEN" "$RESET" "$1"; }
warn()    { printf '  %s!%s %s\n' "$YELLOW" "$RESET" "$1"; }
fail()    { printf '  %s✘%s %s\n' "$RED" "$RESET" "$1"; }
info()    { printf '  %s%s%s\n' "$DIM" "$1" "$RESET"; }

DECISIONS_FILE="$(mktemp)"
RESIDUAL_FILE="$(mktemp)"
OVERRIDE_FILE="$(mktemp)"
trap 'rm -f "$DECISIONS_FILE" "$RESIDUAL_FILE" "$OVERRIDE_FILE"' EXIT

# ── preflight ────────────────────────────────────────────────────────────────

section "Preflight"

if ! git diff --quiet || ! git diff --cached --quiet; then
  fail "Working tree is dirty."
  info "This script rewrites lockfiles and manifests. Commit or stash first,"
  info "so that what it changes is legible in a diff and revertible in one step."
  exit 1
fi
ok "Working tree clean"

for tool in npm jq; do
  command -v "$tool" >/dev/null || { fail "$tool is required but not installed."; exit 1; }
done
ok "npm $(npm --version), jq present"

# Dependency upgrades routinely raise the Node engine floor — that is how a fix
# turns into a breakage. Check the pinned version up front so the failure is a
# one-line message here rather than an obscure one three minutes into the suite.
if [[ -f .nvmrc ]]; then
  WANT_NODE="$(tr -d '[:space:]' < .nvmrc)"
  HAVE_NODE="$(node --version 2>/dev/null | sed 's/^v//')"
  if [[ "$HAVE_NODE" != "$WANT_NODE" ]]; then
    warn "Node $HAVE_NODE in use, but .nvmrc pins $WANT_NODE"
    info "Upgrades can raise the engine floor. Run: nvm use"
  else
    ok "Node $HAVE_NODE matches .nvmrc"
  fi
fi

if $DO_GO; then
  if ! command -v go >/dev/null; then
    warn "Go not installed — skipping Go module checks"
    DO_GO=false
  else
    ok "go $(go version | awk '{print $3}')"
  fi
fi

$DRY_RUN && warn "DRY RUN — nothing will be modified"

# ── go modules ───────────────────────────────────────────────────────────────
#
# govulncheck is used rather than a manifest-diffing scanner because it reports
# only vulnerabilities whose vulnerable symbols are actually *reachable* from
# this program's call graph. That distinction is the same one the risk model
# makes: a weakness that cannot be reached is not the same as one that can, and
# treating them alike produces work that retires no risk.

if $DO_GO; then
  section "Go modules"

  if ! command -v govulncheck >/dev/null; then
    info "Installing govulncheck..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
    export PATH="$PATH:$(go env GOPATH)/bin"
  fi
  command -v govulncheck >/dev/null || {
    fail "govulncheck not on PATH. Add \$(go env GOPATH)/bin to PATH."
    exit 1
  }

  GOVULN_JSON="$(mktemp)"
  govulncheck -format json ./... > "$GOVULN_JSON" 2>/dev/null || true

  # govulncheck reports at three levels of reachability, and they are not the
  # same finding:
  #
  #   called   a vulnerable symbol is reachable from this program's entry points
  #   imported the package is imported, but no vulnerable symbol is called
  #   required the module is in the graph and nothing of it is imported
  #
  # Only the first is a live exposure. Upgrading for the other two retires no
  # risk — it is the "high severity, negligible contact probability" case, and
  # spending a change window on it is tidying, not security. They are recorded
  # so the decision to leave them is deliberate rather than accidental.
  CALLED_FIXABLE=$(jq -rs '
    [ .[] | select(.finding) | .finding
      | select(.trace != null and .trace[0].function != null)
      | select(.fixed_version != null and .fixed_version != "")
      | .trace[0].module ] | unique | .[]' "$GOVULN_JSON" 2>/dev/null || true)

  CALLED_UNFIXED=$(jq -rs '
    [ .[] | select(.finding) | .finding
      | select(.trace != null and .trace[0].function != null)
      | select(.fixed_version == null or .fixed_version == "")
      | "\(.trace[0].module) \(.osv)" ] | unique | .[]' "$GOVULN_JSON" 2>/dev/null || true)

  IMPORTED_ONLY=$(jq -rs '
    [ .[] | select(.finding) | .finding
      | select(.trace != null and .trace[0].function == null)
      | .trace[0].module ] | unique | .[]' "$GOVULN_JSON" 2>/dev/null || true)

  if [[ -z "$CALLED_FIXABLE" && -z "$CALLED_UNFIXED" ]]; then
    ok "No reachable vulnerabilities in Go dependencies"
  fi

  if [[ -n "$CALLED_FIXABLE" ]]; then
    warn "Reachable and fixable — upgrading:"
    while IFS= read -r mod; do
      [[ -z "$mod" ]] && continue
      info "  $mod"
    done <<< "$CALLED_FIXABLE"

    if ! $DRY_RUN; then
      while IFS= read -r mod; do
        [[ -z "$mod" ]] && continue
        go get "$mod@latest" 2>&1 | sed 's/^/    /' || warn "  could not upgrade $mod"
      done <<< "$CALLED_FIXABLE"
      go mod tidy
      ok "Go modules upgraded and tidied"
    fi
  fi

  if [[ -n "$CALLED_UNFIXED" ]]; then
    warn "Reachable with NO published fix — this is real, accepted risk:"
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      info "  $line"
      printf 'go: %s — reachable, no fix published\n' "$line" >> "$RESIDUAL_FILE"
    done <<< "$CALLED_UNFIXED"
  fi

  if [[ -n "$IMPORTED_ONLY" ]]; then
    COUNT=$(printf '%s\n' "$IMPORTED_ONLY" | grep -c . || true)
    info "$COUNT module(s) import vulnerable packages without calling vulnerable symbols."
    info "Not upgraded: no reachable exposure, so no risk is retired by doing so."
    printf 'go: %s module(s) imported-but-not-called — no reachable exposure\n' \
      "$COUNT" >> "$RESIDUAL_FILE"
  fi

  rm -f "$GOVULN_JSON"
fi

# ── npm workspaces ───────────────────────────────────────────────────────────

# Sets the globals AUDIT_TOTAL / AUDIT_C / AUDIT_H / AUDIT_M / AUDIT_L rather
# than echoing them, so that status lines can be printed as they are produced
# instead of being captured by command substitution.
audit_counts() {  # $1 = directory
  local raw
  # `npm audit` exits non-zero when it finds anything, and `set -o pipefail`
  # propagates that through the pipe — so the exit status must be discarded
  # explicitly, or every non-clean workspace reads as clean.
  raw=$( cd "$1" && { npm audit --json 2>/dev/null || true; } \
    | jq -r '.metadata.vulnerabilities | "\(.critical) \(.high) \(.moderate) \(.low)"' 2>/dev/null )
  [[ -z "$raw" ]] && raw="0 0 0 0"
  read -r AUDIT_C AUDIT_H AUDIT_M AUDIT_L <<< "$raw"
  AUDIT_TOTAL=$((AUDIT_C + AUDIT_H + AUDIT_M + AUDIT_L))
}

report_counts() {  # $1 = label
  if [[ $AUDIT_TOTAL -eq 0 ]]; then
    ok "$1: clean"
  else
    warn "$1: $AUDIT_TOTAL total — $AUDIT_C critical, $AUDIT_H high, $AUDIT_M moderate, $AUDIT_L low"
  fi
}

fix_workspace() {  # $1 = directory, $2 = label
  local dir="$1" label="$2"

  section "npm — $label"

  audit_counts "$dir"
  report_counts "before"
  local before=$AUDIT_TOTAL
  [[ $before -eq 0 ]] && return 0

  # Which advisories can only be resolved by a breaking change? `npm audit fix`
  # without --force will not touch these, which is the behaviour we want: a
  # major version bump is a decision, not a patch.
  #
  # It also flags the case npm reports without comment — a "fix" that is
  # actually a *downgrade* of a directly declared dependency. npm will happily
  # recommend moving @angular/cli from 22 to 21 to clear a moderate advisory in
  # a transitive dev tool. That trades a known, bounded issue for an unknown
  # set of them, and it is exactly the "high severity, negligible exposure"
  # ranking error: the label says fix, the arithmetic says do not.
  local breaking
  breaking=$( cd "$dir" && { npm audit --json 2>/dev/null || true; } | jq -r '
    [ .vulnerabilities | to_entries[]
      | select(.value.fixAvailable | type == "object")
      | "\(.value.severity)\t\(.key) — requires \(.value.fixAvailable.name)@\(.value.fixAvailable.version)"
    ] | unique | .[]' 2>/dev/null ) || breaking=""

  # Transitive advisories with a published patch that npm did not apply are
  # usually resolvable with an `overrides` entry pinned to the patched version,
  # scoped by range so unaffected copies of the same package are left alone.
  local overridable
  overridable=$( cd "$dir" && { npm audit --json 2>/dev/null || true; } | jq -r '
    [ .vulnerabilities | to_entries[]
      | select(.value.isDirect == false)
      | select(.value.fixAvailable == true)
      | select([.value.via[] | select(type == "object")] | length > 0)
      | "\(.value.severity)\t\(.key)\trange \(.value.range)"
    ] | unique | .[]' 2>/dev/null ) || overridable=""

  if $DRY_RUN; then
    info "would run: npm audit fix"
  else
    info "Applying non-breaking fixes..."
    ( cd "$dir" && npm audit fix --no-fund 2>&1 | tail -3 | sed 's/^/    /' ) || true
    audit_counts "$dir"
    report_counts "after "
    if [[ $AUDIT_TOTAL -lt $before ]]; then
      ok "Resolved $((before - AUDIT_TOTAL)) advisory(ies) within the declared semver ranges"
    fi
  fi

  if [[ -n "$breaking" ]]; then
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      printf '%s\t[%s]\n' "$line" "$label" >> "$DECISIONS_FILE"
    done <<< "$breaking"
  fi

  # Re-read after the fix pass; anything still transitive-with-a-patch wants an
  # override rather than a version bump of the parent.
  overridable=$( cd "$dir" && { npm audit --json 2>/dev/null || true; } | jq -r '
    [ .vulnerabilities | to_entries[]
      | select(.value.isDirect == false)
      | select(.value.fixAvailable == true)
      | select([.value.via[] | select(type == "object")] | length > 0)
      | "\(.value.severity)\t\(.key)\tvulnerable range: \(.value.range)"
    ] | unique | .[]' 2>/dev/null ) || overridable=""

  if [[ -n "$overridable" ]]; then
    while IFS= read -r line; do
      [[ -z "$line" ]] && continue
      printf '%s\t[%s]\tresolvable via overrides\n' "$line" "$label" >> "$OVERRIDE_FILE"
    done <<< "$overridable"
  fi

  # Residual is only knowable after fixes have actually been applied. In a dry
  # run the remaining count is just the starting count, which says nothing.
  if ! $DRY_RUN && [[ $AUDIT_TOTAL -gt 0 && -z "$breaking" ]]; then
    printf 'npm/%s: %s advisory(ies) remaining with no non-breaking fix\n' \
      "$label" "$AUDIT_TOTAL" >> "$RESIDUAL_FILE"
  fi

  return 0
}

if $DO_NPM; then
  fix_workspace "." "root"
  fix_workspace "app" "app"
fi

# ── verification ─────────────────────────────────────────────────────────────
#
# An unverified fix is a change of unknown sign. The whole argument for a short
# cooldown and an automated gate rests on the test suite actually running.

if ! $DRY_RUN && $VERIFY; then
  section "Verification"
  info "Running the full check suite. This is the part that makes the fix trustworthy."
  if ./test.sh; then
    ok "All checks passed — changes are safe to commit"
  else
    fail "Checks FAILED after applying fixes."
    info ""
    info "The fixes are still in your working tree. Either repair the breakage,"
    info "or discard and treat the advisories as accepted risk with a stated reason:"
    info ""
    info "    git checkout -- package.json package-lock.json app/package.json app/package-lock.json go.mod go.sum"
    info ""
    exit 1
  fi
elif ! $DRY_RUN; then
  warn "Verification skipped (--no-verify). Run ./test.sh before committing."
fi

# ── report ───────────────────────────────────────────────────────────────────

section "Summary"

if [[ -s "$OVERRIDE_FILE" ]]; then
  printf '\n  %sTransitive, patched upstream — resolvable with an override%s\n\n' "$BOLD$BLUE" "$RESET"
  sort -u "$OVERRIDE_FILE" | sed 's/^/    /'
  printf '\n'
  info "npm did not apply these because a parent package's declared range pins"
  info "the vulnerable version. An override pins the patched one directly."
  info ""
  info "Add to the workspace package.json, scoped by range so unaffected copies"
  info "of the same package are left alone:"
  info ""
  info '    "overrides": { "<pkg>@<vulnerable range>": "<patched version>" }'
  info ""
  info "Then: npm install && ./test.sh"
fi

if [[ -s "$DECISIONS_FILE" ]]; then
  printf '\n  %sRequires a decision — not applied automatically%s\n\n' "$BOLD$YELLOW" "$RESET"
  sort -u "$DECISIONS_FILE" | sed 's/^/    /'
  printf '\n'
  info "Check the direction before accepting any of these. npm reports a version"
  info "change as a fix without regard to whether it is an upgrade — it will"
  info "recommend downgrading a major version of a direct dependency to clear a"
  info "moderate advisory in a transitive dev tool. That is usually a worse trade"
  info "than the advisory, and it is a decision, not a patch."
  info ""
  info "To take one, upgrade explicitly and verify:"
  info "    npm --prefix app install <package>@<version> && ./test.sh"
  info ""
  info "To defer, that is a priced acceptance: record which advisory, why,"
  info "who owns it, and when it will be revisited."
fi

if [[ -s "$RESIDUAL_FILE" ]]; then
  printf '\n  %sNo fix available — residual risk%s\n\n' "$BOLD$YELLOW" "$RESET"
  sed 's/^/    /' "$RESIDUAL_FILE"
  printf '\n'
  info "Nothing to apply. These are carried knowingly rather than unknowingly,"
  info "which is the only difference this script can make to them."
fi

if [[ ! -s "$DECISIONS_FILE" && ! -s "$RESIDUAL_FILE" && ! -s "$OVERRIDE_FILE" ]]; then
  printf '\n'
  ok "No outstanding decisions and no residual advisories."
fi

if ! $DRY_RUN; then
  printf '\n'
  info "Changed files:"
  git --no-pager diff --stat | sed 's/^/    /'
  printf '\n'
  info "Review the diff before committing:"
  info "    git diff -- package-lock.json app/package-lock.json go.sum"
fi

printf '\n'
