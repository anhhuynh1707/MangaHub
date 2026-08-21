#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
readonly STATE_DIR="${MANGAHUB_STATE_DIR:-/var/lib/mangahub-deploy}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

usage() {
  echo "Usage: sudo $0 [full-git-sha] [--with-raw] [--with-cloudwatch]" >&2
}

[[ "${EUID}" -eq 0 ]] || fail "run this command with sudo"
[[ $# -le 3 ]] || { usage; exit 2; }

target_release="${1:-}"
target_mode="base"

if [[ -n "$target_release" ]]; then
  [[ "$target_release" =~ ^[0-9a-f]{40}$ ]] \
    || fail "rollback target is not a full 40-character lowercase Git SHA"
  shift
  echo "Rolling back to sha-$target_release with explicit deployment options..."
  exec "$SCRIPT_DIR/deploy.sh" "$target_release" "$@"
else
  [[ -f "$STATE_DIR/previous-version" ]] || fail "no previous release has been recorded"
  target_release="$(<"$STATE_DIR/previous-version")"
  [[ -f "$STATE_DIR/previous-mode" ]] && target_mode="$(<"$STATE_DIR/previous-mode")"
fi

[[ "$target_release" =~ ^[0-9a-f]{40}$ ]] || fail "rollback target is not a full 40-character lowercase Git SHA"

echo "Rolling back to sha-$target_release..."
case "$target_mode" in
  base)
    exec "$SCRIPT_DIR/deploy.sh" "$target_release"
    ;;
  raw | --with-raw)
    exec "$SCRIPT_DIR/deploy.sh" "$target_release" --with-raw
    ;;
  cloudwatch)
    exec "$SCRIPT_DIR/deploy.sh" "$target_release" --with-cloudwatch
    ;;
  raw-cloudwatch)
    exec "$SCRIPT_DIR/deploy.sh" "$target_release" --with-raw --with-cloudwatch
    ;;
  *)
    fail "recorded rollback mode is invalid: $target_mode"
    ;;
esac
