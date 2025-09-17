#!/bin/bash

set -e

echo "Simple connectivity test..."

# Direct test without helpers
export PATH="$PWD/bin:$PATH"
export RNX_CONFIG="${RNX_CONFIG:-/tmp/joblet/config/rnx-config.yml}"

# Set RNX_BINARY if not already set
if [[ -z "$RNX_BINARY" ]]; then
    if [[ -x "$PWD/bin/rnx" ]]; then
        export RNX_BINARY="$PWD/bin/rnx"
    elif command -v rnx >/dev/null 2>&1; then
        export RNX_BINARY="rnx"
    else
        echo "Error: rnx binary not found"
        exit 1
    fi
fi

echo "Config file: $RNX_CONFIG"
echo "RNX binary: $RNX_BINARY"
echo "Checking if config exists..."
ls -la "$RNX_CONFIG" || echo "Config file not found"

echo "Attempting to connect..."
output=$("$RNX_BINARY" job list 2>&1) || {
    exit_code=$?
    echo ""$RNX_BINARY" job list without config failed with exit code: $exit_code"
    echo "Output: $output"
    
    # Try with explicit config
    echo "Trying with --config flag..."
    output=$("$RNX_BINARY" --config "$RNX_CONFIG" job list 2>&1) || {
        echo "Also failed with --config"
        echo "Output: $output"
        exit 1
    }
    
    # If we get here, it worked with --config
    echo "Success with --config flag!"
    echo "Output: $output"
}

echo "Connection successful!"
echo "Output: $output"