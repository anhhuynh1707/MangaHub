# MangaHub — AWS EC2 DevSecOps Upgrade
## Codex Phase Checklist and Implementation Plan

> **Purpose:** Upgrade the existing `anhhuynh1707/MangaHub` project from its current CI-enabled state into a production-style AWS EC2 + Docker + CI/CD + DevSecOps project.
>
> This file is intended to be given to Codex as an implementation checklist. Codex must work phase-by-phase, inspect the repository before changing anything, keep changes small, run verification after each phase, and never skip a failed gate.
>
> Repository: `https://github.com/anhhuynh1707/MangaHub`
>
> Base branch: `main`
>
> Recommended upgrade branch: `feature/aws-devsecops`
>
> Final integration: Pull Request `feature/aws-devsecops` → `main`

---

# 0. Current Repository Baseline — VERIFIED

The repository was checked before creating this plan.

## 0.1 Latest main commit

- [x] `main` currently has 65 commits.
- [x] Latest observed commit:
  - SHA: `0ab469a`
  - Message: `fix(ci): provide an ephemeral JWT_SECRET for the docker smoke test and e2e backend`
  - Date shown by GitHub: June 24, 2026.
- [x] Previous security-related commit:
  - `697d5f5`
  - `fix(auth): require a strong JWT_SECRET at startup across api/tcp/grpc and remove the hardcoded default signing key`

## 0.2 Current CI — ALREADY IMPLEMENTED

The existing `.github/workflows/ci.yml` already contains:

- [x] Go dependency download/verification.
- [x] Backend builds.
- [x] API server build.
- [x] TCP server build.
- [x] UDP server build.
- [x] gRPC server build.
- [x] CLI build.
- [x] Go tests.
- [x] Race-enabled tests for selected packages.
- [x] `go vet`.
- [x] Frontend `npm ci`.
- [x] TypeScript type checking.
- [x] Frontend production build.
- [x] Playwright E2E.
- [x] Docker image build.
- [x] Docker Compose smoke test.
- [x] API health check.
- [x] Docker cleanup.
- [x] GHCR publishing.

The existing publish job pushes the root Docker image to:

```text
ghcr.io/${{ github.repository }}
```

and creates `latest` plus SHA-based tags.

### Baseline conclusion

**CI is already implemented.**

Do NOT rebuild CI from scratch.

The DevSecOps upgrade should extend the current pipeline with:

```text
Existing CI
+
Security gates
+
Production Docker/runtime design
+
AWS infrastructure
+
EC2 deployment
+
CD
+
Secrets
+
HTTPS
+
Monitoring
+
Backup/recovery
+
Rollback
+
Documentation
```

---

# 1. Current MangaHub Architecture — VERIFIED

The application is already a substantial full-stack project.

## Frontend

```text
React 19
TypeScript
Vite
Tailwind CSS
shadcn/ui
TanStack Query
Zustand
React Router
Axios
```

## Backend

```text
Go 1.26
Gin
SQLite + WAL
Redis
JWT
WebSocket
SSE
TCP
UDP
gRPC
```

## Testing

```text
Go tests
go vet
Playwright E2E
Docker smoke test
```

## Containerization

```text
Docker
Docker Compose
```

## CI/CD baseline

```text
GitHub Actions CI
GHCR publishing
```

---

# 2. Current Runtime Architecture — VERIFIED

Current Compose services:

```text
frontend
redis
mangahub-api
mangahub-tcp
mangahub-udp
mangahub-grpc
```

Current important ports:

```text
3000  -> Frontend
8080  -> API + WebSocket + SSE
6379  -> Redis
9090  -> TCP
9091  -> UDP
9092  -> gRPC
```

Current persistent volumes:

```text
mangahub-data
redis-data
```

SQLite is mounted at:

```text
/app/data/mangahub.db
```

The backend uses:

```text
DB_PATH=/app/data/mangahub.db
```

Redis uses:

```text
REDIS_ADDR=redis:6379
```

JWT is already required through:

```text
JWT_SECRET
```

The current project does not rely on a hardcoded default JWT signing key.

---

# 3. Important Production Issues to Solve

The current application is the baseline, but the current Compose file is not yet the final AWS production architecture.

## 3.1 Frontend API URL

Current Compose uses:

```text
VITE_API_URL=http://localhost:8080
```

This is appropriate for local development but is not correct for a public deployment.

The production design should use the public domain/same-origin architecture.

Preferred conceptual design:

```text
https://example.com/
        |
        +--> frontend
        |
        +--> /api      -> API
        +--> /ws       -> WebSocket
        +--> /events   -> SSE
```

Codex must inspect the actual frontend API client before changing paths.

Do not introduce an `/api` prefix if doing so would require unnecessary application-wide refactoring. Same-origin reverse proxying without changing backend routes is acceptable.

## 3.2 Redis exposure

Current Compose publishes:

```text
6379:6379
```

Production Redis should be internal to Docker.

Do not expose Redis to the public Internet.

## 3.3 TCP/UDP/gRPC exposure

Current Compose publishes:

```text
9090:9090
9091:9091/udp
9092:9092
```

Codex must determine whether each service actually needs Internet access.

Do not expose these ports merely because the services exist.

Preferred model:

```text
Internet
   |
   v
Nginx / HTTPS
   |
   +--> Frontend
   |
   +--> API
          |
          +--> Redis
          +--> TCP service
          +--> UDP service
          +--> gRPC service
```

