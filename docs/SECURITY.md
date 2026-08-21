# MangaHub deployment security policy

This policy defines the security boundary for the portfolio environment. It is
an operational requirement for deployment, not a claim that every control is
already implemented.

## Environment classification

MangaHub on AWS is a learning and portfolio demonstration. It must contain only
test users and demonstration content. Do not enter a real password that is used
anywhere else, personal data, payment information, confidential material, or
production credentials.

Because the initial domain-free deployment uses HTTP, browser credentials and
JWTs are not encrypted in transit. HTTP is acceptable only for short-lived demo
testing with throwaway accounts. A real public service requires a domain and
HTTPS before accepting users.

## Trust boundaries

| Boundary | Allowed traffic | Rule |
|---|---|---|
| Internet to edge | HTTP 80 initially; HTTPS 443 in a future domain phase | Only the edge proxy is public for browser traffic |
| Owner to raw services | TCP 9090, UDP 9091, and gRPC 9092 | AWS Security Group source is the owner's current IPv4 `/32` and rules are removed when unused |
| Edge to application | Frontend, API, WebSocket, and SSE over Docker networks | API port 8080 is not published publicly |
| Application to data | Redis and SQLite over private container/storage paths | Redis 6379 and database files are never Internet-facing |
| Administrator to EC2 | Systems Manager Session Manager | No inbound SSH rule; no shared private key |
| EC2 to external services | HTTPS for SSM, updates, GHCR, GitHub, and MangaDex | Outbound access is reviewed as the implementation matures |

HTTPS does not carry raw TCP or UDP automatically. Each protocol retains its
own listener and security controls. The initial raw implementations are suitable
only for the owner-restricted demonstration; Internet-wide exposure would first
require protocol-specific authentication review, TLS or another encrypted
transport, abuse protection, and monitoring.

## Secrets and credentials

Never commit or copy into a Docker image:

- `.env` files containing values;
- `JWT_SECRET`;
- AWS access keys or session tokens;
- GitHub or GHCR tokens;
- SSH private keys;
- passwords, certificates, or backup encryption keys.

Repository examples contain names and placeholders only. The production JWT
secret is generated on the server, stored with owner-only permissions, shared
only by the services that validate MangaHub tokens, and never printed. GitHub
Actions uses built-in or narrowly scoped credentials and masks secret values.

The EC2 workload uses an IAM instance role with temporary credentials. No AWS
access key is stored on EC2. Human AWS access uses MFA and avoids the root user
for routine work.

## Network policy

- Create a dedicated MangaHub Security Group; do not edit the default group.
- Never allow SSH, Redis, the API port, or raw service ports from `0.0.0.0/0`.
- Port 80 is the only initial Internet-wide inbound rule.
- Raw protocol rules use the owner's current public IPv4 `/32` and are removed
  outside demonstrations.
- Docker's remote API is never exposed.
- Nginx or the selected edge proxy forwards the original client address only to
  an application configured to trust that known proxy, so logging and rate
  limiting cannot be bypassed with spoofed headers.

## Application and container policy

- `JWT_SECRET` remains mandatory and strong; rotation invalidates active tokens.
- Production CORS allows the deployed origin only. Wildcard credentialed CORS
  is prohibited.
- Authentication, rate limiting, request IDs, structured logs, WebSocket auth,
  and SSE auth are verified behind the proxy.
- A minimal public health endpoint may report readiness; database counts,
  client lists, cache internals, and Swagger are not exposed publicly without a
  deliberate decision.
- Images use minimal supported bases, run as non-root where SQLite permissions
  allow, contain no build-time secrets, and are scanned before publication.
- Redis is treated as rebuildable cache data. SQLite is persistent application
  data and is never deleted by deployment or rollback.

## Automated scanning policy

Every pull request and selected branch push runs the same reusable security
workflow. Reachable Go vulnerabilities, high/critical npm advisories, secret
findings, repository misconfigurations, and fixable high/critical production
image findings block publication. GitHub Actions are pinned to full commit SHAs,
and Dependabot monitors actions, Go, npm, and Docker dependencies weekly.

Base distributions sometimes publish high/critical records before a package fix
exists. Image scans report and review those records, but the blocking image gate
uses Trivy's `ignore-unfixed` policy. This is not a permanent exception for a
fixable issue: supported minimal bases are refreshed, any available fixed version
must be adopted, and the accepted residual risk is limited to this disposable
portfolio environment. `govulncheck` separately verifies whether Go application
code reaches a known vulnerable symbol.

Gitleaks scans complete history. Its only allowlist is rule-specific and requires
both an exact historical commit and the exact Postman collection path for expired
local fixture JWTs that have been removed from the current collection. It does
not permit a new token anywhere in the repository.

## Deployment, backup, and logging policy

- Deploy immutable SHA-tagged images; record commit, image tag/digest, and time.
- Health checks validate behavior, not just a `running` container state.
- Keep the previous known-good image for rollback.
- Never run `docker compose down -v` during deployment or rollback.
- A backup is valid only after restore is tested on a separate path or volume.
- Local backup files and checksum sidecars are mode `0600`; EC2 storage must be
  EBS-encrypted. The default retention is seven verified pairs. They remain on
  the instance until a future encrypted S3 export is implemented, so they do
  not yet protect against instance or EBS loss.
- Logs exclude authorization headers, JWTs, passwords, environment dumps, and
  other secrets; CloudWatch retention is finite.

## Stop-work conditions

Stop deployment and investigate if a secret appears in Git/history/logs, a scan
finds an unaccepted critical or high issue, a Security Group is broader than
documented, persistence or backup verification fails, health checks fail, costs
cannot be bounded, or rollback would risk deleting the database.
