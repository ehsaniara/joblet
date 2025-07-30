#!/bin/bash

set -e

# Job lifecycle management test for CI environment
# Tests job creation, status checking, stopping, and listing

source "$(dirname "$0")/common/test_helpers.sh"

test_async_job_creation() {
    echo "Testing async job creation..."
    
    # Start long-running job (all jobs are async by default)
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sleep 30 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to create async job"
        echo "Output: $job_output"
        return 1
    fi
    
    echo "✓ Async job created with ID: $job_id"
    
    # Clean up
    rnx --config "$RNX_CONFIG" stop "$job_id" >/dev/null 2>&1 || true
}

test_job_status_checking() {
    echo "Testing job status checking..."
    
    # Start job and extract ID
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sleep 5 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        return 1
    fi
    
    # Check status immediately (should be RUNNING)
    local status_output
    status_output=$(rnx --config "$RNX_CONFIG" status "$job_id" 2>&1)
    local status
    status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')
    
    if [[ "$status" != "RUNNING" && "$status" != "COMPLETED" ]]; then
        echo "Unexpected job status: $status"
        rnx --config "$RNX_CONFIG" stop "$job_id" >/dev/null 2>&1 || true
        return 1
    fi
    
    echo "✓ Job status check passed: $status"
    
    # Clean up
    rnx --config "$RNX_CONFIG" stop "$job_id" >/dev/null 2>&1 || true
}

test_job_stopping() {
    echo "Testing job stopping..."
    
    # Start long-running job
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sleep 60 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        return 1
    fi
    
    # Stop the job
    local stop_result
    stop_result=$(rnx --config "$RNX_CONFIG" stop "$job_id" 2>&1)
    
    # Wait a moment for status to update
    sleep 1
    
    # Verify job was stopped
    local status_output
    status_output=$(rnx --config "$RNX_CONFIG" status "$job_id" 2>&1)
    local final_status
    final_status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')
    
    if [[ "$final_status" != "STOPPED" && "$final_status" != "COMPLETED" ]]; then
        echo "Job was not properly stopped. Status: $final_status"
        return 1
    fi
    
    echo "✓ Job stopping worked correctly"
}

test_job_listing() {
    echo "Testing job listing..."
    
    # Start a few jobs
    local job_output1 job_output2
    job_output1=$(rnx --config "$RNX_CONFIG" run sleep 10 2>&1)
    job_output2=$(rnx --config "$RNX_CONFIG" run sleep 10 2>&1)
    
    local job1_id job2_id
    job1_id=$(echo "$job_output1" | grep "^ID:" | awk '{print $2}')
    job2_id=$(echo "$job_output2" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job1_id" || -z "$job2_id" ]]; then
        echo "Failed to get job IDs"
        return 1
    fi
    
    # List jobs
    local job_list
    job_list=$(rnx --config "$RNX_CONFIG" list 2>&1)
    
    # Check that our jobs are in the list
    if [[ "$job_list" != *"$job1_id"* ]] || [[ "$job_list" != *"$job2_id"* ]]; then
        echo "Jobs not found in job list"
        echo "Looking for: $job1_id and $job2_id"
        echo "Job list:"
        echo "$job_list"
        rnx --config "$RNX_CONFIG" stop "$job1_id" >/dev/null 2>&1 || true
        rnx --config "$RNX_CONFIG" stop "$job2_id" >/dev/null 2>&1 || true
        return 1
    fi
    
    echo "✓ Job listing working correctly"
    
    # Clean up
    rnx --config "$RNX_CONFIG" stop "$job1_id" >/dev/null 2>&1 || true
    rnx --config "$RNX_CONFIG" stop "$job2_id" >/dev/null 2>&1 || true
}

test_completed_job_status() {
    echo "Testing completed job status..."
    
    # Run short job that will complete with specific exit code
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sh -c 'echo "done" && exit 42' 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        return 1
    fi
    
    # Wait for completion
    sleep 2
    
    # Check final status
    local status_output
    status_output=$(rnx --config "$RNX_CONFIG" status "$job_id" 2>&1)
    local status
    status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')
    
    if [[ "$status" != "FAILED" && "$status" != "COMPLETED" ]]; then
        echo "Job should be FAILED or COMPLETED, got: $status"
        echo "Full status:"
        echo "$status_output"
        return 1
    fi
    
    # Note: The current implementation doesn't expose exit codes in the status output
    # So we just verify the job completed/failed
    
    echo "✓ Completed job status tracking working correctly"
}

# Run all tests
main() {
    echo "Starting CI-compatible job lifecycle tests..."
    
    test_async_job_creation
    test_job_status_checking
    test_job_stopping
    test_job_listing
    test_completed_job_status
    
    echo "All job lifecycle tests passed!"
}

main "$@"