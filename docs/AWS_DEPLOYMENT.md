# MangaHub AWS deployment lab

Status: **runbook ready; no AWS resource has been provisioned by this
repository**.

This is the beginner-safe console path for the MangaHub portfolio environment.
Complete one checkpoint at a time. Do not skip the verification at the end of a
checkpoint, because later screens contain similarly named default resources.
Record only sanitized results in `docs/AWS_EVIDENCE.md`; that record starts at
`NOT RUN` and is the gate for changing portfolio wording from prepared to
deployed.

## What the current EC2 screenshot confirms

- The console is already in **Asia Pacific (Sydney)**, `ap-southeast-2`.
- There are no EC2 instances, volumes, key pairs, load balancers, or Elastic IPs
  in that Region.
- The account has a default VPC and default Security Group. MangaHub will not use
  either one.
- The screenshot shows USD 100 of plan credits and 184 days remaining. Credits
  reduce risk but are not a spending limit.

Future screenshots should crop or blur the account number in the top-right
corner. An account ID is not a password, but publishing it is unnecessary.

## Fixed design for this lab

| Item | Value |
|---|---|
| Region | Asia Pacific (Sydney), `ap-southeast-2` |
| VPC | `mangahub-demo-vpc`, `10.20.0.0/16` |
| Public subnet | `mangahub-public-ap-southeast-2a`, `10.20.1.0/24` |
| Internet gateway | `mangahub-demo-igw` |
| Public route table | `mangahub-public-rt` |
| Security Group | `mangahub-demo-ec2-sg` |
| Instance role | `MangaHubDemoEC2Role` |
| EC2 | `mangahub-demo-ec2`, Amazon Linux 2023 x86_64 |
| Instance candidate | `t3.micro` only if the launch screen marks it eligible or its displayed cost is accepted |
| Root storage | One encrypted 8 GiB `gp3` volume |
| Administration | Systems Manager Session Manager; no inbound port 22 |
| Public browser port | TCP 80 |
| Owner-only demo ports | TCP 9090, UDP 9091, TCP 9092 from the owner's current IPv4 `/32` |

The first deployment deliberately has no NAT Gateway, load balancer, RDS,
Elastic IP, domain, or certificate.

## Checkpoint A — secure the account first

### A1. Protect the root user

1. Open the account menu in the top-right corner.
2. If the current identity says **Root user**, use it only for this setup.
3. Open **Security credentials**.
4. Under **Multi-factor authentication (MFA)**, choose **Assign MFA device**.
5. Prefer a passkey/security key or an authenticator app. Register a second
   recovery-capable authenticator if available.
6. Confirm that the root user has no access keys. Do not create one.
7. While still in this root-only setup session, open **Account** from the account
   menu. Under **IAM User and Role Access to Billing Information**, choose
   **Edit**, enable **Activate IAM Access**, and update it. This lets the daily
   administrator create and inspect the budget without returning to root.

