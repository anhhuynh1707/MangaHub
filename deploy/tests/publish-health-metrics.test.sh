#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
DEPLOY_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
readonly DEPLOY_DIR

fail() {
  echo "TEST ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this test as root"
for dependency in jq mktemp; do
  command -v "$dependency" >/dev/null || fail "$dependency is required"
done

test_root="$(mktemp -d)"
readonly TEST_ROOT="$test_root"
trap 'rm -rf "$TEST_ROOT"' EXIT

install -d -m 0755 "$TEST_ROOT/mock-bin"
printf '%s\n' 'HOST_HTTP_PORT=8088' > "$TEST_ROOT/runtime.env"
chmod 0600 "$TEST_ROOT/runtime.env"

cat > "$TEST_ROOT/mock-bin/curl" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
if [[ " $* " == *" /latest/api/token "* ]]; then
  printf '%s\n' 'test-imdsv2-token'
else
  printf '%s\n' 'i-0123456789abcdef0'
fi
MOCK

cat > "$TEST_ROOT/mock-bin/docker" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
case "${1:-}" in
  ps)
    if [[ "${MOCK_UNHEALTHY:-0}" == "1" \
      && " $* " == *"com.docker.compose.service=edge"* ]]; then
      exit 0
    fi
    printf '%s\n' '0123456789abcdef'
    ;;
  inspect)
    printf '%s\n' '{"Running":true,"Health":null}'
    ;;
  *)
    echo "unexpected docker call: $*" >&2
    exit 1
    ;;
esac
MOCK

cat > "$TEST_ROOT/mock-bin/aws" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
[[ "${1:-} ${2:-}" == "cloudwatch put-metric-data" ]] || exit 1
shift 2

namespace=""
metric_data=""
while (( $# > 0 )); do
  case "$1" in
    --namespace)
      namespace="$2"
      shift 2
      ;;
    --metric-data)
      metric_data="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

[[ "$namespace" == "MangaHub/EC2" ]]
"$REAL_JQ" --exit-status \
  --argjson expected_application "$MOCK_EXPECT_APPLICATION" \
  --argjson expected_containers "$MOCK_EXPECT_CONTAINERS" \
  '
    length == 2 and
    (map(select(.MetricName == "ApplicationHealthy" and
      .Value == $expected_application and .StorageResolution == 60)) | length == 1) and
    (map(select(.MetricName == "HealthyContainers" and
      .Value == $expected_containers and .StorageResolution == 60)) | length == 1) and
    all(.[]; .Dimensions == [{"Name":"InstanceId","Value":"i-0123456789abcdef0"}])
  ' <<< "$metric_data" >/dev/null
printf '%s\n' "$metric_data" > "$MOCK_METRIC_OUTPUT"
MOCK

cat > "$TEST_ROOT/healthcheck" <<'MOCK'
#!/usr/bin/env bash
set -Eeuo pipefail
[[ "${MOCK_HEALTH_FAIL:-0}" == "0" ]]
MOCK

chmod 0755 "$TEST_ROOT/mock-bin/aws" "$TEST_ROOT/mock-bin/curl" \
  "$TEST_ROOT/mock-bin/docker" "$TEST_ROOT/healthcheck"

export PATH="$TEST_ROOT/mock-bin:$PATH"
export REAL_JQ
REAL_JQ="$(command -v jq)"
export MOCK_METRIC_OUTPUT="$TEST_ROOT/metric.json"
export MANGAHUB_ENV_FILE="$TEST_ROOT/runtime.env"
export MANGAHUB_HEALTHCHECK_SCRIPT="$TEST_ROOT/healthcheck"

export MOCK_EXPECT_APPLICATION=1
export MOCK_EXPECT_CONTAINERS=7
"$DEPLOY_DIR/scripts/publish-health-metrics.sh"

export MOCK_UNHEALTHY=1
export MOCK_EXPECT_APPLICATION=0
export MOCK_EXPECT_CONTAINERS=6
rm -f "$MOCK_METRIC_OUTPUT"
if "$DEPLOY_DIR/scripts/publish-health-metrics.sh"; then
  fail "the unhealthy fixture unexpectedly succeeded"
fi
[[ -s "$MOCK_METRIC_OUTPUT" ]] \
  || fail "the unhealthy fixture failed before publishing its zero metric"
"$REAL_JQ" --exit-status \
  'map(select(.MetricName == "ApplicationHealthy" and .Value == 0)) | length == 1' \
  "$MOCK_METRIC_OUTPUT" >/dev/null

echo "Health metric publisher tests passed"
