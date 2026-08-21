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
list_only=false
retention="${MANGAHUB_BACKUP_RETENTION:-7}"
if [[ "${1:-}" == "--list" ]]; then
  list_only=true
elif [[ $# -eq 1 ]]; then
  retention="$1"
fi
readonly RETENTION="$retention"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this command with sudo"
[[ $# -le 1 ]] || fail "Usage: sudo $0 [retention-count|--list]"
[[ "$RETENTION" =~ ^[1-9][0-9]*$ ]] || fail "retention must be a positive integer"
[[ "$COMPOSE_PROJECT" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || fail "MANGAHUB_COMPOSE_PROJECT is invalid"
[[ -f "$ENV_FILE" && ! -L "$ENV_FILE" ]] || fail "missing or unsafe $ENV_FILE"
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || fail "$ENV_FILE must have mode 600"
[[ "$(stat -c '%u' "$ENV_FILE")" == "0" ]] || fail "$ENV_FILE must be owned by root"
[[ -f "$STATE_DIR/current-version" ]] || fail "no healthy deployed release is recorded"

for dependency in docker flock; do
  command -v "$dependency" >/dev/null || fail "$dependency is required"
done

umask 077
install -d -m 0700 "$STATE_DIR"
exec 8>"$STATE_DIR/backup.lock"
flock --nonblock 8 || fail "another backup or restore operation is running"
exec 9>"$STATE_DIR/deploy.lock"
flock --nonblock 9 || fail "a deployment or rollback is running"

release_sha="$(<"$STATE_DIR/current-version")"
[[ "$release_sha" =~ ^[0-9a-f]{40}$ ]] || fail "recorded release is not a full Git SHA"
readonly RELEASE_SHA="$release_sha"

export MANGAHUB_IMAGE="${BACKEND_REPOSITORY}:sha-${RELEASE_SHA}"
export MANGAHUB_FRONTEND_IMAGE="${FRONTEND_REPOSITORY}:sha-${RELEASE_SHA}"

compose() {
  docker compose --project-name "$COMPOSE_PROJECT" --env-file "$ENV_FILE" \
    -f "$DEPLOY_DIR/docker/docker-compose.prod.yml" "$@"
}

db_admin() {
  compose --profile operations run --rm --no-deps db-admin "$@"
}

timestamp="$(date -u +'%Y%m%dT%H%M%SZ')"
readonly BACKUP_NAME="mangahub-${timestamp}-sha-${RELEASE_SHA}.db"

echo "Preparing the root-only backup volume..."
compose --profile operations run --rm --no-deps backup-init

if [[ "$list_only" == true ]]; then
  db_admin list
  exit 0
fi

echo "Creating an online SQLite backup for sha-$RELEASE_SHA..."
db_admin backup --name "$BACKUP_NAME"
db_admin verify --name "$BACKUP_NAME"
db_admin prune --keep "$RETENTION"

printf '%s\t%s\t%s\n' \
  "$(date -u +'%Y-%m-%dT%H:%M:%SZ')" "$RELEASE_SHA" "$BACKUP_NAME" \
  >> "$STATE_DIR/backups.tsv"

echo "Backup completed: $BACKUP_NAME"
echo "Location: Docker volume ${COMPOSE_PROJECT}_mangahub-backups"
echo "Retention: $RETENTION verified backup pair(s)"
