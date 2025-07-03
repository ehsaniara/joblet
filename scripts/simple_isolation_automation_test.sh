#!/bin/bash

# Final Working Isolation Test
# Fixed timeout issue - works on macOS and Linux

set -e

CLI_CMD="${CLI_CMD:-./bin/cli}"
OUTPUT_FORMAT="${OUTPUT_FORMAT:-human}"

TESTS_PASSED=0
TESTS_FAILED=0

# Test result tracking
test_result() {
    local test_name="$1"
    local success="$2"
    local message="$3"

    if [ "$success" = "true" ]; then
        ((TESTS_PASSED++))
        if [ "$OUTPUT_FORMAT" = "human" ]; then
            echo "✅ $test_name: $message"
        fi
    else
        ((TESTS_FAILED++))
        if [ "$OUTPUT_FORMAT" = "human" ]; then
            echo "❌ $test_name: $message"
        fi
    fi
}

# Wait for job to complete
wait_for_job_completion() {
    local job_id="$1"
    local timeout="${2:-30}"
    local count=0

    while [ $count -lt $timeout ]; do
        local status
        if status=$("$CLI_CMD" status "$job_id" 2>/dev/null | grep "Status:" | awk '{print $2}'); then
            case "$status" in
                COMPLETED|FAILED|STOPPED)
                    return 0
                    ;;
                RUNNING|INITIALIZING)
                    sleep 1
                    ((count++))
                    ;;
                *)
                    sleep 1
                    ((count++))
                    ;;
            esac
        else
            sleep 1
            ((count++))
        fi
    done

    return 1  # Timeout
}

# Get job logs (without timeout command)
get_job_output() {
    local job_id="$1"

    # For completed jobs, the log command should return immediately
    # We'll use a background process with kill timer instead of timeout
    local log_file="/tmp/job_${job_id}_log.tmp"

    # Run log command in background and capture output
    "$CLI_CMD" log "$job_id" > "$log_file" 2>&1 &
    local log_pid=$!

    # Wait up to 10 seconds for logs
    local count=0
    while [ $count -lt 10 ]; do
        if ! kill -0 $log_pid 2>/dev/null; then
            # Process finished
            wait $log_pid 2>/dev/null
            cat "$log_file"
            rm -f "$log_file"
            return 0
        fi
        sleep 1
        ((count++))
    done

    # Kill if still running
    kill $log_pid 2>/dev/null || true
    wait $log_pid 2>/dev/null || true

    if [ -f "$log_file" ]; then
        cat "$log_file"
        rm -f "$log_file"
    fi

    return 0
}

# Test 1: PID Namespace Isolation
test_pid_isolation() {
    local output job_id logs

    if [ "$OUTPUT_FORMAT" = "human" ]; then
        echo "  Running ps command..."
    fi

    # Run ps command
    if ! output=$("$CLI_CMD" run ps aux 2>&1); then
        test_result "PID_ISOLATION" "false" "Failed to start job"
        return 1
    fi

    job_id=$(echo "$output" | grep "ID:" | awk '{print $2}')
    if [ -z "$job_id" ]; then
        test_result "PID_ISOLATION" "false" "Could not extract job ID"
        return 1
    fi

    if [ "$OUTPUT_FORMAT" = "human" ]; then
        echo "  Job ID: $job_id, waiting for completion..."
    fi

    # Wait for job to complete
    if ! wait_for_job_completion "$job_id" 15; then
        test_result "PID_ISOLATION" "false" "Job did not complete in time"
        return 1
    fi

    # Get logs
    if ! logs=$(get_job_output "$job_id"); then
        test_result "PID_ISOLATION" "false" "Failed to get job logs"
        return 1
    fi

    # Check if job sees itself as PID 1
    if echo "$logs" | grep -q "root.*1.*ps aux"; then
        test_result "PID_ISOLATION" "true" "Job sees itself as PID 1"
        return 0
    else
        test_result "PID_ISOLATION" "false" "Job does not see itself as PID 1"
        return 1
    fi
}

