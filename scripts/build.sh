#!/bin/bash
# -----------------------------------------------------------------------
# Build Script for Quaero
# -----------------------------------------------------------------------

set -euo pipefail

ENVIRONMENT="dev"
VERSION=""
CLEAN=false
TEST=false
VERBOSE=false
RELEASE=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -c|--clean)
            CLEAN=true
            shift
            ;;
        -t|--test)
            TEST=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        -r|--release)
            RELEASE=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS]"
            echo "Options:"
            echo "  -e, --environment ENV  Target environment (dev, staging, prod)"
            echo "  -v, --version VER      Version to embed in binary"
            echo "  -c, --clean           Clean build artifacts before building"
            echo "  -t, --test            Run tests before building"
            echo "  --verbose             Enable verbose output"
            echo "  -r, --release         Build optimized release binary"
            echo "  -h, --help            Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown parameter: $1"
            exit 1
            ;;
    esac
done

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
GRAY='\033[0;37m'
NC='\033[0m'

GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

echo -e "${CYAN}Quaero Build Script${NC}"
echo -e "${CYAN}===================${NC}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
VERSION_FILE="$PROJECT_ROOT/.version"
BIN_DIR="$PROJECT_ROOT/bin"
OUTPUT="$BIN_DIR/quaero"

if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
    OUTPUT="$OUTPUT.exe"
fi

echo -e "${GRAY}Project Root: $PROJECT_ROOT${NC}"
echo -e "${GRAY}Environment: $ENVIRONMENT${NC}"
echo -e "${GRAY}Git Commit: $GIT_COMMIT${NC}"

BUILD_TIMESTAMP=$(date +"%m-%d-%H-%M-%S")

if [[ ! -f "$VERSION_FILE" ]]; then
    cat > "$VERSION_FILE" <<EOF
version: 0.1.0
build: $BUILD_TIMESTAMP
EOF
    echo -e "${GREEN}Created .version file with version 0.1.0${NC}"
