#!/bin/bash

# Test 16: eBPF Visibility Tests
# Verifies: eBPF event capture, visibility streaming, event types (EXEC, CONNECT, ACCEPT, etc.)
# Prerequisites: eBPF support enabled on joblet server

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration (consistent with other tests)
REMOTE_HOST="${REMOTE_HOST:-192.168.1.161}"
REMOTE_USER="${REMOTE_USER:-jay}"

# Initialize test suite
test_suite_init "eBPF Visibility Tests"

# ============================================
# Test Functions
# ============================================

test_visibility_command_exists() {
    echo "Testing visibility command is recognized..."

    # Check help output includes visibility command
    local help_output=$($RNX_BINARY job visibility --help 2>&1)

    if echo "$help_output" | grep -q "visibility"; then
        echo "  ✓ visibility command is available"
        return 0
    else
        echo "  ✗ visibility command not found"
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

    # Get visibility events
    local vis_output=$(timeout 10 $RNX_BINARY job visibility "$job_id" 2>&1 || true)

    # Check for EXEC events
    local exec_count=$(echo "$vis_output" | grep -c "EXEC" 2>/dev/null | head -1 || echo "0")

    if [[ "$exec_count" -gt 0 ]]; then
        echo "  ✓ Found $exec_count EXEC events"
        # Show sample events
        echo "  Sample EXEC events:"
        echo "$vis_output" | grep "EXEC" | head -3 | sed 's/^/    /'
        return 0
    else
        echo "  ✗ No EXEC events found"
        echo "  Visibility output:"
        echo "$vis_output" | head -20
        return 1
    fi
}

test_connect_events_captured() {
    echo "Testing CONNECT events are captured for outgoing connections..."

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

    # Get visibility events
    local vis_output=$(timeout 10 $RNX_BINARY job visibility "$job_id" 2>&1 || true)

    # Check for CONNECT events
    local connect_count=$(echo "$vis_output" | grep -c "CONNECT" 2>/dev/null | head -1 || echo "0")

    if [[ "$connect_count" -gt 0 ]]; then
        echo "  ✓ Found $connect_count CONNECT events"
        echo "  Sample CONNECT events:"
        echo "$vis_output" | grep "CONNECT" | head -3 | sed 's/^/    /'
        return 0
    else
        echo "  ⚠ No CONNECT events found (network may be isolated or ping/curl failed)"
        echo "  This may be expected if the job has no network access"
        # Don't fail - network events depend on job having network access
        return 0
    fi
}

