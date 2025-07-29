#!/bin/bash

set -e

echo "Simple connectivity test..."

# Direct test without helpers
export PATH="$PWD/bin:$PATH"
export RNX_CONFIG="${RNX_CONFIG:-/tmp/joblet/config/rnx-config.yml}"

echo "Config file: $RNX_CONFIG"
echo "Checking if config exists..."
ls -la "$RNX_CONFIG" || echo "Config file not found"

echo "Attempting to connect..."
output=$(rnx list 2>&1) || {
    exit_code=$?
    echo "rnx list without config failed with exit code: $exit_code"
    echo "Output: $output"
    
    # Try with explicit config
    echo "Trying with --config flag..."
    output=$(rnx --config "$RNX_CONFIG" list 2>&1) || {
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