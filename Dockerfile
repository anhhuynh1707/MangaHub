# Build stage
FROM golang:1.25.6 AS builder

WORKDIR /app

# Enable SQLite FTS5 for full-text search support
ENV GOFLAGS=-tags=sqlite_fts5

# The default golang image is Debian-based and ALREADY has gcc installed!
# This completely skips the step that was freezing your build.
# Copy go mod and sum files
COPY go.mod go.sum ./
RUN go mod download


# Copy source code
COPY . .
# Build all binaries (with CGO for SQLite support)
RUN go build -o /app/bin/api-server ./cmd/api-server
RUN go build -o /app/bin/udp-server ./cmd/udp-server
RUN go build -o /app/bin/tcp-server ./cmd/tcp-server
RUN go build -o /app/bin/grpc-server ./cmd/grpc-server
RUN go build -o /app/bin/mangahub ./cmd/cli

# Final stage
FROM debian:12-slim

WORKDIR /app

# Install runtime dependencies (ca-certificates for HTTPS)
RUN apt-get update && apt-get install -y ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

# Copy binaries from builder
COPY --from=builder /app/bin/* /usr/local/bin/

# Create data directory
RUN mkdir -p /app/data

# The container will run api-server by default
CMD ["api-server"]
