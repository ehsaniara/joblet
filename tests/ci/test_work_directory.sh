#!/bin/bash

set -e

# Diagnostic test to understand work directory issues in CI
# This will help us debug why /work is not accessible

source "$(dirname "$0")/common/test_helpers.sh"

test_work_directory_diagnostics() {
    echo "Running work directory diagnostics..."
    
    # Test 1: Check current working directory
    local job_output
    echo "Test 1: Checking current working directory..."
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" run sh -c 'pwd' 2>&1)
    local job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    if [[ -n "$job_id" ]]; then
        sleep 2
        local logs=$("$RNX_BINARY" --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\\[" | grep -v "^$")
        echo "Current directory: $logs"
    fi
    
    # Test 2: List root directory contents
    echo "Test 2: Listing root directory contents..."
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" run sh -c 'ls -la /' 2>&1)
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    if [[ -n "$job_id" ]]; then
        sleep 2
        logs=$("$RNX_BINARY" --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\\[" | grep -v "^$" | head -20)
        echo "Root directory contents:"
        echo "$logs"
    fi
    
    # Test 3: Check if work directory exists
    echo "Test 3: Checking work directory existence..."
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" run sh -c 'test -d /work && echo "WORK EXISTS" || echo "WORK MISSING"' 2>&1)
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    if [[ -n "$job_id" ]]; then
        sleep 2
        logs=$("$RNX_BINARY" --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\\[" | grep -v "^$")
        echo "Work directory status: $logs"
    fi
    
    # Test 4: Try creating a file in current directory
    echo "Test 4: Testing write access in current directory..."
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" run sh -c 'echo "test" > testfile && echo "WRITE SUCCESS" || echo "WRITE FAILED"' 2>&1)
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    if [[ -n "$job_id" ]]; then
        sleep 2
        logs=$("$RNX_BINARY" --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\\[" | grep -v "^$")
        echo "Write test result: $logs"
    fi
    
    echo "Diagnostics completed"
}

# Run diagnostics
main() {
    echo "Starting work directory diagnostic tests..."
    test_work_directory_diagnostics
    echo "Diagnostic tests completed"
}

main "$@"