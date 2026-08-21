# MangaHub release rollback

Status: **two-version local rehearsal passed; first EC2 rehearsal pending**.

MangaHub deploys backend and frontend images with the same full Git SHA. The
deployment script records a release only after all required containers are
running and the edge health gate passes. Rollback calls that same deployment
path, so it pulls the old immutable images and repeats the same checks.

## Safety invariants

- Normal deploy and rollback never run `docker compose down -v`.
- The named `mangahub-data` and `redis-data` volumes survive container
  replacement.
- `/opt/mangahub/.env` is reused; rollback does not expose or rotate the JWT
  secret.
- The previous release includes its mode: `base`, owner-only `raw`,
  `cloudwatch`, or `raw-cloudwatch`.
- A failed candidate does not advance `current-version`.
- Image rollback is not database rollback. Back up SQLite before any schema or
  destructive data change.

Release state is root-only under `/var/lib/mangahub-deploy`:

```text
current-version
current-mode
previous-version
previous-mode
deployments.tsv
deploy.lock
```

`deployments.tsv` records UTC time, full SHA, mode, backend image ID, and
frontend image ID. Never edit these files to simulate a successful release.

## Normal rollback

When a different release has been attempted after a known-good deployment:

```bash
cd /opt/mangahub-src
sudo ./deploy/scripts/rollback.sh
```

The command prints the target, pulls both prior SHA tags, recreates services
without removing volumes, and succeeds only after the health gate passes.

Verify afterward:

```bash
sudo cat /var/lib/mangahub-deploy/current-version
sudo docker compose \
  --project-name mangahub-prod \
  --env-file /opt/mangahub/.env \
  -f deploy/docker/docker-compose.prod.yml \
  ps
curl --fail http://127.0.0.1/health
```

The SHA must be the intended prior release, the long-running containers must be
running, and the public browser path must still work. Do not print the
environment file.

## Explicit recovery target

An operator can deliberately choose an older known-good full SHA:

```bash
sudo ./deploy/scripts/rollback.sh FULL_40_CHARACTER_SHA
```

Restore a release in raw mode only while the AWS Security Group has the three
owner IPv4 `/32` rules:

```bash
sudo ./deploy/scripts/rollback.sh FULL_40_CHARACTER_SHA --with-raw
```

Preserve CloudWatch logging only after its scoped instance policy and seven-day
log group are verified:

```bash
sudo ./deploy/scripts/rollback.sh FULL_40_CHARACTER_SHA --with-cloudwatch
sudo ./deploy/scripts/rollback.sh \
  FULL_40_CHARACTER_SHA --with-raw --with-cloudwatch
```

Use only a SHA whose GitHub Actions run passed and whose matching backend and
frontend GHCR tags exist. Never use `latest` as a recovery target.

## Failure handling

If rollback itself fails:

1. Keep the SQLite volume and protected environment file untouched.
2. Inspect the logs printed by the script.
3. Run the production Compose `ps` command above.
4. Confirm both GHCR SHA tags are anonymously pullable and the EC2 instance has
   outbound HTTPS access.
5. Confirm port 80 is free and Docker has adequate disk space.
6. Retry the explicit known-good SHA only after correcting the cause.

If there is no `previous-version`, no earlier release has been recorded. Fix or
redeploy the first known-good release explicitly; do not invent a SHA and do not
delete the data volume.

## Local rehearsal evidence

On 2026-08-21, the release path was exercised with the anonymous GHCR
`linux/amd64` candidates while preserving the existing production-proof named
volume:

1. `d5473615f97d285c265656ddbc23aa1d4c3ce529` was healthy in raw mode.
2. `675ff8a4231a0b832070d6130ca1518dfd0e2e4a` deployed in base mode and passed
   the edge health gate.
3. The recorded previous state was the full `d547361…` SHA plus raw mode.
4. `rollback.sh` restored that release and passed the same health gate.
5. The SQLite file retained inode `458139` and size `4096` bytes across both
   container replacements; the named volume remained
   `mangahub-prod_mangahub-data`.
6. The post-rollback TCP welcome, UDP request/response, and gRPC TCP listener
   checks passed.

This is local operational evidence, not a claim that the AWS instance exists.
The EC2 rehearsal remains gated by the manual account and VPC checkpoints.
