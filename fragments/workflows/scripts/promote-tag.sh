#!/usr/bin/env bash
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
