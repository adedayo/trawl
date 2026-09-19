#!/usr/bin/env bash
#
# Fails if a live specification names a datastore the engine does not use.
#
# Change 003 replaced Convex with an embedded SQLite store. The code moved and
# the specifications did not, and nobody noticed for a long time — because
# nothing was watching. A requirement naming a component that no longer exists
# is not merely stale: it is unfalsifiable, which is worse, because it goes on
# looking like a requirement while asserting nothing. The scope-authorization
# spec drifted this way, and that is the spec describing where the record
# behind the tool's central guardrail is persisted.
#
# Only `openspec/specs/` and the *live* capability specs are checked.
# Historical references under `changes/` that explain what was migrated away
# from are deliberately permitted: erasing them would destroy the reasoning
# behind a decision future readers would otherwise have to reconstruct.

set -euo pipefail

cd "$(dirname "$0")/.."

# Fail loudly if the tree is not where we think it is. Without this, a wrong
# working directory makes grep match nothing and the check reports success —
# a guard that passes because it looked in the wrong place, which is precisely
# the failure mode it exists to catch.
if [ ! -d openspec ]; then
  echo "check-spec-datastore: cannot find openspec/ from $(pwd)" >&2
  exit 2
fi

# Datastores the engine has never used, or has stopped using. Add to this list
# when a storage decision is superseded, at the same time as the migration —
# not afterwards, which is the mistake this script exists to catch.
FORBIDDEN='Convex|convex'

# Files whose references are historical and are allowed to keep them.
#
# Matched on the change's own name rather than its full path, so that archiving
# a completed change does not turn its historical references into a CI failure.
# The first version of this list was path-anchored and broke the moment three
# changes were archived — a guard that fails on housekeeping teaches people to
# route around it, which costs more than the drift it catches.
ALLOWLIST='.*/001-initial-build/(design|proposal|tasks)\.md
.*/002-susceptibility-scoring-integration/.*
.*/003-go-sqlite-engine/.*
.*/005-cloud-continuous-easm/tasks\.md
.*/014-operator-dashboard/.*
.*/015-spec-datastore-hygiene/.*'

matches=$(grep -rlE "$FORBIDDEN" openspec/ 2>/dev/null || true)

offenders=""
for file in $matches; do
  allowed=false
  while IFS= read -r pattern; do
    [ -z "$pattern" ] && continue
    if [[ "$file" =~ ^${pattern}$ ]]; then
      allowed=true
      break
    fi
  done <<< "$ALLOWLIST"
  if [ "$allowed" = false ]; then
    offenders="${offenders}${file}\n"
  fi
done

if [ -n "$offenders" ]; then
  echo "Specification drift: these files name a datastore the engine does not use."
  echo
  printf "%b" "$offenders"
  echo
  echo "If the reference is historical — explaining a migration — add the file to"
  echo "ALLOWLIST in $0 with a note saying why. If it is a live requirement, it"
  echo "describes a component that does not exist and cannot be verified against"
  echo "anything; rewrite it against the implementation."
  exit 1
fi

echo "Specification datastore check: no live spec names a superseded datastore."
