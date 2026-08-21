#!/usr/bin/env bash
set -Eeuo pipefail

readonly BASE_URL="${1:-${PUBLIC_ORIGIN:-http://127.0.0.1}}"
readonly ATTEMPTS="${HEALTHCHECK_ATTEMPTS:-30}"
readonly DELAY_SECONDS="${HEALTHCHECK_DELAY_SECONDS:-3}"

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "$BASE_URL" =~ ^https?://[^[:space:]]+$ ]] || fail "health URL must start with http:// or https://"
[[ "$ATTEMPTS" =~ ^[1-9][0-9]*$ ]] || fail "HEALTHCHECK_ATTEMPTS must be a positive integer"
[[ "$DELAY_SECONDS" =~ ^[0-9]+$ ]] || fail "HEALTHCHECK_DELAY_SECONDS must be a non-negative integer"

readonly ORIGIN="${BASE_URL%/}"
TEMP_DIR="$(mktemp -d)"
readonly TEMP_DIR
trap 'rm -r -- "$TEMP_DIR"' EXIT

ready=false
for ((attempt = 1; attempt <= ATTEMPTS; attempt++)); do
  if curl --fail --silent --show-error --connect-timeout 5 --max-time 10 \
      --dump-header "$TEMP_DIR/headers" --output "$TEMP_DIR/health.json" \
      "$ORIGIN/health"; then
    if jq --exit-status '.success == true and .data.status == "healthy"' \
        "$TEMP_DIR/health.json" >/dev/null; then
      ready=true
      break
    fi
  fi

  echo "Health attempt $attempt/$ATTEMPTS failed; retrying in ${DELAY_SECONDS}s..."
  sleep "$DELAY_SECONDS"
done

[[ "$ready" == true ]] || fail "$ORIGIN/health did not become ready"

grep -iq '^X-Content-Type-Options: nosniff' "$TEMP_DIR/headers" \
  || fail "public response is missing X-Content-Type-Options"
grep -iq '^X-Frame-Options: DENY' "$TEMP_DIR/headers" \
  || fail "public response is missing X-Frame-Options"

private_health_status="$(curl --silent --show-error --connect-timeout 5 --max-time 10 --output /dev/null \
  --write-out '%{http_code}' "$ORIGIN/api/health/db")"
[[ "$private_health_status" == "404" ]] || fail "detailed health endpoint returned $private_health_status instead of 404"

swagger_status="$(curl --silent --show-error --connect-timeout 5 --max-time 10 --output /dev/null \
  --write-out '%{http_code}' "$ORIGIN/api/swagger/index.html")"
[[ "$swagger_status" == "404" ]] || fail "public Swagger endpoint returned $swagger_status instead of 404"

root_status="$(curl --silent --show-error --connect-timeout 5 --max-time 10 --output /dev/null \
  --write-out '%{http_code}' "$ORIGIN/")"
[[ "$root_status" == "200" ]] || fail "frontend root returned $root_status instead of 200"

echo "MangaHub health gate passed: $ORIGIN"
