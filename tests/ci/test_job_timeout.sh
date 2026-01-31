#!/bin/bash

set -e

# Job timeout test for CI environment
# Tests per-job timeout via --timeout flag

source "$(dirname "$0")/common/test_helpers.sh"

test_job_timeout_terminates() {
    echo "Testing job timeout terminates long-running job..."

    # Run a long-running job with a short timeout
    local job_output
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job run --timeout=3s sleep 60 2>&1)

    # Extract job ID
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        echo "Output: $job_output"
        return 1
    fi

    echo "  Job ID: $job_id (timeout=3s, command=sleep 60)"

    # Wait for job to reach TIMEOUT status (give extra buffer beyond the 3s timeout)
    local elapsed=0
    local max_wait=15
    local status=""

    while [[ $elapsed -lt $max_wait ]]; do
        local status_output
        status_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job status "$job_id" 2>&1)
        status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')

        if [[ "$status" == "TIMEOUT" || "$status" == "FAILED" || "$status" == "COMPLETED" || "$status" == "STOPPED" ]]; then
            break
        fi

        sleep 1
        elapsed=$((elapsed + 1))
    done

    if [[ "$status" != "TIMEOUT" ]]; then
        echo "Expected status TIMEOUT, got: $status"
        return 1
    fi

    echo "✓ Job timeout terminates correctly (status=$status)"
}

test_job_timeout_exit_code() {
    echo "Testing timed-out job has exit code 124..."

    local job_output
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job run --timeout=3s sleep 60 2>&1)

    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        echo "Output: $job_output"
        return 1
    fi

    # Wait for timeout
    local elapsed=0
    local max_wait=15

    while [[ $elapsed -lt $max_wait ]]; do
        local status_output
        status_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job status "$job_id" 2>&1)
        local status
        status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')

        if [[ "$status" == "TIMEOUT" || "$status" == "FAILED" || "$status" == "COMPLETED" || "$status" == "STOPPED" ]]; then
            break
        fi

        sleep 1
        elapsed=$((elapsed + 1))
    done

    # Check exit code
    local status_output
    status_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job status "$job_id" 2>&1)
    local exit_code
    exit_code=$(echo "$status_output" | grep "Exit Code:" | awk '{print $NF}')

    if [[ "$exit_code" != "124" ]]; then
        echo "Expected exit code 124, got: $exit_code"
        echo "Full status output:"
        echo "$status_output"
        return 1
    fi

    echo "✓ Timed-out job has correct exit code 124"
}

test_job_no_timeout_completes_normally() {
    echo "Testing job without timeout completes normally..."

    # Run a quick job without explicit timeout (should complete fine)
    local job_output
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job run echo "no timeout" 2>&1)

    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        echo "Output: $job_output"
        return 1
    fi

    # Wait for completion
    sleep 2

    local status_output
    status_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job status "$job_id" 2>&1)
    local status
    status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')

    if [[ "$status" != "COMPLETED" ]]; then
        echo "Expected status COMPLETED, got: $status"
        return 1
    fi

    echo "✓ Job without timeout completes normally"
}

test_job_completes_before_timeout() {
    echo "Testing job that finishes before timeout..."

    # Run a quick job with a generous timeout
    local job_output
    job_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job run --timeout=30s echo "fast job" 2>&1)

    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        echo "Output: $job_output"
        return 1
    fi

    # Wait for completion
    sleep 2

    local status_output
    status_output=$("$RNX_BINARY" --config "$RNX_CONFIG" job status "$job_id" 2>&1)
    local status
    status=$(echo "$status_output" | grep "^Status:" | awk '{print $2}')

    if [[ "$status" != "COMPLETED" ]]; then
        echo "Job with generous timeout should COMPLETE, got: $status"
        return 1
    fi

    echo "✓ Job completes normally before timeout"
}

# Run all tests
main() {
    echo "Starting CI-compatible job timeout tests..."

    test_job_timeout_terminates
    test_job_timeout_exit_code
    test_job_no_timeout_completes_normally
    test_job_completes_before_timeout

    echo "All job timeout tests passed!"
}

main "$@"
