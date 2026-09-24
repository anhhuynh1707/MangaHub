# MangaHub AWS verification record

Status: **an encrypted replacement EC2 instance is running the immutable
MangaHub release in Sydney with Session Manager and CloudWatch mode. Evidence
closeout remains for account/IAM/metadata/Security Group console state,
backup/restore, release rollback, the final alarm history, cost, and cleanup**.

This record prevents screenshots or plausible-looking console state from being
mistaken for proof. Complete rows only after the expected state is observed in
Asia Pacific (Sydney), `ap-southeast-2`. The click-by-click procedure remains in
`docs/AWS_DEPLOYMENT.md`.

## Evidence handling rules

- Never record passwords, MFA QR codes or one-time codes, JWTs, cookies, access
  keys, secret values, payment details, database contents, or protected
  environment output.
- Crop or blur the AWS account ID, root email, notification email, full user or
  role ARN, public IPv4, and unrelated resources before sharing a screenshot.
- Do not open `/opt/mangahub/.env` for evidence. Record only its owner and mode.
- Do not use application log bodies as portfolio screenshots. A log group name,
  stream names, retention, timestamps, and event counts are enough.
- Prefer a console summary plus a short, reproducible command result. A
  screenshot alone does not prove recovery, persistence, or rollback.
- Keep raw screenshots outside Git. Add only deliberately sanitized portfolio
  images after a separate secret/privacy review.
- Use UTC timestamps in technical records and note the Sydney Region on every
  AWS artifact.

## Verification matrix

`NOT RUN` is the only valid initial state. Use `PASS` only when every item in the
row is proven; otherwise use `FAIL` or `NEEDS REVIEW`.

| ID | Checkpoint | Required evidence | State |
|---|---|---|---|
| A1 | Root protection | Root MFA enabled; root access keys zero; no secret material visible | NOT RUN |
| A2 | Daily administrator | Console login requires MFA; access keys zero; existing user or `mangahub-admin` recorded without ARN | NOT RUN |
| A3 | Billing guardrail | `MangaHub-demo-zero-spend` template exists and its notification email is correct | NOT RUN |
| A4 | Region | Console shows Sydney / `ap-southeast-2` | PASS |
| B1 | Instance role | `MangaHubDemoEC2Role` trusts EC2; SSM core plus only the scoped monitoring policy after K1 | NEEDS REVIEW |
| C1 | VPC | `mangahub-demo-vpc`, `10.20.0.0/16`, project tags | NEEDS REVIEW |
| C2 | Public subnet | `mangahub-public-ap-southeast-2a`, `10.20.1.0/24`, auto-assign public IPv4 enabled | NEEDS REVIEW |
| C3 | Internet route | Named IGW attached; named route table associated; `0.0.0.0/0` targets that IGW | PASS |
| D1 | Security Group | HTTP 80 public; no SSH; temporary raw `/32` rules absent after demonstration | NEEDS REVIEW |
| E1 | EC2 baseline | One AL2023 x86_64 instance; approved type; encrypted 8 GiB gp3; role and project SG attached | PASS |
| E2 | Metadata and storage | IMDSv2 required; delete-on-termination understood; no Elastic IP | NEEDS REVIEW |
| F1 | Session Manager | Browser Session Manager reaches the instance with no inbound port 22 | PASS |
| G1 | Docker baseline | Docker and Compose versions return successfully; daemon is not remotely exposed | NEEDS REVIEW |
| H1 | Empty inventory | Named replacement project resources recorded before application deployment | PASS |
| I1 | Immutable deployment | Exact full Git SHA equals `current-version`; matching backend/frontend tags; health gate passes | PASS |
| I2 | Public behavior | Register, login, browse, library, progress, review, chat, live events, logout work with demo data | NEEDS REVIEW |
| I3 | Raw protocols | TCP, UDP, and gRPC work only after owner `/32` rules and `--with-raw`; rules removed afterward | NEEDS REVIEW |
| R1 | Release rollback | Known-good previous SHA restored; health passes; SQLite volume identity and demo data persist | NOT RUN |
| J1 | Online backup | Timestamped full-SHA backup, integrity result, checksum sidecar, mode/owner, retention list | NOT RUN |
| J2 | Restore | Controlled demo-state change is reversed; pre-restore recovery point exists; health passes | NOT RUN |
| K1 | Monitoring IAM | Scoped customer policy attached; no broad CloudWatch policy | NEEDS REVIEW |
| K2 | Logs and metrics | Seven streams, seven-day retention, four expected custom metrics for the correct instance | NEEDS REVIEW |
| K3 | Alarm notification | Five alarms exist; SNS email confirmed; normal application state is `OK` | NEEDS REVIEW |
| K4 | Detection and recovery | Stopped edge publishes zero, alarm enters `ALARM`, immutable deploy recovers, alarm returns `OK` | NEEDS REVIEW |
| L1 | Cost review | Billing checked after rehearsal; no unexpected services or duplicate resources | NOT RUN |
| L2 | Cleanup | Raw rules removed; stop/terminate/delete decisions recorded; wanted backup handled first | NOT RUN |

