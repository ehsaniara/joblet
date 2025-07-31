#!/bin/bash

set -e

# Debug test for volume operations to identify specific issues

source "$(dirname "$0")/common/test_helpers.sh"

test_volume_debug() {
    echo "Debugging volume operations..."
    
    # Test 1: Basic volume create command
    echo "Test 1: Creating a simple volume..."
    local output
    output=$("$RNX_BINARY" --config "$RNX_CONFIG" volume create debug-vol --size=10MB 2>&1)
    echo "Volume creation output:"
    echo "$output"
    echo "---"
    
    if [[ "$output" == *"Volume created successfully"* ]]; then
        echo "✓ Volume creation succeeded"
        
        # Test 2: List volumes
        echo "Test 2: Listing volumes..."
        output=$("$RNX_BINARY" --config "$RNX_CONFIG" volume list 2>&1)
        echo "Volume list output:"
        echo "$output"
        echo "---"
        
        # Test 3: Use volume in job
        echo "Test 3: Using volume in a job..."
        output=$("$RNX_BINARY" --config "$RNX_CONFIG" run --volume=debug-vol sh -c 'echo "Volume job test"' 2>&1)
        local job_id=$(echo "$output" | grep "^ID:" | awk '{print $2}')
        echo "Job creation output:"
        echo "$output"
        
        if [[ -n "$job_id" ]]; then
            sleep 2
            local logs=$("$RNX_BINARY" --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\\[" | grep -v "^$")
            echo "Job logs:"
            echo "$logs"
        fi
        
        # Cleanup
        echo "Cleaning up debug volume..."
        "$RNX_BINARY" --config "$RNX_CONFIG" volume remove debug-vol 2>/dev/null || echo "Cleanup failed"
    else
        echo "✗ Volume creation failed"
        return 1
    fi
}

# Run debug test
main() {
    echo "Starting volume debug test..."
    test_volume_debug
    echo "Volume debug test completed"
}

main "$@"