#!/bin/bash
# -----------------------------------------------------------------------
# Deployment Script for Quaero (Linux/macOS)
# -----------------------------------------------------------------------
#
# SYNOPSIS
#     Deploy and manage Quaero
#
# DESCRIPTION
#     This script helps deploy and manage the Quaero service
#     in different environments (local, Docker, production).
#
# PARAMETERS
#     -t, --target <target>    Deployment target: local, docker, or production (default: local)
#     -c, --config <path>      Path to configuration file
#     -b, --build              Build before deploying
#     -s, --stop               Stop the running service
#     -r, --restart            Restart the running service
#     --status                 Show service status
#     -l, --logs               Show service logs
#     -h, --help               Show this help message
#
# EXAMPLES
#     ./deploy.sh -t local
#         Deploy to local environment
#
#     ./deploy.sh -t docker -b
#         Build and deploy to Docker
#
#     ./deploy.sh --status
#         Show current service status
#
# -----------------------------------------------------------------------

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

# Default values
TARGET="local"
CONFIG_PATH=""
BUILD=false
STOP=false
RESTART=false
STATUS=false
LOGS=false

# Setup paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_ROOT/bin"

# Detect executable name based on OS
if [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "win32" ]]; then
    EXECUTABLE_PATH="$BIN_DIR/quaero.exe"
else
    EXECUTABLE_PATH="$BIN_DIR/quaero"
fi

show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Deploy and manage Quaero service"
    echo ""
    echo "Options:"
    echo "  -t, --target <target>   Deployment target: local, docker, production (default: local)"
    echo "  -c, --config <path>     Path to configuration file"
    echo "  -b, --build             Build before deploying"
    echo "  -s, --stop              Stop the running service"
    echo "  -r, --restart           Restart the running service"
    echo "  --status                Show service status"
    echo "  -l, --logs              Show service logs"
    echo "  -h, --help              Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0 -t local             Deploy to local environment"
    echo "  $0 -t docker -b         Build and deploy to Docker"
    echo "  $0 --status             Show current service status"
    echo "  $0 -t docker -s         Stop Docker containers"
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -t|--target)
            TARGET="$2"
            shift 2
            ;;
        -c|--config)
            CONFIG_PATH="$2"
            shift 2
            ;;
        -b|--build)
            BUILD=true
            shift
            ;;
        -s|--stop)
            STOP=true
            shift
            ;;
        -r|--restart)
            RESTART=true
            shift
            ;;
        --status)
            STATUS=true
            shift
            ;;
        -l|--logs)
            LOGS=true
            shift
            ;;
        -h|--help)
            show_help
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# Validate target
if [[ "$TARGET" != "local" && "$TARGET" != "docker" && "$TARGET" != "production" ]]; then
    echo -e "${RED}Invalid target: $TARGET. Must be: local, docker, or production${NC}"
    exit 1
fi

echo -e "${CYAN}Quaero Deployment Script${NC}"
echo -e "${CYAN}========================${NC}"
echo -e "${GRAY}Target: $TARGET${NC}"

# Determine config path
if [ -z "$CONFIG_PATH" ]; then
    case $TARGET in
        local)
            CONFIG_PATH="$PROJECT_ROOT/deployments/local/quaero.toml"
            if [ ! -f "$CONFIG_PATH" ]; then
                CONFIG_PATH="$BIN_DIR/quaero.toml"
            fi
            ;;
        docker)
            CONFIG_PATH="$PROJECT_ROOT/deployments/docker/config/quaero.toml"
            ;;
        production)
            CONFIG_PATH="$PROJECT_ROOT/deployments/quaero.toml"
            ;;
    esac
fi

echo -e "${GRAY}Config: $CONFIG_PATH${NC}"