## Recorded evidence

### 2026-09-24 — Checkpoint H initial inventory

This is the pre-application inventory supplied from the Sydney console. The
public IPv4 was recorded privately but is deliberately omitted from this public
repository. These instance-specific values will become historical after the
unencrypted instance is replaced.

| Resource | Observed value |
|---|---|
| Region | `ap-southeast-2` (Sydney) |
| VPC | `vpc-0888c5169f8ba6704` |
| Subnet | `subnet-083137c9a78a69094` |
| Route table | `rtb-0ba27a268632cb28c` |
| Security Group | `sg-02adbd1291c053bc8` |
| EC2 instance | `i-07c91e7b6095b4169` |
| Public IPv4 | Recorded privately; omitted from Git |
| Instance type | `t3.micro` |
| AMI | `al2023-ami-2023.12.20260918.0-kernel-6.18-x86_64` |
| Root EBS volume | `vol-0bd7bea80846f881e` |
| Root EBS encryption | **Not encrypted — baseline failure** |
| Session Manager | Browser connection verified; agent active |
| Docker | Recovered after bootstrap failure; service active |
| Docker Compose | Reported verified; exact version output still required |

Result: **NEEDS REVIEW**. Do not begin Checkpoint I with this instance. Replace
the empty instance with one encrypted 8 GiB `gp3` root volume, rerun the Docker
and Compose version checks, and record the replacement instance, volume, and
public-address evidence. The first bootstrap failed because Amazon Linux 2023's
installed `curl-minimal` conflicts with the full `curl` package requested by the
old user-data command; the deployment guide now omits that conflicting package.

Historical resolution: the old instance was terminated after termination
protection was disabled. It is not the active MangaHub host.

### 2026-09-24 — Encrypted replacement infrastructure

The operator supplied the replacement Sydney inventory below. The public IPv4
was observed but remains deliberately omitted from Git.

| Resource | Observed value |
|---|---|
| Region | `ap-southeast-2` (Sydney) |
| VPC | `vpc-0888c5169f8ba6704` |
| Subnet | `subnet-083137c9a78a69094` |
| Route table | `rtb-0ba27a268632cb28c` |
| Security Group | `sg-02adbd1291c053bc8` |
| EC2 instance | `i-02415115b0e9de080` |
| Public IPv4 | Observed privately; omitted from Git |
| Instance type | `t3.micro` |
| AMI | `al2023-ami-2023.12.20260918.0-kernel-6.18-x86_64` |
| Root EBS volume | `vol-0338ba494a4d6b2b0` |
| Root EBS encryption | **Encrypted** |
| Session Manager | Browser connection verified; agent active |
| Docker Compose | `v5.5.0` reported by the operator |

Result: **PASS for E1, F1, and H1**. Preserve a sanitized EC2 details capture
before cleanup. B1, C1, C2, D1, E2, and G1 stay `NEEDS REVIEW` until their full
row requirements are captured without account, public-address, or email data.
The Docker and Compose commands succeeded, but the daemon-listener portion of
G1 has not yet been retained as evidence.

### 2026-09-24 — Immutable EC2 release and runtime health

The deployed application release is:

```text
7be4bc8181f394af8446e717d39cd191149dafaa
```

