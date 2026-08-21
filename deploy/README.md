# MangaHub deployment assets

This directory is the home of the production deployment for MangaHub. The
existing root `docker-compose.yml` remains the local development stack.

The production stack will be implemented and verified in later checklist
phases. This file establishes the deployment contract before configuration is
added, so local and AWS behavior do not become mixed together.

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

## Planned structure

Files are added only when their phase is implemented and tested:

```text
deploy/
├── docker/
│   ├── docker-compose.prod.yml  # EC2 runtime definition
│   └── .env.example             # placeholders and documented inputs only
├── nginx/
│   └── nginx.conf               # SPA, API, WebSocket, and SSE routing
└── scripts/
    ├── deploy.sh                # immutable deployment + health gate
    ├── healthcheck.sh           # edge and dependency checks
    ├── rollback.sh              # return to previous known-good image
    ├── backup.sh                # consistent SQLite backup
    └── restore.sh               # explicit, guarded recovery procedure
```

## Intended release flow

```text
feature branch -> pull request -> CI and security gates -> main
     -> SHA-tagged images in GHCR -> manual approval -> EC2 deployment
     -> health verification -> record deployed version
```

Feature branches and pull requests must not deploy production. The first EC2
deployment will be manual; CD is added only after the same immutable images and
production Compose stack have been proven manually.