# Build if requested
if [ "$BUILD" = true ]; then
    echo -e "\n${YELLOW}Building application...${NC}"
    BUILD_SCRIPT="$SCRIPT_DIR/build.sh"

    if [ "$TARGET" = "docker" ]; then
        # For docker, we'll build using the deploy process below
        echo -e "${GRAY}Docker build will be performed during deployment${NC}"
    else
        "$BUILD_SCRIPT"
        if [ $? -ne 0 ]; then
            echo -e "${RED}Build failed!${NC}"
            exit 1
        fi
        echo -e "${GREEN}Build completed successfully${NC}"
    fi
fi

# Function to get service status
get_service_status() {
    local pids
    pids=$(pgrep -f "quaero" 2>/dev/null || true)

    if [ -n "$pids" ]; then
        local pid=$(echo "$pids" | head -1)
        local start_time=$(ps -o lstart= -p "$pid" 2>/dev/null || echo "unknown")
        local memory=$(ps -o rss= -p "$pid" 2>/dev/null | awk '{printf "%.2f", $1/1024}' || echo "0")
        echo "running|$pid|$start_time|$memory"
    else
        echo "stopped"
    fi
}

# Show status
if [ "$STATUS" = true ]; then
    echo -e "\n${CYAN}Service Status:${NC}"
    status_info=$(get_service_status)

    if [[ "$status_info" == "stopped" ]]; then
        echo -e "${YELLOW}Status: STOPPED${NC}"
    else
        IFS='|' read -r status pid start_time memory <<< "$status_info"
        echo -e "${GREEN}Status: RUNNING${NC}"
        echo -e "${GREEN}PID: $pid${NC}"
        echo -e "${GREEN}Started: $start_time${NC}"
        echo -e "${GREEN}Memory: ${memory} MB${NC}"
    fi

    # Check Docker status
    if [ "$TARGET" = "docker" ]; then
        echo -e "\n${CYAN}Docker Status:${NC}"
        if command -v docker &> /dev/null; then
            docker_status=$(docker ps --filter "name=quaero" --format "{{.Status}}" 2>/dev/null || true)
            if [ -n "$docker_status" ]; then
                echo -e "${GREEN}Docker Container: $docker_status${NC}"
            else
                echo -e "${YELLOW}Docker Container: NOT RUNNING${NC}"
            fi
        else
            echo -e "${YELLOW}Docker not available${NC}"
        fi
    fi

    exit 0
fi

# Stop service
if [ "$STOP" = true ] || [ "$RESTART" = true ]; then
    echo -e "\n${YELLOW}Stopping service...${NC}"

    if [ "$TARGET" = "docker" ]; then
        # Stop Docker containers
        if command -v docker-compose &> /dev/null; then
            docker-compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" down 2>/dev/null || true
            echo -e "${GREEN}Docker containers stopped${NC}"
        elif command -v docker &> /dev/null && docker compose version &> /dev/null; then
            docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" down 2>/dev/null || true
            echo -e "${GREEN}Docker containers stopped${NC}"
        else
            echo -e "${RED}Docker Compose not available${NC}"
        fi
    else
        # Stop local process
        pids=$(pgrep -f "quaero" 2>/dev/null || true)

        if [ -n "$pids" ]; then
            pkill -f "quaero" 2>/dev/null || true
            sleep 2
            echo -e "${GREEN}Service stopped${NC}"
        else
            echo -e "${YELLOW}Service not running${NC}"
        fi
    fi

    if [ "$RESTART" = false ]; then
        exit 0
    fi
fi

# Show logs
if [ "$LOGS" = true ]; then
    echo -e "\n${CYAN}Service Logs:${NC}"

    if [ "$TARGET" = "docker" ]; then
        if command -v docker-compose &> /dev/null; then
            docker-compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" logs -f
        elif command -v docker &> /dev/null && docker compose version &> /dev/null; then
            docker compose -f "$PROJECT_ROOT/deployments/docker/docker-compose.yml" logs -f
        else
            echo -e "${RED}Docker Compose not available${NC}"
        fi
    else
        LOG_PATH="$PROJECT_ROOT/logs/quaero.log"
        if [ -f "$LOG_PATH" ]; then
            tail -50 -f "$LOG_PATH"
        else
            echo -e "${YELLOW}Log file not found: $LOG_PATH${NC}"
            echo -e "${GRAY}Check console output or configure file logging${NC}"
        fi
    fi

    exit 0