The matching branch CI and GHCR publication passed in
[`CI run #57`](https://github.com/anhhuynh1707/MangaHub/actions/runs/35978268375).
EC2 recorded the exact full SHA and mode `cloudwatch`. The public health request
returned healthy JSON, and `docker ps` showed seven long-running services. The
edge alone published host port `80`; raw ports shown without a host mapping on
the API image are image/container metadata, not Internet exposure.

After the controlled edge stop, the recovered edge had a shorter uptime than
the other six services and reported healthy. This is expected K9 evidence that
the edge was recreated/restarted without a full-stack or volume deletion.

Result: **PASS for I1 and the host-side portion of K9**. I2 remains
`NEEDS REVIEW` until the complete disposable browser behavior checklist is
recorded. I3 remains `NEEDS REVIEW` until the three client results and the
post-test Security Group cleanup are recorded together.

The branch later advanced to documentation-only commit `5701cfd...`. That does
not require replacing the healthy `7be4bc8...` EC2 application release.

### 2026-09-24 — CloudWatch runtime observation

The API container reported this bounded logging configuration without exposing
application log bodies:

- driver `awslogs`;
- Region `ap-southeast-2`;
- group `/mangahub/demo/containers`;
- stream `mangahub-prod/api`;
- automatic group creation disabled;
- non-blocking delivery with a `4m` buffer.

`amazon-cloudwatch-agent` and `mangahub-health-publisher.timer` both reported
active. The timer-triggered one-shot publisher exited `0/SUCCESS` after
publishing `ApplicationHealthy=1` and `HealthyContainers=7`. Its later
`inactive (dead)` state is normal for a completed one-shot service, not a
monitoring failure.

The operator reports completing the five-alarm/SNS setup and controlled edge
failure. Before K1–K4 become `PASS`, retain sanitized console evidence of:

1. the instance role containing SSM core and only the scoped MangaHub monitoring
   policy;
2. four current metrics and all seven expected streams with seven-day
   retention;
3. five named alarms plus a confirmed SNS email subscription; and
4. `MangaHub-demo-app-unhealthy` history showing `OK -> ALARM -> OK`.

Result: **NEEDS REVIEW for K1–K4** pending those bounded captures. The host-side
health and immutable recovery are verified; do not infer the final CloudWatch
alarm transition from host output alone.

## Evidence entry template

Copy this block below the relevant checkpoint when it is performed:

```text
Evidence ID:
UTC time:
Region: ap-southeast-2
Operator identity: IAM username only; no ARN/account ID
Expected state:
Observed state:
Reproduction command or console path:
Sanitized artifact filename/link: none unless reviewed
Result: PASS / FAIL / NEEDS REVIEW
Notes and corrective action:
```

## Safe command evidence on EC2

These commands expose operational state, not secrets:

```bash
uname -m
grep -E '^(NAME|VERSION_ID)=' /etc/os-release
docker --version
docker compose version

if sudo ss -lntp | grep -Eq ':(2375|2376)([[:space:]]|$)'; then
  echo "REVIEW: Docker API listener detected"
else
  echo "PASS: no remote Docker API listener"
fi

sudo stat -c '%U:%G %a %n' /opt/mangahub/.env
sudo sed -n '1p' /var/lib/mangahub-deploy/current-version
sudo sed -n '1p' /var/lib/mangahub-deploy/current-mode
sudo tail -n 3 /var/lib/mangahub-deploy/deployments.tsv

cd /opt/mangahub-src
sudo ./deploy/scripts/healthcheck.sh http://127.0.0.1
sudo docker ps --filter label=com.docker.compose.project=mangahub-prod \
  --format 'table {{.Names}}\t{{.Status}}'
```

Do not run `env`, `printenv`, `docker inspect` without a narrow format, `set -x`,
or commands that print `/opt/mangahub/.env` during evidence collection.

## Minimum sanitized portfolio set

After all associated rows pass, curate at most these screenshots:

1. GitHub Actions run with all required jobs green and the commit SHA visible.
2. VPC resource map showing only the MangaHub VPC/subnet/IGW/route relationship.
3. EC2 summary showing running state, Region, role, Security Group name, encrypted
   storage, and IMDSv2—after redaction.
4. Browser application page reached over the EC2 address using demo data.
5. CloudWatch dashboard/alarm summary showing healthy metrics and the controlled
   failure transition, without log bodies.
6. A terminal excerpt showing full-SHA release state, health success, and a
   successful rollback or restore, without environment output.

More screenshots create more privacy risk without adding stronger evidence.

## Final claim gate

The README may describe the manually observed encrypted EC2 deployment and its
verified command results with the limitations stated there. Do not use the
stronger final CV paragraph or mark the entire AWS phase complete until its
required matrix rows pass or are explicitly waived with a documented reason.
Do not claim automated EC2 deployment, HTTPS, off-instance backup, high
availability, release rollback, or AWS backup/restore unless those separate
capabilities are implemented and their matching evidence is recorded.
