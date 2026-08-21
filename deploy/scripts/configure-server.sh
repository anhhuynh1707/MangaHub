#!/usr/bin/env bash
set -Eeuo pipefail

readonly BACKEND_REPOSITORY="${MANGAHUB_BACKEND_REPOSITORY:-ghcr.io/anhhuynh1707/mangahub}"
readonly FRONTEND_REPOSITORY="${MANGAHUB_FRONTEND_REPOSITORY:-ghcr.io/anhhuynh1707/mangahub-frontend}"
readonly ENV_FILE="${MANGAHUB_ENV_FILE:-/opt/mangahub/.env}"

usage() {
  echo "Usage: sudo $0 <full-git-sha> <public-ipv4>" >&2
}

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this command with sudo"
[[ $# -eq 2 ]] || { usage; exit 2; }

readonly RELEASE_SHA="$1"
readonly PUBLIC_IPV4="$2"

[[ "$RELEASE_SHA" =~ ^[0-9a-f]{40}$ ]] || fail "release must be a full 40-character lowercase Git SHA"
[[ "$PUBLIC_IPV4" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}$ ]] || fail "public address must be an IPv4 address"

IFS=. read -r octet1 octet2 octet3 octet4 <<< "$PUBLIC_IPV4"
for octet in "$octet1" "$octet2" "$octet3" "$octet4"; do
  (( 10#$octet >= 0 && 10#$octet <= 255 )) || fail "public address contains an invalid IPv4 octet"
done

[[ ! -e "$ENV_FILE" ]] || fail "$ENV_FILE already exists; refusing to replace its JWT secret"
command -v openssl >/dev/null || fail "openssl is required"

install -d -m 0750 "$(dirname "$ENV_FILE")"
umask 077
jwt_secret="$(openssl rand -hex 48)"

{
  printf 'MANGAHUB_IMAGE=%s:sha-%s\n' "$BACKEND_REPOSITORY" "$RELEASE_SHA"
  printf 'MANGAHUB_FRONTEND_IMAGE=%s:sha-%s\n' "$FRONTEND_REPOSITORY" "$RELEASE_SHA"
  printf 'JWT_SECRET=%s\n' "$jwt_secret"
  printf 'HOST_HTTP_PORT=80\n'
  printf 'PUBLIC_ORIGIN=http://%s\n' "$PUBLIC_IPV4"
  printf 'LOG_LEVEL=info\n'
  printf 'RAW_BIND_ADDRESS=0.0.0.0\n'
} > "$ENV_FILE"

chmod 0600 "$ENV_FILE"
unset jwt_secret

echo "Created protected MangaHub environment: $ENV_FILE"
echo "Public origin: http://$PUBLIC_IPV4"
echo "The JWT secret was generated directly into the file and was not printed."