# Test 2: Process Isolation
test_process_isolation() {
    local long_output long_job_id ps_output ps_job_id logs process_count

    if [ "$OUTPUT_FORMAT" = "human" ]; then
        echo "  Starting background job..."
    fi

    # Start long-running job
    if ! long_output=$("$CLI_CMD" run sleep 20 2>&1); then
        test_result "PROCESS_ISOLATION" "false" "Failed to start background job"
        return 1
    fi

    long_job_id=$(echo "$long_output" | grep "ID:" | awk '{print $2}')
    if [ -z "$long_job_id" ]; then
        test_result "PROCESS_ISOLATION" "false" "Could not extract background job ID"
        return 1
    fi

    # Give background job time to start
    sleep 3

    if [ "$OUTPUT_FORMAT" = "human" ]; then
        echo "  Starting ps job while background job runs..."
    fi

    # Start ps job while background job runs
    if ! ps_output=$("$CLI_CMD" run ps aux 2>&1); then
        "$CLI_CMD" stop "$long_job_id" >/dev/null 2>&1 || true
        test_result "PROCESS_ISOLATION" "false" "Failed to start ps job"
        return 1
    fi

    ps_job_id=$(echo "$ps_output" | grep "ID:" | awk '{print $2}')
    if [ -z "$ps_job_id" ]; then
        "$CLI_CMD" stop "$long_job_id" >/dev/null 2>&1 || true
        test_result "PROCESS_ISOLATION" "false" "Could not extract ps job ID"
        return 1
    fi

    # Wait for ps job to complete
    if ! wait_for_job_completion "$ps_job_id" 15; then
        "$CLI_CMD" stop "$long_job_id" >/dev/null 2>&1 || true
        test_result "PROCESS_ISOLATION" "false" "PS job did not complete in time"
        return 1
    fi

    # Get ps job logs
    if ! logs=$(get_job_output "$ps_job_id"); then
        "$CLI_CMD" stop "$long_job_id" >/dev/null 2>&1 || true
        test_result "PROCESS_ISOLATION" "false" "Failed to get ps job logs"
        return 1
    fi

    # Count processes (should be ≤ 3: ps command, maybe shell, maybe init)
    process_count=$(echo "$logs" | grep -c "root.*[0-9]" 2>/dev/null || echo "0")

    # Clean up background job
    "$CLI_CMD" stop "$long_job_id" >/dev/null 2>&1 || true

    if [ "$process_count" -le 3 ]; then
        test_result "PROCESS_ISOLATION" "true" "Only $process_count processes visible (good isolation)"
        return 0
    else
        test_result "PROCESS_ISOLATION" "false" "Too many processes visible: $process_count"
        return 1
    fi
}

# Test 3: Container Setup
test_container_setup() {
    local output job_id logs

    if [ "$OUTPUT_FORMAT" = "human" ]; then
        echo "  Running container test..."
    fi

    if ! output=$("$CLI_CMD" run echo "container-test" 2>&1); then
        test_result "CONTAINER_SETUP" "false" "Failed to start test job"
        return 1
    fi

    job_id=$(echo "$output" | grep "ID:" | awk '{print $2}')
    if [ -z "$job_id" ]; then
        test_result "CONTAINER_SETUP" "false" "Could not extract job ID"
        return 1
    fi

    # Wait for job to complete
    if ! wait_for_job_completion "$job_id" 15; then
        test_result "CONTAINER_SETUP" "false" "Job did not complete in time"
        return 1
    fi

    # Get logs
    if ! logs=$(get_job_output "$job_id"); then
        test_result "CONTAINER_SETUP" "false" "Failed to get job logs"
        return 1
    fi

    # Check for job-init or namespace isolation indicators
    if echo "$logs" | grep -E "(job-init|remounting /proc|namespace isolation)" >/dev/null; then
        test_result "CONTAINER_SETUP" "true" "Container isolation detected"
        return 0
    else
        test_result "CONTAINER_SETUP" "false" "No container isolation detected"
        return 1
    fi
}

# Output results
output_results() {
    local total=$((TESTS_PASSED + TESTS_FAILED))

    case "$OUTPUT_FORMAT" in
        json)
            echo "{"
            echo "  \"total\": $total,"
            echo "  \"passed\": $TESTS_PASSED,"
            echo "  \"failed\": $TESTS_FAILED,"
            echo "  \"status\": \"$([ $TESTS_FAILED -eq 0 ] && echo "PASS" || echo "FAIL")\""
            echo "}"
            ;;
        human)
            echo
            echo "===================="
            echo "  ISOLATION RESULTS"
            echo "===================="
            echo "Total: $total"
            echo "Passed: $TESTS_PASSED"
            echo "Failed: $TESTS_FAILED"
            echo
            if [ $TESTS_FAILED -eq 0 ]; then
                echo "🎉 All isolation tests PASSED!"
                echo
                echo "Your job isolation is working perfectly:"
                echo "✅ PID namespace isolation (jobs see themselves as PID 1)"
                echo "✅ Process isolation (jobs can't see each other)"
                echo "✅ Container setup (proper namespace isolation)"
            else
                echo "❌ Some isolation tests FAILED!"
            fi
            ;;
    esac
}

# Cleanup
cleanup() {
    # Stop any running jobs
    local running_jobs
    if running_jobs=$("$CLI_CMD" list 2>/dev/null | grep "RUNNING" | awk '{print $1}'); then
        for job in $running_jobs; do
            "$CLI_CMD" stop "$job" >/dev/null 2>&1 || true
        done
    fi

    # Clean up temp files
    rm -f /tmp/job_*_log.tmp
}

# Main execution
main() {
    trap cleanup EXIT

    # Check prerequisites
    if [ ! -f "$CLI_CMD" ]; then
        echo "ERROR: CLI not found at $CLI_CMD" >&2
        exit 2
    fi

    if ! "$CLI_CMD" list >/dev/null 2>&1; then
        echo "ERROR: Cannot connect to server" >&2
        exit 2
    fi

    if [ "$OUTPUT_FORMAT" = "human" ]; then
        echo "Testing job isolation..."
        echo "CLI: $CLI_CMD (using default server)"
        echo
    fi

    # Run tests (don't exit on failure)
    set +e
    test_pid_isolation
    test_process_isolation
    test_container_setup
    set -e

    # Output results
    output_results

    # Exit with proper code
    [ $TESTS_FAILED -eq 0 ] && exit 0 || exit 1
}

main "$@"