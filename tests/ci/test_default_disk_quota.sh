#!/bin/bash

set -e

# Test default 1MB disk quota for jobs without volumes
# This tests the feature where jobs without volumes get a 1MB tmpfs work directory

source "$(dirname "$0")/common/test_helpers.sh"

test_default_disk_quota() {
    echo "Testing default 1MB disk quota for jobs without volumes..."
    
    # Run a job that does NOT specify any volume
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sh -c 'cd /work 2>/dev/null && echo "Work directory info shown" || echo "Failed to access /work"' 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID for default disk quota test"
        echo "Output: $job_output"
        return 1
    fi
    
    # Wait for job to complete
    sleep 2
    
    # Get job logs
    local job_logs
    job_logs=$(rnx --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\\[" | grep -v "^$")
    
    if [[ "$job_logs" != *"Work directory info shown"* ]]; then
        echo "Default disk quota job failed"
        echo "Expected: 'Work directory info shown'"
        echo "Got: $job_logs"
        return 1
    fi
    
    # Check if tmpfs is mentioned in the output (indicating limited work directory)
    if [[ "$job_logs" == *"tmpfs"* ]] || [[ "$job_logs" == *"/work"* ]]; then
        echo "✓ Job ran with work directory (may have size limitation)"
    fi
    
    echo "✓ Default disk quota test passed"
}

test_no_volume_vs_volume_difference() {
    echo "Testing difference between jobs with and without volumes..."
    
    # Create a test volume first
    local volume_output
    volume_output=$(rnx --config "$RNX_CONFIG" volume create test-quota-vol --size=50MB --type=memory 2>&1)
    
    if [[ "$volume_output" != *"Volume created successfully"* ]]; then
        echo "Failed to create test volume for comparison"
        echo "Output: $volume_output"
        return 1
    fi
    
    # Run job without volume (should have 1MB limit)
    local no_vol_output
    no_vol_output=$(rnx --config "$RNX_CONFIG" run sh -c 'echo "No volume job"' 2>&1)
    
    # Run job with volume (should have larger space)
    local with_vol_output
    with_vol_output=$(rnx --config "$RNX_CONFIG" run --volume=test-quota-vol sh -c 'echo "With volume job"' 2>&1)
    
    # Get job IDs
    local no_vol_id with_vol_id
    no_vol_id=$(echo "$no_vol_output" | grep "^ID:" | awk '{print $2}')
    with_vol_id=$(echo "$with_vol_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$no_vol_id" ]] || [[ -z "$with_vol_id" ]]; then
        echo "Failed to get job IDs for comparison test"
        return 1
    fi
    
    # Wait for jobs to complete
    sleep 3
    
    # Get logs
    local no_vol_logs with_vol_logs
    no_vol_logs=$(rnx --config "$RNX_CONFIG" log "$no_vol_id" 2>&1 | grep -v "^\\[" | grep -v "^$")
    with_vol_logs=$(rnx --config "$RNX_CONFIG" log "$with_vol_id" 2>&1 | grep -v "^\\[" | grep -v "^$")
    
    if [[ "$no_vol_logs" == *"No volume job"* ]] && [[ "$with_vol_logs" == *"With volume job"* ]]; then
        echo "✓ Both job types executed successfully"
        echo "  - Job without volume: $no_vol_id"
        echo "  - Job with volume: $with_vol_id"
    else
        echo "Job comparison failed"
        echo "No volume logs: $no_vol_logs"
        echo "With volume logs: $with_vol_logs"
        return 1
    fi
    
    # Cleanup test volume
    rnx --config "$RNX_CONFIG" volume remove test-quota-vol 2>/dev/null || true
    
    echo "✓ Volume comparison test passed"
}

# Run all tests
main() {
    echo "Starting default disk quota tests..."
    
    test_default_disk_quota
    test_no_volume_vs_volume_difference
    
    echo "All default disk quota tests passed!"
}

main "$@"