#!/bin/bash
# Python 3.11 PyTorch CUDA Runtime Setup - Multi-platform main script
# Detects platform and delegates to appropriate platform-specific setup script

set -e

echo "Starting Python 3.11 PyTorch CUDA runtime setup (multi-platform)..."
echo "Build ID: ${BUILD_ID:-unknown}"
echo "Runtime Spec: ${RUNTIME_SPEC:-unknown}"
echo "Chroot: ${JOBLET_CHROOT:-false}"

# Simple platform detection
detect_architecture() {
    case "$(uname -m)" in
        x86_64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) echo "amd64" ;;
    esac
}

detect_distribution() {
    if [ -f /etc/os-release ]; then
        source /etc/os-release
        case "$ID" in
            ubuntu|debian) echo "ubuntu" ;;
            rhel|centos|rocky|alma) echo "rhel" ;;
            amzn) echo "amzn" ;;
            *) echo "ubuntu" ;;
        esac
    else
        echo "ubuntu"
    fi
}

# Get script directory
SCRIPT_DIR="$(dirname "$0")"

# Detect current platform
ARCHITECTURE=$(detect_architecture)
DISTRIBUTION=$(detect_distribution)

echo "Detected platform:"
echo "  Architecture: $ARCHITECTURE"
echo "  Distribution: $DISTRIBUTION"

# Get platform-specific script name
PLATFORM_SCRIPT="setup-${DISTRIBUTION}-${ARCHITECTURE}.sh"
PLATFORM_SCRIPT_PATH="$SCRIPT_DIR/$PLATFORM_SCRIPT"

if [ -f "$PLATFORM_SCRIPT_PATH" ]; then
    echo "Delegating to platform-specific script: $PLATFORM_SCRIPT"
    chmod +x "$PLATFORM_SCRIPT_PATH"
    exec "$PLATFORM_SCRIPT_PATH"
else
    echo "ERROR: Platform-specific script $PLATFORM_SCRIPT not found"
    echo "Available platform combinations:"
    echo "  - ubuntu-amd64, ubuntu-arm64"
    echo "  - rhel-amd64, rhel-arm64"
    echo "  - amzn-amd64, amzn-arm64"
    echo "Detected: $DISTRIBUTION-$ARCHITECTURE"
    exit 1
fi
