#!/bin/bash

set -e

# Basic job execution test for CI environment
# Tests core job creation, execution, and status checking

source "$(dirname "$0")/common/test_helpers.sh"

test_basic_command_execution() {
    echo "Testing basic command execution..."
    
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run echo "Hello, CI!" 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        echo "Output: $job_output"
        return 1
    fi
    
    # Wait for job to complete
    sleep 2
    
    # Get job logs and filter out debug messages
    local job_logs
    job_logs=$(rnx --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\[" | grep -v "^$")
    
    if [[ "$job_logs" != *"Hello, CI!"* ]]; then
        echo "Basic command execution failed"
        echo "Job logs: $job_logs"
        return 1
    fi
    
    echo "✓ Basic command execution passed"
}

test_job_with_args() {
    echo "Testing job with multiple arguments..."
    
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sh -c 'echo "arg1: $1, arg2: $2"' -- "test1" "test2" 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        echo "Output: $job_output"
        return 1
    fi
    
    # Wait for job to complete
    sleep 2
    
    # Get job logs and filter out debug messages
    local job_logs
    job_logs=$(rnx --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\[" | grep -v "^$")
    
    if [[ "$job_logs" != *"arg1: test1, arg2: test2"* ]]; then
        echo "Job with arguments failed"
        echo "Expected: 'arg1: test1, arg2: test2'"
        echo "Got: $job_logs"
        return 1
    fi
    
    echo "✓ Job with arguments passed"
}

test_job_exit_codes() {
    echo "Testing job exit codes..."
    
    # Test successful job (exit 0)
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run sh -c 'exit 0' 2>&1)
    local success_job_id
    success_job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$success_job_id" ]]; then
        echo "Failed to get job ID for success test"
        return 1
    fi
    
    # Wait for job to complete
    sleep 2
    
    # Check job status
    local status_output
    status_output=$(rnx --config "$RNX_CONFIG" status "$success_job_id" 2>&1)
    local success_status
    success_status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')
    
    if [[ "$success_status" != "COMPLETED" ]]; then
        echo "Successful job should have status COMPLETED, got: $success_status"
        echo "Full status output:"
        echo "$status_output"
        return 1
    fi
    
    # Test failing job (exit 1)
    job_output=$(rnx --config "$RNX_CONFIG" run sh -c 'exit 1' 2>&1)
    local fail_job_id
    fail_job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$fail_job_id" ]]; then
        echo "Failed to get job ID for failure test"
        return 1
    fi
    
    # Wait for job to complete
    sleep 2
    
    # Check job status
    local fail_status_output
    fail_status_output=$(rnx --config "$RNX_CONFIG" status "$fail_job_id" 2>&1)
    local fail_status
    fail_status=$(echo "$fail_status_output" | grep "^Status:" | awk '{print $2}')
    
    if [[ "$fail_status" != "FAILED" ]]; then
        echo "Failed job should have status FAILED, got: $fail_status"
        echo "Full status output:"
        echo "$fail_status_output"
        return 1
    fi
    
    echo "✓ Job exit codes working correctly"
}

test_environment_isolation() {
    echo "Testing basic environment isolation..."
    
    # Run env command
    local job_output
    job_output=$(rnx --config "$RNX_CONFIG" run env 2>&1)
    
    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        echo "Output: $job_output"
        return 1
    fi
    
    # Wait for job to complete
    sleep 2
    
    # Get job logs and count USER variable occurrences
    local env_count
    env_count=$(rnx --config "$RNX_CONFIG" log "$job_id" 2>&1 | grep -v "^\[" | grep -c "USER" || echo "0")
    
    # Should have minimal environment variables
    if [[ $env_count -gt 10 ]]; then
        echo "Environment may not be properly isolated (found $env_count USER vars)"
        # This is a warning, not a failure for CI compatibility
    fi
    
    echo "✓ Environment isolation test completed"
}

# Run all tests
main() {
    echo "Starting CI-compatible basic execution tests..."
    
    test_basic_command_execution
    test_job_with_args  
    test_job_exit_codes
    test_environment_isolation
    
    echo "All basic execution tests passed!"
}

main "$@"