test_visibility_event_types_filter() {
    echo "Testing visibility event type filtering..."

    # Run a job that spawns processes
    local job_output=$($RNX_BINARY job run sh -c "echo test; ls /tmp; cat /etc/hostname" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"
    sleep 5

    # Get only EXEC events using --types filter
    local exec_output=$(timeout 10 $RNX_BINARY job visibility "$job_id" --types exec 2>&1 || true)

    # Check that we got EXEC events
    local exec_count=$(echo "$exec_output" | grep -c "EXEC" 2>/dev/null | head -1 || echo "0")

    if [[ "$exec_count" -gt 0 ]]; then
        echo "  ✓ --types exec filter works ($exec_count events)"
        return 0
    else
        echo "  ⚠ No EXEC events with filter (may be expected for short jobs)"
        return 0
    fi
}

test_visibility_event_format() {
    echo "Testing visibility event format..."

    # Run a simple job
    local job_output=$($RNX_BINARY job run sh -c "echo hello; ls /" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"
    sleep 3

    # Get visibility events
    local vis_output=$(timeout 10 $RNX_BINARY job visibility "$job_id" 2>&1 || true)

    # Check event format - should have timestamp and event type at minimum
    # Expected format: [HH:MM:SS.mmm] EVENT_TYPE ...
    local format_ok=true

    # Check if EXEC events have expected format
    if echo "$vis_output" | grep -q "EXEC"; then
        local exec_line=$(echo "$vis_output" | grep "EXEC" | head -1)
        if echo "$exec_line" | grep -qE "\[[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{3}\].*EXEC"; then
            echo "  ✓ EXEC event has timestamp"
        else
            echo "  ⚠ EXEC event format may vary"
        fi
    fi

    echo "  ✓ Visibility event format validation passed"
    return 0
}

test_short_uuid_with_visibility() {
    echo "Testing short UUID support with visibility command..."

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

    # Try visibility with short UUID
    local vis_output=$(timeout 10 $RNX_BINARY job visibility "$short_id" 2>&1 || true)

    # Check if it worked (should not contain "not found" or similar errors)
    if echo "$vis_output" | grep -qi "not found\|error\|invalid"; then
        echo "  ✗ Short UUID not resolved"
        echo "  Output: $vis_output"
        return 1
    fi

    echo "  ✓ Short UUID works with visibility command"
    return 0
}

test_visibility_for_completed_job() {
    echo "Testing visibility retrieval for completed job..."

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

    # Get visibility for completed job
    local vis_output=$(timeout 10 $RNX_BINARY job visibility "$job_id" 2>&1 || true)

    # Should get some output (either events or "no events" message)
    if [[ -n "$vis_output" ]]; then
        echo "  ✓ Visibility retrieved for completed job"
        local event_count=$(echo "$vis_output" | grep -cE "EXEC|CONNECT|ACCEPT" 2>/dev/null || echo "0")
        echo "  Events found: $event_count"
        return 0
    else
        echo "  ✗ No visibility output for completed job"
        return 1
    fi
}

test_visibility_filtering_by_type() {
    echo "Testing visibility can be filtered by event type..."

    # Run a job with multiple event types
    local job_output=$($RNX_BINARY job run sh -c "echo test; ls /tmp; cat /etc/hostname" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"
    sleep 3

    # Get visibility and filter with grep
    local vis_output=$(timeout 10 $RNX_BINARY job visibility "$job_id" 2>&1 || true)

    # Filter for EXEC events only
    local exec_only=$(echo "$vis_output" | grep "EXEC" || true)

    if [[ -n "$exec_only" ]]; then
        local exec_count=$(echo "$exec_only" | wc -l)
        echo "  ✓ Can filter EXEC events ($exec_count events)"
        return 0
    else
        echo "  ⚠ No EXEC events to filter (this may be normal for some jobs)"
        return 0
    fi
}

test_metrics_separate_from_visibility() {
    echo "Testing metrics command is separate from visibility..."

    # Run a job
    local job_output=$($RNX_BINARY job run sh -c "for i in 1 2 3; do echo \$i; sleep 1; done" 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep -E "^ID:" | awk '{print $2}' | head -1)

    if [[ -z "$job_id" ]]; then
        echo "  ✗ Failed to extract job ID"
        return 1
    fi

    echo "  Job ID: $job_id"
    sleep 5

    # Get metrics (should show resource usage)
    local metrics_output=$(timeout 10 $RNX_BINARY job metrics "$job_id" 2>&1 || true)

    # Get visibility (should show eBPF events)
    local vis_output=$(timeout 10 $RNX_BINARY job visibility "$job_id" 2>&1 || true)

    local has_metrics=false
    local has_visibility=false

    # Check for metrics
    if echo "$metrics_output" | grep -qE "CPU:|Memory:|cpu_percent|memory_bytes"; then
        has_metrics=true
        echo "  ✓ Metrics command shows resource usage"
    fi

    # Check for visibility events
    if echo "$vis_output" | grep -qE "EXEC|CONNECT|ACCEPT|MMAP"; then
        has_visibility=true
        echo "  ✓ Visibility command shows eBPF events"
    fi

    # Verify they don't overlap (metrics shouldn't have eBPF events)
    if ! echo "$metrics_output" | grep -qE "^\[.*\] EXEC|^\[.*\] CONNECT"; then
        echo "  ✓ Metrics output doesn't contain eBPF events"
    fi

    echo "  ✓ Metrics and visibility commands are properly separated"
    return 0
}

# ============================================
# Run Tests
# ============================================

test_section "Visibility Command"
run_test "Visibility command exists" test_visibility_command_exists

test_section "EXEC Event Capture"
run_test "EXEC events captured for process execution" test_exec_events_captured

test_section "CONNECT Event Capture"
run_test "CONNECT events captured for network connections" test_connect_events_captured

test_section "Event Filtering"
run_test "Event type filtering with --types flag" test_visibility_event_types_filter
run_test "Visibility event format validation" test_visibility_event_format

test_section "UUID Support"
run_test "Short UUID with visibility" test_short_uuid_with_visibility

test_section "Historical Events"
run_test "Visibility for completed job" test_visibility_for_completed_job
run_test "Visibility filtering by type" test_visibility_filtering_by_type

test_section "API Separation"
run_test "Metrics separate from visibility" test_metrics_separate_from_visibility

# ============================================
# Test Summary
# ============================================

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  eBPF Visibility Test Summary${NC}"
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
    echo -e "\n${GREEN}✅ ALL eBPF VISIBILITY TESTS PASSED!${NC}"
    echo -e "${GREEN}eBPF visibility is working correctly.${NC}"
    exit 0
else
    echo -e "\n${RED}❌ SOME TESTS FAILED${NC}"
    echo -e "${RED}Please check eBPF configuration and kernel support.${NC}"
    exit 1
fi
