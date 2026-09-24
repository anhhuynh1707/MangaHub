# MangaHub AWS verification record

Status: **evidence collection in progress; the initial EC2 baseline requires
replacement because its root EBS volume is not encrypted**.

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
| A4 | Region | Console shows Sydney / `ap-southeast-2` | NOT RUN |
| B1 | Instance role | `MangaHubDemoEC2Role` trusts EC2 and initially has only `AmazonSSMManagedInstanceCore` | NOT RUN |
| C1 | VPC | `mangahub-demo-vpc`, `10.20.0.0/16`, project tags | NOT RUN |
| C2 | Public subnet | `mangahub-public-ap-southeast-2a`, `10.20.1.0/24`, auto-assign public IPv4 enabled | NOT RUN |
| C3 | Internet route | Named IGW attached; named route table associated; `0.0.0.0/0` targets that IGW | NOT RUN |
| D1 | Security Group | HTTP 80 public; no SSH; raw ports absent initially | NOT RUN |
| E1 | EC2 baseline | One AL2023 x86_64 instance; approved type; encrypted 8 GiB gp3; role and project SG attached | FAIL |
| E2 | Metadata and storage | IMDSv2 required; delete-on-termination understood; no Elastic IP | NOT RUN |
| F1 | Session Manager | Browser Session Manager reaches the instance with no inbound port 22 | PASS |
| G1 | Docker baseline | Docker and Compose versions return successfully; daemon is not remotely exposed | NEEDS REVIEW |
| H1 | Empty inventory | Named project resources recorded before application deployment | NEEDS REVIEW |
| I1 | Immutable deployment | Exact full Git SHA equals `current-version`; matching backend/frontend tags; health gate passes | NOT RUN |
| I2 | Public behavior | Register, login, browse, library, progress, review, chat, live events, logout work with demo data | NOT RUN |
| I3 | Raw protocols | TCP, UDP, and gRPC work only after owner `/32` rules and `--with-raw`; rules removed afterward | NOT RUN |
| R1 | Release rollback | Known-good previous SHA restored; health passes; SQLite volume identity and demo data persist | NOT RUN |
| J1 | Online backup | Timestamped full-SHA backup, integrity result, checksum sidecar, mode/owner, retention list | NOT RUN |
| J2 | Restore | Controlled demo-state change is reversed; pre-restore recovery point exists; health passes | NOT RUN |
| K1 | Monitoring IAM | Scoped customer policy attached; no broad CloudWatch policy | NOT RUN |
| K2 | Logs and metrics | Seven streams, seven-day retention, four expected custom metrics for the correct instance | NOT RUN |
| K3 | Alarm notification | Five alarms exist; SNS email confirmed; normal application state is `OK` | NOT RUN |
| K4 | Detection and recovery | Stopped edge publishes zero, alarm enters `ALARM`, immutable deploy recovers, alarm returns `OK` | NOT RUN |
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

Do not change “prepared for AWS” to “deployed on AWS” in the README, CV, pull
request, or architecture status until A1–K4 are `PASS`. Do not claim automated
EC2 deployment, HTTPS, off-instance backup, or high availability unless those
separate capabilities are later implemented and verified.
