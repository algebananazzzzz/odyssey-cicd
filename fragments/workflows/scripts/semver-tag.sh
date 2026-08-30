#!/usr/bin/env bash
# Print the next release tag on stdout; diagnostics go to stderr.
#
#   TITLE   required. Only its first line decides the bump level.
#   BODY    optional. Searched, with TITLE, for BREAKING CHANGE.
#   SUFFIX  optional. "-beta" walks the pre-release ladder.
set -euo pipefail

: "${TITLE:?TITLE is required}"
BODY="${BODY-}"
SUFFIX="${SUFFIX-}"

subject="${TITLE%%$'\n'*}"

git fetch --tags --force >/dev/null 2>&1 || true

# -v:refname mis-orders mixed pre-release and final tags unless
# versionsort.suffix is configured, so glob one suffix only.
if [ -n "$SUFFIX" ]; then
  latest="$(git tag -l "v[0-9]*.[0-9]*.[0-9]*${SUFFIX}" --sort=-v:refname | head -n1)"
else
  latest="$(git tag -l 'v[0-9]*.[0-9]*.[0-9]*' --sort=-v:refname | grep -v -- '-' | head -n1 || true)"
fi

if   [[ "$subject" == *'!:'* || "$TITLE$BODY" == *'BREAKING CHANGE'* ]]; then
  bump=major
elif [[ "$subject" =~ ^feat(\(.+\))?: ]]; then
  bump=minor
else
  bump=patch
fi

if [ -z "$latest" ]; then
  next="v0.1.0${SUFFIX}"
else
  core="${latest#v}"
  core="${core%$SUFFIX}"
  if [[ ! "$core" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Latest tag '$latest' is not vX.Y.Z${SUFFIX}." >&2
    exit 1
  fi
  IFS='.' read -r MAJOR MINOR PATCH <<< "$core"

  case "$bump" in
    major) next="v$((MAJOR + 1)).0.0${SUFFIX}" ;;
    minor) next="v${MAJOR}.$((MINOR + 1)).0${SUFFIX}" ;;
    patch) next="v${MAJOR}.${MINOR}.$((PATCH + 1))${SUFFIX}" ;;
  esac
fi

if git rev-parse -q --verify "refs/tags/$next" >/dev/null; then
  echo "$next already exists. A previous release likely failed after tagging." >&2
  exit 1
fi

{ echo "subject:  $subject"
  echo "bump:     $bump"
  echo "previous: ${latest:-<none>}"
  echo "next:     $next"; } >&2

echo "$next"
