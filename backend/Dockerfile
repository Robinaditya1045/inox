# ==========================================
# Stage 1: Build the statically linked binaries
# ==========================================
FROM golang:1.25-alpine AS builder

# Install SSL ca-certificates so our app can securely connect to external APIs/databases later.
RUN apk update && apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy dependency manifests first to leverage Docker layer caching for module downloads.
# If go.mod/go.sum haven't changed, Docker reuses the cached layer instead of re-downloading modules.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the backend source code.
COPY . .

# BuildKit supplies these; they default to the builder's own platform, which is
# what we want when building directly on the target host. Pinning GOARCH to a
# literal (as this did) produces binaries that cannot execute on an arm64 VM.
ARG TARGETOS=linux
ARG TARGETARCH

# Compile the binaries:
# CGO_ENABLED=0 creates pure static Go binaries without dynamic C library dependencies.
# -ldflags="-s -w" strips debugging symbols, reducing binary file size by ~25%.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /app/bin/server ./cmd/server/main.go && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /app/bin/worker ./cmd/worker/main.go && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o /app/bin/migrate ./cmd/migrate/main.go

# ==========================================
# Stage 2: Minimal Production Runtime
# ==========================================
FROM alpine:3.19 AS runner

# Copy CA certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# Install FFmpeg for HLS transcoding (Adaptive Bitrate ladder generation).
# Without this, the media processor silently falls back to direct-stream proxy mode,
# meaning uploaded videos are never transcoded into multi-bitrate HLS segments.
# The worker is the process that shells out to it; server and worker share one image.
RUN apk add --no-cache ffmpeg

# Create a non-privileged system user for Kubernetes security compliance.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Local-storage fallback target (used when MinIO is unreachable at boot). Owned by
# appuser so the non-root process can actually write into a mounted volume.
RUN mkdir -p /app/storage_data && chown -R appuser:appgroup /app/storage_data

USER appuser

# Copy compiled binaries from Stage 1
COPY --from=builder /app/bin/server ./server
COPY --from=builder /app/bin/worker ./worker
COPY --from=builder /app/bin/migrate ./migrate

# Copy SQL migration files so the server can run them on startup
COPY --from=builder /app/migrations ./migrations

# Expose our application HTTP port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./server"]