fi

# Deploy/Start service
echo -e "\n${YELLOW}Deploying service...${NC}"

if [ "$TARGET" = "docker" ]; then
    # Docker deployment - build image with configs baked in
    echo -e "${YELLOW}Preparing Docker build staging...${NC}"

    # Paths
    COMMON_CONFIG="$PROJECT_ROOT/deployments/common"
    DOCKER_CONFIG="$PROJECT_ROOT/deployments/docker/config"
    STAGING_PATH="$PROJECT_ROOT/deployments/docker/.docker-staging"

    # Clean and create staging directory
    rm -rf "$STAGING_PATH"
    mkdir -p "$STAGING_PATH/config"
    mkdir -p "$STAGING_PATH/job-definitions"
    mkdir -p "$STAGING_PATH/job-templates"

    # Stage config files: common first, then docker-specific overrides
    echo -e "${GRAY}  Staging configuration files...${NC}"

    # Copy common configs first (base layer)
    [ -f "$COMMON_CONFIG/connectors.toml" ] && cp "$COMMON_CONFIG/connectors.toml" "$STAGING_PATH/config/"
    [ -f "$COMMON_CONFIG/email.toml" ] && cp "$COMMON_CONFIG/email.toml" "$STAGING_PATH/config/"

    # Copy docker-specific configs (override layer)
    [ -f "$DOCKER_CONFIG/quaero.toml" ] && cp "$DOCKER_CONFIG/quaero.toml" "$STAGING_PATH/config/"
    [ -f "$DOCKER_CONFIG/connectors.toml" ] && cp "$DOCKER_CONFIG/connectors.toml" "$STAGING_PATH/config/"
    [ -f "$DOCKER_CONFIG/email.toml" ] && cp "$DOCKER_CONFIG/email.toml" "$STAGING_PATH/config/"
    [ -f "$DOCKER_CONFIG/variables.toml" ] && cp "$DOCKER_CONFIG/variables.toml" "$STAGING_PATH/config/"

    # Copy .env file: check deployments/env/ first, then docker/config/
    ENV_PATH="$PROJECT_ROOT/deployments/env/.env"
    DOCKER_ENV_PATH="$DOCKER_CONFIG/.env"
    if [ -f "$DOCKER_ENV_PATH" ]; then
        cp "$DOCKER_ENV_PATH" "$STAGING_PATH/config/.env"
    elif [ -f "$ENV_PATH" ]; then
        cp "$ENV_PATH" "$STAGING_PATH/config/.env"
    fi

    # Stage job-definitions: common first, then docker overrides
    echo -e "${GRAY}  Staging job-definitions...${NC}"
    if [ -d "$COMMON_CONFIG/job-definitions" ]; then
        for file in "$COMMON_CONFIG/job-definitions"/*.toml; do
            [ -f "$file" ] && cp "$file" "$STAGING_PATH/job-definitions/"
        done
    fi
    if [ -d "$DOCKER_CONFIG/job-definitions" ]; then
        for file in "$DOCKER_CONFIG/job-definitions"/*.toml; do
            [ -f "$file" ] && cp "$file" "$STAGING_PATH/job-definitions/"
        done
    fi

    # Stage job-templates: common first, then docker overrides
    echo -e "${GRAY}  Staging job-templates...${NC}"
    if [ -d "$COMMON_CONFIG/job-templates" ]; then
        for file in "$COMMON_CONFIG/job-templates"/*.toml; do
            [ -f "$file" ] && cp "$file" "$STAGING_PATH/job-templates/"
        done
    fi
    if [ -d "$DOCKER_CONFIG/job-templates" ]; then
        for file in "$DOCKER_CONFIG/job-templates"/*.toml; do
            [ -f "$file" ] && cp "$file" "$STAGING_PATH/job-templates/"
        done
    fi

    # Get version info for build args
    VERSION_FILE="$PROJECT_ROOT/.version"
    VERSION="dev"
    BUILD_NUM="unknown"
    if [ -f "$VERSION_FILE" ]; then
        VERSION=$(grep "^version:" "$VERSION_FILE" | sed 's/version:\s*//' | tr -d ' ')
        BUILD_NUM=$(grep "^build:" "$VERSION_FILE" | sed 's/build:\s*//' | tr -d ' ')
    fi
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

    echo -e "${YELLOW}Building Docker image...${NC}"
    echo -e "${GRAY}  Version: $VERSION, Build: $BUILD_NUM, Commit: $GIT_COMMIT${NC}"

    # Build the Docker image
    cd "$PROJECT_ROOT"
    docker build \
        --build-arg VERSION="$VERSION" \
        --build-arg BUILD="$BUILD_NUM" \
        --build-arg GIT_COMMIT="$GIT_COMMIT" \
        -t quaero:latest \
        -f deployments/docker/Dockerfile \
        .

    if [ $? -ne 0 ]; then
        echo -e "${RED}Docker build failed!${NC}"
        rm -rf "$STAGING_PATH"
        exit 1
    fi
    echo -e "${GREEN}Docker image built successfully${NC}"

    # Clean up staging directory
    rm -rf "$STAGING_PATH"

    echo -e "${YELLOW}Starting Docker container...${NC}"

    DOCKER_COMPOSE_FILE="$PROJECT_ROOT/deployments/docker/docker-compose.yml"

    if [ ! -f "$DOCKER_COMPOSE_FILE" ]; then
        echo -e "${RED}docker-compose.yml not found: $DOCKER_COMPOSE_FILE${NC}"
        exit 1
    fi

    # Use docker-compose or docker compose
    if command -v docker-compose &> /dev/null; then
        docker-compose -f "$DOCKER_COMPOSE_FILE" up -d
    elif command -v docker &> /dev/null && docker compose version &> /dev/null; then
        docker compose -f "$DOCKER_COMPOSE_FILE" up -d
    else
        echo -e "${RED}Docker Compose not available${NC}"
        exit 1
    fi

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}Docker container started successfully${NC}"
        sleep 2
        if command -v docker-compose &> /dev/null; then
            docker-compose -f "$DOCKER_COMPOSE_FILE" ps
        else
            docker compose -f "$DOCKER_COMPOSE_FILE" ps
        fi
    else
        echo -e "${RED}Failed to start Docker container${NC}"
        exit 1
    fi
