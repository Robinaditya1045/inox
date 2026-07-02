# ==========================================
# Stage 1: Build the statically linked binary
# ==========================================
FROM golang:1.24-alpine AS builder

# Install SSL ca-certificates so our app can securely connect to external APIs/databases later.
RUN apk update && apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy go.mod first to leverage Docker layer caching for dependency downloads.
# If go.mod hasn't changed, Docker reuses the cached layer instead of re-downloading modules.
COPY go.mod ./

# Copy the rest of the backend source code.
COPY . .

# Compile the binary:
# CGO_ENABLED=0 creates a pure static Go binary without dynamic C library dependencies.
# -ldflags="-s -w" strips debugging symbols, reducing binary file size by ~25%.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/server ./cmd/server/main.go

# ==========================================
# Stage 2: Minimal Production Runtime
# ==========================================
FROM alpine:3.19 AS runner

# Copy CA certificates and timezone data from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

WORKDIR /app

# Create a non-privileged system user for Kubernetes security compliance.
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Copy only the compiled binary from Stage 1
COPY --from=builder /app/bin/server ./server

# Expose our application HTTP port
EXPOSE 8080

# Run the binary
ENTRYPOINT ["./server"]