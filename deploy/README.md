# MangaHub deployment assets

This directory is the home of the production deployment for MangaHub. The
existing root `docker-compose.yml` remains the local development stack.

The production stack and its local proof are implemented. The server scripts are
locally validated; their first EC2 execution remains pending the manual AWS
foundation checkpoint.

## Deployment contract

- Only the edge proxy publishes web ports on the EC2 host.
- Redis, the API, SQLite, TCP, UDP, and gRPC use private Docker networking by
  default.
- Raw TCP, UDP, and gRPC host ports may be enabled for a controlled portfolio
  demonstration. Their AWS Security Group sources must be the owner's current
  public IPv4 address as a `/32`, never the entire Internet.
- The SQLite database lives on persistent storage and survives container
  recreation, deployment, and rollback.
- Production secrets are supplied at runtime. They are not stored in Git,
  Dockerfiles, Compose files, container images, or deployment logs.
- Deployments use immutable `sha-<commit>` image tags. `latest` is never the
  source of truth for deployment or rollback.
- A deployment succeeds only after application health checks pass.
- Normal deployment and rollback must never run `docker compose down -v`.

## Structure and implementation state

Files are added only when they have a real purpose:

```text
deploy/
├── docker/
│   ├── docker-compose.prod.yml        # production runtime definition
│   ├── docker-compose.prod.local.yml  # local image-build override
│   ├── docker-compose.raw.yml         # opt-in TCP/UDP/gRPC host bindings
│   └── .env.example                   # placeholders and documented inputs
├── nginx/
│   └── nginx.conf                     # edge, REST, WebSocket, and SSE routing
└── scripts/
    ├── configure-server.sh            # create protected first-run environment
    ├── deploy.sh                      # pull and start one exact full-SHA release
    ├── healthcheck.sh                 # behavior and exposure gate
    ├── rollback.sh                    # redeploy the previously recorded SHA
    ├── backup.sh                      # online SQLite backup, integrity, checksum, retention
    └── restore.sh                     # offline atomic restore with automatic recovery
```

The base production file never publishes raw service ports. Add
`docker-compose.raw.yml` only for a deliberate protocol demonstration. It binds
to `127.0.0.1` by default; EC2 may use `0.0.0.0` only together with owner-IP
`/32` AWS Security Group rules. The override adds a dedicated ingress bridge so
Docker can publish those ports while the services keep their private backend
network for API-to-service traffic.

## Local production-style test

Docker Desktop must be running. Generate an ephemeral secret in the current
terminal, then build the production images and start the stack:

```bash
export JWT_SECRET="$(openssl rand -base64 48)"
export HOST_HTTP_PORT=8088
export PUBLIC_ORIGIN=http://localhost:8088

docker compose \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  up -d --build
```

Verify the edge and API readiness:

```bash
docker compose \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  ps

curl --fail http://localhost:8088/health
```

For a local raw-protocol test, include the opt-in override:

```bash
docker compose \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  -f deploy/docker/docker-compose.raw.yml \
  up -d --build
```

The CLI defaults to local endpoints. For the later EC2 demonstration, set these
only in the terminal used for testing, replacing the documentation address:

```bash
export MANGAHUB_API_URL=http://203.0.113.10/api
export MANGAHUB_TCP_ADDR=203.0.113.10:9090
export MANGAHUB_UDP_ADDR=203.0.113.10:9091
export MANGAHUB_GRPC_ADDR=203.0.113.10:9092
```

Stop containers without deleting persistent volumes:

```bash
docker compose \
  -f deploy/docker/docker-compose.prod.yml \
  -f deploy/docker/docker-compose.prod.local.yml \
  down
```

## Intended release flow

```text
feature branch -> pull request -> CI and security gates -> main
     -> SHA-tagged images in GHCR -> manual approval -> EC2 deployment
     -> health verification -> record deployed version
```

The selected `features/devsecops` branch may publish full-commit SHA candidate
images after all gates pass so the requested manual EC2 proof can happen before
the pull request. It never updates `latest` and never deploys automatically.
Pull requests publish nothing. `main` publishes both its full-SHA images and the
convenience `latest` tag. The first EC2 deployment remains manual; CD is added
only after the same immutable images and production Compose stack have been
proven manually.

## EC2 script contract

Run the scripts with `sudo` because Docker daemon access is root-equivalent. On
the first server setup, `configure-server.sh` accepts the exact full Git SHA and
the EC2 public IPv4 address. It writes `/opt/mangahub/.env` with mode `0600` and
generates the JWT secret directly into that file without printing it:

```bash
sudo ./deploy/scripts/configure-server.sh FULL_40_CHARACTER_SHA PUBLIC_IPV4
```

Deploy the same backend/frontend SHA without raw host ports:

```bash
sudo ./deploy/scripts/deploy.sh FULL_40_CHARACTER_SHA
```

After the owner-only AWS Security Group rules exist, deliberately enable raw
bindings for the same release:

```bash
sudo ./deploy/scripts/deploy.sh FULL_40_CHARACTER_SHA --with-raw
```

The deploy command pulls before changing containers, never builds on EC2, never
uses `down -v`, and advances `/var/lib/mangahub-deploy/current-version` only
after the public health, security-header, private-endpoint, Swagger, and frontend
checks pass. If a later release fails, inspect its logs and restore the recorded
previous release:

```bash
sudo ./deploy/scripts/rollback.sh
```

Rollback is unavailable until a known-good release has been followed by an
attempted different release. See `docs/ROLLBACK.md` for the operator procedure.
Online backup and atomic restore are implemented and locally rehearsed; see
`docs/BACKUP.md`. Their first EC2 rehearsal remains a manual gate. Rollback is
not a substitute for a SQLite backup.
