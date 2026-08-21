#!/usr/bin/env bash
set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
DEPLOY_DIR="$(cd -- "$SCRIPT_DIR/.." && pwd)"
readonly DEPLOY_DIR
readonly ENV_FILE="${MANGAHUB_ENV_FILE:-/opt/mangahub/.env}"
readonly AWS_REGION="${MANGAHUB_AWS_REGION:-ap-southeast-2}"
readonly LOG_GROUP="${MANGAHUB_CLOUDWATCH_LOG_GROUP:-/mangahub/demo/containers}"
readonly LOG_RETENTION_DAYS=7
readonly AGENT_CONFIG=/opt/aws/amazon-cloudwatch-agent/etc/cloudwatch-agent.json

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

[[ "${EUID}" -eq 0 ]] || fail "run this command with sudo"
[[ $# -eq 0 ]] || fail "Usage: sudo $0"
[[ "$AWS_REGION" == "ap-southeast-2" ]] || fail "monitoring is locked to ap-southeast-2"
[[ "$LOG_GROUP" == "/mangahub/demo/containers" ]] || fail "unexpected CloudWatch log group"
[[ -f "$ENV_FILE" && ! -L "$ENV_FILE" ]] || fail "missing or unsafe $ENV_FILE"
[[ "$(stat -c '%a' "$ENV_FILE")" == "600" ]] || fail "$ENV_FILE must have mode 600"
[[ "$(stat -c '%u' "$ENV_FILE")" == "0" ]] || fail "$ENV_FILE must be owned by root"

for dependency in aws curl docker dnf jq systemctl; do
  command -v "$dependency" >/dev/null || fail "$dependency is required"
done

[[ -r /etc/os-release ]] || fail "cannot identify the operating system"
# shellcheck disable=SC1091
source /etc/os-release
[[ "${ID:-}" == "amzn" && "${VERSION_ID:-}" == "2023" ]] \
  || fail "this setup supports Amazon Linux 2023 only"

caller_arn="$(aws sts get-caller-identity --region "$AWS_REGION" \
  --query Arn --output text 2>/dev/null)" \
  || fail "instance-role credentials are unavailable"
[[ "$caller_arn" == *":assumed-role/MangaHubDemoEC2Role/"* ]] \
  || fail "EC2 must use MangaHubDemoEC2Role"
unset caller_arn

log_groups="$(aws logs describe-log-groups --region "$AWS_REGION" \
  --log-group-name-prefix "$LOG_GROUP" --output json)" \
  || fail "cannot inspect the CloudWatch log group"
jq --exit-status --arg name "$LOG_GROUP" --argjson retention "$LOG_RETENTION_DAYS" \
  '.logGroups[] | select(.logGroupName == $name and .retentionInDays == $retention)' \
  <<< "$log_groups" >/dev/null \
  || fail "$LOG_GROUP must exist with exactly ${LOG_RETENTION_DAYS}-day retention"
unset log_groups

echo "Installing the Amazon Linux CloudWatch agent package..."
dnf install -y amazon-cloudwatch-agent

install -d -m 0755 "$(dirname "$AGENT_CONFIG")"
install -m 0644 "$DEPLOY_DIR/cloudwatch/amazon-cloudwatch-agent.json" "$AGENT_CONFIG"

install -d -m 0755 /usr/local/libexec/mangahub
install -m 0755 "$SCRIPT_DIR/healthcheck.sh" /usr/local/libexec/mangahub/healthcheck.sh
install -m 0755 "$SCRIPT_DIR/publish-health-metrics.sh" /usr/local/sbin/mangahub-publish-health-metrics
install -m 0644 "$DEPLOY_DIR/systemd/mangahub-health-publisher.service" /etc/systemd/system/mangahub-health-publisher.service
install -m 0644 "$DEPLOY_DIR/systemd/mangahub-health-publisher.timer" /etc/systemd/system/mangahub-health-publisher.timer

/opt/aws/amazon-cloudwatch-agent/bin/amazon-cloudwatch-agent-ctl \
  -a fetch-config -m ec2 -s -c "file:$AGENT_CONFIG"

systemctl daemon-reload
systemctl enable --now mangahub-health-publisher.timer
systemctl start mangahub-health-publisher.service

systemctl is-active --quiet amazon-cloudwatch-agent \
  || fail "CloudWatch agent did not become active"
systemctl is-active --quiet mangahub-health-publisher.timer \
  || fail "health publisher timer did not become active"

echo "MangaHub monitoring setup passed in $AWS_REGION"
echo "Metrics namespace: MangaHub/EC2"
echo "Container log group: $LOG_GROUP (${LOG_RETENTION_DAYS}-day retention)"
echo "Redeploy the recorded release with --with-cloudwatch to route container logs."