If a real external TCP/UDP client requires public access, document the exact requirement and restrict AWS Security Group sources as much as possible.

## 3.4 SQLite

Do not automatically migrate SQLite to RDS for this first upgrade.

Keep:

```text
EC2
 |
Docker
 |
SQLite persistent volume
```

for the initial AWS DevSecOps project.

However, implement:

- persistent storage
- backup
- restore
- deployment safety
- rollback safety

A future SQLite → PostgreSQL/RDS migration can be a separate project.

---

# 4. Git Strategy — REQUIRED

Keep `main` as the stable application baseline.

All AWS/DevSecOps work should be performed on a separate branch.

## 4.1 Create branch

```bash
git checkout main
git pull origin main
git checkout -b feature/aws-devsecops
git push -u origin feature/aws-devsecops
```

## 4.2 Branch rule

Do not develop the AWS upgrade directly on `main`.

## 4.3 Commit rule

Use small logical commits, for example:

```text
chore(devops): create production deployment structure
feat(docker): harden production compose networking
feat(security): add dependency and image scanning
feat(infra): document AWS EC2 deployment
feat(cd): add EC2 deployment workflow
feat(monitoring): add deployment health checks
docs(devops): document AWS architecture
```

Avoid one giant commit.

## 4.4 Final PR

At the end:

```text
feature/aws-devsecops
        |
        v
Pull Request
        |
        v
main
```

The PR should include:

- summary
- architecture diagram
- security changes
- CI/CD changes
- AWS deployment
- test evidence
- rollback procedure
- known limitations

---

# 5. PHASE 0 — Baseline Verification

## Goal

Prove the current `main` branch works before modifying it.

## Checklist

- [ ] Pull latest `main`.
- [ ] Verify working tree is clean.
- [ ] Verify current commit.
- [ ] Copy `.env.example` to `.env`.
- [ ] Generate a local JWT secret.
- [ ] Start Compose.
- [ ] Check containers.
- [ ] Check `/health`.
- [ ] Open frontend.
- [ ] Test registration.
- [ ] Test login.
- [ ] Test manga browsing.
- [ ] Test library.
- [ ] Test progress.
- [ ] Test reviews.
- [ ] Test chat.
- [ ] Test live events.
- [ ] Run backend tests.
- [ ] Run frontend build.
- [ ] Run Playwright E2E.
- [ ] Verify Docker smoke test.

Commands:

```bash
git checkout main
git pull origin main
git status
git log -1 --oneline
```

```bash
cp .env.example .env
printf 'JWT_SECRET=%s
' "$(openssl rand -base64 48)" >> .env
```

```bash
docker compose up -d --build
docker compose ps
curl http://localhost:8080/health
```

Frontend:

```text
http://localhost:3000
```

## Gate

Do not continue if baseline functionality is broken.

---

# 6. PHASE 1 — Create the DevSecOps Branch

## Goal

Create a clean branch from the verified baseline.

```bash
git checkout main
git pull origin main
git checkout -b feature/aws-devsecops
git push -u origin feature/aws-devsecops
```

Checklist:

- [ ] Branch exists locally.
- [ ] Branch exists remotely.
- [ ] Branch starts from current `main`.
- [ ] No application changes yet.
- [ ] Existing CI runs on branch.
- [ ] CI passes.

## Gate

```text
feature/aws-devsecops
        |
        +--> existing CI PASS
```

---

# 7. PHASE 2 — Repository DevSecOps Structure

Recommended structure:

```text
.github/
  workflows/
    ci.yml
    cd.yml
    security.yml

deploy/
  docker/
    docker-compose.prod.yml
    .env.example
  nginx/
    nginx.conf
  scripts/
    deploy.sh
    rollback.sh
    healthcheck.sh

infra/
  aws/
    README.md
    architecture.md
    security-group.md
    ec2.md

docs/
  DEVSECOPS.md
  AWS_DEPLOYMENT.md
  SECURITY.md
  ROLLBACK.md
```

Do not create empty files just to match this tree. Every file must have a real purpose.

Checklist:

- [x] Production deployment directory created.
- [x] AWS documentation created.
- [x] Security documentation created.
- [x] Existing local development files remain understandable.
- [x] No secrets added.

Phase 2 evidence:

- `deploy/README.md` defines the production deployment contract and the purpose
  of each future deployment artifact without creating empty placeholders.
- `infra/aws/README.md` records the agreed Sydney learning-VPC architecture,
  resource purpose/cost/security/dependencies, and owner-only raw protocol
  access.
- `docs/DEVSECOPS.md` records decisions, gates, status, and portfolio claim
  rules.
- `docs/SECURITY.md` defines the temporary HTTP demo boundary, secrets policy,
  network policy, data safety, and stop-work conditions.
- `git diff --check` and a targeted credential/private-key pattern scan passed.

---

# 8. PHASE 3 — Production Docker Architecture

## Goal

Separate local development Compose from AWS production Compose.

Recommended:

```text
docker-compose.yml
    |
    +--> local development

deploy/docker/docker-compose.prod.yml
    |
    +--> AWS EC2 production
```

Production Compose requirements:

- [x] restart policies
- [x] internal Docker networking
- [x] no unnecessary public ports
- [x] environment variables
- [x] no secrets in source
- [x] persistent SQLite data
- [x] health checks
- [x] production frontend build
- [x] production Go runtime
- [x] Nginx
- [ ] deterministic image tags
- [x] safe restart
- [x] no development-only commands

