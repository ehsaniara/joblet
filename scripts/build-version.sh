#!/bin/bash
# Build script with version injection for unified or separate builds
# Usage: ./scripts/build-version.sh [component] [output-dir]
# Example: ./scripts/build-version.sh rnx

set -e

# Configuration
COMPONENT="${1:-all}"  # Component to build (rnx, joblet, api, or all)
OUTPUT_DIR="${2:-dist}"  # Output directory
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Version information gathering
get_version_info() {
    cd "$REPO_ROOT"

    # Get version from git tag, fallback to dev
    if GIT_TAG=$(git describe --tags --exact-match 2>/dev/null); then
        VERSION="$GIT_TAG"
    elif GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null); then
        VERSION="${GIT_TAG}+dev"
    else
        VERSION="dev"
        GIT_TAG="unknown"
    fi

    # Get git information
    if GIT_COMMIT=$(git rev-parse HEAD 2>/dev/null); then
        GIT_COMMIT_SHORT=$(git rev-parse --short HEAD 2>/dev/null)
    else
        GIT_COMMIT="unknown"
        GIT_COMMIT_SHORT="unknown"
    fi

    # Build date
    BUILD_DATE=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

    # Build ID for uniqueness
    BUILD_ID="${GIT_COMMIT_SHORT}-$(date -u +%Y%m%d%H%M%S)"

    # Get proto version directly from joblet-proto repository
    PROTO_REPO_PATH="$REPO_ROOT/../joblet-proto"
    if [ -d "$PROTO_REPO_PATH/.git" ]; then
        PROTO_COMMIT=$(cd "$PROTO_REPO_PATH" && git rev-parse HEAD 2>/dev/null || echo "unknown")
        PROTO_TAG=$(cd "$PROTO_REPO_PATH" && git describe --tags --exact-match 2>/dev/null || echo "unknown")
    else
        PROTO_COMMIT="unknown"
        PROTO_TAG="unknown"
    fi

    echo "Version Info:"
    echo "  Version: $VERSION"
    echo "  Git Tag: $GIT_TAG"
    echo "  Git Commit: $GIT_COMMIT"
    echo "  Build Date: $BUILD_DATE"
    echo "  Build ID: $BUILD_ID"
    echo "  Proto Tag: $PROTO_TAG"
    echo "  Proto Commit: $PROTO_COMMIT"
    echo
}

# Build ldflags for Go
build_ldflags() {
    local component="$1"
    echo "-s -w \
        -X joblet/pkg/version.Version=$VERSION \
        -X joblet/pkg/version.GitCommit=$GIT_COMMIT \
        -X joblet/pkg/version.GitTag=$GIT_TAG \
        -X joblet/pkg/version.BuildDate=$BUILD_DATE \
        -X joblet/pkg/version.Component=$component \
        -X joblet/pkg/version.ProtoCommit=$PROTO_COMMIT \
        -X joblet/pkg/version.ProtoTag=$PROTO_TAG"
}

# Build specific component
build_component() {
    local component="$1"
    local cmd_path="./cmd/$component"
    local output_name="$component"
    local ldflags

    if [ ! -d "$cmd_path" ]; then
        echo "Error: Component '$component' not found at $cmd_path"
        return 1
    fi

    echo "Building $component..."
    ldflags=$(build_ldflags "$component")

    # Create output directory
    mkdir -p "$OUTPUT_DIR"

    # Build with version injection
    go build -a -ldflags="$ldflags" -o "$OUTPUT_DIR/$output_name" "$cmd_path"

    echo "Built $component -> $OUTPUT_DIR/$output_name"
}

# Main build function
main() {
    cd "$REPO_ROOT"

    echo "Joblet Build Script with Version Injection"
    echo "=========================================="

    # Get version information
    get_version_info

    # Build based on component selection
    case "$COMPONENT" in
        "rnx")
            build_component "rnx"
            ;;
        "joblet")
            build_component "joblet"
            ;;
        "api")
            if [ -d "./cmd/api" ]; then
                build_component "api"
            else
                echo "Warning: API component not found, skipping"
            fi
            ;;
        "all")
            echo "Building all components..."
            build_component "rnx"
            build_component "joblet"
            if [ -d "./cmd/api" ]; then
                build_component "api"
            fi
            ;;
        *)
            echo "Error: Unknown component '$COMPONENT'"
            echo "Available components: rnx, joblet, api, all"
            exit 1
            ;;
    esac

    echo
    echo "Build completed successfully!"
    echo "Binaries available in: $OUTPUT_DIR/"
    ls -la "$OUTPUT_DIR/"
}

# Run main function
main "$@"