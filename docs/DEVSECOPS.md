# MangaHub DevSecOps implementation record

This document is the concise source of truth for the AWS DevSecOps upgrade. The
detailed checklist remains in
`docs/MangaHub_AWS_DevSecOps_Phase_Checklist.md`; this file records what the
repository actually implements and avoids claiming planned components as done.

## Agreed project decisions

| Decision | Choice |
|---|---|
| Purpose | CV/portfolio demonstration and AWS learning environment |
| Git branch | Continue the existing `features/devsecops` branch |
| Integration | Pull request from `features/devsecops` to `main` after all gates pass |
| AWS Region | Asia Pacific (Sydney), `ap-southeast-2` |
| Network | Dedicated learning VPC, not the default VPC |
| Runtime | One EC2 instance running Docker Compose |
| Database | Fresh SQLite database on persistent instance storage |
| Public web access | Temporary HTTP on port 80; HTTPS is a documented future upgrade |
| Administration | AWS Systems Manager Session Manager; no public SSH rule |
| Browser traffic | Edge proxy to frontend, REST API, WebSocket, and SSE |
| Raw protocols | Retained for owner-only demos; TCP/UDP/gRPC restricted to owner IPv4 `/32` |
| Deployment order | Local production proof, then manual EC2 deployment, then controlled CD |

The branch name differs from the checklist's example `feature/aws-devsecops`.
This is an accepted project decision, not a second branch to create.

## Current status

| Checklist area | State | Evidence |
|---|---|---|
| Phase 0: baseline | Historical baseline exists; full gate will be rerun before production changes | Existing CI and local Compose |
| Phase 1: branch | Branch complete; CI gate pending PR | `features/devsecops` tracks `origin/features/devsecops`; the current workflow does not run on a feature-branch push |
| Phase 2: structure | Complete | `deploy/`, `infra/aws/`, this record, and security policy |
| Phase 3 onward | Not implemented yet | Must not be presented as complete |

## Delivery gates

Work proceeds in small commits and stops on a failed gate:

1. Baseline: backend tests, frontend build, existing Compose health, and current
   CI behavior are understood.
2. Local production: production images and Compose start locally; frontend,
   API, authentication, WebSocket, SSE, persistence, and health checks work.
3. Security: dependencies, secrets, source, images, and configuration are
   scanned with an explicit severity policy.
4. Manual AWS: the exact SHA-tagged images deploy to EC2 and pass functional
   tests before GitHub Actions receives deployment responsibility.
5. CD: deployment has a health gate, records the deployed version, and rolls
   back without deleting SQLite data.
6. Operations: monitoring, backup, restore, failure recovery, cleanup, and cost
   controls are tested and documented.
7. Pull request: no secrets or local artifacts, CI/security gates pass, and the
   architecture diagram matches the implementation.

## Planned repository layout

```text
.github/workflows/       CI, security gates, and controlled CD
deploy/                  production Compose, proxy, and operational scripts
infra/aws/               resource plan and step-by-step AWS runbook
docs/DEVSECOPS.md         implementation record and phase evidence
docs/SECURITY.md          security boundaries and operator rules
docs/AWS_DEPLOYMENT.md    hands-on deployment guide (added with implementation)
docs/ROLLBACK.md          tested rollback procedure (added with scripts)
```

Local development continues to use the root `docker-compose.yml`. Production
configuration will be separate and must not make local development harder to
understand.

## Portfolio claims policy

The README, CV description, diagrams, and screenshots may mention a technology
or control only after it is implemented and verified. Planned HTTPS, S3 backup,
CloudWatch agents, security scanners, or automated deployment must stay labeled
as planned until their gates pass.
