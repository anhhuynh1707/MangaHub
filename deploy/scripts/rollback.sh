#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
readonly STATE_DIR="${MANGAHUB_STATE_DIR:-/var/lib/mangahub-deploy}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this command with sudo"

target_release="${1:-}"
target_mode="base"

if [[ -z "$target_release" ]]; then
  [[ -f "$STATE_DIR/previous-version" ]] || fail "no previous release has been recorded"
  target_release="$(<"$STATE_DIR/previous-version")"
  [[ -f "$STATE_DIR/previous-mode" ]] && target_mode="$(<"$STATE_DIR/previous-mode")"
else
  [[ $# -le 2 ]] || fail "Usage: sudo $0 [full-git-sha] [--with-raw]"
  target_mode="${2:-base}"
fi

[[ "$target_release" =~ ^[0-9a-f]{40}$ ]] || fail "rollback target is not a full 40-character lowercase Git SHA"
[[ "$target_mode" == "base" || "$target_mode" == "raw" || "$target_mode" == "--with-raw" ]] \
  || fail "rollback mode must be base or --with-raw"

echo "Rolling back to sha-$target_release..."
if [[ "$target_mode" == "raw" || "$target_mode" == "--with-raw" ]]; then
  exec "$SCRIPT_DIR/deploy.sh" "$target_release" --with-raw
fi
exec "$SCRIPT_DIR/deploy.sh" "$target_release"
