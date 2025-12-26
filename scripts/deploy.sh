#!/bin/bash
# -----------------------------------------------------------------------
# Deployment Script for Quaero (Docker)
# -----------------------------------------------------------------------
# Builds Docker image and deploys to Docker container
# -----------------------------------------------------------------------

set -e

# Setup paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

echo -e "\033[0;36mQuaero Docker Deployment\033[0m"
echo -e "\033[0;36m========================\033[0m"

# Parse arguments
case "${1:-}" in
    --status|-s)
        echo -e "\n\033[0;36mDocker Status:\033[0m"
        docker ps --filter "name=quaero" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        exit 0
        ;;
    --logs|-l)
        docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" logs -f
        exit 0
        ;;
    --stop)
        echo -e "\033[1;33mStopping Docker containers...\033[0m"
        docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" down
        echo -e "\033[0;32mStopped\033[0m"
        exit 0
        ;;
    --rebuild|-r|"")
        # Default action - continue to build and deploy below
        ;;
    --help|-h)
        echo "Usage: $0 [--status|--logs|--stop|--rebuild|--help]"
        echo ""
        echo "Options:"
        echo "  --rebuild, -r  Stop, rebuild and redeploy (default)"
        echo "  --status, -s   Show Docker container status"
        echo "  --logs, -l     Follow Docker container logs"
        echo "  --stop         Stop Docker containers"
        echo "  --help, -h     Show this help"
        exit 0
        ;;
    *)
        echo -e "\033[0;31mUnknown option: $1\033[0m"
        echo "Use --help for usage information"
        exit 1
        ;;
esac

# Build and deploy
echo -e "\033[1;33mPreparing Docker build...\033[0m"

# Paths
COMMON_CONFIG="$PROJECT_ROOT/deployments/common"
DOCKER_CONFIG="$PROJECT_ROOT/deployments/docker/config"
STAGING="$PROJECT_ROOT/deployments/docker/.docker-staging"

# Clean and create staging
rm -rf "$STAGING"
mkdir -p "$STAGING/config" "$STAGING/job-definitions" "$STAGING/job-templates" "$STAGING/docs"

# Stage configs
echo -e "\033[0;90m  Staging configuration...\033[0m"
[ -f "$COMMON_CONFIG/connectors.toml" ] && cp "$COMMON_CONFIG/connectors.toml" "$STAGING/config/"
[ -f "$COMMON_CONFIG/email.toml" ] && cp "$COMMON_CONFIG/email.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/quaero.toml" ] && cp "$DOCKER_CONFIG/quaero.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/connectors.toml" ] && cp "$DOCKER_CONFIG/connectors.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/email.toml" ] && cp "$DOCKER_CONFIG/email.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/variables.toml" ] && cp "$DOCKER_CONFIG/variables.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/.env" ] && cp "$DOCKER_CONFIG/.env" "$STAGING/config/"

# Stage job-definitions
echo -e "\033[0;90m  Staging job-definitions...\033[0m"
[ -d "$COMMON_CONFIG/job-definitions" ] && cp "$COMMON_CONFIG/job-definitions"/*.toml "$STAGING/job-definitions/" 2>/dev/null || true
[ -d "$DOCKER_CONFIG/job-definitions" ] && cp "$DOCKER_CONFIG/job-definitions"/*.toml "$STAGING/job-definitions/" 2>/dev/null || true

# Stage job-templates
echo -e "\033[0;90m  Staging job-templates...\033[0m"
[ -d "$COMMON_CONFIG/job-templates" ] && cp "$COMMON_CONFIG/job-templates"/*.toml "$STAGING/job-templates/" 2>/dev/null || true
[ -d "$DOCKER_CONFIG/job-templates" ] && cp "$DOCKER_CONFIG/job-templates"/*.toml "$STAGING/job-templates/" 2>/dev/null || true

# Stage docs
echo -e "\033[0;90m  Staging documentation...\033[0m"
[ -f "$PROJECT_ROOT/README.md" ] && cp "$PROJECT_ROOT/README.md" "$STAGING/docs/"
[ -d "$PROJECT_ROOT/docs/architecture" ] && cp "$PROJECT_ROOT/docs/architecture"/*.md "$STAGING/docs/" 2>/dev/null || true

# Get version info
VERSION="dev"
BUILD_NUM="unknown"
VERSION_FILE="$PROJECT_ROOT/.version"
if [ -f "$VERSION_FILE" ]; then
    VERSION=$(grep "^version:" "$VERSION_FILE" | sed 's/version:\s*//' | tr -d ' ')
    BUILD_NUM=$(grep "^build:" "$VERSION_FILE" | sed 's/build:\s*//' | tr -d ' ')
fi
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo -e "\033[1;33mBuilding Docker image...\033[0m"
echo -e "\033[0;90m  Version: $VERSION, Build: $BUILD_NUM, Commit: $GIT_COMMIT\033[0m"

# Build Docker image
cd "$PROJECT_ROOT"
docker build \
    --build-arg VERSION="$VERSION" \
    --build-arg BUILD="$BUILD_NUM" \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    -t quaero:latest \
    -f deployments/docker/Dockerfile \
    .

# Cleanup staging
rm -rf "$STAGING"

echo -e "\033[0;32mDocker image built\033[0m"

# Check if container is running and stop it
if docker ps --filter "name=quaero" --format "{{.Names}}" | grep -q .; then
    echo -e "\033[1;33mStopping existing container...\033[0m"
    docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" down
fi

# Start container
echo -e "\033[1;33mStarting Docker container...\033[0m"
docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" up -d

echo -e "\n\033[0;32mDeployment complete!\033[0m"
echo -e "\033[0;36mAccess at: http://localhost:8080\033[0m"
docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" ps