AWS requires root-user MFA and recommends an administrative identity for daily
work instead of root credentials. See [AWS account administrator security best
practices](https://docs.aws.amazon.com/signin/latest/userguide/best-practices-admin.html)
and the [root-only billing-access
setting](https://docs.aws.amazon.com/cost-management/latest/userguide/control-access-billing.html).

### A2. Create the daily administrative identity without losing free-plan credits

The screenshot identifies this as an AWS **Free account plan**. Do **not** enable
an organization instance of IAM Identity Center for this lab. AWS states that a
free-plan account automatically upgrades when it creates or joins AWS
Organizations and that its remaining free-plan credits then expire immediately.
An Identity Center account instance is not an alternative because it does not
support AWS account access or permission sets.

For this one-person, single-account lab, use a console-only IAM administrator as
an explicit temporary exception to AWS's federation preference:

1. Open the account menu and identify the current session. If it says **IAM
   user**, record only the user name—not the account ID—and inspect that user in
   **IAM → Users** before creating anything. Do not create a duplicate if the
   existing user already satisfies steps 7–9.
2. If the current session is **Root user**, search for **IAM** and open **User
   groups**.
3. Choose **Create group**, name it `MangaHubAdministrators`, select the AWS
   managed `AdministratorAccess` policy, and create the group. This broad policy
   is only for initial account bootstrap.
4. Open **Users → Create user** and name the user `mangahub-admin`.
5. Enable **AWS Management Console** access. If the console recommends Identity
   Center, acknowledge that this IAM user is the documented free-plan exception.
6. Add the user to `MangaHubAdministrators`, complete creation, and securely
   perform the first console sign-in/password change through the account's IAM
   sign-in URL.
7. As `mangahub-admin`, open **Security credentials → Multi-factor
   authentication (MFA) → Assign MFA device**. Prefer a passkey/security key or
   use an authenticator app.
8. In the same page, confirm **Access keys = 0**. Do not create an access key;
   browser administration and EC2's instance role need none.
9. Verify the user belongs only to `MangaHubAdministrators`, sign out, and prove
   a fresh IAM-user sign-in requires MFA. Stop using root for daily work.

AWS documents the [free-plan Organizations
effect](https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/free-tier-FAQ.html),
the [console-only IAM-user
flow](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_users_create.html), and
[IAM-user MFA](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_mfa_enable_virtual.html).
When the project intentionally moves to a paid or multi-account setup, migrate
human access to an organization instance of IAM Identity Center and temporary
credentials, then remove the lab IAM user.

### A3. Create a cost budget

1. Search for **Billing and Cost Management**.
2. Open **Budgets** and choose **Create budget**.
3. Choose **Use a template (simplified)**.
4. Select **Zero spend budget**.
5. Name it `MangaHub-demo-zero-spend`.
6. Use an email address that you check and confirm any verification message.
7. Choose **Create budget**.
8. Do not attach an automatic IAM action or create a paid Budget Report. A
   budget sends delayed alerts; it is not a hard spending limit and does not
   stop EC2 automatically.

AWS documents the zero-spend template in [Using a budget template
(simplified)](https://docs.aws.amazon.com/cost-management/latest/userguide/budget-templates.html).

### A4. Lock the working Region

1. Return to the AWS console home.
2. Select **Asia Pacific (Sydney) — ap-southeast-2** in the Region menu.
3. Keep this Region selected for VPC, EC2, Systems Manager, and CloudWatch.

Checkpoint A is complete only when root MFA is enabled, the daily IAM
administrator requires MFA and has no access keys, the budget exists, and the
console says Sydney.

## Checkpoint B — create the EC2 role for Session Manager

This role gives the instance temporary Systems Manager credentials. It is not a
human login and contains no access key.

1. Search for **IAM**, then open **Roles**.
2. Choose **Create role**.
3. Trusted entity type: **AWS service**.
4. Use case: **EC2**.
5. Search for and select only `AmazonSSMManagedInstanceCore`.
6. Choose **Next**.
7. Role name: `MangaHubDemoEC2Role`.
8. Add tags:
   - `Project=MangaHub`
   - `Environment=demo`
   - `ManagedBy=manual`
9. Choose **Create role**.

AWS lists these exact steps under the instance-profile alternative in [Configure
instance permissions required for Systems
Manager](https://docs.aws.amazon.com/systems-manager/latest/userguide/setup-instance-permissions.html).
Do not attach S3 or CloudWatch permissions at this checkpoint. The narrowly
scoped monitoring policy is added only at Checkpoint K; S3 remains unimplemented.

## Checkpoint C — create the learning VPC manually

Do not choose **VPC and more** for this lab. That wizard can create resources we
do not need, including a NAT Gateway. Create each resource so its purpose is
visible.

### C1. VPC

1. Search for **VPC** and open **Your VPCs**.
2. Choose **Create VPC**.
3. Resources to create: **VPC only**.
4. Name tag: `mangahub-demo-vpc`.
5. IPv4 CIDR manual input: `10.20.0.0/16`.
6. IPv6 CIDR: **No IPv6 CIDR block**.
7. Tenancy: **Default**.
8. Add `Project=MangaHub`, `Environment=demo`, and `ManagedBy=manual` tags.
9. Choose **Create VPC**.
10. Select the VPC, choose **Actions → Edit VPC settings**, and ensure both DNS
    resolution and DNS hostnames are enabled.

### C2. Public subnet

1. In the VPC console, open **Subnets** and choose **Create subnet**.
2. VPC: `mangahub-demo-vpc`.
3. Subnet name: `mangahub-public-ap-southeast-2a`.
4. Availability Zone: `ap-southeast-2a`.
5. IPv4 subnet CIDR: `10.20.1.0/24`.
6. Add the project tags and choose **Create subnet**.
7. Select the new subnet, choose **Actions → Edit subnet settings**.
8. Enable **Auto-assign public IPv4 address** and save.

AWS notes that subnet settings control whether launched instances receive public
addresses in [Create a
subnet](https://docs.aws.amazon.com/vpc/latest/userguide/create-subnets.html).

### C3. Internet gateway

1. Open **Internet gateways** and choose **Create internet gateway**.
2. Name: `mangahub-demo-igw`; add the project tags.
3. Choose **Create internet gateway**.
4. Choose **Actions → Attach to a VPC**.
5. Select `mangahub-demo-vpc` and attach it.

### C4. Public route table

1. Open **Route tables** and choose **Create route table**.
2. Name: `mangahub-public-rt`.
3. VPC: `mangahub-demo-vpc`; add the project tags and create it.
4. Select the route table, open **Routes**, then **Edit routes**.
5. Add destination `0.0.0.0/0` with target **Internet Gateway** →
   `mangahub-demo-igw`.
6. Save changes.
7. Open **Subnet associations → Edit subnet associations**.
8. Select `mangahub-public-ap-southeast-2a` and save.

The finished table has the automatic local `10.20.0.0/16` route and the new
Internet Gateway default route. AWS documents the same create, route, and
associate sequence in [Create a route table for your
VPC](https://docs.aws.amazon.com/vpc/latest/userguide/create-vpc-route-table.html).

Checkpoint C is complete only when the Internet Gateway state is attached and
the custom route table is explicitly associated with the MangaHub subnet.

## Checkpoint D — create the Security Group

1. In the VPC or EC2 console, open **Security Groups**.
2. Choose **Create security group**.
3. Name: `mangahub-demo-ec2-sg`.
4. Description: `MangaHub demo edge; raw protocols owner-only`.
5. VPC: `mangahub-demo-vpc`.
6. Add one initial inbound rule:
   - Type **HTTP**, TCP 80, source `0.0.0.0/0`, description `Public demo HTTP`.
7. Do **not** add SSH 22, HTTPS 443, Redis 6379, frontend 3000, or API 8080.
8. Leave the default allow-all IPv4 outbound rule for initial SSM, operating
   system updates, GHCR, GitHub, and application API access.
9. Add the project tags and create the group.

Add the raw demo rules only immediately before testing them:

| Type | Port | Source | Description |
|---|---:|---|---|
| Custom TCP | 9090 | **My IP**, shown as one IPv4 `/32` | Owner TCP demo |
| Custom UDP | 9091 | **My IP**, shown as one IPv4 `/32` | Owner UDP demo |
| Custom TCP | 9092 | **My IP**, shown as one IPv4 `/32` | Owner gRPC demo |

Never replace those three `/32` sources with `0.0.0.0/0`. Remove the rules when
the demonstration ends and recreate/update them if the owner's public IP changes.

## Checkpoint E — launch one EC2 instance

Return to the EC2 dashboard shown in the screenshot and choose **Launch
instance**.

1. **Name and tags**
   - Name: `mangahub-demo-ec2`.
   - Add `Project=MangaHub`, `Environment=demo`, and `ManagedBy=manual`.
2. **Application and OS Images**
   - Quick Start: **Amazon Linux**.
   - AMI: current AWS-published **Amazon Linux 2023**.
   - Architecture: **64-bit (x86)**.
3. **Instance type**
   - Start with `t3.micro` only if the wizard marks it eligible or its displayed
     estimate is accepted.
   - Do not choose Spot for the first deployment.
4. **Key pair**
   - Choose **Proceed without a key pair** and acknowledge the warning. Session
     Manager is the intended login path.
5. **Network settings → Edit**
   - VPC: `mangahub-demo-vpc` — not the default VPC.
   - Subnet: `mangahub-public-ap-southeast-2a`.
   - Auto-assign public IP: **Enable**.
   - Firewall: **Select existing security group**.
   - Select only `mangahub-demo-ec2-sg`.
6. **Configure storage**
   - One 8 GiB `gp3` root volume.
   - Encrypted: **Yes** using the default EBS key.
   - Delete on termination: **Yes** for this fresh disposable demo. Back up data
     before any later termination.
7. **Advanced details**
   - IAM instance profile: `MangaHubDemoEC2Role`.
   - Shutdown behavior: **Stop**.
   - Termination protection: **Enable**.
   - Stop protection: leave disabled so the instance can be stopped to save
     compute cost.
   - Detailed CloudWatch monitoring: leave disabled initially.
   - Credit specification, if shown for T3: **Standard** to avoid unlimited CPU
     credit charges.
   - Metadata accessible: **Enabled**.
   - Metadata version: **V2 only (token required)**.
   - Metadata response hop limit: **2** because Docker containers run on the
     host. AWS recommends this container setting in [Configure metadata options
     for new instances](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/configuring-IMDS-new-instances.html).
8. **User data** — paste only this non-secret bootstrap:

```bash
#!/bin/bash
set -euxo pipefail
dnf upgrade -y
# Amazon Linux 2023 includes curl-minimal, which already provides `curl`.
# Requesting the mutually exclusive full `curl` package can abort cloud-init.
dnf install -y docker git jq openssl util-linux
install -d -m 0755 /etc/docker
cat > /etc/docker/daemon.json <<'JSON'
{
  "log-driver": "local",
  "log-opts": {
    "max-size": "10m",
    "max-file": "3"
  }
}
JSON
systemctl enable --now docker
```

9. Review the summary carefully. It must say one instance, the MangaHub VPC and
   subnet, the MangaHub Security Group, and one 8 GiB volume.
10. Choose **Launch instance** once.

AWS explains every launch-wizard field in [EC2 instance launch
parameters](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-launch-parameters.html).

## Checkpoint F — connect without SSH

1. Wait until both EC2 status checks pass.
2. Select `mangahub-demo-ec2` and choose **Connect**.
3. Open the **Session Manager** tab.
4. Choose **Connect**. The button can take several minutes to become available.
5. In the session, run:

```bash
whoami
sudo systemctl is-active amazon-ssm-agent
sudo systemctl is-active docker
sudo docker version
```

Amazon Linux 2023 normally includes SSM Agent, but AWS still recommends checking
that it is running; see [AMIs with SSM Agent
preinstalled](https://docs.aws.amazon.com/systems-manager/latest/userguide/ami-preinstalled-agent.html).

If Session Manager is unavailable, check these in order:

1. The instance has `MangaHubDemoEC2Role` attached.
2. The role has `AmazonSSMManagedInstanceCore`.
3. The instance has a public IPv4 address.
4. The subnet uses `mangahub-public-rt` and its `0.0.0.0/0` route targets the
   attached Internet Gateway.
5. Outbound HTTPS is allowed.
6. The SSM Agent service is running.

Do not solve an SSM mistake by opening SSH to the entire Internet.

## Checkpoint G — install and verify Docker Compose

Amazon Linux supplies Docker Engine, but its Compose plugin availability/version
can lag. Install the current pinned plugin and verify its published checksum:

```bash
cd /tmp
curl -fLO https://github.com/docker/compose/releases/download/v5.5.0/docker-compose-linux-x86_64
curl -fLO https://github.com/docker/compose/releases/download/v5.5.0/docker-compose-linux-x86_64.sha256
sha256sum --check docker-compose-linux-x86_64.sha256
sudo install -d -m 0755 /usr/local/lib/docker/cli-plugins
sudo install -m 0755 docker-compose-linux-x86_64 /usr/local/lib/docker/cli-plugins/docker-compose
sudo docker compose version
sudo docker run --rm hello-world
```

The first checksum command must print `OK`. The installation location and command
follow the [official Docker Compose plugin
guide](https://docs.docker.com/compose/install/linux/). Continue using `sudo
docker`; membership in the Docker group is effectively root access.

## Checkpoint H — record the empty infrastructure

Before deploying MangaHub, record these values without exposing credentials:

- VPC ID;
- subnet ID;
- route-table ID;
- Security Group ID;
- instance ID;
- public IPv4 address;
- instance type and AMI name;
- EBS volume ID and encryption status;
- Session Manager and Docker/Compose verification result.

Do not record the JWT secret, session cookies, temporary credentials, or account
number in a public screenshot.

## Checkpoint I — deploy one immutable candidate

Do this only after the feature-branch GitHub Actions run is green and both GHCR
packages contain `sha-<full-commit>` for the same commit.

### I1. Make the two demo packages anonymously pullable

1. On GitHub, open the MangaHub repository and its **Packages** section.
2. Open the backend package `mangahub`, then **Package settings**.
3. Confirm it is connected to this public repository and change visibility to
   **Public** if needed.
4. Repeat for `mangahub-frontend`.
5. Do not create or copy a GitHub personal access token to EC2. Public GHCR
   images can be pulled anonymously.

### I2. Fetch only the deployment definition and pin its commit

In Session Manager, replace `FULL_SHA` with the 40-character commit displayed by
the successful Actions run:

```bash
cd /opt
sudo git clone --branch features/devsecops --single-branch \
  https://github.com/anhhuynh1707/MangaHub.git mangahub-src
cd /opt/mangahub-src
sudo git checkout --detach FULL_SHA
sudo git -C /opt/mangahub-src rev-parse HEAD
```

The last command must exactly equal `FULL_SHA`. Source exists only to provide
versioned Compose, proxy, and operation files; EC2 does not build it.

### I3. Generate the protected server environment

Use the instance's current public IPv4 from the EC2 details page:

```bash
cd /opt/mangahub-src
sudo ./deploy/scripts/configure-server.sh FULL_SHA PUBLIC_IPV4
sudo stat -c '%a %U:%G %n' /opt/mangahub/.env
```

The `stat` result must start with `600`. Do not print or open the environment file
in screenshots because it contains the JWT signing secret.

### I4. Deploy and pass the HTTP gate

```bash
sudo ./deploy/scripts/deploy.sh FULL_SHA
sudo docker compose \
  --project-name mangahub-prod \
  --env-file /opt/mangahub/.env \
  -f deploy/docker/docker-compose.prod.yml \
  ps
```

Open `http://PUBLIC_IPV4` in a browser and create only a disposable demo account
with a password used nowhere else. The deployment is accepted only if the
script reports that its health gate passed and every long-running service is
running/healthy.

### I5. Enable the raw protocol demonstration temporarily

1. Add the three owner `/32` rules from Checkpoint D to the Security Group.
2. Redeploy the same release with the opt-in raw override:

```bash
sudo ./deploy/scripts/deploy.sh FULL_SHA --with-raw
```

3. On the local Mac terminal, set the EC2 address for the CLI:

```bash
export MANGAHUB_API_URL=http://PUBLIC_IPV4/api
export MANGAHUB_TCP_ADDR=PUBLIC_IPV4:9090
export MANGAHUB_UDP_ADDR=PUBLIC_IPV4:9091
export MANGAHUB_GRPC_ADDR=PUBLIC_IPV4:9092
```

4. Run the TCP, UDP, and gRPC demonstrations from the same owner network whose
   `/32` is in the Security Group.
5. Remove the three Security Group rules afterward. The containers can stay up;
   AWS then drops all Internet traffic to those ports.

For a later bad release, `sudo ./deploy/scripts/rollback.sh` redeploys the
previously recorded SHA and runs the same health gate. It never deletes volumes.

## Checkpoint J — prove SQLite backup and restore

Do this only with disposable demo accounts. Restore intentionally removes every
database change made after the selected backup.

1. Confirm one baseline demo account can log in.
2. Create and list a verified online backup:

```bash
cd /opt/mangahub-src
sudo ./deploy/scripts/backup.sh
sudo ./deploy/scripts/backup.sh --list
```

3. Copy the exact newest `mangahub-...db` filename from the output. Do not copy
   or display database contents.
4. In the browser, register a second account named only for restore proof and
   confirm it can log in.
5. Restore the earlier backup, replacing `BACKUP_FILENAME`:

```bash
sudo ./deploy/scripts/restore.sh BACKUP_FILENAME
```

6. The command must report a verified pre-restore recovery point, healthy
   services, and a passed health gate.
7. Confirm the baseline account still works and the post-backup proof account no
   longer exists. This demonstrates actual data recovery rather than merely
   creating a file.
8. List the retained backups again:

```bash
sudo ./deploy/scripts/backup.sh --list
```

The backups live in the separate
`mangahub-prod_mangahub-backups` Docker volume and default to seven retained
backup/checksum pairs. Both volumes are initially on the same encrypted EBS
disk, so export a wanted backup off the instance before termination. The future
S3 export is not implemented yet. See `docs/BACKUP.md` for the recovery model
and failure rules.

## Checkpoint K — enable and prove CloudWatch monitoring

Do this only after the immutable deployment and recovery checkpoints pass.

1. In **IAM → Policies**, create customer-managed policy
   `MangaHubDemoCloudWatchPolicy` using the exact JSON in
   `deploy/aws/MangaHubDemoCloudWatchPolicy.json`.
2. Attach that policy to `MangaHubDemoEC2Role`; keep
   `AmazonSSMManagedInstanceCore` attached. Do not attach an administrator
   CloudWatch policy.
3. In **CloudWatch → Logs → Log groups**, create
   `/mangahub/demo/containers` in Sydney with **Standard** class, AWS-managed
   encryption, **7 days** retention, and the project tags.
4. In Session Manager, configure the agent/timer and redeploy the recorded SHA:

```bash
cd /opt/mangahub-src
sudo ./deploy/scripts/configure-monitoring.sh

CURRENT_SHA="$(sudo sed -n '1p' /var/lib/mangahub-deploy/current-version)"
sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" --with-cloudwatch
```

5. Confirm `mem_used_percent`, root `disk_used_percent`,
   `ApplicationHealthy=1`, and `HealthyContainers=7` under custom namespace
   `MangaHub/EC2`; confirm all seven expected container log streams exist.
6. Create the five standard alarms and confirmed SNS email notification listed
   in `docs/MONITORING.md`.
7. Run its controlled edge-container failure. The application alarm must change
   to `ALARM`, immutable redeployment must recover it without deleting volumes,
   and the alarm must return to `OK`.

The detailed IAM boundary, commands, alarm thresholds, privacy rules, evidence,
and cleanup procedure are in `docs/MONITORING.md`. Until this EC2 rehearsal
passes, monitoring is prepared but must not be represented as AWS-verified.

## HTTP, HTTPS, and the raw protocols

HTTPS does not disable TCP, UDP, or gRPC. They are separate listeners:

```text
Browser now:       HTTP 80  -> edge -> React + REST + WebSocket + SSE
Browser later:     HTTPS 443 -> edge -> React + REST + WebSocket + SSE
Owner TCP demo:    TCP 9090 -> progress service
Owner UDP demo:    UDP 9091 -> notification service
Owner gRPC demo:   TCP 9092 -> gRPC service
```

A future domain and certificate can upgrade browser traffic to HTTPS while all
three raw ports remain. The current raw services are not encrypted for public
Internet use, so Security Group `/32` restriction is mandatory. gRPC can later
use TLS on 9092; TCP can use TLS or a VPN/tunnel; UDP can use DTLS/QUIC or a VPN.

## Cost and cleanup notes

The VPC, subnet, route table, Internet Gateway, Security Group, and IAM role have
no direct hourly charge. The instance, EBS, data transfer, logs, snapshots, and
public IPv4 can consume credits or incur charges. AWS currently lists public IPv4
at USD 0.005 per hour (about USD 3.65 for 730 hours), whether in use or idle; recheck
the [official VPC pricing page](https://aws.amazon.com/vpc/pricing/) before launch.

- **Stop** the instance when pausing the lab. Compute stops, but EBS storage and
  some other resources can still cost money.
- A normal auto-assigned EC2 public IPv4 can change after stop/start. If it does,
  replace only the `PUBLIC_ORIGIN` line without displaying the protected file,
  then redeploy the recorded release in its recorded mode:

  ```bash
  NEW_PUBLIC_IPV4=REPLACE_WITH_NEW_EC2_IPV4
  sudo sed -i "s|^PUBLIC_ORIGIN=.*|PUBLIC_ORIGIN=http://${NEW_PUBLIC_IPV4}|" /opt/mangahub/.env
  sudo chmod 0600 /opt/mangahub/.env
  CURRENT_SHA="$(sudo sed -n '1p' /var/lib/mangahub-deploy/current-version)"
  CURRENT_MODE="$(sudo sed -n '1p' /var/lib/mangahub-deploy/current-mode)"
  cd /opt/mangahub-src
  case "$CURRENT_MODE" in
    base) sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" ;;
    raw) sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" --with-raw ;;
    cloudwatch) sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" --with-cloudwatch ;;
    raw-cloudwatch) sudo ./deploy/scripts/deploy.sh "$CURRENT_SHA" --with-raw --with-cloudwatch ;;
    *) echo "Unexpected mode: $CURRENT_MODE" >&2 ;;
  esac
  ```

  Use the new browser URL. The command preserves the recorded logging mode.
  Preserve raw mode only after its owner-only rules are reviewed again.
  Separately, if the owner's home/public IPv4 changes, update the three Security
  Group `/32` sources before the next raw protocol demo.
- Do not allocate an Elastic IP for this first lab.
- **Terminate** only after any wanted SQLite backup is verified. Termination is
  destructive when delete-on-termination is enabled.

## First hand-off checkpoint

Before proceeding to image deployment, confirm all of the following:

- [ ] Root MFA is enabled and root is no longer used daily.
- [ ] Root activated IAM access to Billing and Cost Management.
- [ ] The console-only IAM administrator login requires MFA and has zero access
      keys.
- [ ] The USD 5 monthly budget and email alerts exist.
- [ ] The console Region is Sydney.
- [ ] `MangaHubDemoEC2Role` exists with only the SSM core policy.
- [ ] Dedicated VPC/subnet/IGW/route table are connected exactly as documented.
- [ ] The MangaHub Security Group has HTTP 80 and no SSH rule.
- [ ] One encrypted 8 GiB EC2 instance is running.
- [ ] Session Manager connects.
- [ ] Docker Engine and `docker compose` work.

The deployment checkpoint uses an immutable `sha-<full-commit>` backend image
and matching frontend image. It does not build source code on EC2 and does not
use `latest` as the deployment source of truth.
