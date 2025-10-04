# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build arguments for versioning
ARG VERSION=dev
ARG BUILD=unknown
ARG GIT_COMMIT=unknown

# Build the application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s \
    -X 'github.com/ternarybob/quaero/internal/common.Version=${VERSION}' \
    -X 'github.com/ternarybob/quaero/internal/common.Build=${BUILD}' \
    -X 'github.com/ternarybob/quaero/internal/common.GitCommit=${GIT_COMMIT}'" \
    -o quaero \
    ./cmd/quaero

# Runtime stage
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 quaero && \
    adduser -D -u 1000 -G quaero quaero

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/quaero .

# Create directories
RUN mkdir -p /app/data /app/logs && \
    chown -R quaero:quaero /app

# Switch to non-root user
USER quaero

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["/app/quaero"]
CMD ["serve", "-c", "/app/config/quaero.toml"]
