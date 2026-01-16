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

# --- Logging Setup ---
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/deploy-$(date +%Y-%m-%d-%H-%M-%S).log"

# Function to limit log files to most recent 10
limit_deploy_log_files() {
    local count=$(ls -1 "$LOG_DIR"/deploy-*.log 2>/dev/null | wc -l)
    if [ "$count" -gt 10 ]; then
        ls -1t "$LOG_DIR"/deploy-*.log | tail -n +11 | xargs rm -f
        echo -e "\033[0;90mRemoved old log files\033[0m"
    fi
}

# Limit old log files before starting transcript
limit_deploy_log_files

# Start transcript - capture all output to log file while displaying to terminal
exec > >(tee -a "$LOG_FILE") 2>&1

echo -e "\033[0;35mternarybob (parent) -> quaero\033[0m"
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

# Stop existing container and clean up Docker resources
echo -e "\033[1;33mStopping existing containers...\033[0m"
docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" down -v 2>/dev/null || true

echo -e "\033[1;33mPruning Docker resources...\033[0m"
docker container prune -f 2>/dev/null || true
docker image prune -f 2>/dev/null || true
docker volume prune -f 2>/dev/null || true

# Remove old quaero image to force fresh build
OLD_IMAGE=$(docker images -q quaero:latest 2>/dev/null)
if [ -n "$OLD_IMAGE" ]; then
    echo -e "\033[1;33mRemoving old quaero:latest image...\033[0m"
    docker rmi quaero:latest -f 2>/dev/null || true
fi

# Paths
COMMON_CONFIG="$PROJECT_ROOT/deployments/common"
DOCKER_CONFIG="$PROJECT_ROOT/deployments/docker/config"
STAGING="$PROJECT_ROOT/deployments/docker/.docker-staging"

# Clean and create staging
rm -rf "$STAGING"
mkdir -p "$STAGING/config" "$STAGING/job-definitions" "$STAGING/templates" "$STAGING/docs"

# Stage configs
echo -e "\033[0;90m  Staging configuration...\033[0m"
[ -f "$COMMON_CONFIG/connectors.toml" ] && cp "$COMMON_CONFIG/connectors.toml" "$STAGING/config/"
[ -f "$COMMON_CONFIG/email.toml" ] && cp "$COMMON_CONFIG/email.toml" "$STAGING/config/"
# Use docker-specific config template from common, or fall back to docker config dir
if [ -f "$COMMON_CONFIG/quaero.docker.toml" ]; then
    cp "$COMMON_CONFIG/quaero.docker.toml" "$STAGING/config/quaero.toml"
elif [ -f "$DOCKER_CONFIG/quaero.toml" ]; then
    cp "$DOCKER_CONFIG/quaero.toml" "$STAGING/config/"
else
    echo -e "\033[0;31mERROR: No quaero.toml found for Docker deployment!\033[0m"
    echo "Expected at: $COMMON_CONFIG/quaero.docker.toml or $DOCKER_CONFIG/quaero.toml"
    exit 1
fi
[ -f "$DOCKER_CONFIG/connectors.toml" ] && cp "$DOCKER_CONFIG/connectors.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/email.toml" ] && cp "$DOCKER_CONFIG/email.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/variables.toml" ] && cp "$DOCKER_CONFIG/variables.toml" "$STAGING/config/"
[ -f "$DOCKER_CONFIG/.env" ] && cp "$DOCKER_CONFIG/.env" "$STAGING/config/"

# Stage job-definitions (Docker uses ONLY job-definitions-docker, not common job-definitions)
echo -e "\033[0;90m  Staging job-definitions from: $COMMON_CONFIG/job-definitions-docker/\033[0m"
if [ -d "$COMMON_CONFIG/job-definitions-docker" ]; then
    JOB_DEF_FILES=("$COMMON_CONFIG/job-definitions-docker"/*.toml)
    if [ -e "${JOB_DEF_FILES[0]}" ]; then
        JOB_DEF_COUNT=$(ls -1 "$COMMON_CONFIG/job-definitions-docker"/*.toml 2>/dev/null | wc -l)
        echo -e "\033[0;90m  Found $JOB_DEF_COUNT job definition(s):\033[0m"
        for file in "$COMMON_CONFIG/job-definitions-docker"/*.toml; do
            if [ -f "$file" ]; then
                filename=$(basename "$file")
                echo -e "\033[0;36m    - $filename\033[0m"
                cp "$file" "$STAGING/job-definitions/"
            fi
        done
    else
        echo -e "\033[1;33m  WARNING: No job definitions found in job-definitions-docker!\033[0m"
    fi
else
    echo -e "\033[1;33m  WARNING: job-definitions-docker directory not found!\033[0m"
fi

# Stage templates
echo -e "\033[0;90m  Staging templates...\033[0m"
[ -d "$COMMON_CONFIG/templates" ] && cp "$COMMON_CONFIG/templates"/*.toml "$STAGING/templates/" 2>/dev/null || true
[ -d "$DOCKER_CONFIG/templates" ] && cp "$DOCKER_CONFIG/templates"/*.toml "$STAGING/templates/" 2>/dev/null || true

# Stage docs
echo -e "\033[0;90m  Staging documentation...\033[0m"
[ -f "$PROJECT_ROOT/README.md" ] && cp "$PROJECT_ROOT/README.md" "$STAGING/docs/"
[ -d "$PROJECT_ROOT/docs/architecture" ] && cp "$PROJECT_ROOT/docs/architecture"/*.md "$STAGING/docs/" 2>/dev/null || true

# Ensure directories are not empty (Docker COPY fails on empty dirs)
[ -z "$(ls -A "$STAGING/config" 2>/dev/null)" ] && touch "$STAGING/config/.gitkeep"
[ -z "$(ls -A "$STAGING/job-definitions" 2>/dev/null)" ] && touch "$STAGING/job-definitions/.gitkeep"
[ -z "$(ls -A "$STAGING/templates" 2>/dev/null)" ] && touch "$STAGING/templates/.gitkeep"

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

# Build Docker image (--no-cache ensures staged configs are always picked up)
cd "$PROJECT_ROOT"
docker build \
    --no-cache \
    --build-arg VERSION="$VERSION" \
    --build-arg BUILD="$BUILD_NUM" \
    --build-arg GIT_COMMIT="$GIT_COMMIT" \
    -t quaero:latest \
    -f deployments/docker/Dockerfile \
    .
BUILD_RESULT=$?

# Cleanup staging
rm -rf "$STAGING"

if [ $BUILD_RESULT -ne 0 ]; then
    echo -e "\033[0;31mDocker build failed!\033[0m"
    exit 1
fi
echo -e "\033[0;32mDocker image built\033[0m"

# Start container
echo -e "\033[1;33mStarting Docker container...\033[0m"
docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" up -d

if [ $? -eq 0 ]; then
    echo -e "\n\033[0;32mDeployment complete!\033[0m"
    echo -e "\033[0;36mAccess at: http://localhost:9000\033[0m"
    docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" ps
else
    echo -e "\033[0;31mFailed to start container\033[0m"
    exit 1
fi
