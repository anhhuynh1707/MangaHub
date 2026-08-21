#!/usr/bin/env bash
set -Eeuo pipefail

readonly ENV_FILE="${MANGAHUB_ENV_FILE:-/opt/mangahub/.env}"
readonly COMPOSE_PROJECT="${MANGAHUB_COMPOSE_PROJECT:-mangahub-prod}"
readonly AWS_REGION="${MANGAHUB_AWS_REGION:-ap-southeast-2}"
readonly METRIC_NAMESPACE="${MANGAHUB_CLOUDWATCH_NAMESPACE:-MangaHub/EC2}"
readonly HEALTHCHECK_SCRIPT="${MANGAHUB_HEALTHCHECK_SCRIPT:-/usr/local/libexec/mangahub/healthcheck.sh}"
readonly EXPECTED_CONTAINER_COUNT=7
readonly EXPECTED_SERVICES=(
  edge
  frontend
  mangahub-api
  redis
  mangahub-tcp
  mangahub-udp
  mangahub-grpc
)

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this command as root"
[[ "$AWS_REGION" == "ap-southeast-2" ]] || fail "monitoring is locked to ap-southeast-2"
[[ "$METRIC_NAMESPACE" == "MangaHub/EC2" ]] || fail "unexpected CloudWatch namespace"
[[ "$COMPOSE_PROJECT" =~ ^[a-z0-9][a-z0-9_-]*$ ]] || fail "MANGAHUB_COMPOSE_PROJECT is invalid"
[[ -f "$ENV_FILE" && ! -L "$ENV_FILE" ]] || fail "missing or unsafe $ENV_FILE"
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || fail "$ENV_FILE must have mode 600"
[[ "$(stat -c '%u' "$ENV_FILE")" == "0" ]] || fail "$ENV_FILE must be owned by root"
[[ -x "$HEALTHCHECK_SCRIPT" && ! -L "$HEALTHCHECK_SCRIPT" ]] || fail "healthcheck helper is missing or unsafe"

for dependency in aws curl docker jq; do
  command -v "$dependency" >/dev/null || fail "$dependency is required"
done

host_http_port="$(awk -F= '$1 == "HOST_HTTP_PORT" {print $2; exit}' "$ENV_FILE")"
[[ "$host_http_port" =~ ^[1-9][0-9]{0,4}$ ]] || fail "HOST_HTTP_PORT in $ENV_FILE is invalid"
(( host_http_port <= 65535 )) || fail "HOST_HTTP_PORT in $ENV_FILE exceeds 65535"
readonly HEALTH_URL="${MANGAHUB_HEALTH_URL:-http://127.0.0.1:${host_http_port}}"

imds_token="$(curl --fail --silent --show-error --max-time 3 \
  --request PUT --header 'X-aws-ec2-metadata-token-ttl-seconds: 60' \
  http://169.254.169.254/latest/api/token)" || fail "IMDSv2 token request failed"
trap 'unset imds_token' EXIT
instance_id="$(curl --fail --silent --show-error --max-time 3 \
  --header "X-aws-ec2-metadata-token: $imds_token" \
  http://169.254.169.254/latest/meta-data/instance-id)" || fail "instance ID request failed"
[[ "$instance_id" =~ ^i-[0-9a-f]+$ ]] || fail "IMDS returned an invalid instance ID"
readonly INSTANCE_ID="$instance_id"
unset imds_token instance_id

healthy_containers=0
for service in "${EXPECTED_SERVICES[@]}"; do
  container_id="$(docker ps --quiet \
    --filter "label=com.docker.compose.project=$COMPOSE_PROJECT" \
    --filter "label=com.docker.compose.service=$service")"
  if [[ "$container_id" =~ ^[0-9a-f]+$ ]]; then
    state_json="$(docker inspect --format '{{json .State}}' "$container_id")"
    if jq --exit-status \
        '.Running == true and (.Health == null or .Health.Status == "healthy")' \
        <<< "$state_json" >/dev/null; then
      ((healthy_containers += 1))
    fi
  fi
done

public_health=0
if HEALTHCHECK_ATTEMPTS=1 HEALTHCHECK_DELAY_SECONDS=0 \
    "$HEALTHCHECK_SCRIPT" "$HEALTH_URL" >/dev/null 2>&1; then
  public_health=1
fi

application_healthy=0
if (( public_health == 1 && healthy_containers == EXPECTED_CONTAINER_COUNT )); then
  application_healthy=1
fi

metric_data="$(jq --null-input --compact-output \
  --arg instance_id "$INSTANCE_ID" \
  --argjson application_healthy "$application_healthy" \
  --argjson healthy_containers "$healthy_containers" \
  '[
    {
      MetricName: "ApplicationHealthy",
      Dimensions: [{Name: "InstanceId", Value: $instance_id}],
      Unit: "Count",
      Value: $application_healthy,
      StorageResolution: 60
    },
    {
      MetricName: "HealthyContainers",
      Dimensions: [{Name: "InstanceId", Value: $instance_id}],
      Unit: "Count",
      Value: $healthy_containers,
      StorageResolution: 60
    }
  ]')"

aws cloudwatch put-metric-data \
  --region "$AWS_REGION" \
  --namespace "$METRIC_NAMESPACE" \
  --metric-data "$metric_data"

echo "Published MangaHub health metrics: application=$application_healthy containers=$healthy_containers/$EXPECTED_CONTAINER_COUNT"
(( application_healthy == 1 )) || fail "MangaHub is unhealthy; a zero metric was published"
