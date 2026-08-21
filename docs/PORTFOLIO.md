# MangaHub portfolio and CV claims

Status: **repository and local DevSecOps controls verified; AWS deployment claim
withheld until `docs/AWS_EVIDENCE.md` passes**.

This file provides truthful wording for the current project and a clearly
marked future version for after the EC2 rehearsal. It is not evidence by itself.

## Current truthful project title

**MangaHub — Production-style DevSecOps Deployment Path**

## Current CV description

> Engineered a production-style deployment path for a Go/Gin and
> React/TypeScript manga platform, adding hardened multi-service Docker Compose,
> an Nginx same-origin edge, full-SHA GHCR images, gated GitHub Actions security
> scanning, health-checked deployment and rollback, and verified SQLite
> backup/restore. Prepared a least-privilege, cost-bounded AWS EC2 and CloudWatch
> runbook for a dedicated Sydney learning VPC; cloud execution remains pending.

This wording can be used now because it distinguishes implemented repository
controls from the unexecuted AWS environment.

## Current verified technology list

```text
Go / Gin
React / TypeScript / Vite
SQLite WAL / Redis
HTTP REST / WebSocket / SSE / TCP / UDP / gRPC
Docker / Docker Compose / Nginx
GitHub Actions / GHCR
Govulncheck / npm audit / Gitleaks / Trivy / Dependabot
Playwright
Health-gated immutable deployment and rollback
WAL-safe backup and atomic restore
AWS EC2 / VPC / Systems Manager / CloudWatch — designed and repository-prepared
```

Keep the qualification on the AWS line until the evidence gate passes.

## Future CV description after AWS verification

Do not use this paragraph yet:

> Deployed a containerized Go/Gin and React/TypeScript platform to Amazon EC2 in
> a dedicated Sydney VPC, using Systems Manager administration, owner-restricted
> TCP/UDP/gRPC listeners, an Nginx web edge, immutable GHCR releases, CloudWatch
> metrics/logs/alarms, health-gated rollback, and tested SQLite backup recovery.
> Extended GitHub Actions with dependency, secret, configuration, and container
> image security gates before image publication.

It becomes valid only when the account, VPC, EC2 deployment, rollback, restore,
and monitoring rows in `docs/AWS_EVIDENCE.md` are `PASS`.

## Claims that remain prohibited

- “Production-grade” or “highly available.”
- “HTTPS secured” or “encrypted in transit” for the domain-free deployment.
- “Automated AWS deployment” or “continuous deployment to EC2.”
- “Zero-downtime deployment.”
- “Off-site/disaster-recovery backup.”
- “Kubernetes,” “ECS,” “RDS,” “load balanced,” or “auto scaled.”
- Any AWS uptime, cost, recovery-time, or performance number not measured and
  recorded.

## Portfolio page structure

1. **Problem:** convert a feature-rich local multi-protocol application into an
   explainable, recoverable deployment.
2. **Architecture:** use `docs/AWS_ARCHITECTURE.md` and state which boundary is
   pending or verified.
3. **Delivery:** show branch → CI/security gates → full-SHA GHCR images → manual
   health-gated deployment.
4. **Security decisions:** no public SSH, no embedded secrets, private Redis,
   minimal port exposure, non-root/read-only containers, pinned dependencies,
   bounded logs.
5. **Operations:** demonstrate health, rollback, backup/restore, monitoring, and
   cleanup rather than showing only a running home page.
6. **Trade-offs:** one EC2 instance and SQLite minimize cost and maximize
   learning clarity, but remain a single failure domain.
7. **Evidence:** link the green workflow and use only the sanitized artifacts
   approved in `docs/AWS_EVIDENCE.md`.
8. **Next improvements:** domain/TLS, encrypted off-instance backups, controlled
   CD, and a multi-instance database architecture only if the project scope
   later requires them.

## Interview narrative

The strongest story is the sequence of decisions:

- kept local development separate from production Compose;
- routed browser traffic through one same-origin edge while preserving TCP,
  UDP, and gRPC as owner-only demonstrations;
- made image publication depend on tests and security gates;
- deployed by immutable full Git SHA rather than `latest`;
- treated successful health checks as the release-state boundary;
- separated image rollback from SQLite data recovery;
- used Session Manager and an instance role instead of SSH and access keys;
- bounded CloudWatch permissions, metrics, streams, retention, and alarms;
- documented the limitations instead of implying enterprise availability.

That explanation demonstrates engineering judgment more clearly than a long
service-name list.