Immutable `sha-<commit>` release tags remain deliberately open until the image
publishing/CD phases. The local production runtime does not use `latest`.

---

# 9. PHASE 4 — Production Frontend

## Goal

Build React into static production files.

Desired flow:

```text
React source
    |
npm ci
    |
npm run build
    |
dist/
    |
Nginx
```

Checklist:

- [x] Inspect `frontend/Dockerfile`.
- [x] Verify production build.
- [x] Verify Node version.
- [x] Use `npm ci`.
- [x] Use `npm run build`.
- [x] Serve `dist/` with Nginx.
- [x] Remove Vite dev server from production runtime.
- [x] Verify SPA fallback.
- [x] Verify API URL.
- [x] Verify WebSocket URL.
- [x] Verify SSE URL.

## Gate

Production frontend must not require:

```bash
npm run dev
```

---

# 10. PHASE 5 — Production Backend Images

## Goal

Create efficient and secure runtime images.

Inspect the current root `Dockerfile`.

Checklist:

- [x] Use multi-stage build where useful.
- [x] Build Go binaries in builder stage.
- [x] Use minimal runtime image.
- [x] Do not embed secrets.
- [x] Use non-root runtime where compatible with SQLite permissions.
- [x] Preserve `/app/data`.
- [x] Verify API server.
- [x] Verify TCP server.
- [x] Verify UDP server.
- [x] Verify gRPC server.
- [x] Verify health endpoint.

Gate:

```text
All production images build successfully.
```

---

# 11. PHASE 6 — Production Compose Networking

Desired conceptual architecture:

```text
                 Public network
                      |
                      v
                    Nginx
                      |
                 frontend_net
                      |
             +--------+--------+
             |                 |
          Frontend           API
                                 |
                            backend_net
                                 |
                    +------------+------------+
                    |            |            |
                  Redis         TCP          gRPC
```

Rules:

- [x] Redis internal only.
- [x] SQLite internal.
- [x] Only required public ports published.
- [x] gRPC not public unless required.
- [x] TCP not public unless required.
- [x] UDP not public unless required.
- [x] Nginx is the public HTTP/HTTPS entry point.
- [x] Internal services use Docker DNS/service names.

Raw protocols are an explicit owner-demo requirement. They remain unpublished
in the base Compose file; the opt-in override binds them to loopback locally and
will rely on owner IPv4 `/32` Security Group rules on EC2.

---

# 12. PHASE 7 — Nginx Reverse Proxy

Desired:

```text
Internet
   |
HTTPS :443
   |
Nginx
   |
   +---- / --------> Frontend
   |
   +---- API ------> Backend
   |
   +---- /ws ------> Backend WebSocket
   |
   +---- /events --> Backend SSE
```

Checklist:

- [x] Configure Nginx.
- [x] Configure SPA fallback.
- [x] Configure API proxy.
- [x] Configure WebSocket upgrade.
- [x] Configure SSE correctly.
- [x] Set suitable proxy timeouts.
- [x] Add basic security headers.
- [x] Hide internal service ports.
- [x] Test locally.

Codex must inspect the actual route structure before modifying proxy paths.

---

# 13. PHASE 8 — Local Production Test

## Goal

Prove the AWS-style architecture locally before AWS.

Use the production Compose file:

```bash
docker compose -f deploy/docker/docker-compose.prod.yml up -d
```

Test:

```text
Frontend
Login
Registration
Manga browsing
Library
Progress
Reviews
Chat
SSE
WebSocket
Health
```

Checklist:

- [x] Frontend loads.
- [x] API works.
- [x] Login works.
- [x] JWT works.
- [x] SQLite persists.
- [x] Redis works.
- [x] WebSocket works.
- [x] SSE works.
- [x] Required TCP works.
- [x] Required UDP works.
- [x] Required gRPC works.
- [x] Container restart preserves data.
- [x] Logs are understandable.

Phase 8 evidence (2026-08-21):

- Built the backend and frontend production images and started the complete
  Compose stack through the edge on `http://localhost:8088`.
- All long-running containers reached running/healthy state. Only the edge was
  web-accessible; Redis, API, and frontend ports stayed inside Docker.
- `GET /health` returned `200` with security headers. Public requests for
  `/api/health/db` and `/api/swagger/index.html` returned `404`.
- Playwright's production-edge journey passed registration, login, browsing,
  library, progress, review, chat/WebSocket, and SSE (`1 passed`).
- Authenticated TCP strategy lookup, UDP echo, and authenticated gRPC search
  passed through loopback-only ports `9090`, `9091/udp`, and `9092`.
- A full service restart preserved the disposable account and SQLite manga
  data; Redis reloaded its snapshot and all health checks recovered.
- Backend containers were verified as UID/GID `10001:10001`, read-only root
  filesystems, all Linux capabilities dropped, and `no-new-privileges` enabled.
- A shutdown defect discovered in the UDP logs was fixed and covered by a race
  test; restart logs are now concise and graceful.

## Gate

Do not create the EC2 deployment until this passes.

---

# 14. PHASE 9 — DevSecOps Security Scanning

## Goal

Extend CI into a security-aware pipeline.

Security areas:

