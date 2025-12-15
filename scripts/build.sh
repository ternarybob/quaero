#!/bin/bash
# -----------------------------------------------------------------------
# Build Script for Quaero (Linux/macOS)
# -----------------------------------------------------------------------
# Simplified: 2025-11-08
# Removed backward compatibility parameters (-Clean, -Verbose, -Release,
# -ResetDatabase, -Environment, -Version)
# See docs/simplify-build-script/ for migration guide
# -----------------------------------------------------------------------
#
# SYNOPSIS
#     Build script for Quaero
#
# DESCRIPTION
#     This script builds Quaero for local development and testing.
#
#     Four operations supported:
#     1. Default build (no parameters) - Builds executable silently, no deployment
#     2. --deploy - Builds and deploys all files to bin directory (stops service if running)
#     3. --run - Builds, deploys, and starts application in background
#     4. --web - Deploys only pages directory and restarts application (no build, no version update)
#
# PARAMETERS
#     --deploy    Deploy all required files to bin directory after building
#                 (config, pages, Chrome extension, job definitions)
#                 Stops any running service before deployment
#
#     --run       Build, deploy, and run the application in the background
#                 Automatically triggers deployment before starting the service
#
#     --web       Deploy only the pages directory and restart the application
#                 Does not build or update version - for rapid frontend development
#                 Stops service, copies pages, restarts service
#
# EXAMPLES
#     ./build.sh
#         Build quaero executable only (no deployment, silent on success)
#
#     ./build.sh --deploy
#         Build and deploy all files to bin directory (stops service if running)
#
#     ./build.sh --run
#         Build, deploy, and start the application in the background
#
#     ./build.sh --web
#         Deploy only pages directory and restart application (for rapid frontend iteration)
#
# NOTES
#     Default build operation does NOT increment version number, only updates build timestamp.
#     Version number must be manually incremented in .version file when needed.
#
#     For advanced operations removed in simplification (clean, database reset, etc.),
#     see docs/simplify-build-script/migration-guide.md
# -----------------------------------------------------------------------

set -e

# Detect WSL and use Windows Go if native Go not available
if ! command -v go &> /dev/null; then
    if [[ -f "/mnt/c/Program Files/Go/bin/go.exe" ]]; then
        GO_CMD="/mnt/c/Program Files/Go/bin/go.exe"
    else
        echo "Go not found. Please install Go or ensure Windows Go is accessible."
        exit 1
    fi
else
    GO_CMD="go"
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

# Parse arguments
RUN=false
DEPLOY=false
WEB=false

show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --deploy, -deploy    Build and deploy all files to bin directory"
    echo "  --run, -run          Build, deploy, and start the application"
    echo "  --web, -web          Deploy only pages and restart (no build)"
    echo "  --help, -h           Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0                   Build only (no deployment)"
    echo "  $0 --deploy          Build and deploy files"
    echo "  $0 --run             Build, deploy, and run"
    echo "  $0 --web             Quick pages deploy and restart"
}

while [[ $# -gt 0 ]]; do
    case $1 in
        -run|--run)
            RUN=true
            shift
            ;;
        -deploy|--deploy)
            DEPLOY=true
            shift
            ;;
        -web|--web)
            WEB=true
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

# Setup paths
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
VERSION_FILE="$PROJECT_ROOT/.version"
BIN_DIR="$PROJECT_ROOT/bin"
OUTPUT_PATH="$BIN_DIR/quaero"

# Detect OS for executable extension
# Check for Windows environments: MSYS, Cygwin, native Windows, or WSL using Windows Go
if [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "cygwin" ]] || [[ "$OSTYPE" == "win32" ]] || [[ "$GO_CMD" == *".exe"* ]]; then
    OUTPUT_PATH="$BIN_DIR/quaero.exe"
    MCP_OUTPUT_PATH="$BIN_DIR/quaero-mcp/quaero-mcp.exe"
else
    MCP_OUTPUT_PATH="$BIN_DIR/quaero-mcp/quaero-mcp"
fi

# Setup logging
LOG_DIR="$SCRIPT_DIR/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/build-$(date +%Y-%m-%d-%H-%M-%S).log"

