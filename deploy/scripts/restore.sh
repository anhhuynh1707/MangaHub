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
readonly RETENTION="${MANGAHUB_BACKUP_RETENTION:-7}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this command with sudo"
[[ $# -eq 1 ]] || fail "Usage: sudo $0 <backup-filename>"
readonly TARGET_NAME="$1"
[[ "$TARGET_NAME" =~ ^mangahub-[0-9]{8}T[0-9]{6}Z(-pre-restore)?-sha-[0-9a-f]{40}\.db$ ]] \
  || fail "backup must be a timestamped MangaHub full-SHA filename, not a path"
[[ "$RETENTION" =~ ^[1-9][0-9]*$ ]] || fail "MANGAHUB_BACKUP_RETENTION must be a positive integer"
[[ "$COMPOSE_PROJECT" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || fail "MANGAHUB_COMPOSE_PROJECT is invalid"
[[ -f "$ENV_FILE" && ! -L "$ENV_FILE" ]] || fail "missing or unsafe $ENV_FILE"
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || fail "$ENV_FILE must have mode 600"
[[ "$(stat -c '%u' "$ENV_FILE")" == "0" ]] || fail "$ENV_FILE must be owned by root"
[[ -f "$STATE_DIR/current-version" ]] || fail "no healthy deployed release is recorded"

for dependency in docker curl jq flock; do
  command -v "$dependency" >/dev/null || fail "$dependency is required"
done

umask 077
install -d -m 0700 "$STATE_DIR"
exec 8>"$STATE_DIR/backup.lock"
flock --nonblock 8 || fail "another backup or restore operation is running"
exec 9>"$STATE_DIR/deploy.lock"
flock --nonblock 9 || fail "a deployment or rollback is running"

release_sha="$(<"$STATE_DIR/current-version")"
release_mode="base"
[[ -f "$STATE_DIR/current-mode" ]] && release_mode="$(<"$STATE_DIR/current-mode")"
[[ "$release_sha" =~ ^[0-9a-f]{40}$ ]] || fail "recorded release is not a full Git SHA"
[[ "$release_mode" == "base" || "$release_mode" == "raw" \
  || "$release_mode" == "cloudwatch" || "$release_mode" == "raw-cloudwatch" ]] \
  || fail "recorded release mode is invalid"
readonly RELEASE_SHA="$release_sha"
readonly RELEASE_MODE="$release_mode"

host_http_port="$(awk -F= '$1 == "HOST_HTTP_PORT" {print $2; exit}' "$ENV_FILE")"
[[ "$host_http_port" =~ ^[1-9][0-9]{0,4}$ ]] || fail "HOST_HTTP_PORT in $ENV_FILE is invalid"
(( host_http_port <= 65535 )) || fail "HOST_HTTP_PORT in $ENV_FILE exceeds 65535"
readonly HEALTH_URL="${MANGAHUB_HEALTH_URL:-http://127.0.0.1:${host_http_port}}"

export MANGAHUB_IMAGE="${BACKEND_REPOSITORY}:sha-${RELEASE_SHA}"
export MANGAHUB_FRONTEND_IMAGE="${FRONTEND_REPOSITORY}:sha-${RELEASE_SHA}"

compose_files=( -f "$DEPLOY_DIR/docker/docker-compose.prod.yml" )
if [[ "$RELEASE_MODE" == "raw" || "$RELEASE_MODE" == "raw-cloudwatch" ]]; then
  compose_files+=( -f "$DEPLOY_DIR/docker/docker-compose.raw.yml" )
fi
if [[ "$RELEASE_MODE" == "cloudwatch" || "$RELEASE_MODE" == "raw-cloudwatch" ]]; then
  compose_files+=( -f "$DEPLOY_DIR/docker/docker-compose.cloudwatch.yml" )
fi

compose() {
  docker compose --project-name "$COMPOSE_PROJECT" --env-file "$ENV_FILE" \
    "${compose_files[@]}" "$@"
}

db_admin() {
  compose --profile operations run --rm --no-deps db-admin "$@"
}

show_diagnostics() {
  compose ps >&2 || true
  compose logs --tail 100 >&2 || true
}

services_stopped=false
restore_applied=false
pre_restore_name=""

recover_on_failure() {
  status=$?
  trap - EXIT
  [[ "$status" -ne 0 ]] || exit 0
  set +e

  if [[ "$restore_applied" == true && -n "$pre_restore_name" ]]; then
    echo "Restore failed after replacement; recovering the pre-restore database..." >&2
    compose stop edge mangahub-api mangahub-tcp mangahub-udp mangahub-grpc >&2
    if ! db_admin restore --name "$pre_restore_name" >&2; then
      echo "CRITICAL: automatic database recovery failed; keep all volumes and inspect manually" >&2
    fi
  fi

  if [[ "$services_stopped" == true || "$restore_applied" == true ]]; then
    echo "Restarting the recorded application release after restore failure..." >&2
    compose up --detach --remove-orphans >&2
    "$SCRIPT_DIR/healthcheck.sh" "$HEALTH_URL" >&2
  fi
  exit "$status"
}
trap recover_on_failure EXIT

echo "Preparing and verifying the backup volume..."
compose --profile operations run --rm --no-deps backup-init
db_admin verify --name "$TARGET_NAME"

timestamp="$(date -u +'%Y%m%dT%H%M%SZ')"
pre_restore_name="mangahub-${timestamp}-pre-restore-sha-${RELEASE_SHA}.db"
echo "Creating automatic pre-restore recovery point: $pre_restore_name"
db_admin backup --name "$pre_restore_name"

services_stopped=true
echo "Stopping every database-using service before atomic restore..."
compose stop edge mangahub-api mangahub-tcp mangahub-udp mangahub-grpc

restore_applied=true
db_admin restore --name "$TARGET_NAME"

echo "Restarting sha-$RELEASE_SHA in $RELEASE_MODE mode..."
compose up --detach --remove-orphans

services=(edge frontend mangahub-api redis mangahub-tcp mangahub-udp mangahub-grpc)
for service in "${services[@]}"; do
  container_id="$(compose ps --quiet "$service")"
  [[ -n "$container_id" ]] || fail "$service has no container after restore"
  [[ "$(docker inspect --format '{{.State.Running}}' "$container_id")" == "true" ]] \
    || fail "$service is not running after restore"
done

if ! "$SCRIPT_DIR/healthcheck.sh" "$HEALTH_URL"; then
  show_diagnostics
  fail "restored database failed the application health gate"
fi

db_admin verify --name "$TARGET_NAME"
db_admin prune --keep "$RETENTION"
printf '%s\t%s\t%s\t%s\n' \
  "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$RELEASE_SHA" "$TARGET_NAME" "$pre_restore_name" \
  >> "$STATE_DIR/restores.tsv"

services_stopped=false
restore_applied=false
trap - EXIT

echo "Restore passed: $TARGET_NAME"
echo "Application release preserved: sha-$RELEASE_SHA ($RELEASE_MODE mode)"
echo "Automatic recovery point: $pre_restore_name"