```text
Source code
Dependencies
Secrets
Docker images
Infrastructure configuration
```

## 14.1 Dependency scanning

Scan:

```text
go.mod
go.sum
frontend/package-lock.json
```

Suggested initial policy:

```text
Critical -> fail
High     -> fail
Medium   -> review
Low      -> report
```

Do not blindly fail on every scanner finding.

Implemented policy:

- [x] `govulncheck` blocks reachable Go vulnerabilities.
- [x] `npm audit --audit-level=high` blocks high and critical advisories.
- [x] Trivy blocks high and critical dependency, configuration, secret, and
  fixable production-image findings.
- [x] Unfixed operating-system findings are reviewed but do not block. They
  cannot be remediated by an application dependency bump; base images remain
  minimal, supported, and updated through Dependabot.

## 14.2 Secret scanning

Detect:

```text
JWT_SECRET
AWS credentials
GHCR credentials
private keys
tokens
passwords
```

No secrets may be committed.

- [x] Gitleaks scans the complete Git history.
- [x] Historical expired local Postman JWT fixtures are removed from the current
  collection and narrowly allowlisted by exact commit plus exact file path.
- [x] New JWTs in the collection or any other path still fail the gate.

## 14.3 Container scanning

Desired pipeline:

```text
Docker build
      |
      v
Container scan
      |
      +--> policy violation -> fail
      |
      v
Publish
```

Do not publish production images before required security gates pass.

- [x] Backend and frontend images build before scanning.
- [x] Trivy scans OS and language packages.
- [x] Publishing depends on every required security job.

## 14.4 Static analysis

Keep:

```text
go vet
```

Add further analyzers only where useful.

- [x] `go vet` remains required.
- [x] Workflow actions are immutable-SHA pinned and validated with Actionlint.
- [x] Dependabot monitors Go, npm, Docker, and GitHub Actions dependencies.

Phase 9 evidence (2026-08-21):

- `govulncheck ./...`: no reachable vulnerabilities.
- `npm audit`: zero vulnerabilities.
- Gitleaks `v8.30.1`: 70 commits scanned, no leaks found.
- Trivy `v0.72.0`: repository, backend image, and frontend image passed the
  blocking high/critical policy; the scan also drove the Go dependency upgrades
  and the move to digest-pinned, unprivileged Nginx on Alpine 3.24.
- Backend, frontend, edge, Compose, Playwright, TCP, UDP, and gRPC checks were
  rerun after remediation.

---

# 15. PHASE 10 — GitHub Actions CI Refactor

Desired pipeline:

```text
                    Pull Request
                         |
             +-----------+-----------+
             |           |           |
             v           v           v
          Backend     Frontend    Security
           tests       build       scans
             |           |           |
             +-----------+-----------+
                         |
                         v
                    E2E Tests
                         |
                         v
                  Docker Build
                         |
                         v
                  Image Security
                         |
                         v
                    Publish
```

Checklist:

- [x] Preserve backend tests.
- [x] Preserve frontend build.
- [x] Preserve Playwright.
- [x] Preserve Docker smoke test.
- [x] Preserve GHCR publishing.
- [x] Add security jobs.
- [x] PRs do not deploy production.
- [x] Candidate SHA publishing occurs only on the selected feature branch or
  `main`; pull requests publish nothing.
- [x] Use immutable SHA tags.
- [x] Keep `latest` only as a `main` convenience tag.

Phase 10 implementation is locally validated with Actionlint. The first remote
branch run remains a gate after these changes are pushed; AWS deployment is not
triggered from feature branches or pull requests.

---

# 16. PHASE 11 — AWS Account Preparation

Checklist:

- [ ] Create/use AWS account.
- [ ] Enable MFA.
- [ ] Do not use root for routine infrastructure operations.
- [ ] Create appropriate IAM identity.
- [ ] Enable billing visibility.
- [ ] Create budget alerts.
- [ ] Select AWS Region.
- [ ] Decide EC2 instance type.
- [ ] Understand expected cost before provisioning.
- [ ] Never commit AWS credentials.

Security principle:

```text
least privilege
```

---

# 17. PHASE 12 — AWS VPC

Target architecture:

```text
AWS Account
   |
Region
   |
VPC
   |
Public Subnet
   |
Internet Gateway
   |
Route Table
   |
EC2
```

Checklist:

- [ ] Create VPC.
- [ ] Choose CIDR.
- [ ] Create public subnet.
- [ ] Attach Internet Gateway.
- [ ] Create route table.
- [ ] Associate subnet.
- [ ] Add `0.0.0.0/0` route to Internet Gateway.
- [ ] Verify public IPv4 capability.

---

# 18. PHASE 13 — AWS Security Group

Recommended initial inbound rules:

```text
SSH 22      -> trusted IP only
HTTP 80     -> public
HTTPS 443   -> public
```

Do not initially expose:

```text
6379
8080
9090
9091
9092
3000
```

to the entire Internet.

Only add public TCP/UDP ports if the actual application architecture requires them.

Review outbound access rather than blindly assuming every rule is necessary.

---

# 19. PHASE 14 — EC2

Target:

```text
EC2
 |
 +-- Docker
 |
 +-- Docker Compose
 |
 +-- Nginx
 |
 +-- MangaHub
```

Checklist:

- [ ] Create EC2.
- [ ] Select supported Linux distribution.
- [ ] Create/use key pair.
- [ ] Attach Security Group.
- [ ] Enable public IPv4.
- [ ] Connect using SSH.
- [ ] Update packages.
- [ ] Install Docker.
- [ ] Install Docker Compose plugin.
- [ ] Verify Docker.
- [ ] Verify Compose.

Security:

- [ ] Do not expose Docker daemon.
- [ ] Do not store GitHub tokens in shell history.
- [ ] Do not store unnecessary AWS access keys on EC2.
- [ ] Use scoped credentials where possible.

---

# 20. PHASE 15 — Manual EC2 Deployment

Before automation, manually prove the deployment.

Flow:

```text
GitHub
  |
  v
GHCR
  |
  v
EC2
  |
  v
docker compose pull
  |
  v
docker compose up -d
```

Checklist:

- [ ] Authenticate EC2 to GHCR.
- [ ] Pull exact image tag.
- [ ] Configure production environment securely.
- [ ] Start Compose.
- [ ] Check containers.
- [ ] Check Nginx.
- [ ] Check API health.
- [ ] Open application.
- [ ] Test login.
- [ ] Test browsing.
- [ ] Test library.
- [ ] Test progress.
- [ ] Test reviews.
- [ ] Test chat.
- [ ] Test SSE.
- [ ] Verify persistent SQLite.

## Gate

Manual deployment must work before CD automation.

---

# 21. PHASE 16 — Secrets and Configuration

Never commit:

```text
.env
AWS_ACCESS_KEY_ID
AWS_SECRET_ACCESS_KEY
JWT_SECRET
private keys
passwords
tokens
```

Checklist:

- [ ] `.env` remains gitignored.
- [ ] `.env.example` contains placeholders only.
- [ ] Production JWT secret exists only on secure server-side configuration/secret storage.
- [ ] GitHub secrets contain only required deployment credentials.
- [ ] Workflow logs cannot reveal secrets.
- [ ] Docker images contain no secrets.
- [ ] Git history contains no secrets.

For the first EC2 version, a documented secure server-side configuration approach is acceptable. A future enhancement can use AWS Secrets Manager.

---

# 22. PHASE 17 — CI → CD

Desired:

```text
Developer
    |
    v
Pull Request
    |
    v
CI + Security
    |
    v
Merge to main
    |
    v
Build image
    |
    v
Security scan
    |
    v
Push GHCR
    |
    v
Deploy EC2
    |
    v
Health check
    |
    +---- PASS ---> complete
    |
    +---- FAIL ---> rollback
```

Production deployment should be tied to the agreed production branch, not every feature branch.

Recommended:

```text
feature/*
    |
    v
Pull Request
    |
    v
main
    |
    v
production deployment
```

---

# 23. PHASE 18 — Immutable Image Deployment

Do not rely on an ambiguous production image.

Preferred:

```text
ghcr.io/anhhuynh1707/mangahub:sha-<commit>
```

Record:

```text
Git commit SHA
Docker image tag/digest
deployment timestamp
```

This makes rollback deterministic.

---

# 24. PHASE 19 — Deployment Script

Create:

```text
deploy/scripts/deploy.sh
```

Responsibilities:

1. Validate required configuration.
2. Pull requested image.
3. Verify image availability.
4. Recreate required services.
5. Preserve persistent volumes.
6. Start services.
7. Wait for health.
8. Return non-zero on failure.
9. Print useful logs.
10. Record deployed version.

Do not make the deployment destructive.

Never use:

```bash
docker compose down -v
```

as part of normal production deployment.

---

# 25. PHASE 20 — Health Checks

At minimum:

```text
GET /health
```

must be checked.

Also check where appropriate:

```text
Nginx
Frontend
API
Redis
SQLite availability
```

A deployment is not successful merely because containers show `running`.

---

# 26. PHASE 21 — Rollback

Keep the previous known-good version.

```text
previous SHA
current SHA
```

Create:

```text
deploy/scripts/rollback.sh
```

Requirements:

- [x] Identify previous version.
- [x] Pull previous image.
- [x] Restart affected services.
- [x] Wait for health.
- [x] Verify application.
- [x] Print rollback result.
- [x] Preserve database.

Never delete the database during rollback.

---

# 27. PHASE 22 — HTTPS

Target:

```text
Internet
   |
HTTPS :443
   |
Nginx
   |
MangaHub
```

Checklist:

- [ ] Obtain domain/subdomain.
- [ ] Point DNS to EC2.
- [ ] Configure Nginx.
- [ ] Obtain TLS certificate.
- [ ] Redirect HTTP to HTTPS.
- [ ] Verify certificate.
- [ ] Verify WebSocket over TLS.
- [ ] Verify SSE over TLS.
- [ ] Verify frontend uses HTTPS.
- [ ] Eliminate mixed-content issues.

---

# 28. PHASE 23 — AWS Monitoring

Monitor:

```text
EC2 CPU
EC2 memory
EC2 disk
application health
container status
application logs
Nginx logs
```

Checklist:

- [ ] Configure CloudWatch where appropriate.
- [ ] Create basic alarms.
- [ ] Monitor disk.
- [ ] Monitor application health.
- [ ] Keep logs useful.
- [ ] Never log secrets.
- [ ] Define log retention.

---

# 29. PHASE 24 — SQLite Backup and Recovery

Architecture:

```text
MangaHub
   |
SQLite
   |
Persistent volume
   |
Backup
```

