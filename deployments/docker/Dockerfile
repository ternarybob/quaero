# Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Allow Go toolchain to download newer versions if needed
ENV GOTOOLCHAIN=auto
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

# Install runtime dependencies including Chromium for web crawling
RUN apk add --no-cache ca-certificates tzdata chromium && \
    ln -s /usr/bin/chromium-browser /usr/bin/google-chrome

# Configure Chromium for headless operation
ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROMEDP_DISABLE_GPU=true
ENV CHROMEDP_NO_SANDBOX=true

# Create non-root user
RUN addgroup -g 1000 quaero && \
    adduser -D -u 1000 -G quaero quaero

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/quaero .

# Copy pages (HTML templates) from builder
COPY --from=builder /build/pages /app/pages

# Create directories for config and data
RUN mkdir -p /app/config /app/data /app/logs /app/job-definitions /app/templates

# Copy configuration files from staging directory
# These are prepared by deploy.ps1 before docker build
COPY deployments/docker/.docker-staging/config/ /app/config/
COPY deployments/docker/.docker-staging/job-definitions/ /app/job-definitions/
COPY deployments/docker/.docker-staging/templates/ /app/templates/

# Copy .env to app root if present (for KV store loading)
# The .env file in /app/config/ is also symlinked to /app/.env for compatibility
RUN if [ -f /app/config/.env ]; then cp /app/config/.env /app/.env; fi

# Set ownership
RUN chown -R quaero:quaero /app

# Switch to non-root user
USER quaero

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
# Note: Flags must come before 'serve' for proper parsing
ENTRYPOINT ["/app/quaero"]
CMD ["-c", "/app/config/quaero.toml", "serve"]
