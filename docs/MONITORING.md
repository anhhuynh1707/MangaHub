# MangaHub CloudWatch monitoring

Status: **repository-side monitoring path implemented; first EC2 metrics, logs,
alarm, and failure rehearsal pending**.

The demo uses AWS-native monitoring without storing an access key on EC2:

```text
EC2 basic metrics ─────────────── CPU + status checks
CloudWatch Agent ──────────────── memory + root-disk usage
systemd health timer ──────────── application health + 7 container states
Docker awslogs driver ─────────── application, Redis, raw-service, and Nginx logs
                                      │
                                      ▼
                     CloudWatch metrics, alarms, and 7-day log group
```

Only the explicit `--with-cloudwatch` deployment mode sends container logs to
AWS. Local development and the initial EC2 deployment continue to use Docker's
local logging behavior.

## IAM boundary

Create customer-managed policy `MangaHubDemoCloudWatchPolicy` from
`deploy/aws/MangaHubDemoCloudWatchPolicy.json`, then attach it to the existing
`MangaHubDemoEC2Role`.

The policy can only:

- publish custom metrics to namespace `MangaHub/EC2`;
- inspect log-group configuration; and
- inspect the one configured log group's streams; and
- create and write only the seven named `mangahub-prod/*` streams inside
  `/mangahub/demo/containers` in Sydney.

It cannot create/delete log groups, alter retention, read log events, configure
alarms, use X-Ray, or access another Region's log group. Keep
`AmazonSSMManagedInstanceCore` attached for Session Manager. Do not attach
`CloudWatchAgentAdminPolicy` or create an access key.