Checklist:

- [ ] Identify database file.
- [ ] Identify Docker volume.
- [ ] Create backup procedure.
- [ ] Test backup.
- [ ] Test restore.
- [ ] Document backup location.
- [ ] Document retention.
- [ ] Test database persistence after container recreation.
- [ ] Never use `docker compose down -v` for normal deployment.

## Gate

A backup is not considered verified until a restore has actually been tested.

---

# 30. PHASE 25 — Security Hardening

## AWS

- [ ] MFA.
- [ ] Least-privilege IAM.
- [ ] Security Group reviewed.
- [ ] SSH restricted.
- [ ] Unused ports closed.
- [ ] Budget alerts enabled.

## EC2

- [ ] OS updated.
- [ ] Docker updated.
- [ ] SSH key protected.
- [ ] Password login disabled if not needed.
- [ ] Docker daemon not public.
- [ ] Disk monitored.

## Docker

- [ ] Minimal images.
- [ ] No embedded secrets.
- [ ] Non-root where practical.
- [ ] Read-only filesystem where compatible.
- [ ] Drop unnecessary capabilities where practical.
- [ ] Resource limits considered.
- [ ] Internal Redis/service networking.

## Application

- [ ] Strong JWT secret.
- [ ] Rate limiting retained.
- [ ] CORS reviewed.
- [ ] Security headers.
- [ ] Error responses do not expose internals.
- [ ] Logs do not expose credentials.
- [ ] WebSocket authentication reviewed.
- [ ] SSE authentication reviewed.

---

# 31. PHASE 26 — Production CORS

The application already has CORS behavior.

Production must use the real application origin.

Do not blindly configure:

```text
Allow-Origin: *
```

for authenticated production traffic.

Codex must inspect the existing middleware before modifying it.

---

# 32. PHASE 27 — Production Rate Limiting

The current application already includes per-IP rate limiting.

Do not remove it.

Verify:

- [ ] rate limiter remains enabled.
- [ ] reverse proxy does not break client IP handling.
- [ ] trusted proxy configuration is correct.
- [ ] health endpoint remains usable.
- [ ] login/register endpoints remain protected.

---

# 33. PHASE 28 — Docker Image Tagging

Use:

```text
latest
sha-<commit>
```

Recommended deployment tag:

```text
sha-<commit>
```

Example:

```text
ghcr.io/anhhuynh1707/mangahub:sha-abcdef123
```

---

# 34. PHASE 29 — Pull Request Validation

Before the final PR:

```bash
git status
git diff main...feature/aws-devsecops
```

Check:

- [ ] no `.env`
- [ ] no AWS credentials
- [ ] no private keys
- [ ] no tokens
- [ ] no passwords
- [ ] no `.DS_Store`
- [ ] no temporary files
- [ ] no debug code
- [ ] no accidental database file
- [ ] no localhost production URL
- [ ] no unnecessary public ports

---

# 35. PHASE 30 — Final CI/CD Test

## A. Pull Request

```text
feature/aws-devsecops
        |
        v
Pull Request
        |
        v
CI
        |
        +--> backend
        +--> frontend
        +--> E2E
        +--> Docker
        +--> security
```

Expected:

```text
PASS
```

## B. Merge

```text
PR approved
    |
    v
main
```

## C. Deployment

```text
main
 |
 v
GitHub Actions
 |
 v
Docker build
 |
 v
Security scan
 |
 v
GHCR
 |
 v
EC2
 |
 v
Deploy
 |
 v
Health check
```

## D. Application

Verify:

```text
Register
Login
Browse
Manga detail
Library
Progress
Review
Friends
Activity
Chat
Notifications
Logout
```

---

# 36. PHASE 31 — Failure Testing

A DevSecOps project should demonstrate failure handling.

## Test 1 — Backend container failure

Stop the API container in a controlled test environment.

Expected:

- restart policy recovers it, or
- monitoring detects failure.

## Test 2 — Bad image

Attempt a controlled deployment with an invalid image tag.

Expected:

```text
deployment fails
previous version remains available
```

## Test 3 — Health failure

Expected:

```text
deployment fails
```

## Test 4 — Rollback

Restore the previous known-good version.

Expected:

```text
application returns
```

## Test 5 — EC2 restart

Restart the instance.

Expected:

```text
Docker recovers
Compose services recover
persistent data remains
```

---

# 37. PHASE 32 — Documentation

Recommended:

```text
README.md
docs/DEVSECOPS.md
docs/AWS_DEPLOYMENT.md
docs/SECURITY.md
docs/ROLLBACK.md
docs/ARCHITECTURE.md
```

Document:

- architecture
- local setup
- production deployment
- CI/CD
- security
- monitoring
- backup
- rollback
- screenshots
- known limitations

Never document real secrets.

---

# 38. PHASE 33 — Final Architecture Diagram

The final diagram must represent the implementation that actually exists.

Recommended:

```text
                         GitHub
                           |
                    Pull Request
                           |
                           v
                  GitHub Actions CI
                           |
          +----------------+----------------+
          |                |                |
       Backend          Frontend         Security
        Tests             Build            Scan
          |                |                |
          +----------------+----------------+
                           |
                         E2E
                           |
                      Docker Build
                           |
                     Image Scan
                           |
                           v
                         GHCR
                           |
                           | SHA-tagged image
                           v
                    AWS EC2 Instance
                           |
                     Security Group
                           |
                        Nginx :443
                           |
             +-------------+-------------+
             |                           |
          Frontend                    Backend
                                         |
                              +----------+----------+
                              |          |          |
                           SQLite      Redis      Services
                              |
                           Backups
```

