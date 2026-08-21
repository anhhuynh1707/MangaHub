# MangaHub AWS DevSecOps architecture

Status: **repository, CI, images, and local production runtime verified; AWS
resources and EC2 execution pending manual evidence**.

This document separates what currently exists from the Sydney target
environment. A dashed AWS boundary means “designed and implemented in the
repository, but not yet observed in the account.” Update that boundary only
after the matching record in `docs/AWS_EVIDENCE.md` passes.

## End-to-end delivery and runtime

```mermaid
flowchart TB
    Developer["Developer<br/>features/devsecops"]

    subgraph Delivery["Verified repository and delivery path"]
        CI["GitHub Actions<br/>tests, E2E, scans, Compose gates"]
        GHCR["GHCR<br/>backend + frontend<br/>sha-&lt;full-commit&gt;"]
        DeployContract["Health-gated deploy / rollback<br/>WAL-safe backup / restore"]
    end

    Developer -->|push| CI
    CI -->|publish only after all gates pass| GHCR
    CI -->|"lint and contract tests"| DeployContract

    Browser["Browser<br/>HTTP, WebSocket, SSE"]
    OwnerCLI["Owner CLI<br/>TCP, UDP, gRPC"]
    Administrator["Administrator<br/>console MFA, no access key"]

    subgraph Sydney["AWS ap-southeast-2 target — verification pending"]
        Budget["AWS Budget<br/>USD 5 alerts"]
        IAMRole["MangaHubDemoEC2Role<br/>SSM + scoped CloudWatch writes"]
        SSM["Systems Manager<br/>Session Manager"]
        CloudWatch["CloudWatch<br/>4 custom metrics, 5 alarms,<br/>7-day container logs"]

        subgraph VPC["mangahub-demo-vpc 10.20.0.0/16"]
            IGW["Internet Gateway"]
            Route["mangahub-public-rt<br/>0.0.0.0/0 → IGW"]

            subgraph Subnet["Public subnet 10.20.1.0/24"]
                SG["mangahub-demo-ec2-sg<br/>80 public; 9090/9091/9092 owner /32;<br/>no SSH"]

                subgraph Host["mangahub-demo-ec2<br/>Amazon Linux 2023 x86_64<br/>encrypted 8 GiB gp3"]
                    Deploy["deploy.sh<br/>exact full Git SHA"]

                    subgraph Compose["Docker Compose project: mangahub-prod"]
                        Edge["edge Nginx<br/>only public HTTP binding"]
                        Frontend["React static frontend"]
                        API["Go API<br/>REST + WebSocket + SSE"]
                        TCP["TCP sync :9090"]
                        UDP["UDP notifications :9091"]
                        GRPC["gRPC :9092"]
                        Redis[("Redis cache<br/>internal only")]
                        SQLite[("SQLite WAL<br/>mangahub-data")]
                        Backups[("Verified SQLite backups<br/>separate Docker volume")]
                    end
                end
            end
        end
    end

    Browser -->|"TCP 80"| IGW
    OwnerCLI -->|"TCP 9090 / UDP 9091 / TCP 9092<br/>temporary owner IPv4 /32"| IGW
    IGW --> Route --> SG
    SG --> Edge
    SG --> TCP
    SG --> UDP
    SG --> GRPC

    Administrator -->|"browser console"| SSM
    Host -->|"outbound HTTPS session channels; no port 22"| SSM
    IAMRole --> Host
    Host -->|"metrics and bounded logs"| CloudWatch

    GHCR -. "anonymous pull of exact SHA tags" .-> Deploy
    Deploy --> Compose
    Edge --> Frontend
    Edge --> API
    API --> Redis
    API --> SQLite
    TCP --> SQLite
    GRPC --> SQLite
    SQLite -->|"online backup API + checksum"| Backups

    classDef verified fill:#e8f5e9,stroke:#2e7d32,stroke-width:2px;
    classDef pending fill:#fff8e1,stroke:#f57f17,stroke-width:2px,stroke-dasharray:6 4;
    class Developer,CI,GHCR,DeployContract verified;
    class Budget,IAMRole,SSM,CloudWatch,IGW,Route,SG,Deploy,Edge,Frontend,API,TCP,UDP,GRPC,Redis,SQLite,Backups pending;
    style Sydney fill:#fffdf5,stroke:#f57f17,stroke-width:2px,stroke-dasharray:6 4
    style VPC fill:#fffdf5,stroke:#f57f17,stroke-width:2px,stroke-dasharray:6 4
    style Subnet fill:#fffdf5,stroke:#f57f17,stroke-width:2px,stroke-dasharray:6 4
    style Host fill:#fffdf5,stroke:#f57f17,stroke-width:2px,stroke-dasharray:6 4
    style Compose fill:#fffdf5,stroke:#f57f17,stroke-width:2px,stroke-dasharray:6 4
```

