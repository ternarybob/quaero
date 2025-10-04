# Docker Deployment

This directory contains Docker configuration for deploying Quaero in containers.

## Files

- `Dockerfile` - Multi-stage build configuration
- `docker-compose.yml` - Docker Compose orchestration

## Quick Start

### 1. Configure

Create a config directory and add your configuration:

```bash
mkdir config
cp ../quaero.toml config/
```

Edit `config/quaero.toml` with your settings.

### 2. Build and Run

```bash
# Build and start
docker-compose up -d

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

## Configuration

### Environment Variables

Set in `.env` file or pass to docker-compose:

```bash
VERSION=1.0.0
BUILD=production
GIT_COMMIT=$(git rev-parse HEAD)
PORT=8080
LOG_LEVEL=info
LOG_OUTPUT=both
```

### Volumes

- `quaero-data` - Application data storage
- `quaero-logs` - Log files
- `./config` - Configuration files (read-only)

## Build Arguments

Pass version information during build:

```bash
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg BUILD=production \
  --build-arg GIT_COMMIT=$(git rev-parse HEAD) \
  -f deployments/docker/Dockerfile \
  -t quaero:1.0.0 \
  .
```

## Health Check

The container includes a health check that pings `/health` every 30 seconds.

View health status:
```bash
docker ps
docker inspect quaero | grep Health
```

## Security

- Runs as non-root user (UID 1000)
- Minimal Alpine-based image
- Config mounted read-only
- CA certificates included for HTTPS

## Deployment

### Development

```bash
docker-compose up
```

### Production

```bash
# With specific version
VERSION=1.0.0 BUILD=prod GIT_COMMIT=$(git rev-parse HEAD) docker-compose up -d

# View logs
docker-compose logs -f quaero
```

## Troubleshooting

### Container won't start

```bash
# Check logs
docker-compose logs quaero

# Check health
docker inspect quaero
```

### Port conflicts

Change port in `.env`:
```bash
PORT=9090
```

### Volume permissions

Ensure volumes are writable by UID 1000:
```bash
docker-compose down -v
docker-compose up -d
```

## See Also

- [Local Deployment](../local/README.md) - Local development setup
- [Main README](../../README.md) - Full project documentation