Do not claim components that are not implemented.

---

# 39. PHASE 34 — CV/Portfolio Version

Suggested project title:

**MangaHub — AWS DevSecOps Deployment**

Potential technology list:

```text
Go
Gin
React
TypeScript
Vite
SQLite
Redis
Docker
Docker Compose
GitHub Actions
GHCR
AWS EC2
AWS VPC
Nginx
HTTPS
CloudWatch
```

Example CV description:

> Built and deployed a containerized full-stack MangaHub platform using Go/Gin and React/TypeScript, extending an existing GitHub Actions CI pipeline into a DevSecOps workflow with automated testing, Docker image security scanning, GHCR publishing, AWS EC2 deployment, reverse proxying, HTTPS, health checks, persistent SQLite storage, backup/recovery, and rollback procedures.

Only claim technologies that are actually implemented.

---

# 40. Master Phase Matrix

| Phase | Area | Status |
|---|---|---|
| 0 | Baseline verification | ✅ |
| 1 | DevSecOps branch | 🟨 Branch complete; CI gate pending PR |
| 2 | Repository structure | ✅ |
| 3 | Production Docker | ✅ |
| 4 | Production frontend | ✅ |
| 5 | Production backend | ✅ |
| 6 | Docker networking | ✅ |
| 7 | Nginx | ✅ |
| 8 | Local production test | ✅ |
| 9 | Security scanning | ✅ Local gates passed |
| 10 | CI refactor | 🟨 Implemented locally; remote run pending push |
| 11 | AWS account | ⬜ |
| 12 | AWS VPC | ⬜ |
| 13 | Security Group | ⬜ |
| 14 | EC2 | ⬜ |
| 15 | Manual deployment | ⬜ |
| 16 | Secrets | ⬜ |
| 17 | CI/CD | ⬜ |
| 18 | Immutable images | ⬜ |
| 19 | Deploy script | ⬜ |
| 20 | Health checks | ⬜ |
| 21 | Rollback | ⬜ |
| 22 | HTTPS | ⬜ |
| 23 | Monitoring | ⬜ |
| 24 | SQLite backup | ⬜ |
| 25 | Security hardening | ⬜ |
| 26 | CORS | ⬜ |
| 27 | Rate limiting | ⬜ |
| 28 | Image tagging | ⬜ |
| 29 | PR validation | ⬜ |
| 30 | Final CI/CD test | ⬜ |
| 31 | Failure testing | ⬜ |
| 32 | Documentation | ⬜ |
| 33 | Architecture diagram | ⬜ |
| 34 | CV/portfolio | ⬜ |

---

# 41. Codex Operating Rules

## Rule 1 — Inspect before modifying

Before changing a file:

```text
read it
understand its role
identify dependencies
identify current behavior
then modify it
```

Never blindly replace configuration.

## Rule 2 — Preserve application functionality

Do not remove:

```text
authentication
manga browsing
library
progress
reviews
friends
activity feed
chat
WebSocket
SSE
TCP
UDP
gRPC
Redis
SQLite
```

unless explicitly justified.

## Rule 3 — Do not duplicate existing functionality

If CI already does something, extend it instead of creating conflicting pipelines.

## Rule 4 — No secrets

Never create or commit:

```text
.env
AWS keys
production JWT secret
private SSH key
GHCR token
password
```

## Rule 5 — Do not deploy from feature branches

Production deployment should use the agreed production branch.

## Rule 6 — Verify after every phase

After each phase:

```text
format
lint
test
build
run relevant integration tests
inspect git diff
```

## Rule 7 — No destructive production Docker operations

Do not use:

```bash
docker compose down -v
```

for normal deployment.

## Rule 8 — Do not expose internal services unnecessarily

Redis, SQLite, gRPC, TCP and UDP should not automatically become Internet-facing.

## Rule 9 — Do not invent AWS resources

Before creating AWS resources, document:

```text
resource
purpose
estimated cost
security impact
dependency
```

## Rule 10 — Stop at gates

If a phase fails:

```text
STOP
diagnose
fix
rerun
continue only after success
```

---

# 42. Exact Recommended Execution Order

Codex should execute the work in this order:

```text
1. Inspect repository
2. Verify main baseline
3. Create feature/aws-devsecops
4. Review existing Docker architecture
5. Create production Docker configuration
6. Fix production frontend URL strategy
7. Harden Compose networking
8. Add Nginx
9. Test production stack locally
10. Add security scanning
11. Refactor CI
12. Verify PR pipeline
13. Prepare AWS
14. Create VPC
15. Create subnet
16. Create Internet Gateway
17. Create route table
18. Create Security Group
19. Create EC2
20. Install Docker
21. Deploy manually
22. Verify application
23. Add production secrets/configuration
24. Implement CD
25. Deploy immutable image
26. Add health check
27. Add rollback
28. Configure HTTPS
29. Configure monitoring
30. Configure SQLite backup
31. Run security audit
32. Run failure tests
33. Update documentation
34. Create architecture diagram
35. Push final branch
36. Open Pull Request
37. Review CI
38. Review security
39. Merge only when all gates pass
```

