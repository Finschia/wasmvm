#!/bin/bash
set -o errexit -o nounset -o pipefail
command -v shellcheck >/dev/null && shellcheck "$0"

gnused="$(command -v gsed || echo sed)"

function print_usage() {
  echo "Usage: $0 NEW_VERSION"
  echo ""
  echo "e.g. $0 0.8.0"
}

if [ "$#" -ne 1 ]; then
  print_usage
  exit 1
fi

# Check repo
SCRIPT_DIR="$(realpath "$(dirname "$0")")"
if [[ "$(realpath "$SCRIPT_DIR/..")" != "$(pwd)" ]]; then
  echo "Script must be called from the repo root"
  exit 2
fi

# Ensure repo is not dirty
CHANGES_IN_REPO=$(git status --porcelain)
if [[ -n "$CHANGES_IN_REPO" ]]; then
  echo "Repository is dirty. Showing 'git status' and 'git --no-pager diff' for debugging now:"
  git status && git --no-pager diff
  exit 3
fi

NEW="$1"
OLD=$(cargo tree -i wasmvm --manifest-path libwasmvm/Cargo.toml | grep -oE "[0-9]+(\.[0-9]+){2}[\+\-][0-9]+(\.[0-9]+){2}(-[0-9a-zA-Z.]+)*(\+[0-9a-zA-Z.\-]+)*")
echo "Updating old version $OLD to new version $NEW ..."

CARGO_TOML="libwasmvm/Cargo.toml"
"$gnused" -i -e "s/version[[:space:]]*=[[:space:]]*\"$OLD\"/version = \"$NEW\"/" "$CARGO_TOML"

cargo check --manifest-path libwasmvm/Cargo.toml
