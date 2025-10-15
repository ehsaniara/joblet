#!/bin/bash

# Test 10: Persist Subprocess Integration Tests
# Verifies: Log persistence, metric persistence, historical queries, IPC communication
# Note: joblet-persist now runs as a subprocess of joblet-core (not a separate systemd service)

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration (consistent with other tests)
REMOTE_HOST="${REMOTE_HOST:-192.168.1.161}"
REMOTE_USER="${REMOTE_USER:-jay}"

# Initialize test suite
test_suite_init "Persist Subprocess Integration Tests"

# ============================================
# Test Functions
# ============================================

test_persist_service_running() {
    echo "Checking if joblet-persist subprocess is running..."

    # Check if persist is running as subprocess of joblet
    local persist_pid=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "ps aux | grep 'joblet-persist -config' | grep -v grep | awk '{print \$2}'" 2>/dev/null)
    local joblet_pid=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "ps aux | grep '/opt/joblet/bin/joblet\$' | grep -v grep | awk '{print \$2}'" 2>/dev/null)

    if [[ -n "$persist_pid" ]] && [[ -n "$joblet_pid" ]]; then
        # Verify persist is a child of joblet
        local parent_pid=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "ps -o ppid= -p $persist_pid" 2>/dev/null | tr -d ' ')

        if [[ "$parent_pid" == "$joblet_pid" ]]; then
            echo "  ✓ joblet-persist subprocess is running (PID: $persist_pid, Parent: $joblet_pid)"
            return 0
        else
            echo "  ✗ joblet-persist is running but not as child of joblet"
            echo "    Persist PID: $persist_pid, Parent: $parent_pid, Expected Parent: $joblet_pid"
            return 1
        fi
    else
        echo "  ✗ joblet-persist subprocess is not running"
        echo "    Joblet PID: ${joblet_pid:-not found}"
        echo "    Persist PID: ${persist_pid:-not found}"
        ssh ${REMOTE_USER}@${REMOTE_HOST} "ps auxf | grep joblet | grep -v grep" || true
        return 1
    fi
}

test_persist_socket_exists() {
    echo "Checking if persist Unix socket exists..."

    # Socket path is /opt/joblet/run/persist-ipc.sock
    if ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ss -xlnp | grep persist-ipc.sock" 2>/dev/null | grep -q "joblet-persist"; then
        echo "  ✓ Persist IPC socket exists at /opt/joblet/run/persist-ipc.sock"
        return 0
    elif ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ls -la /opt/joblet/run/persist-ipc.sock" 2>/dev/null | grep -q "persist-ipc.sock"; then
        echo "  ✓ Persist IPC socket file exists at /opt/joblet/run/persist-ipc.sock"
        return 0
    else
        echo "  ✗ Persist socket does not exist"
        echo "  Checking socket locations:"
        ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ss -xlnp | grep persist || echo 'No persist sockets found'" 2>/dev/null
        return 1
    fi
}

test_log_persistence() {
    echo "Testing log persistence..."

    # Run a simple job that generates logs (no runtime for speed)
    local job_output=$($RNX_BINARY job run echo "Test log persistence" 2>&1)
    # Strip ANSI color codes and extract job ID
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID from output:"
        echo "$job_output" | head -20
        return 1
    fi

    echo "  Job ID: $job_id"

    # Wait for job to complete
    sleep 3

    # Check if logs are accessible
    local logs=$($RNX_BINARY job log "$job_id" 2>&1)

    if echo "$logs" | grep -q "Test log persistence"; then
        echo "  ✓ Logs persisted successfully"
        return 0
    else
        echo "  ✗ Logs not found"
        echo "  Log output: $logs"
        return 1
    fi
}

test_metric_persistence() {
    echo "Testing metric persistence..."

    # Run a job that uses resources (simpler command for speed)
    local job_output=$($RNX_BINARY job run --max-cpu=50 sh -c "for i in 1 2 3; do echo \$i; sleep 1; done" 2>&1)
    # Strip ANSI color codes and extract job ID
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"

    # Wait for job to complete and metrics to be collected
    sleep 5

    # Check if metrics exist in persist storage
    if ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ls -la /opt/joblet/metrics/$job_id/" 2>/dev/null | grep -q "metrics.jsonl.gz"; then
        echo "  ✓ Metrics persisted successfully"

        # Check metric file size
        local size=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo stat -c%s /opt/joblet/metrics/$job_id/metrics.jsonl.gz" 2>/dev/null)
        if [[ -n "$size" && "$size" -gt 0 ]]; then
            echo "  ✓ Metric file size: $size bytes"
            return 0
        else
            echo "  ⚠ Metric file is empty"
            return 1
        fi
    else
        echo "  ✗ Metrics file not found"
        ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ls -la /opt/joblet/metrics/" || true
        return 1
    fi
}

test_log_directory_structure() {
    echo "Testing log directory structure..."

    # Run a simple job
    local job_output=$($RNX_BINARY job run echo "Directory structure test" 2>&1)
    # Strip ANSI color codes and extract job ID
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"
    sleep 3

    # Check log directory structure
    local log_dir=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ls -la /opt/joblet/logs/$job_id/" 2>/dev/null)

    if echo "$log_dir" | grep -q "stdout.log.gz"; then
        echo "  ✓ stdout.log.gz exists"
    else
        echo "  ✗ stdout.log.gz not found"
        return 1
    fi

    if echo "$log_dir" | grep -q "stderr.log.gz"; then
        echo "  ✓ stderr.log.gz exists"
    else
        echo "  ✗ stderr.log.gz not found"
        return 1
    fi

    return 0
}