else
    CURRENT_VERSION=$(grep "^version:" "$VERSION_FILE" | awk '{print $2}')

    if [[ $CURRENT_VERSION =~ ^([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
        MAJOR="${BASH_REMATCH[1]}"
        MINOR="${BASH_REMATCH[2]}"
        PATCH="${BASH_REMATCH[3]}"

        PATCH=$((PATCH + 1))
        NEW_VERSION="$MAJOR.$MINOR.$PATCH"

        cat > "$VERSION_FILE" <<EOF
version: $NEW_VERSION
build: $BUILD_TIMESTAMP
EOF
        echo -e "${GREEN}Incremented version: $CURRENT_VERSION -> $NEW_VERSION${NC}"
    else
        sed -i "s/^build:.*/build: $BUILD_TIMESTAMP/" "$VERSION_FILE"
        echo -e "${YELLOW}Version format not recognized, keeping: $CURRENT_VERSION${NC}"
    fi
fi

VERSION_INFO_VERSION=$(grep "^version:" "$VERSION_FILE" | awk '{print $2}')
VERSION_INFO_BUILD=$(grep "^build:" "$VERSION_FILE" | awk '{print $2}')

echo -e "${CYAN}Using version: $VERSION_INFO_VERSION, build: $VERSION_INFO_BUILD${NC}"

if [[ "$CLEAN" == true ]]; then
    echo -e "${YELLOW}Cleaning build artifacts...${NC}"
    [[ -d "$BIN_DIR" ]] && rm -rf "$BIN_DIR"
    [[ -f "$PROJECT_ROOT/go.sum" ]] && rm -f "$PROJECT_ROOT/go.sum"
fi

mkdir -p "$BIN_DIR"

if [[ "$TEST" == true ]]; then
    echo -e "${YELLOW}Running tests...${NC}"
    cd "$PROJECT_ROOT"

    TEST_SCRIPT="$PROJECT_ROOT/test/run-tests.sh"
    if [[ -f "$TEST_SCRIPT" ]]; then
        bash "$TEST_SCRIPT" all
        if [[ $? -ne 0 ]]; then
            echo -e "${RED}Tests failed!${NC}"
            exit 1
        fi
    else
        go test ./... -v
        if [[ $? -ne 0 ]]; then
            echo -e "${RED}Tests failed!${NC}"
            exit 1
        fi
    fi
    echo -e "${GREEN}Tests passed!${NC}"
fi

echo -e "${YELLOW}Tidying dependencies...${NC}"
cd "$PROJECT_ROOT"
go mod tidy
if [[ $? -ne 0 ]]; then
    echo -e "${RED}Failed to tidy dependencies!${NC}"
    exit 1
fi

echo -e "${YELLOW}Downloading dependencies...${NC}"
go mod download
if [[ $? -ne 0 ]]; then
    echo -e "${RED}Failed to download dependencies!${NC}"
    exit 1
fi

MODULE="github.com/ternarybob/quaero/internal/common"
BUILD_FLAGS="-X ${MODULE}.Version=${VERSION_INFO_VERSION} -X ${MODULE}.Build=${VERSION_INFO_BUILD} -X ${MODULE}.GitCommit=${GIT_COMMIT}"

if [[ "$RELEASE" == true ]]; then
    BUILD_FLAGS="$BUILD_FLAGS -w -s"
fi

echo -e "${YELLOW}Building quaero...${NC}"

export CGO_ENABLED=0
if [[ "$RELEASE" == true ]]; then
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        export GOOS=linux
        export GOARCH=amd64
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        export GOOS=darwin
        export GOARCH=amd64
    elif [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
        export GOOS=windows
        export GOARCH=amd64
    fi
fi

BUILD_ARGS=("build" "-ldflags=$BUILD_FLAGS" "-o" "$OUTPUT" "./cmd/quaero")

if [[ "$VERBOSE" == true ]]; then
    BUILD_ARGS+=("-v")
fi

echo -e "${GRAY}Build command: go ${BUILD_ARGS[*]}${NC}"

cd "$PROJECT_ROOT"
go "${BUILD_ARGS[@]}"

if [[ $? -ne 0 ]]; then
    echo -e "${RED}Build failed!${NC}"
    exit 1
fi

CONFIG_SOURCE="$PROJECT_ROOT/deployments/quaero.toml"
CONFIG_DEST="$BIN_DIR/quaero.toml"

if [[ -f "$CONFIG_SOURCE" ]]; then
    if [[ ! -f "$CONFIG_DEST" ]]; then
        cp "$CONFIG_SOURCE" "$CONFIG_DEST"
        echo -e "${GREEN}Copied configuration: deployments/quaero.toml -> bin/${NC}"
    else
        echo -e "${CYAN}Using existing bin/quaero.toml (preserving customizations)${NC}"
    fi
fi

if [[ ! -f "$OUTPUT" ]]; then
    echo -e "${RED}Build completed but executable not found: $OUTPUT${NC}"
    exit 1
fi

FILE_SIZE=$(du -h "$OUTPUT" | awk '{print $1}')

echo ""
echo -e "${CYAN}==== Build Summary ====${NC}"
echo -e "${GREEN}Status: SUCCESS${NC}"
echo -e "${GREEN}Environment: $ENVIRONMENT${NC}"
echo -e "${GREEN}Version: $VERSION_INFO_VERSION${NC}"
echo -e "${GREEN}Build: $VERSION_INFO_BUILD${NC}"
echo -e "${GREEN}Output: $OUTPUT ($FILE_SIZE)${NC}"
echo -e "${GREEN}Build Time: $(date '+%Y-%m-%d %H:%M:%S')${NC}"

if [[ "$TEST" == true ]]; then
    echo -e "${GREEN}Tests: EXECUTED${NC}"
fi

if [[ "$CLEAN" == true ]]; then
    echo -e "${GREEN}Clean: EXECUTED${NC}"
fi

echo ""
echo -e "${GREEN}Build completed successfully!${NC}"
echo -e "${CYAN}Executable: $OUTPUT${NC}"
echo ""
echo -e "${YELLOW}To run with local config:${NC}"
echo -e "${WHITE}./bin/quaero serve -c deployments/local/quaero.toml${NC}"