# Function to log and display
log() {
    echo "$1" | tee -a "$LOG_FILE"
}

# Function to limit log files to most recent 10
limit_log_files() {
    local count=$(ls -1 "$LOG_DIR"/build-*.log 2>/dev/null | wc -l)
    if [ "$count" -gt 10 ]; then
        ls -1t "$LOG_DIR"/build-*.log | tail -n +11 | xargs rm -f
        echo -e "${GRAY}Removed old log files${NC}"
    fi
}

# Function to get server port from config
get_server_port() {
    local config_path="$BIN_DIR/quaero.toml"
    local port=8085
    if [ -f "$config_path" ]; then
        local found_port=$(grep -E '^port\s*=' "$config_path" | head -1 | sed 's/port\s*=\s*//' | tr -d ' ')
        if [ -n "$found_port" ]; then
            port=$found_port
        fi
    fi
    echo "$port"
}

# Function to stop Quaero service gracefully
stop_quaero_service() {
    local port=$1
    local pids
    local http_shutdown_succeeded=false
    local max_attempts=3
    local timeout
    local elapsed=0
    local check_interval=0.5

    pids=$(pgrep -f "quaero" 2>/dev/null || true)

    if [ -n "$pids" ]; then
        echo -e "${YELLOW}Stopping existing Quaero process(es)...${NC}"

        # Try HTTP shutdown first with retries
        echo -e "${GRAY}  Attempting HTTP graceful shutdown on port $port...${NC}"

        for attempt in $(seq 1 $max_attempts); do
            if curl -s -X POST "http://localhost:$port/api/shutdown" --connect-timeout 5 >/dev/null 2>&1; then
                echo -e "${GRAY}  HTTP shutdown request sent successfully${NC}"
                http_shutdown_succeeded=true
                break
            else
                if [ $attempt -lt $max_attempts ]; then
                    sleep 0.5
                else
                    echo -e "${GRAY}  HTTP shutdown not available (server may not be responding)${NC}"
                fi
            fi
        done

        # Wait for graceful shutdown
        if [ "$http_shutdown_succeeded" = true ]; then
            timeout=12
        else
            timeout=5
        fi

        while pgrep -f "quaero" >/dev/null 2>&1 && [ "$elapsed" -lt "$timeout" ]; do
            sleep $check_interval
            elapsed=$((elapsed + 1))

            if [ "$http_shutdown_succeeded" = true ] && [ "$elapsed" -eq 5 ]; then
                echo -e "${GRAY}  Still waiting for graceful shutdown...${NC}"
            fi
        done

        # Check if processes exited gracefully
        pids=$(pgrep -f "quaero" 2>/dev/null || true)

        if [ -n "$pids" ]; then
            if [ "$http_shutdown_succeeded" = true ]; then
                echo -e "${YELLOW}  Process(es) did not exit gracefully within ${timeout}s, forcing termination...${NC}"
            fi
            pkill -f "quaero" 2>/dev/null || true
            sleep 0.5

            if pgrep -f "quaero" >/dev/null 2>&1; then
                echo -e "${YELLOW}  Warning: Some processes may still be running${NC}"
            else
                echo -e "${YELLOW}  Process(es) force-stopped${NC}"
            fi
        else
            echo -e "${GREEN}  Process(es) stopped gracefully${NC}"
        fi
    else
        echo -e "${GRAY}No Quaero process found running${NC}"
    fi
}

