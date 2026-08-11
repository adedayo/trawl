#!/usr/bin/env bash
set -euo pipefail

# Trawl — release cutter.
#
# This script does exactly one thing that matters: it establishes that the tree
# is in a state worth tagging, and then tags it. Everything after the tag is
# the workflow's job. The division is deliberate — a release script that also
# builds and uploads artefacts has two failure modes that look identical from
# the terminal, and the one that matters (a half-published release) is the one
# that is hardest to undo.
#
# Usage:
#   ./scripts/release.sh v1.2.3
#   ./scripts/release.sh v1.2.3 --dry-run    # check everything, tag nothing

VERSION="${1:-}"
DRY_RUN=false
for arg in "${@:2}"; do
  case "${arg}" in
    --dry-run) DRY_RUN=true ;;
    *) echo "Unknown option: ${arg}" >&2; exit 1 ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

die() { echo "✖ $*" >&2; exit 1; }
step() { echo ""; echo "── $* ──"; }

# ─── Preconditions ──────────────────────────────────────────────────────────

[ -n "${VERSION}" ] || die "Version required. Usage: ./scripts/release.sh v1.2.3"

# Anchored, and permitting a prerelease suffix. The workflow keys its
# `prerelease` flag off the presence of a hyphen, so "v1.2.3-rc.1" is a
# supported input rather than an accident.
[[ "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] \
  || die "Invalid version '${VERSION}'. Expected vMAJOR.MINOR.PATCH[-prerelease]."

git rev-parse --git-dir > /dev/null 2>&1 || die "Not a git repository."

# A dirty tree means the tag would point at a commit that does not contain what
# was tested. The artefacts would be built from the commit, not the working
# copy, so this is not pedantry: it is the difference between shipping what was
# verified and shipping something adjacent to it.
[ -z "$(git status --porcelain)" ] \
  || { git status --short >&2; die "Working tree is not clean."; }

git rev-parse "${VERSION}" > /dev/null 2>&1 \
  && die "Tag ${VERSION} already exists. Releases are immutable; choose a new version."

BRANCH="$(git rev-parse --abbrev-ref HEAD)"
if [ "${BRANCH}" != "main" ]; then
  echo "⚠ Releasing from '${BRANCH}', not 'main'."
  read -r -p "  Continue? [y/N] " reply
  [[ "${reply}" =~ ^[Yy]$ ]] || die "Aborted."
fi

step "Fetching remote state"
git fetch --tags --quiet origin
LOCAL="$(git rev-parse @)"
REMOTE="$(git rev-parse '@{u}' 2>/dev/null || echo "${LOCAL}")"
[ "${LOCAL}" = "${REMOTE}" ] \
  || die "Local branch differs from its upstream. Pull or push before releasing."

echo "✔ ${VERSION} is a valid, unused version on a clean, synchronised ${BRANCH}"

# ─── Verification ───────────────────────────────────────────────────────────

step "Full test suite"
./test.sh

step "Packaging manifest validation"
./scripts/validate-packaging.sh

# ─── Version bump ───────────────────────────────────────────────────────────
#
# The stripped form is what package metadata wants: npm and Wails both reject
# or mangle a leading "v", while git tags and Go module versions require it.
BARE="${VERSION#v}"

step "Bumping version to ${VERSION}"

bump_json() {
  local file="$1"
  # A targeted expression rather than a whole-file rewrite, so that key order
  # and formatting survive and the diff shows one line.
  python3 - "$file" "$BARE" <<'PY'
import re, sys
path, version = sys.argv[1], sys.argv[2]
with open(path) as f:
    text = f.read()
new, n = re.subn(r'("version"\s*:\s*)"[^"]*"', r'\g<1>"%s"' % version, text, count=1)
if n != 1:
    sys.exit(f"no version field found in {path}")
with open(path, "w") as f:
    f.write(new)
print(f"  {path} -> {version}")
PY
}

bump_json package.json
bump_json app/package.json

python3 - "$BARE" <<'PY'
import json, sys, re
version = sys.argv[1]
path = "wails.json"
with open(path) as f:
    text = f.read()
new, n = re.subn(r'("productVersion"\s*:\s*)"[^"]*"', r'\g<1>"%s"' % version, text, count=1)
if n != 1:
    sys.exit("no info.productVersion in wails.json")
with open(path, "w") as f:
    f.write(new)
print(f"  wails.json -> {version}")
PY

# pkg/version's default stays "dev" on purpose. It is the value a build with no
# ldflags reports, and a source tree that claims to be a release is exactly the
# confusion this whole change exists to remove.

step "Verifying the bumped tree builds"
go build ./pkg/... ./cmd/...
npm run build > /dev/null
echo "✔ engine and bundle build at ${VERSION}"

# ─── Tag and push ───────────────────────────────────────────────────────────

if [ "${DRY_RUN}" = true ]; then
  step "Dry run — reverting version bump"
  git checkout -- package.json app/package.json wails.json
  echo "✔ Everything that would gate ${VERSION} passed. Nothing was tagged."
  exit 0
fi

step "Committing and tagging"
git add package.json app/package.json wails.json

# The manifests may already carry this version — most often when a previous
# attempt at the same release committed the bump and then failed further on, as
# the first v0.1.0 did. `git commit` exits non-zero with nothing staged, which
# under `set -e` aborted the script here, after the bump and before the tag: the
# most confusing place to stop, because the tree looks released and nothing is.
# The tag is what marks a release, not the bump commit, so an already-correct
# version is a state to proceed from rather than an error.
if git diff --cached --quiet; then
  echo "  Version manifests already at ${VERSION#v} — nothing to commit."
else
  git commit -m "build(release): ${VERSION}"
fi

git tag -a "${VERSION}" -m "Trawl ${VERSION}"

git push origin HEAD
git push origin "${VERSION}"

cat <<EOF

✔ ${VERSION} tagged and pushed.

  The Release workflow is now building macOS, Windows and Linux artefacts, and
  the Containers workflow is publishing the server and dashboard images. Neither is
  finished, and neither will publish anything if any platform fails.

  Watch:   https://github.com/adedayo/trawl/actions
  Release: https://github.com/adedayo/trawl/releases/tag/${VERSION}
EOF