The diagram shows target placement, not a claim that the AWS nodes exist. The
same Compose topology inside the dashed boundary has passed locally; running it
on the named EC2 instance remains a separate gate.

## Request and protocol paths

| Client path | Public host listener | Edge/application destination | Exposure rule |
|---|---:|---|---|
| Frontend | TCP 80 | edge → frontend Nginx | Public demo |
| REST API | TCP 80, `/api/*` | edge strips `/api` → Go API | Public demo |
| WebSocket | TCP 80, `/api/ws/*` | edge preserves upgrade → Go API | JWT-authenticated application path |
| SSE | TCP 80, `/api/events/*` | edge disables buffering → Go API | JWT validated by the SSE handler |
| TCP sync | TCP 9090 | standalone TCP container | Owner IPv4 `/32`, only while demonstrating |
| UDP notifications | UDP 9091 | standalone UDP container | Owner IPv4 `/32`, only while demonstrating |
| gRPC | TCP 9092 | standalone gRPC container | Owner IPv4 `/32`, only while demonstrating |
| Administration | No inbound listener | Systems Manager Session Manager | IAM/MFA and instance role; no port 22 |

HTTPS does not replace or disable the raw listeners. A later domain can add TLS
for browser traffic and, independently, TLS for TCP/gRPC. The current HTTP and
raw-protocol demo is intentionally temporary and uses disposable data.

## Security boundaries

1. The project VPC and Security Group are separate from the account defaults.
2. Only the edge container binds a public web port in base mode. Redis, the API,
   frontend, and databases remain on Docker networks.
3. Raw bindings require both the `--with-raw` Compose override and three
   owner-only Security Group rules. Removing either closes external access.
4. The EC2 role supplies temporary credentials. No AWS access key or registry
   token is stored on the instance.
5. The protected runtime environment is root-owned mode `0600`; deployment
   scripts never print it.
6. Images use full-SHA tags and are pulled before the running release changes.
7. A failed health gate does not advance the recorded healthy release.
8. Deploy, rollback, backup, and restore never delete Docker volumes.
9. CloudWatch permissions are restricted to `MangaHub/EC2` and seven named log
   streams; the role cannot create the log group or change retention.

## Verification boundary

| Area | Current evidence | State |
|---|---|---|
| CI and security gates | GitHub Actions branch history; pinned workflow actions and scanners | Verified |
| Immutable images | Public matching backend/frontend full-SHA GHCR manifests | Verified |
| Production Compose and edge routing | Local HTTP, WebSocket, SSE, TCP, UDP, gRPC, hardening, persistence, and restart rehearsals | Verified locally |
| Release rollback | Two-version local rehearsal with the same SQLite volume/inode | Verified locally |
| SQLite recovery | Isolated online backup, checksum/integrity validation, atomic restore, and failure guard | Verified locally |
| Monitoring implementation | IAM policy, agent config, log override, systemd units, publisher tests | Verified locally |
| Account, budget, IAM MFA | Requires Checkpoint A evidence | Pending AWS |
| VPC, subnet, routes, Security Group | Requires Checkpoints C–D evidence | Pending AWS |
| EC2, SSM, Docker | Requires Checkpoints E–G evidence | Pending AWS |
| Public application and raw protocols | Requires Checkpoint I evidence | Pending AWS |
| EC2 rollback and recovery | Requires controlled EC2 rehearsals | Pending AWS |
| CloudWatch metrics, logs, SNS, alarm transition | Requires Checkpoint K evidence | Pending AWS |

## Known limitations and intentional exclusions

- One instance and one SQLite database are a single failure domain; this is not
  a highly available architecture.
- HTTP and raw protocols are not encrypted in the domain-free demo.
- The auto-assigned public IPv4 may change after stop/start.
- Backups are on a separate Docker volume but the same EBS device; off-instance
  encrypted backup is not implemented.
- GitHub Actions publishes images but does not deploy to EC2 yet. Manual
  deployment must pass before controlled CD is considered.
- No NAT Gateway, load balancer, RDS, Elastic IP, domain, or certificate is part
  of the first lab.

Operational details live in `docs/AWS_DEPLOYMENT.md`, `docs/SECURITY.md`,
`docs/ROLLBACK.md`, `docs/BACKUP.md`, and `docs/MONITORING.md`.
