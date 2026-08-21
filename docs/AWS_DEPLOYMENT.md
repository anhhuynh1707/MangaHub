# MangaHub AWS deployment lab

Status: **runbook ready; no AWS resource has been provisioned by this
repository**.

This is the beginner-safe console path for the MangaHub portfolio environment.
Complete one checkpoint at a time. Do not skip the verification at the end of a
checkpoint, because later screens contain similarly named default resources.

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

AWS requires root-user MFA and recommends an administrative identity for daily
work instead of root credentials. See [AWS account administrator security best
practices](https://docs.aws.amazon.com/signin/latest/userguide/best-practices-admin.html).

### A2. Create the daily administrative identity

Recommended path for a new standalone account:

1. Search for **IAM Identity Center**.
2. Keep the console in **Asia Pacific (Sydney)**.
3. Choose **Enable** and use the AWS Organizations organization-instance option.
   Creating the one-account organization is expected.
4. Open **Users** and choose **Add user**.
5. Use an email address you control and send the setup invitation.
6. Open **Permission sets** and create the predefined
   **AdministratorAccess** permission set for initial bootstrap.
7. Open **AWS accounts**, select this account, choose **Assign users or groups**,
   select the new user, and assign `AdministratorAccess`.
8. Accept the email invitation, create the password, and register MFA.
9. Sign out of the root session. Sign back in through the AWS access portal and
   open the `AdministratorAccess` role.

This follows the [AWS Identity Center administrative-user
guide](https://docs.aws.amazon.com/singlesignon/latest/userguide/quick-start-default-idc.html).
Later, a narrower MangaHub permission set can replace daily administrator access.
Do not create an IAM access key for this console lab.

If the account is already a member of an AWS Organization, or Identity Center
shows an organization/Region ownership warning, stop here rather than creating a
second identity setup.

### A3. Create a cost budget

1. Search for **Billing and Cost Management**.
2. Open **Budgets** and choose **Create budget**.
3. Choose **Customize (advanced)**, then **Cost budget**.
4. Name it `MangaHub-demo-monthly-5USD`.
5. Period: **Monthly**. Budget amount: **USD 5.00**.
6. Add email alerts for:
   - 50% actual spend;
   - 80% actual spend;
   - 100% actual spend;
   - 100% forecasted spend.
7. Use an email address that you check and confirm any verification message.
8. Do not attach an automatic IAM action yet; alerts are easier to understand
   during the first lab.

AWS documents the current console flow in [Creating a cost
budget](https://docs.aws.amazon.com/cost-management/latest/userguide/budgets-create.html).

### A4. Lock the working Region

1. Return to the AWS console home.
2. Select **Asia Pacific (Sydney) — ap-southeast-2** in the Region menu.
3. Keep this Region selected for VPC, EC2, Systems Manager, and CloudWatch.

Checkpoint A is complete only when root MFA is enabled, daily login uses the
Identity Center user, the budget exists, and the console says Sydney.

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
Do not attach S3 or CloudWatch permissions yet; they are added only when those
features are implemented.

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
dnf install -y docker git jq curl openssl
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
- A normal auto-assigned public IPv4 can change after stop/start. Update the
  MangaHub `PUBLIC_ORIGIN`, browser URL, and owner `/32` rules when addresses
  change.
- Do not allocate an Elastic IP for this first lab.
- **Terminate** only after any wanted SQLite backup is verified. Termination is
  destructive when delete-on-termination is enabled.

## First hand-off checkpoint

Before proceeding to image deployment, confirm all of the following:

- [ ] Root MFA is enabled and root is no longer used daily.
- [ ] Identity Center administrator login works with MFA.
- [ ] The USD 5 monthly budget and email alerts exist.
- [ ] The console Region is Sydney.
- [ ] `MangaHubDemoEC2Role` exists with only the SSM core policy.
- [ ] Dedicated VPC/subnet/IGW/route table are connected exactly as documented.
- [ ] The MangaHub Security Group has HTTP 80 and no SSH rule.
- [ ] One encrypted 8 GiB EC2 instance is running.
- [ ] Session Manager connects.
- [ ] Docker Engine and `docker compose` work.

The next deployment checkpoint will use an immutable `sha-<full-commit>` backend
image and matching frontend image. It will not build source code on EC2 and will
not use `latest` as the deployment source of truth.
