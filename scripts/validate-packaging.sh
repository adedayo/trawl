#!/usr/bin/env bash
set -euo pipefail

# Trawl — packaging manifest validation.
#
# Packaging defects have a characteristic shape: they are invisible locally,
# invisible in CI, and discovered by a user on a platform the author does not
# own, some hours after a tag was pushed. This script moves the cheap subset of
# those checks — the ones that are just "is this file well-formed and does it
# refer to things that exist" — to a point before the tag.
#
# It does not build packages. It checks the inputs that would be used to.

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}"

FAILURES=0
ok()   { echo "  ✔ $*"; }
fail() { echo "  ✖ $*" >&2; FAILURES=$((FAILURES + 1)); }

echo "Validating packaging manifests"

# ─── Files that must exist ──────────────────────────────────────────────────
for f in \
  build/appicon.png \
  build/darwin/Info.plist \
  build/darwin/Info.dev.plist \
  build/windows/icon.ico \
  build/windows/info.json \
  build/windows/wails.exe.manifest \
  build/windows/installer/project.nsi \
  packaging/linux/nfpm.yaml \
  packaging/linux/trawl.desktop \
  packaging/homebrew/trawl.rb \
  packaging/homebrew/trawl-cli.rb \
  packaging/scoop/trawl.json \
  .github/workflows/release.yml \
  .github/workflows/containers.yml
do
  [ -f "$f" ] && ok "$f" || fail "missing: $f"
done

# ─── Well-formedness ────────────────────────────────────────────────────────
python3 - <<'PY' || exit 1
import json, sys, plistlib, re

failures = 0

def check(label, fn):
    global failures
    try:
        fn()
        print(f"  ✔ {label}")
    except Exception as e:
        print(f"  ✖ {label}: {e}", file=sys.stderr)
        failures += 1

check("build/windows/info.json is valid JSON",
      lambda: json.load(open("build/windows/info.json")))
check("packaging/scoop/trawl.json is valid JSON",
      lambda: json.load(open("packaging/scoop/trawl.json")))

# The plists are Go templates, so they are not parseable as plists. Checking
# the template delimiters balance catches the realistic failure — a hand-edit
# that drops a brace — without pretending to validate what the template emits.
#
# The identifier checks look at <string> values rather than raw text, because
# both files discuss the com.wails.* default in a comment explaining why it is
# not used, and a check that cannot tell an explanation from a defect will
# eventually be silenced rather than fixed.
def plist_templates():
    for p in ("build/darwin/Info.plist", "build/darwin/Info.dev.plist"):
        text = open(p).read()
        if text.count("{{") != text.count("}}"):
            raise ValueError(f"{p}: unbalanced template delimiters")

        values = re.findall(r"<string>(.*?)</string>", text, re.DOTALL)
        if any(v.startswith("com.wails.") for v in values):
            raise ValueError(f"{p}: still uses the com.wails.* default bundle identifier")
        if not any(v.startswith("com.adedayo.trawl") for v in values):
            raise ValueError(f"{p}: missing the com.adedayo.trawl bundle identifier")
check("darwin plists are templated and correctly identified", plist_templates)

try:
    import yaml
except ImportError:
    print("  ~ PyYAML not installed; skipping YAML checks")
    yaml = None

if yaml:
    def workflows():
        for p in (".github/workflows/release.yml", ".github/workflows/containers.yml"):
            d = yaml.safe_load(open(p))
            if not d.get("jobs"):
                raise ValueError(f"{p}: no jobs defined")

    check("release workflows are valid YAML", workflows)

    # nfpm.yaml carries ${VERSION}/${ARCH} placeholders, which are valid YAML
    # scalars, so this parses as-is.
    def nfpm():
        d = yaml.safe_load(open("packaging/linux/nfpm.yaml"))
        for key in ("name", "arch", "version", "contents", "depends"):
            if key not in d:
                raise ValueError(f"missing key: {key}")
        # The binary path must match what `wails build` actually emits, which
        # is derived from outputfilename in wails.json. A mismatch produces an
        # empty package rather than an error.
        expected = json.load(open("wails.json"))["outputfilename"]
        src = d["contents"][0]["src"]
        if not src.endswith(expected):
            raise ValueError(f"nfpm source {src!r} does not match wails outputfilename {expected!r}")
    check("packaging/linux/nfpm.yaml is coherent with wails.json", nfpm)

# The cask is the file most likely to be edited by hand under time pressure,
# and :no_check is the specific edit that quietly removes the only integrity
# check in the macOS install path. Matched as a directive rather than as a
# substring, so that the comment above it explaining the rule does not trip it.
def cask():
    text = open("packaging/homebrew/trawl.rb").read()
    if re.search(r"^\s*sha256\s+:no_check", text, re.MULTILINE):
        raise ValueError("cask uses sha256 :no_check, which disables integrity verification")
    if not re.search(r'^\s*sha256\s+"[0-9a-f]{64}"', text, re.MULTILINE):
        raise ValueError("cask has no 64-character sha256 placeholder or value")
check("homebrew cask verifies its download", cask)

def desktop_entry():
    text = open("packaging/linux/trawl.desktop").read()
    for key in ("Type=", "Name=", "Exec=", "Icon="):
        if key not in text:
            raise ValueError(f"missing {key}")
check("linux desktop entry has the required keys", desktop_entry)

sys.exit(1 if failures else 0)
PY

if [ "${FAILURES}" -ne 0 ]; then
  echo "" >&2
  echo "✖ ${FAILURES} packaging problem(s)." >&2
  exit 1
fi

echo "✔ packaging manifests validate"
