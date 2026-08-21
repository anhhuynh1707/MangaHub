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
| Phase 1: branch | Complete and remotely verified | `features/devsecops` tracks `origin/features/devsecops`; CI includes this branch and reruns for every push |
| Phase 2: structure | Complete | `deploy/`, `infra/aws/`, this record, and security policy |
| Phases 3-7: production runtime | Complete and locally verified | Separate Compose/overrides, same-origin `/api`, hardened images, private networks, and edge Nginx |
| Phase 8: local production test | Complete | Production-edge Playwright journey, HTTP health/security, raw TCP/UDP/gRPC, restart persistence, container hardening, and logs passed on 2026-08-21 |
| Phase 9: security scanning | Implemented and locally verified | Govulncheck, npm audit, Gitleaks history, Trivy repository/image scans, pinned actions, and Dependabot |
| Phase 10: CI refactor | Implemented; branch gates and candidate publication pass at the current verified tip | Existing tests/builds remain; candidate GHCR publishing requires security, Docker, E2E, and deployment-script gates |
| Phases 11-14: AWS foundation | Beginner console runbook ready; manual execution pending | `docs/AWS_DEPLOYMENT.md` covers account security, budget, IAM role, VPC, Security Group, EC2, SSM, Docker, and cost checkpoints |
| Phases 15-17: manual EC2 deployment | Runbook and public candidate images ready; AWS execution pending | Matching anonymous backend/frontend full-SHA manifests are verified after each successful branch-tip run |
| Phases 18-21: immutable release, health, rollback | Implemented; two-version local rollback rehearsal passed | `675ff8a…` deployed, `d547361…` restored, the same SQLite inode remained, and HTTP/TCP/UDP/gRPC checks passed |
| Phase 22: HTTPS | Deferred by project decision | A domain and certificate are intentionally absent; HTTP is limited to disposable demo credentials |
| Phase 23: AWS monitoring | Repository path implemented; EC2 verification pending | Scoped instance policy, CloudWatch Agent memory/disk metrics, Docker log override, health/container timer, seven-day retention contract, alarms, and failure rehearsal runbook |
| Phase 24: SQLite backup/recovery | Implemented; isolated local restore rehearsal passed | Online WAL-safe copy, integrity/checksum verification, separate volume, retention, pre-restore point, atomic restore, failure guard, and health gate |
| Phase 25 onward | Partially implemented | Existing container/application hardening is verified; AWS controls, final audit, architecture evidence, and PR remain pending |

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

Current branch verification history:
<https://github.com/anhhuynh1707/MangaHub/actions/workflows/ci.yml?query=branch%3Afeatures%2Fdevsecops>.

## Repository layout

```text
.github/workflows/       CI, security gates, and controlled CD
deploy/                  production Compose, proxy, and operational scripts
infra/aws/               resource plan and step-by-step AWS runbook
docs/DEVSECOPS.md         implementation record and phase evidence
docs/SECURITY.md          security boundaries and operator rules
docs/AWS_DEPLOYMENT.md    hands-on account, network, EC2, and deployment guide
docs/ROLLBACK.md          non-destructive rollback procedure and failure handling
docs/BACKUP.md            verified SQLite backup, restore, retention, and evidence
docs/MONITORING.md        bounded CloudWatch metrics, logs, alarms, and rehearsal
```

Local development continues to use the root `docker-compose.yml`. Production
configuration will be separate and must not make local development harder to
understand.

## Portfolio claims policy

The README, CV description, diagrams, and screenshots may mention a technology
or control only after it is implemented and verified. Planned HTTPS, S3 backup,
CloudWatch must stay labeled as prepared—not AWS-verified—until metrics, logs,
alarms, notification, and the controlled EC2 failure rehearsal pass. Security
scanners or automated deployment must stay labeled as planned until their gates
pass.
