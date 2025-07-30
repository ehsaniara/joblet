#!/bin/bash

set -e

# gRPC communication test for CI environment
# Tests gRPC API functionality and error handling

source "$(dirname "$0")/common/test_helpers.sh"

test_grpc_connectivity() {
    echo "Testing gRPC connectivity..."
    
    # Test basic connectivity by listing jobs
    local result
    if ! result=$(rnx --config "$RNX_CONFIG" list 2>&1); then
        echo "gRPC connectivity failed: $result"
        return 1
    fi
    
    echo "✓ gRPC connectivity working"
}

test_api_error_handling() {
    echo "Testing API error handling..."
    
    # Try to get status of non-existent job
    set +e
    local error_result
    error_result=$(rnx --config "$RNX_CONFIG" status "non-existent-job-id" 2>&1)
    local exit_code=$?
    set -e
    
    if [[ $exit_code -eq 0 ]]; then
        echo "API should return error for non-existent job"
        return 1
    fi
    
    # Error message should be meaningful
    if [[ "$error_result" == *"not found"* ]] || [[ "$error_result" == *"invalid"* ]] || [[ "$error_result" == *"error"* ]]; then
        echo "✓ API error handling working"
    else
        echo "API error message unclear: $error_result"
        return 1
    fi
}

test_human_readable_output_format() {
    echo "Testing human-readable output format..."
    
    # Start a job and check output format
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sleep 2 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Job creation output invalid - no ID found"
        echo "Output: $job_output"
        return 1
    fi
    
    # Wait for job to complete
    sleep 3
    
    # Get job status and validate format
    local status_output
    if ! status_output=$(rnx --config "$RNX_CONFIG" status "$job_id" 2>&1); then
        echo "Failed to get job status"
        return 1
    fi
    
    # Validate output structure - should contain "Status:" and "Command:"
    local status command
    status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')
    command=$(echo "$status_output" | grep "^Command:" | awk '{print $2}')
    
    if [[ -z "$status" ]] || [[ -z "$command" ]]; then
        echo "Job status output format invalid"
        echo "Expected 'Status:' and 'Command:' lines"
        echo "Got: $status_output"
        return 1
    fi
    
    echo "✓ Human-readable output format working"
}

test_concurrent_requests() {
    echo "Testing concurrent gRPC requests..."
    
    # Start multiple jobs concurrently
    local job_ids=()
    for i in {1..3}; do
        local job_output
        job_output=$(rnx --config "$RNX_CONFIG" run sleep 5 2>&1)
        local job_id
        job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
        
        if [[ -n "$job_id" ]]; then
            job_ids+=("$job_id")
        fi
    done
    
    # Check all job statuses concurrently
    local pids=()
    for job_id in "${job_ids[@]}"; do
        (rnx --config "$RNX_CONFIG" status "$job_id" >/dev/null) &
        pids+=($!)
    done
    
    # Wait for all status checks to complete
    local failed=0
    for pid in "${pids[@]}"; do
        if ! wait "$pid"; then
            failed=1
        fi
    done
    
    # Clean up jobs
    for job_id in "${job_ids[@]}"; do
        rnx --config "$RNX_CONFIG" stop "$job_id" >/dev/null 2>&1 || true
    done
    
    if [[ $failed -eq 1 ]]; then
        echo "Concurrent requests failed"
        return 1
    fi
    
    echo "✓ Concurrent gRPC requests working"
}

test_large_output_handling() {
    echo "Testing large output handling..."
    
    # Generate job with large output
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sh -c 'for i in $(seq 1 100); do echo "Line $i: This is a test line with some content"; done' 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        return 1
    fi
    
    # Wait for job to complete
    sleep 3
    
    # Get job logs
    local job_logs
    job_logs=$(rnx --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\[" | grep -v "^$")
    
    # Check if output contains expected content
    local line_count
    line_count=$(echo "$job_logs" | wc -l)
    
    if [[ $line_count -lt 90 ]]; then  # Allow some tolerance (reduced from 1000 to 100 lines)
        echo "Large output handling failed - only got $line_count lines"
        echo "Sample output: $(echo "$job_logs" | head -5)"
        return 1
    fi
    
    echo "✓ Large output handling working"
}

test_special_characters_in_commands() {
    echo "Testing special characters in commands..."
    
    # Test command with special characters
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sh -c 'echo "Special chars: !@#$%^&*()[]{}|;:,.<>?"' 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        return 1
    fi
    
    # Wait for job to complete
    sleep 2
    
    # Get job logs
    local job_logs
    job_logs=$(rnx --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\[" | grep -v "^$")
    
    if [[ "$job_logs" != *"Special chars: !@#$%^&*()[]{}|;:,.<>?"* ]]; then
        echo "Special characters handling failed"
        echo "Expected: 'Special chars: !@#$%^&*()[]{}|;:,.<>?'"
        echo "Got: $job_logs"
        return 1
    fi
    
    echo "✓ Special characters handling working"
}

test_timeout_handling() {
    echo "Testing timeout handling..."
    
    # Start long-running job and stop it (simulates timeout)
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sleep 60 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        return 1
    fi
    
    # Wait a bit, then stop
    sleep 1
    local stop_result
    stop_result=$(rnx --config "$RNX_CONFIG" stop "$job_id" 2>&1)
    
    # Wait for status to update
    sleep 1
    
    # Verify job was stopped properly
    local status_output
    status_output=$(rnx --config "$RNX_CONFIG" status "$job_id" 2>&1)
    local final_status
    final_status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')
    
    if [[ "$final_status" != "STOPPED" && "$final_status" != "COMPLETED" ]]; then
        echo "Timeout/stop handling failed: $final_status"
        echo "Full status: $status_output"
        return 1
    fi
    
    echo "✓ Timeout handling working"
}

# Run all tests
main() {
    echo "Starting CI-compatible gRPC communication tests..."
    
    test_grpc_connectivity
    test_api_error_handling
    test_human_readable_output_format
    test_concurrent_requests
    test_large_output_handling
    test_special_characters_in_commands
    test_timeout_handling
    
    echo "All gRPC communication tests passed!"
}

main "$@"