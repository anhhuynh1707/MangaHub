# MangaHub AWS portfolio environment

Status: **planned, not provisioned**. This document records the agreed design;
it does not claim that any AWS resource already exists.

Follow the click-by-click beginner procedure in
[`docs/AWS_DEPLOYMENT.md`](../../docs/AWS_DEPLOYMENT.md). This file remains the
compact architecture and cost rationale; the runbook is the operational source.
The final target diagram and its verification boundary are in
[`docs/AWS_ARCHITECTURE.md`](../../docs/AWS_ARCHITECTURE.md), while
[`docs/AWS_EVIDENCE.md`](../../docs/AWS_EVIDENCE.md) controls when planned AWS
nodes may be represented as deployed.

## Goal and scope

Run MangaHub as a low-cost, single-instance portfolio demonstration in Asia
Pacific (Sydney), `ap-southeast-2`. The environment is deliberately simple so
its networking, security, deployment, monitoring, and recovery can be explained
and demonstrated clearly.

This is not a highly available production design. A single EC2 instance and a
single SQLite database are one failure domain and do not support horizontal
scaling.

## Target architecture

```text
Internet
   |
   +-- HTTP 80 -----------------------------+
   |                                        |
   +-- TCP 9090 / UDP 9091 / gRPC 9092 -----|-- owner IPv4 /32 only
                                            v
AWS Region: ap-southeast-2
  VPC: 10.20.0.0/16
    Internet Gateway
      Public subnet: 10.20.1.0/24
        Route table: 0.0.0.0/0 -> Internet Gateway
          Security Group
            EC2 (Amazon Linux 2023, one small x86_64 instance)
              Docker Compose
                edge proxy -> frontend + API/WebSocket/SSE
                              -> Redis + TCP + UDP + gRPC
                              -> persistent SQLite data

Administrator -> AWS Systems Manager Session Manager -> EC2
                 (no inbound SSH rule or long-lived AWS key on EC2)
```

HTTPS is independent of the raw protocols. A future domain can add HTTPS for
the browser while TCP, UDP, and gRPC continue on their own ports. For this
domain-free demo, HTTP is temporary and must use test-only accounts because
credentials and JWTs are not encrypted in transit.

## Address plan

| Network | CIDR | Purpose |
|---|---:|---|
| Learning VPC | `10.20.0.0/16` | Isolate the portfolio environment from the default VPC |
| Public subnet | `10.20.1.0/24` | EC2 with direct Internet Gateway route |

No NAT Gateway, load balancer, RDS database, or private subnet is required for
the initial single-EC2 demonstration. These would add cost or complexity without
improving the learning goal at this stage.

## Planned resources

AWS prices and Free Plan eligibility change. Verify the labels shown in the
console and the AWS Pricing Calculator immediately before provisioning.

| Resource | Purpose | Cost exposure | Security impact | Dependency |
|---|---|---|---|---|
| Cost Budget and alerts | Warn before credits are consumed unexpectedly | Alerts are not a hard stop | Billing notifications go only to the owner | AWS account billing access |
| VPC | Network boundary | No direct hourly VPC charge | Separates the demo from the default VPC | None |
| Public subnet | Hosts the single EC2 instance | Public IPv4 is billable | Internet-routable only through explicit routes and SG rules | VPC |
| Internet Gateway | Inbound web/raw demo traffic and outbound package/image access | Data transfer may be billable | Route alone does not open ports; SG still controls access | VPC |
| Route table | `0.0.0.0/0` route for the public subnet | No direct hourly charge | Makes associated instances Internet-routable when they have a public IP | Subnet and IGW |
| Security Group | Instance firewall | No direct hourly charge | Only explicitly approved inbound traffic | VPC |
| EC2 | Runs Docker and MangaHub | Running time consumes credits or incurs charges | Must use SSM, patched OS, least privilege, and no public Docker daemon | Subnet, SG, instance role |
| 8 GiB gp3 root EBS | OS, images, Compose files, and persistent data | Storage and snapshots can incur charges | Encrypted volume; deletion/backup behavior must be documented | EC2 |
| Public IPv4 | Lets the demo receive Internet traffic | Charged while allocated/in use | Address may change after stop/start unless an Elastic IP is used | EC2 |
| IAM instance role | Allows Session Manager | No long-lived key required on EC2 | Start with `AmazonSSMManagedInstanceCore`; add permissions only when needed | EC2 trust relationship |
| CloudWatch | Basic metrics, alarms, and selected logs | Logs/alarms beyond allowances can consume credits | Retention must be finite and logs must not contain secrets | EC2/agent configuration |
| S3 backup bucket (later phase) | Off-instance encrypted SQLite backups | Storage and requests can incur small charges | Block public access; least-privilege EC2 access and lifecycle retention | Tested backup procedure |

## Security Group design

Create a project-specific Security Group. Do not modify the default Security
Group.

| Direction | Protocol | Port | Source/destination | Reason |
|---|---|---:|---|---|
| Inbound | TCP | 80 | `0.0.0.0/0` | Temporary public HTTP portfolio demo |
| Inbound | TCP | 9090 | owner's current public IPv4 `/32` | Controlled TCP client demonstration |
| Inbound | UDP | 9091 | owner's current public IPv4 `/32` | Controlled UDP client demonstration |
| Inbound | TCP | 9092 | owner's current public IPv4 `/32` | Controlled gRPC client demonstration |
| Inbound | TCP | 22 | no rule | Administration uses Session Manager |
| Outbound | All | All | `0.0.0.0/0` initially | OS updates, SSM, GHCR, MangaDex, and external HTTPS APIs |

Do not expose ports `3000`, `6379`, or `8080`. The raw services currently lack
the transport protection expected of public services: access is temporary,
limited to the owner, tested with demo data, and closed when not in use.

The `/32` source must be updated when the owner's public IP changes. Before each
raw-protocol demonstration, verify the rule still points to the current IP; after
the demonstration, removing the three raw inbound rules is the safest state.

## Instance baseline

- Amazon Linux 2023 standard, x86_64.
- One instance type explicitly marked Free Tier eligible in the launch wizard;
  `t3.micro` is the preferred starting candidate when eligible.
- One encrypted 8 GiB gp3 root volume; no additional volumes initially.
- Auto-assign public IPv4 enabled for the public subnet deployment.
- IAM role with `AmazonSSMManagedInstanceCore` for Session Manager.
- Project tags: `Project=MangaHub`, `Environment=demo`, `ManagedBy=manual`.
- Detailed monitoring remains off initially unless its cost is explicitly
  accepted; basic monitoring and targeted alarms are sufficient for the demo.

## Cost and cleanup guardrails

Before EC2 is launched:

1. Enable MFA and use a non-root identity for routine work.
2. Create low actual and forecast monthly budget alerts.
3. Confirm the AMI and instance type are marked Free Tier eligible.
4. Confirm only one instance and one small EBS volume will be created.
5. Record every created resource so it can be cleaned up later.

Stopping EC2 stops compute charges but does not necessarily stop EBS, snapshot,
public IPv4, log, or backup charges. The final runbook will distinguish stop,
terminate, release, and delete operations and will back up anything important
before the Free Plan or credits expire.