else
    # Local deployment
    if [ ! -f "$EXECUTABLE_PATH" ]; then
        echo -e "${RED}Executable not found: $EXECUTABLE_PATH${NC}"
        echo -e "${YELLOW}Run with -b flag to build first${NC}"
        exit 1
    fi

    if [ ! -f "$CONFIG_PATH" ]; then
        echo -e "${RED}Config file not found: $CONFIG_PATH${NC}"
        exit 1
    fi

    echo -e "${YELLOW}Starting service...${NC}"

    # Start in background
    cd "$BIN_DIR"
    "$EXECUTABLE_PATH" -c "$CONFIG_PATH" &

    sleep 2

    status_info=$(get_service_status)
    if [[ "$status_info" != "stopped" ]]; then
        IFS='|' read -r status pid start_time memory <<< "$status_info"
        echo -e "${GREEN}Service started successfully${NC}"
        echo -e "${GREEN}PID: $pid${NC}"
        echo -e "${GREEN}Config: $CONFIG_PATH${NC}"
    else
        echo -e "${RED}Failed to start service${NC}"
        exit 1
    fi
fi

echo -e "\n${GREEN}Deployment completed!${NC}"

if [ "$TARGET" = "local" ]; then
    echo -e "\n${CYAN}Access the web interface at: http://localhost:8080${NC}"
    echo -e "${YELLOW}Available commands:${NC}"
    echo -e "  quaero serve   - Start web server and API"
    echo -e "  quaero collect - Run data collection"
    echo -e "  quaero query   - Execute search query"
    echo -e "  quaero version - Show version info"
fi
