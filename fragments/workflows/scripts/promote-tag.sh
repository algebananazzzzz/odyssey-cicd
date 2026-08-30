#!/usr/bin/env bash
# Strip the pre-release suffix from TAG, tag the same commit with the result and
# push it. Prints the new tag; diagnostics go to stderr. Idempotent.
#
#   TAG     required. The vX.Y.Z-beta tag that passed preprod.
#   SUFFIX  optional, default -beta.
set -euo pipefail

: "${TAG:?TAG is required}"
SUFFIX="${SUFFIX--beta}"

git fetch --tags --force >/dev/null 2>&1 || true

prd="${TAG%"$SUFFIX"}"
if [ "$prd" = "$TAG" ] || [[ ! "$prd" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "'$TAG' is not a vX.Y.Z${SUFFIX} tag." >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$prd" >/dev/null; then
  echo "$prd already exists, reusing it." >&2
else
  git tag "$prd" "${TAG}^{commit}"
  git push origin "$prd"
fi

echo "$prd"