---

# 43. Definition of Done

## Application

- [ ] MangaHub works.
- [ ] Frontend works.
- [ ] Backend works.
- [ ] SQLite works.
- [ ] Redis works.
- [ ] WebSocket works.
- [ ] SSE works.
- [ ] Required TCP/UDP/gRPC functionality works.

## Docker

- [ ] Production images build.
- [ ] Production Compose works.
- [ ] Internal services are not unnecessarily public.
- [ ] Persistent data survives restart.

## CI

- [ ] Backend tests pass.
- [ ] Frontend build passes.
- [ ] E2E passes.
- [ ] Docker smoke test passes.

## Security

- [ ] Dependency scan.
- [ ] Secret scan.
- [ ] Container scan.
- [ ] No secrets in repository.
- [ ] Security Group minimized.
- [ ] Production CORS reviewed.
- [ ] JWT secret protected.

## AWS

- [ ] VPC.
- [ ] Public subnet.
- [ ] Internet Gateway.
- [ ] Route table.
- [ ] Security Group.
- [ ] EC2.
- [ ] Docker.
- [ ] Production deployment.

## CD

- [ ] GHCR publishing.
- [ ] Immutable image tag.
- [ ] EC2 deployment.
- [ ] Health check.
- [ ] Rollback.

## Operations

- [ ] HTTPS.
- [ ] Monitoring.
- [ ] Logs.
- [ ] SQLite backup.
- [ ] Restore tested.

## GitHub

- [ ] Feature branch.
- [ ] Small commits.
- [ ] PR created.
- [ ] CI passes.
- [ ] Security passes.
- [ ] Documentation complete.
- [ ] PR reviewed.
- [ ] Merge into `main`.

---

# 44. Final Project Architecture

```text
                         ┌──────────────────────┐
                         │       Developer      │
                         │        macOS         │
                         └──────────┬───────────┘
                                    │
                                  git push
                                    │
                                    v
                         ┌──────────────────────┐
                         │       GitHub         │
                         │ feature/aws-devsecops│
                         └──────────┬───────────┘
                                    │
                               Pull Request
                                    │
                                    v
                         ┌──────────────────────┐
                         │   GitHub Actions     │
                         │                      │
                         │ Build                │
                         │ Test                 │
                         │ E2E                  │
                         │ Security             │
                         │ Docker               │
                         └──────────┬───────────┘
                                    │
                              merge to main
                                    │
                                    v
                         ┌──────────────────────┐
                         │       GHCR           │
                         │ Docker Image         │
                         │ SHA-tagged           │
                         └──────────┬───────────┘
                                    │
                                    v
                 ┌────────────────────────────────────┐
                 │              AWS                   │
                 │                                    │
                 │   VPC                              │
                 │    │                               │
                 │    └── Public Subnet               │
                 │             │                      │
                 │             v                      │
                 │          Security Group             │
                 │             │                      │
                 │             v                      │
                 │            EC2                     │
                 │             │                      │
                 │          Docker                    │
                 │             │                      │
                 │        Docker Compose               │
                 │             │                      │
                 │       ┌─────┴──────┐               │
                 │       │            │               │
                 │     Nginx       MangaHub            │
                 │       │            │               │
                 │    HTTPS     ┌─────┼─────┐         │
                 │              │     │     │         │
                 │           SQLite  Redis Services   │
                 │              │                     │
                 │           Backups                  │
                 └────────────────────────────────────┘
```

---

# 45. Most Important Starting Point

Because the current MangaHub repository already has:

```text
Backend CI
Frontend CI
Playwright E2E
Docker smoke test
GHCR publishing
JWT secret hardening
```

do **not** start the upgrade by reinstalling tools or rewriting the application.

Start here:

```text
CURRENT MAIN
    |
    | 0ab469a baseline
    v
Verified CI baseline
    |
    v
Create feature/aws-devsecops
    |
    v
Production Docker/Compose
    |
    v
Nginx + networking
    |
    v
Local production verification
    |
    v
DevSecOps security gates
    |
    v
AWS VPC + EC2
    |
    v
Manual deployment
    |
    v
Automated CD
    |
    v
HTTPS + monitoring + backup
    |
    v
Final PR
    |
    v
main
```

**Do not create the AWS infrastructure before the local production stack passes its gate.**

This keeps application problems, Docker problems, and AWS infrastructure problems separated.

---

# 46. Current Starting Status

Based on the repository inspection:

```text
Application development       COMPLETE
Docker Compose                 COMPLETE
Backend CI                     COMPLETE
Frontend CI                    COMPLETE
Playwright E2E                 COMPLETE
Docker smoke test              COMPLETE
GHCR publishing                COMPLETE
JWT secret hardening           COMPLETE

Production AWS architecture   NOT COMPLETE
DevSecOps security pipeline    TO EXTEND
AWS VPC                        NOT COMPLETE
EC2 deployment                 NOT COMPLETE
CD                             NOT COMPLETE
HTTPS                          NOT COMPLETE
Monitoring                     NOT COMPLETE
Backup/recovery                NOT COMPLETE
Rollback                       NOT COMPLETE
```

Therefore:

> **MangaHub AWS DevSecOps should be treated as an upgrade of the existing application, not a new application.**

The existing `main` branch remains the stable baseline. `feature/aws-devsecops` is the upgrade branch and should eventually be merged through a Pull Request.