# Function to deploy files
deploy_files() {
    local project_root=$1
    local bin_dir=$2

    # Deploy configuration file (only if not exists)
    local config_source="$project_root/deployments/local/quaero.toml"
    local config_dest="$bin_dir/quaero.toml"
    if [ -f "$config_source" ] && [ ! -f "$config_dest" ]; then
        cp "$config_source" "$config_dest"
    fi

    # Deploy README
    if [ -f "$project_root/README.md" ]; then
        cp "$project_root/README.md" "$bin_dir/README.md"
    fi

    # Deploy Chrome extension
    local ext_source="$project_root/cmd/quaero-chrome-extension"
    local ext_dest="$bin_dir/quaero-chrome-extension"
    if [ -d "$ext_source" ]; then
        rm -rf "$ext_dest"
        cp -r "$ext_source" "$ext_dest"
    fi

    # Deploy MCP server documentation
    local mcp_source="$project_root/cmd/quaero-mcp"
    local mcp_dest="$bin_dir/quaero-mcp"
    if [ -d "$mcp_source" ]; then
        mkdir -p "$mcp_dest"
        if [ -f "$mcp_source/README.md" ]; then
            cp "$mcp_source/README.md" "$mcp_dest/README.md"
        fi
        # Deploy MCP config (only if not exists)
        local mcp_config_source="$project_root/deployments/local/quaero-mcp.toml"
        local mcp_config_dest="$mcp_dest/quaero-mcp.toml"
        if [ -f "$mcp_config_source" ] && [ ! -f "$mcp_config_dest" ]; then
            cp "$mcp_config_source" "$mcp_config_dest"
        fi
    fi

    # Deploy pages directory
    local pages_source="$project_root/pages"
    local pages_dest="$bin_dir/pages"
    if [ -d "$pages_source" ]; then
        rm -rf "$pages_dest"
        cp -r "$pages_source" "$pages_dest"
    fi

    # Create job-definitions directory
    mkdir -p "$bin_dir/job-definitions"

    # Deploy auth directory (only new files)
    local auth_source="$project_root/deployments/local/auth"
    local auth_dest="$bin_dir/auth"
    if [ -d "$auth_source" ]; then
        mkdir -p "$auth_dest"
        for file in "$auth_source"/*; do
            if [ -f "$file" ]; then
                local filename=$(basename "$file")
                if [ ! -f "$auth_dest/$filename" ]; then
                    cp "$file" "$auth_dest/$filename"
                fi
            fi
        done
    fi

    # Create variables directory
    mkdir -p "$bin_dir/variables"
}

# Limit old log files
limit_log_files

# Handle --web parameter early (skip build, version update, and most deployment)
if [ "$WEB" = true ]; then
    echo -e "${CYAN}Quaero Web Deployment${NC}"
    echo -e "${CYAN}=====================${NC}"
    echo -e "${YELLOW}Deploying pages directory and restarting application...${NC}"

    CONFIG_PATH="$BIN_DIR/quaero.toml"

    # Verify executable exists
    if [ ! -f "$OUTPUT_PATH" ]; then
        echo -e "${RED}Quaero executable not found: $OUTPUT_PATH. Run ./build.sh first to create it.${NC}"
        exit 1
    fi

    # Get server port and stop service
    SERVER_PORT=$(get_server_port)

    # Stop Quaero service
    echo -e "${YELLOW}Stopping Quaero service...${NC}"
    stop_quaero_service "$SERVER_PORT"

    # Deploy pages directory only
    echo -e "${YELLOW}Deploying pages directory...${NC}"
    if [ -d "$PROJECT_ROOT/pages" ]; then
        rm -rf "$BIN_DIR/pages"
        cp -r "$PROJECT_ROOT/pages" "$BIN_DIR/pages"
        echo -e "${GREEN}  Pages deployed successfully${NC}"
    else
        echo -e "${RED}Pages directory not found: $PROJECT_ROOT/pages${NC}"
        exit 1
    fi

    # Restart application
    echo -e "${YELLOW}Starting application...${NC}"
    cd "$BIN_DIR"
    "$OUTPUT_PATH" -c "$CONFIG_PATH" &

    echo ""
    echo -e "${GREEN}==== Web Deployment Complete ====${NC}"
    echo -e "${CYAN}Pages deployed and application restarted${NC}"
    echo -e "${GRAY}No build or version update performed${NC}"
    exit 0
fi

# Get git commit
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo -e "${CYAN}Quaero Build Script${NC}"
echo -e "${CYAN}===================${NC}"
echo -e "${GRAY}Project Root: $PROJECT_ROOT${NC}"
echo -e "${GRAY}Git Commit: $GIT_COMMIT${NC}"

# Handle version file
BUILD_TIMESTAMP=$(date +%m-%d-%H-%M-%S)

if [ ! -f "$VERSION_FILE" ]; then
    cat > "$VERSION_FILE" << EOF
version: 0.1.0
build: $BUILD_TIMESTAMP
EOF
    echo -e "${GREEN}Created .version file with version 0.1.0${NC}"
else
    # Update only build timestamp
    sed -i.bak "s/^build:.*/build: $BUILD_TIMESTAMP/" "$VERSION_FILE"
    rm -f "$VERSION_FILE.bak"
fi

# Read version info
VERSION=$(grep "^version:" "$VERSION_FILE" | sed 's/version:\s*//' | tr -d ' ')
BUILD=$(grep "^build:" "$VERSION_FILE" | sed 's/build:\s*//' | tr -d ' ')

echo -e "${CYAN}Using version: $VERSION, build: $BUILD${NC}"

# Create bin directory
mkdir -p "$BIN_DIR"

# Stop services if running
SERVER_PORT=$(get_server_port)
stop_quaero_service "$SERVER_PORT"

# Tidy dependencies
echo -e "${YELLOW}Tidying dependencies...${NC}"
cd "$PROJECT_ROOT"
"$GO_CMD" mod tidy
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to tidy dependencies!${NC}"
    exit 1
fi

# Download dependencies
echo -e "${YELLOW}Downloading dependencies...${NC}"
"$GO_CMD" mod download
if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to download dependencies!${NC}"
    exit 1
fi

# Build flags
MODULE="github.com/ternarybob/quaero/internal/common"
LDFLAGS="-X $MODULE.Version=$VERSION -X $MODULE.Build=$BUILD -X $MODULE.GitCommit=$GIT_COMMIT"

# Build the Go application
echo -e "${YELLOW}Building quaero...${NC}"
echo -e "${GRAY}Build command: $GO_CMD build -ldflags=\"$LDFLAGS\" -o $OUTPUT_PATH ./cmd/quaero${NC}"

"$GO_CMD" build -ldflags="$LDFLAGS" -o "$OUTPUT_PATH" ./cmd/quaero

if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

# Verify executable
if [ ! -f "$OUTPUT_PATH" ]; then
    echo -e "${RED}Build completed but executable not found: $OUTPUT_PATH${NC}"
    exit 1
fi

echo -e "${GREEN}Main executable built: $OUTPUT_PATH${NC}"

# Build MCP server
echo -e "${YELLOW}Building quaero-mcp...${NC}"
mkdir -p "$BIN_DIR/quaero-mcp"

echo -e "${GRAY}Build command: $GO_CMD build -ldflags=\"$LDFLAGS\" -o $MCP_OUTPUT_PATH ./cmd/quaero-mcp${NC}"

"$GO_CMD" build -ldflags="$LDFLAGS" -o "$MCP_OUTPUT_PATH" ./cmd/quaero-mcp

if [ $? -ne 0 ]; then
    echo -e "${RED}MCP server build failed!${NC}"
    exit 1
fi

if [ ! -f "$MCP_OUTPUT_PATH" ]; then
    echo -e "${RED}MCP build completed but executable not found: $MCP_OUTPUT_PATH${NC}"
    exit 1
fi

echo -e "${GREEN}MCP server built: $MCP_OUTPUT_PATH${NC}"

# Handle deployment and execution based on parameters
if [ "$RUN" = true ] || [ "$DEPLOY" = true ]; then
    # Deploy files to bin directory
    deploy_files "$PROJECT_ROOT" "$BIN_DIR"

    if [ "$RUN" = true ]; then
        # Start application in background
        echo ""
        echo -e "${YELLOW}==== Starting Application ====${NC}"

        CONFIG_PATH="$BIN_DIR/quaero.toml"
        cd "$BIN_DIR"
        "$OUTPUT_PATH" -c "$CONFIG_PATH" &

        echo -e "${GREEN}Application started in background${NC}"
        echo -e "${CYAN}Command: quaero -c quaero.toml${NC}"
        echo -e "${GRAY}Config: bin/quaero.toml${NC}"
        echo -e "${YELLOW}Use 'pkill quaero' or send SIGTERM to stop gracefully${NC}"
        echo -e "${YELLOW}Check bin/logs/ for application logs${NC}"
    fi
fi
