#!/bin/bash

# E2E Test: Job Execution Timeout
# Tests per-job timeout via --timeout flag
# Verifies that jobs exceeding their timeout are terminated with TIMEOUT status and exit code 124

source "$(dirname "$0")/../lib/test_framework.sh"

test_suite_init "Job Execution Timeout Tests"

# ============================================
# Test Helpers
# ============================================

# Wait for a job to reach a terminal status, with a maximum wait time
wait_for_terminal_status() {
    local job_id="$1"
    local max_wait="${2:-20}"
    local elapsed=0

    while [[ $elapsed -lt $max_wait ]]; do
        local status
        status=$(check_job_status "$job_id")
        if [[ "$status" == "COMPLETED" || "$status" == "FAILED" || "$status" == "STOPPED" || "$status" == "TIMEOUT" ]]; then
            echo "$status"
            return 0
        fi
        sleep 1
        elapsed=$((elapsed + 1))
    done

    echo "UNKNOWN"
    return 1
}

# Get exit code from job status output
get_exit_code() {
    local job_id="$1"
    "$RNX_BINARY" job status "$job_id" 2>/dev/null | grep "Exit Code:" | sed 's/\x1b\[[0-9;]*m//g' | awk '{print $NF}'
}

# ============================================
# Tests
# ============================================

test_section "Per-Job Timeout Enforcement"

# Test 1: Job exceeding timeout is terminated with TIMEOUT status
test_timeout_terminates_job() {
    local job_output
    job_output=$("$RNX_BINARY" job run --timeout=3s sleep 60 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "    Failed to create job"
        return 1
    fi

    echo "    Job $job_id: timeout=3s, command=sleep 60"

    local status
    status=$(wait_for_terminal_status "$job_id" 15)

    assert_equals "$status" "TIMEOUT" "Job should have TIMEOUT status"
}
run_test "Job exceeding timeout gets TIMEOUT status" test_timeout_terminates_job

# Test 2: Timed-out job has exit code 124
test_timeout_exit_code() {
    local job_output
    job_output=$("$RNX_BINARY" job run --timeout=3s sleep 60 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "    Failed to create job"
        return 1
    fi

    wait_for_terminal_status "$job_id" 15 >/dev/null

    local exit_code
    exit_code=$(get_exit_code "$job_id")

    assert_equals "$exit_code" "124" "Exit code should be 124 (Unix timeout convention)"
}
run_test "Timed-out job has exit code 124" test_timeout_exit_code

test_section "Timeout Does Not Affect Normal Jobs"

# Test 3: Job completing before timeout succeeds normally
test_fast_job_with_timeout() {
    local job_output
    job_output=$("$RNX_BINARY" job run --timeout=30s echo "fast" 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "    Failed to create job"
        return 1
    fi

    local status
    status=$(wait_for_terminal_status "$job_id" 10)

    assert_equals "$status" "COMPLETED" "Fast job with generous timeout should COMPLETE"
}
run_test "Fast job with generous timeout completes normally" test_fast_job_with_timeout

# Test 4: Job without --timeout uses global config (should complete quickly)
test_no_timeout_flag() {
    local job_output
    job_output=$("$RNX_BINARY" job run echo "no timeout flag" 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "    Failed to create job"
        return 1
    fi

    local status
    status=$(wait_for_terminal_status "$job_id" 10)

    assert_equals "$status" "COMPLETED" "Job without timeout flag should complete normally"
}
run_test "Job without --timeout flag completes normally" test_no_timeout_flag

# Test 5: Job that fails before timeout gets FAILED (not TIMEOUT)
test_failing_job_with_timeout() {
    local job_output
    job_output=$("$RNX_BINARY" job run --timeout=30s sh -c "exit 1" 2>&1)
    local job_id
    job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')

    if [[ -z "$job_id" ]]; then
        echo "    Failed to create job"
        return 1
    fi

    local status
    status=$(wait_for_terminal_status "$job_id" 10)

    assert_equals "$status" "FAILED" "Failing job should get FAILED status, not TIMEOUT"
}
run_test "Failing job with timeout gets FAILED (not TIMEOUT)" test_failing_job_with_timeout

# ============================================
# Summary
# ============================================

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Timeout Test Results${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  Total:   $TOTAL_TESTS"
echo -e "  ${GREEN}Passed:  $PASSED_TESTS${NC}"
echo -e "  ${RED}Failed:  $FAILED_TESTS${NC}"
echo -e "  ${YELLOW}Skipped: $SKIPPED_TESTS${NC}"

if [[ $FAILED_TESTS -gt 0 ]]; then
    exit 1
fi
