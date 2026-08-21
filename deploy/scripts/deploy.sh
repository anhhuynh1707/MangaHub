#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
DEPLOY_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
readonly DEPLOY_DIR
readonly ENV_FILE="${MANGAHUB_ENV_FILE:-/opt/mangahub/.env}"
readonly STATE_DIR="${MANGAHUB_STATE_DIR:-/var/lib/mangahub-deploy}"
readonly COMPOSE_PROJECT="${MANGAHUB_COMPOSE_PROJECT:-mangahub-prod}"
readonly BACKEND_REPOSITORY="${MANGAHUB_BACKEND_REPOSITORY:-ghcr.io/anhhuynh1707/mangahub}"
readonly FRONTEND_REPOSITORY="${MANGAHUB_FRONTEND_REPOSITORY:-ghcr.io/anhhuynh1707/mangahub-frontend}"

usage() {
  echo "Usage: sudo $0 <full-git-sha> [--with-raw] [--with-cloudwatch]" >&2
}

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this command with sudo"
[[ $# -ge 1 && $# -le 3 ]] || { usage; exit 2; }

readonly RELEASE_SHA="$1"
[[ "$RELEASE_SHA" =~ ^[0-9a-f]{40}$ ]] || fail "release must be a full 40-character lowercase Git SHA"
shift

raw_enabled=false
cloudwatch_enabled=false
for option in "$@"; do
  case "$option" in
    --with-raw)
      [[ "$raw_enabled" == false ]] || fail "--with-raw was provided more than once"
      raw_enabled=true
      ;;
    --with-cloudwatch)
      [[ "$cloudwatch_enabled" == false ]] || fail "--with-cloudwatch was provided more than once"
      cloudwatch_enabled=true
      ;;
    *)
      usage
      exit 2
      ;;
  esac
done
readonly RAW_ENABLED="$raw_enabled"
readonly CLOUDWATCH_ENABLED="$cloudwatch_enabled"

[[ "$COMPOSE_PROJECT" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || fail "MANGAHUB_COMPOSE_PROJECT is invalid"
[[ -f "$ENV_FILE" ]] || fail "missing $ENV_FILE; run configure-server.sh first"
[[ ! -L "$ENV_FILE" ]] || fail "$ENV_FILE must not be a symbolic link"
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || fail "$ENV_FILE must have mode 600"
[[ "$(stat -c '%u' "$ENV_FILE")" == "0" ]] || fail "$ENV_FILE must be owned by root"
grep -qE '^JWT_SECRET=.{64,}$' "$ENV_FILE" || fail "JWT_SECRET is missing or too short"

for dependency in docker curl jq flock; do
  command -v "$dependency" >/dev/null || fail "$dependency is required"
done

public_origin="$(awk -F= '$1 == "PUBLIC_ORIGIN" {sub(/^[^=]*=/, ""); print; exit}' "$ENV_FILE")"
[[ "$public_origin" =~ ^https?://[^[:space:]]+$ ]] || fail "PUBLIC_ORIGIN in $ENV_FILE is invalid"

host_http_port="$(awk -F= '$1 == "HOST_HTTP_PORT" {print $2; exit}' "$ENV_FILE")"
[[ "$host_http_port" =~ ^[1-9][0-9]{0,4}$ ]] || fail "HOST_HTTP_PORT in $ENV_FILE is invalid"
(( host_http_port <= 65535 )) || fail "HOST_HTTP_PORT in $ENV_FILE exceeds 65535"
readonly HEALTH_URL="${MANGAHUB_HEALTH_URL:-http://127.0.0.1:${host_http_port}}"

umask 077
install -d -m 0700 "$STATE_DIR"
exec 9>"$STATE_DIR/deploy.lock"
flock --nonblock 9 || fail "another deployment is already running"

export MANGAHUB_IMAGE="${BACKEND_REPOSITORY}:sha-${RELEASE_SHA}"
export MANGAHUB_FRONTEND_IMAGE="${FRONTEND_REPOSITORY}:sha-${RELEASE_SHA}"

compose_files=(
  -f "$DEPLOY_DIR/docker/docker-compose.prod.yml"
)
mode_name="base"
if [[ "$RAW_ENABLED" == true ]]; then
  compose_files+=( -f "$DEPLOY_DIR/docker/docker-compose.raw.yml" )
  mode_name="raw"
fi
if [[ "$CLOUDWATCH_ENABLED" == true ]]; then
  compose_files+=( -f "$DEPLOY_DIR/docker/docker-compose.cloudwatch.yml" )
  if [[ "$RAW_ENABLED" == true ]]; then
    mode_name="raw-cloudwatch"
  else
    mode_name="cloudwatch"
  fi
fi

compose() {
  docker compose --project-name "$COMPOSE_PROJECT" --env-file "$ENV_FILE" \
    "${compose_files[@]}" "$@"
}

current_release=""
current_mode="base"
[[ -f "$STATE_DIR/current-version" ]] && current_release="$(<"$STATE_DIR/current-version")"
[[ -f "$STATE_DIR/current-mode" ]] && current_mode="$(<"$STATE_DIR/current-mode")"

if [[ -n "$current_release" && "$current_release" != "$RELEASE_SHA" ]]; then
  printf '%s\n' "$current_release" > "$STATE_DIR/previous-version"
  printf '%s\n' "$current_mode" > "$STATE_DIR/previous-mode"
fi

echo "Pulling MangaHub release sha-$RELEASE_SHA..."
compose config --quiet
compose pull

echo "Starting MangaHub in $mode_name mode without deleting volumes..."
compose up --detach --remove-orphans

services=(edge frontend mangahub-api redis mangahub-tcp mangahub-udp mangahub-grpc)
for service in "${services[@]}"; do
  container_id="$(compose ps --quiet "$service")"
  [[ -n "$container_id" ]] || fail "$service has no container after deployment"
  [[ "$(docker inspect --format '{{.State.Running}}' "$container_id")" == "true" ]] \
    || fail "$service is not running after deployment"
done

if ! "$SCRIPT_DIR/healthcheck.sh" "$HEALTH_URL"; then
  compose ps >&2 || true
  compose logs --tail 100 >&2 || true
  fail "release sha-$RELEASE_SHA failed its health gate; current-version was not advanced"
fi

backend_image_id="$(docker image inspect --format '{{.Id}}' "$MANGAHUB_IMAGE")"
frontend_image_id="$(docker image inspect --format '{{.Id}}' "$MANGAHUB_FRONTEND_IMAGE")"

printf '%s\n' "$RELEASE_SHA" > "$STATE_DIR/current-version"
printf '%s\n' "$mode_name" > "$STATE_DIR/current-mode"
printf '%s\t%s\t%s\t%s\t%s\n' \
  "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$RELEASE_SHA" "$mode_name" \
  "$backend_image_id" "$frontend_image_id" >> "$STATE_DIR/deployments.tsv"

echo "Deployment passed: sha-$RELEASE_SHA ($mode_name mode)"
echo "State recorded in $STATE_DIR"
