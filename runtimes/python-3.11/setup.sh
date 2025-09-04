#!/bin/bash
# Python 3.11 Runtime Setup - Platform Detection
# Entry point for Python 3.11 runtime installation

set -e

# Detect platform and architecture
detect_platform() {
    local os_id=""
    local arch=""
    
    # Detect architecture
    arch=$(uname -m)
    case "$arch" in
        x86_64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) echo "Unsupported architecture: $arch"; exit 1 ;;
    esac
    
    # Detect OS
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        case "$ID" in
            ubuntu) os_id="ubuntu" ;;
            rhel|centos|rocky) os_id="rhel" ;;
            amzn) os_id="amzn" ;;
            *) echo "Unsupported OS: $ID"; exit 1 ;;
        esac
    else
        echo "Cannot detect OS - /etc/os-release not found"
        exit 1
    fi
    
    echo "${os_id}-${arch}"
}

# Main execution
main() {
    echo "Starting Python 3.11 Runtime Setup..."
    
    local platform=$(detect_platform)
    local setup_script="setup-${platform}.sh"
    
    echo "Detected platform: $platform"
    echo "Using setup script: $setup_script"
    
    # Check if platform-specific script exists
    if [ ! -f "$setup_script" ]; then
        echo "Error: Setup script $setup_script not found"
        echo "Available scripts:"
        ls -1 setup-*.sh 2>/dev/null || echo "  No setup scripts found"
        exit 1
    fi
    
    # Make script executable and run it
    chmod +x "$setup_script"
    exec "./$setup_script" "$@"
}

main "$@"