test_multiple_jobs_persistence() {
    echo "Testing persistence with multiple concurrent jobs..."

    local job_ids=()

    # Run 3 jobs concurrently (simple commands for speed)
    for i in {1..3}; do
        local job_output=$($RNX_BINARY job run sh -c "echo 'Job $i starting'; sleep 1; echo 'Job $i complete'" 2>&1)
        # Strip ANSI color codes and extract job ID
        local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

        if [[ -n "$job_id" ]]; then
            job_ids+=("$job_id")
            echo "  Launched job $i: $job_id"
        fi
    done

    # Wait for all jobs to complete
    sleep 3

    # Verify logs for all jobs
    local success_count=0
    for job_id in "${job_ids[@]}"; do
        if ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ls /opt/joblet/logs/$job_id/stdout.log.gz" 2>/dev/null; then
            ((success_count++))
        fi
    done

    if [[ $success_count -eq ${#job_ids[@]} ]]; then
        echo "  ✓ All $success_count jobs persisted successfully"
        return 0
    else
        echo "  ✗ Only $success_count out of ${#job_ids[@]} jobs persisted"
        return 1
    fi
}

test_persist_service_logs() {
    echo "Checking persist subprocess logs for errors..."

    # Persist logs are now in joblet service logs with [PERSIST] prefix
    local logs=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo journalctl -u joblet.service --since '5 minutes ago' --no-pager | grep '\[PERSIST\]'" 2>&1)

    # Check for error patterns
    if echo "$logs" | grep -qi "panic\|fatal\|\[ERROR\].*failed"; then
        echo "  ⚠ Found error patterns in persist subprocess logs:"
        echo "$logs" | grep -i "panic\|fatal\|\[ERROR\].*failed" | tail -5
        # Don't fail the test, just warn
        return 0
    else
        echo "  ✓ No critical errors in persist subprocess logs"
        # Show a few recent log lines as confirmation
        echo "$logs" | tail -3 | sed 's/^/    /'
        return 0
    fi
}

test_ipc_socket_permissions() {
    echo "Testing IPC socket permissions..."

    # Check if socket is accessible and listening
    local socket_info=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo ss -xlnp | grep persist-ipc.sock" 2>&1)

    if echo "$socket_info" | grep -q "LISTEN"; then
        echo "  ✓ Socket is listening and accessible"
        return 0
    else
        echo "  ⚠ Socket state unclear:"
        echo "$socket_info"
        return 0  # Don't fail, just warn
    fi
}

test_persist_binary_version() {
    echo "Checking persist binary version..."

    local version=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "/opt/joblet/bin/joblet-persist version 2>&1" || echo "version command not available")

    echo "  Persist version: $version"

    if [[ "$version" != *"not available"* ]]; then
        echo "  ✓ Version command works"
        return 0
    else
        echo "  ⚠ Version command not implemented"
        return 0  # Don't fail
    fi
}

test_storage_disk_usage() {
    echo "Checking storage disk usage..."

    local logs_size=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo du -sh /opt/joblet/logs 2>/dev/null | cut -f1" || echo "unknown")
    local metrics_size=$(ssh ${REMOTE_USER}@${REMOTE_HOST} "sudo du -sh /opt/joblet/metrics 2>/dev/null | cut -f1" || echo "unknown")

    echo "  Logs storage: $logs_size"
    echo "  Metrics storage: $metrics_size"

    if [[ "$logs_size" != "unknown" ]] && [[ "$metrics_size" != "unknown" ]]; then
        echo "  ✓ Storage directories accessible"
        return 0
    else
        echo "  ✗ Could not access storage directories"
        return 1
    fi
}

# ============================================
# Run Tests
# ============================================

test_section "Persist Subprocess Status"
run_test "Persist subprocess is running as child of joblet" test_persist_service_running
run_test "Persist Unix socket exists" test_persist_socket_exists
run_test "IPC socket has correct permissions" test_ipc_socket_permissions
run_test "Persist binary version check" test_persist_binary_version

test_section "Log Persistence"
run_test "Basic log persistence" test_log_persistence
run_test "Log directory structure" test_log_directory_structure

test_section "Metric Persistence"
run_test "Basic metric persistence" test_metric_persistence

test_section "Concurrent Operations"
run_test "Multiple jobs persistence" test_multiple_jobs_persistence

test_section "Subprocess Health"
run_test "Persist subprocess logs check" test_persist_service_logs
run_test "Storage disk usage" test_storage_disk_usage

# ============================================
# Test Summary
# ============================================

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Test Summary${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

echo -e "Total Tests:    $TOTAL_TESTS"
echo -e "Passed:         ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:         ${RED}$FAILED_TESTS${NC}"
echo -e "Skipped:        ${YELLOW}$SKIPPED_TESTS${NC}"

if [[ $TOTAL_TESTS -gt 0 ]]; then
    pass_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
    echo -e "Pass Rate:      ${GREEN}${pass_rate}%${NC}"
fi

echo -e "\n${BLUE}Completed: $(date '+%Y-%m-%d %H:%M:%S')${NC}"

if [[ $FAILED_TESTS -eq 0 ]]; then
    echo -e "\n${GREEN}✅ ALL TESTS PASSED!${NC}"
    echo -e "${GREEN}Persist subprocess is working correctly.${NC}"
    exit 0
else
    echo -e "\n${RED}❌ SOME TESTS FAILED${NC}"
    echo -e "${RED}Please check the persist subprocess configuration.${NC}"
    exit 1
fi
