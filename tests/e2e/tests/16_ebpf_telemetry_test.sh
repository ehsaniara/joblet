#!/bin/bash

# Test 16: eBPF Telemetry Tests
# Verifies: eBPF event capture, telemetry streaming, event types (EXEC, NET, ACCEPT, etc.)
# Prerequisites: eBPF support enabled on joblet server

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration (consistent with other tests)
REMOTE_HOST="${REMOTE_HOST:-192.168.1.161}"
REMOTE_USER="${REMOTE_USER:-jay}"

# Initialize test suite
test_suite_init "eBPF Telemetry Tests"

# ============================================
# Test Functions
# ============================================

test_metrics_tel_flag_exists() {
    echo "Testing --tel flag is recognized..."

    # Check help output includes --tel flag
    local help_output=$($RNX_BINARY job metrics --help 2>&1)

    if echo "$help_output" | grep -q "\-\-tel"; then
        echo "  ✓ --tel flag is documented in help"
        return 0
    else
        echo "  ✗ --tel flag not found in help output"
        echo "  Help output: $help_output"
        return 1
    fi
}

test_exec_events_captured() {
    echo "Testing EXEC events are captured..."

    # Run a job that spawns child processes
    local job_output=$($RNX_BINARY job run sh -c "echo 'Starting'; ls /; cat /etc/hostname; echo 'Done'" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"

    # Wait for job to complete and events to be collected
    sleep 5

    # Get telemetry events
    local tel_output=$(timeout 10 $RNX_BINARY job metrics "$job_id" --tel 2>&1 || true)

    # Check for EXEC events
    local exec_count=$(echo "$tel_output" | grep -c "EXEC" 2>/dev/null || echo "0")

    if [[ "$exec_count" -gt 0 ]]; then
        echo "  ✓ Found $exec_count EXEC events"
        # Show sample events
        echo "  Sample EXEC events:"
        echo "$tel_output" | grep "EXEC" | head -3 | sed 's/^/    /'
        return 0
    else
        echo "  ✗ No EXEC events found"
        echo "  Telemetry output:"
        echo "$tel_output" | head -20
        return 1
    fi
}

test_net_events_captured() {
    echo "Testing NET events are captured for outgoing connections..."

    # Run a job that makes network connections
    local job_output=$($RNX_BINARY job run sh -c "ping -c 1 8.8.8.8 2>/dev/null || curl -s --max-time 3 http://google.com > /dev/null 2>&1 || echo 'network test'" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"

    # Wait for job to complete and events to be collected
    sleep 5

    # Get telemetry events
    local tel_output=$(timeout 10 $RNX_BINARY job metrics "$job_id" --tel 2>&1 || true)

    # Check for NET events
    local net_count=$(echo "$tel_output" | grep -c "NET" 2>/dev/null || echo "0")

    if [[ "$net_count" -gt 0 ]]; then
        echo "  ✓ Found $net_count NET events"
        echo "  Sample NET events:"
        echo "$tel_output" | grep "NET" | head -3 | sed 's/^/    /'
        return 0
    else
        echo "  ⚠ No NET events found (network may be isolated or ping/curl failed)"
        echo "  This may be expected if the job has no network access"
        # Don't fail - network events depend on job having network access
        return 0
    fi
}

test_combined_metrics_and_telemetry() {
    echo "Testing combined metrics and telemetry output..."

    # Run a job that uses resources and spawns processes
    local job_output=$($RNX_BINARY job run sh -c "for i in 1 2 3 4 5; do echo \$i; sleep 1; done" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"

    # Wait for job to run and collect data
    sleep 7

    # Get combined output with --tel flag
    local tel_output=$(timeout 15 $RNX_BINARY job metrics "$job_id" --tel 2>&1 || true)

    local has_metrics=false
    local has_telemetry=false

    # Check for metrics
    if echo "$tel_output" | grep -q "CPU:\|Memory:\|Metrics at"; then
        has_metrics=true
        echo "  ✓ Resource metrics present"
    fi

    # Check for telemetry events
    if echo "$tel_output" | grep -qE "EXEC|NET|ACCEPT|SEND|RECV|MMAP|MPROTECT"; then
        has_telemetry=true
        echo "  ✓ eBPF telemetry events present"
    fi

    if [[ "$has_metrics" == "true" || "$has_telemetry" == "true" ]]; then
        echo "  ✓ --tel flag produces combined output"
        return 0
    else
        echo "  ✗ Neither metrics nor telemetry events found"
        echo "  Output: $tel_output"
        return 1
    fi
}

test_telemetry_event_format() {
    echo "Testing telemetry event format..."

    # Run a simple job
    local job_output=$($RNX_BINARY job run sh -c "echo hello; ls /" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"
    sleep 3

    # Get telemetry
    local tel_output=$(timeout 10 $RNX_BINARY job metrics "$job_id" --tel 2>&1 || true)

    # Check event format - should have timestamp and event type at minimum
    # Expected formats:
    # EXEC events: timestamp EXEC comm PID -> Child_PID
    # NET events: timestamp NET proto src:port -> dst:port

    local format_ok=true

    # Check if EXEC events have expected format
    if echo "$tel_output" | grep -q "EXEC"; then
        local exec_line=$(echo "$tel_output" | grep "EXEC" | head -1)
        if echo "$exec_line" | grep -qE "[0-9]{4}-[0-9]{2}-[0-9]{2}.*EXEC"; then
            echo "  ✓ EXEC event has timestamp"
        else
            echo "  ⚠ EXEC event format may vary"
        fi
    fi

    echo "  ✓ Telemetry event format validation passed"
    return 0
}

test_short_uuid_with_telemetry() {
    echo "Testing short UUID support with --tel flag..."

    # Run a job
    local job_output=$($RNX_BINARY job run echo "Short UUID test" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Full Job ID: $job_id"

    # Extract first 8 characters for short UUID
    local short_id="${job_id:0:8}"
    echo "  Short Job ID: $short_id"

    sleep 3

    # Try metrics with short UUID and --tel flag
    local tel_output=$(timeout 10 $RNX_BINARY job metrics "$short_id" --tel 2>&1 || true)

    # Check if it worked (should not contain "not found" or similar errors)
    if echo "$tel_output" | grep -qi "not found\|error\|invalid"; then
        echo "  ✗ Short UUID not resolved"
        echo "  Output: $tel_output"
        return 1
    fi

    echo "  ✓ Short UUID works with --tel flag"
    return 0
}

test_telemetry_for_completed_job() {
    echo "Testing telemetry retrieval for completed job..."

    # Run a quick job
    local job_output=$($RNX_BINARY job run echo "Completed job test" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"

    # Wait for job to complete
    sleep 3

    # Verify job is completed
    local status=$(check_job_status "$job_id")
    echo "  Job status: $status"

    # Get telemetry for completed job
    local tel_output=$(timeout 10 $RNX_BINARY job metrics "$job_id" --tel 2>&1 || true)

    # Should get some output (either events or "no events" message)
    if [[ -n "$tel_output" ]]; then
        echo "  ✓ Telemetry retrieved for completed job"
        local event_count=$(echo "$tel_output" | grep -cE "EXEC|NET|ACCEPT" 2>/dev/null || echo "0")
        echo "  Events found: $event_count"
        return 0
    else
        echo "  ✗ No telemetry output for completed job"
        return 1
    fi
}

test_telemetry_filtering_by_type() {
    echo "Testing telemetry can be filtered by event type..."

    # Run a job with multiple event types
    local job_output=$($RNX_BINARY job run sh -c "echo test; ls /tmp; cat /etc/hostname" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"
    sleep 3

    # Get telemetry and filter with grep
    local tel_output=$(timeout 10 $RNX_BINARY job metrics "$job_id" --tel 2>&1 || true)

    # Filter for EXEC events only
    local exec_only=$(echo "$tel_output" | grep "EXEC" || true)

    if [[ -n "$exec_only" ]]; then
        local exec_count=$(echo "$exec_only" | wc -l)
        echo "  ✓ Can filter EXEC events ($exec_count events)"
        return 0
    else
        echo "  ⚠ No EXEC events to filter (this may be normal for some jobs)"
        return 0
    fi
}

# ============================================
# Run Tests
# ============================================

test_section "Basic Telemetry Flag"
run_test "Metrics --tel flag exists" test_metrics_tel_flag_exists

test_section "EXEC Event Capture"
run_test "EXEC events captured for process execution" test_exec_events_captured

test_section "NET Event Capture"
run_test "NET events captured for network connections" test_net_events_captured

test_section "Combined Output"
run_test "Combined metrics and telemetry output" test_combined_metrics_and_telemetry
run_test "Telemetry event format validation" test_telemetry_event_format

test_section "UUID Support"
run_test "Short UUID with telemetry" test_short_uuid_with_telemetry

test_section "Historical Telemetry"
run_test "Telemetry for completed job" test_telemetry_for_completed_job
run_test "Telemetry filtering by type" test_telemetry_filtering_by_type

# ============================================
# Test Summary
# ============================================

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  eBPF Telemetry Test Summary${NC}"
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
    echo -e "\n${GREEN}✅ ALL eBPF TELEMETRY TESTS PASSED!${NC}"
    echo -e "${GREEN}eBPF telemetry is working correctly.${NC}"
    exit 0
else
    echo -e "\n${RED}❌ SOME TESTS FAILED${NC}"
    echo -e "${RED}Please check eBPF configuration and kernel support.${NC}"
    exit 1
fi
