# syntax=docker/dockerfile:1

# Build stage. Keep the Go patch version aligned with go.mod/CI.
FROM golang:1.25.6-bookworm AS builder

WORKDIR /app

# Enable SQLite FTS5 for full-text search support
ENV GOFLAGS=-tags=sqlite_fts5

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download && go mod verify

# Copy only backend build inputs. This keeps frontend files, local environment
# files, databases, and repository metadata out of the image build.
COPY cmd ./cmd
COPY data ./data
COPY docs ./docs
COPY internal ./internal
COPY pkg ./pkg
COPY proto ./proto

# CGO is required by mattn/go-sqlite3. Cache compiler output between builds.
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app/bin/api-server ./cmd/api-server && \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app/bin/udp-server ./cmd/udp-server && \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app/bin/tcp-server ./cmd/tcp-server && \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app/bin/grpc-server ./cmd/grpc-server && \
    CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /app/bin/mangahub ./cmd/cli

# Final stage
FROM debian:12-slim

WORKDIR /app

# curl is used only by the Compose health check. No compiler or package manager
# cache remains in the runtime layer.
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates curl tzdata && \
    rm -rf /var/lib/apt/lists/* && \
    groupadd --system --gid 10001 mangahub && \
    useradd --system --uid 10001 --gid mangahub --home-dir /nonexistent \
      --shell /usr/sbin/nologin mangahub && \
    install -d -o mangahub -g mangahub /app/data

COPY --from=builder --chown=10001:10001 /app/bin/* /usr/local/bin/

USER 10001:10001

EXPOSE 8080 9090 9091/udp 9092
STOPSIGNAL SIGTERM

CMD ["api-server"]