This split follows the official [CloudWatch Logs action and resource-type
mapping](https://docs.aws.amazon.com/service-authorization/latest/reference/list_logs.html):
`DescribeLogStreams` is scoped to the log group, while `CreateLogStream` and
`PutLogEvents` are scoped to the exact log-stream ARNs.

## Create the bounded log group

In **CloudWatch → Logs → Log groups**, create exactly:

| Setting | Value |
|---|---|
| Log group | `/mangahub/demo/containers` |
| Log class | Standard |
| Retention | 7 days |
| KMS | AWS-managed encryption |
| Tags | `Project=MangaHub`, `Environment=demo`, `ManagedBy=manual` |

The instance role deliberately cannot create this group or change retention. The
setup script fails closed if the exact group and seven-day policy do not already
exist.

## Install and activate monitoring

Run after the base application deployment is healthy:

```bash
cd /opt/mangahub-src
sudo ./deploy/scripts/configure-monitoring.sh

CURRENT_SHA="$(sudo sed -n '1p' /var/lib/mangahub-deploy/current-version)"
sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" --with-cloudwatch
```

If the three owner-only raw protocol rules are currently reviewed and enabled,
preserve raw mode explicitly:

```bash
sudo ./deploy/scripts/deploy.sh \
  "$CURRENT_SHA" --with-raw --with-cloudwatch
```

The setup installs the Amazon Linux 2023 CloudWatch Agent package, a 60-second
host-metrics configuration, and a hardened systemd timer. It refuses a different
Region, role name, log group, retention period, operating system, unsafe runtime
environment file, or unavailable instance-role credentials.

The configuration follows the official [CloudWatch Agent configuration
reference](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Agent-Configuration-File-Details.html)
and [Amazon Linux command-line installation
procedure](https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/download-CloudWatch-Agent-on-EC2-Instance-commandline-first.html).

Verify on EC2 without printing the protected environment:

```bash
sudo systemctl is-active amazon-cloudwatch-agent
sudo systemctl is-active mangahub-health-publisher.timer
sudo systemctl status mangahub-health-publisher.service --no-pager

sudo docker inspect --format '{{json .HostConfig.LogConfig}}' \
  mangahub-prod-mangahub-api-1 | jq
```

The driver must be `awslogs`, with non-blocking delivery and the expected Sydney
group. Never put a secret in an AWS log-driver option or a systemd environment
line. Docker documents both the [`awslogs`
options](https://docs.docker.com/engine/logging/drivers/awslogs/) and the
[non-blocking delivery trade-off](https://docs.docker.com/engine/logging/configure/):
when the bounded local buffer fills, Docker can drop log events rather than
blocking the application. CloudWatch alarms and health metrics therefore remain
the availability signal; logs are diagnostic evidence, not a health check.

## Expected telemetry

Under **CloudWatch → Metrics**, select custom namespace `MangaHub/EC2` and the
current instance ID:

| Metric | Source | Healthy meaning |
|---|---|---|
| `mem_used_percent` | CloudWatch Agent | host memory usage is visible |
| `disk_used_percent` | CloudWatch Agent | root path `/` usage is visible |
| `ApplicationHealthy` | systemd timer | value `1` |
| `HealthyContainers` | systemd timer | value `7` |

The health publisher requests IMDSv2 directly, checks all seven long-running
Compose services, executes the public behavior/security health gate once, and
publishes `0` before exiting unsuccessfully when the application is unhealthy.
If the timer stops publishing, an application alarm must treat missing data as
breaching.

Under **CloudWatch → Logs → Log groups → `/mangahub/demo/containers`**, expect
these streams:

```text
mangahub-prod/edge
mangahub-prod/frontend
mangahub-prod/api
mangahub-prod/redis
mangahub-prod/tcp
mangahub-prod/udp
mangahub-prod/grpc
```

Generate one harmless request with `curl http://127.0.0.1/health`, then confirm a
recent edge/API event arrives. Do not search for or screenshot tokens, cookies,
passwords, environment values, or database contents.

## Create basic alarms

Create one SNS email topic during the first alarm wizard and confirm the email
subscription. Create these standard-resolution alarms in Sydney:

| Alarm | Namespace / metric | Condition | Periods | Missing data |
|---|---|---|---|---|
| `MangaHub-demo-status-check` | `AWS/EC2` → `StatusCheckFailed` | Maximum `>= 1` | 1 × 5 min | not breaching |
| `MangaHub-demo-cpu-high` | `AWS/EC2` → `CPUUtilization` | Average `>= 80` | 2 × 5 min | not breaching |
| `MangaHub-demo-memory-high` | `MangaHub/EC2` → `mem_used_percent` | Average `>= 85` | 2 × 5 min | breaching |
| `MangaHub-demo-disk-high` | `MangaHub/EC2` → root `disk_used_percent` | Average `>= 80` | 2 × 5 min | breaching |
| `MangaHub-demo-app-unhealthy` | `MangaHub/EC2` → `ApplicationHealthy` | Minimum `< 1` | 2 × 1 min | breaching |

Select only metrics whose `InstanceId` is the MangaHub EC2 instance. Send alarm
notifications to the confirmed SNS topic. Do not configure automatic stop or
termination actions for this first lab.

## Controlled failure rehearsal

Use a disposable session after the application and alarms are healthy:

```bash
sudo docker stop mangahub-prod-edge-1
```

Within two health-publisher periods, `ApplicationHealthy` must become `0` and the
alarm must enter `ALARM`. Capture only the alarm name, state, Region, metric, and
time. Recover through the immutable deploy path—not by deleting or recreating
volumes:

```bash
CURRENT_SHA="$(sudo sed -n '1p' /var/lib/mangahub-deploy/current-version)"
CURRENT_MODE="$(sudo sed -n '1p' /var/lib/mangahub-deploy/current-mode)"

case "$CURRENT_MODE" in
  cloudwatch)
    sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" --with-cloudwatch
    ;;
  raw-cloudwatch)
    sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" --with-raw --with-cloudwatch
    ;;
  *)
    echo "Unexpected monitoring mode: $CURRENT_MODE" >&2
    ;;
esac
```

The deployment health gate must pass, the metric must return to `1`, and the
alarm must return to `OK`. This proves detection and recovery without claiming
automatic remediation.

## Cost, privacy, and cleanup

This design uses four custom metrics, five standard alarms, and one short-lived
log group. The [CloudWatch pricing
page](https://aws.amazon.com/cloudwatch/pricing/) currently advertises a free
tier that includes ten custom/detailed metrics, ten alarms, and 5 GB of logs,
but free-tier eligibility and prices can change. Keep the USD 5 budget alerts
active and check Billing after the rehearsal.

Application logs can include client IP addresses, request IDs, user IDs, paths,
and status codes. Seven-day retention limits exposure; it does not make logs
public. The application intentionally excludes authorization headers, JWTs,
passwords, cookies, request bodies, and environment dumps.

Before terminating the lab, delete the five alarms and SNS topic, detach and
delete `MangaHubDemoCloudWatchPolicy`, and delete the log group only after you no
longer need its evidence. Log-group deletion is permanent. Stopping EC2 does not
delete retained CloudWatch logs or alarms.
