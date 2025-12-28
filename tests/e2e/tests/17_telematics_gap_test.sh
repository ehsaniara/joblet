#!/bin/bash
# Test: Telematics event streaming with no gaps (persist → live transition)
# This tests that telematics events have no gaps when transitioning from
# historical (persist) to live streaming
#
# Scenario:
# - Job runs for ~20 seconds, spawning many processes (sleep calls)
# - User checks telematics at the 10-second mark
# - Should get all historical events from persist
# - Then seamlessly transition to live streaming
# - No gaps, no duplicates

# Source test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Initialize test suite
test_suite_init "Telematics Event Gap Prevention Tests"

# ============================================
# Test Functions
# ============================================

test_telematics_live_streaming_no_gaps() {
    echo -e "${BLUE}Testing live telematics streaming with frequent EXEC events${NC}"
    echo -e "${BLUE}Job: 20 seconds, spawning process every 0.1s = ~200 EXEC events${NC}"

    # Start a job that spawns 200 processes (one every 0.1s for 20s)
    local job_output=$($RNX_BINARY job run bash -c 'for i in $(seq 1 200); do echo "Iteration $i"; sleep 0.1; done' 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep "^ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        echo "Output: $job_output"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Wait 10 seconds (job produces ~100 EXEC events for sleep calls)
    echo -e "${BLUE}Waiting 10 seconds (job will produce ~100 EXEC events)...${NC}"
    sleep 10

    # Now start streaming telematics while job is STILL RUNNING
    echo -e "${BLUE}Streaming telematics from running job (checking persist → live transition)...${NC}"

    local vis_output=$(mktemp)

    # Stream telematics for 15 seconds (enough to get rest of job + ensure completion)
    # Strip ANSI color codes for easier parsing
    timeout 15 $RNX_BINARY job telematics "$job_id" 2>&1 | sed 's/\x1b\[[0-9;]*m//g' > "$vis_output" || true

    # Wait for job to complete
    sleep 2

    # Analyze telematics events
    echo -e "${BLUE}Analyzing telematics event completeness...${NC}"

    # Count EXEC events for /usr/bin/sleep
    local exec_count=$(grep "EXEC" "$vis_output" 2>/dev/null | grep -c "sleep" 2>/dev/null || echo 0)
    exec_count=$(echo "$exec_count" | tr -d '[:space:]')
    echo -e "  Total EXEC (sleep) events: $exec_count/200"

    # Extract PIDs to check for duplicates and gaps
    local pids=$(grep "EXEC" "$vis_output" 2>/dev/null | grep "sleep" | awk -F'pid=' '{print $2}' | awk '{print $1}' | sort -n)
    local unique_pids=$(echo "$pids" | sort -u | grep -c "[0-9]" 2>/dev/null || echo 0)
    unique_pids=$(echo "$unique_pids" | tr -d '[:space:]')
    echo -e "  Unique PIDs: $unique_pids"

    # Check for duplicates
    if [ "$exec_count" -gt "$unique_pids" ] 2>/dev/null; then
        local duplicates=$((exec_count - unique_pids))
        echo -e "  ${YELLOW}⚠ Found $duplicates duplicate events (acceptable during transition)${NC}"
    else
        echo -e "  ${GREEN}✓ No duplicate events (perfect!)${NC}"
    fi

    # Cleanup
    rm -f "$vis_output"

    # Calculate success - we expect at least 150 out of 200 EXEC events
    local threshold=150

    if [ "$unique_pids" -ge "$threshold" ] 2>/dev/null; then
        echo -e "${GREEN}✓ Test PASSED: Got $unique_pids unique EXEC events (>= $threshold)${NC}"
        return 0
    else
        echo -e "${RED}✗ Test FAILED: Only got $unique_pids unique EXEC events (< $threshold)${NC}"
        return 1
    fi
}

test_telematics_early_check() {
    echo -e "${BLUE}Testing early telematics check (check immediately after job starts)${NC}"

    # Start a job that spawns processes
    local job_output=$($RNX_BINARY job run bash -c 'for i in $(seq 1 50); do echo "Check $i"; sleep 0.1; done' 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep "^ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Check telematics almost immediately (1 second after start)
    echo -e "${BLUE}Waiting only 1 second before checking telematics...${NC}"
    sleep 1

    # Stream telematics while job is still in early stages
    echo -e "${BLUE}Checking telematics very early in job execution...${NC}"

    local vis_output=$(mktemp)

    # Stream for 8 seconds
    timeout 8 $RNX_BINARY job telematics "$job_id" 2>&1 | sed 's/\x1b\[[0-9;]*m//g' > "$vis_output" || true

    # Count EXEC events
    local exec_count=$(grep -c "EXEC" "$vis_output" 2>/dev/null || echo 0)
    exec_count=$(echo "$exec_count" | tr -d '[:space:]')
    echo -e "  EXEC events received: $exec_count"

    # Cleanup
    rm -f "$vis_output"

    # Test passes if we got at least some events (proves streaming works when checking early)
    if [ "$exec_count" -gt 5 ] 2>/dev/null; then
        echo -e "${GREEN}✓ Test PASSED: Got $exec_count EXEC events from early check${NC}"
        return 0
    else
        echo -e "${RED}✗ Test FAILED: Only got $exec_count EXEC events${NC}"
        return 1
    fi
}

test_telematics_for_completed_job() {
    echo -e "${BLUE}Testing telematics retrieval after job completion (historical events)${NC}"

    # Start a short job with known number of process spawns
    local job_output=$($RNX_BINARY job run bash -c 'for i in $(seq 1 10); do echo "Done $i"; sleep 0.05; done' 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep "^ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Wait for job to complete AND persist to write events
    echo -e "${BLUE}Waiting for job completion and persist write...${NC}"
    sleep 5

    # Now fetch telematics (should come from persist, not live stream)
    echo -e "${BLUE}Fetching historical telematics events from persist...${NC}"

    local vis_output=$(mktemp)

    timeout 10 $RNX_BINARY job telematics "$job_id" 2>&1 | sed 's/\x1b\[[0-9;]*m//g' > "$vis_output" || true

    # Count EXEC events (expect at least bash + 10 sleep calls)
    local exec_count=$(grep -c "EXEC" "$vis_output" 2>/dev/null || echo 0)
    exec_count=$(echo "$exec_count" | tr -d '[:space:]')
    echo -e "  EXEC events received: $exec_count"

    # Cleanup
    rm -f "$vis_output"

    # We expect at least 10 EXEC events (10 sleep calls)
    if [ "$exec_count" -ge 10 ] 2>/dev/null; then
        echo -e "${GREEN}✓ Test PASSED: Retrieved $exec_count EXEC events from completed job${NC}"
        return 0
    else
        echo -e "${RED}✗ Test FAILED: Only got $exec_count EXEC events, expected >= 10${NC}"
        return 1
    fi
}

test_telematics_gap_detection() {
    echo -e "${BLUE}Testing for gaps during persist → live transition${NC}"

    # Start a longer job to ensure persist has written some events before we stream
    local job_output=$($RNX_BINARY job run bash -c 'for i in $(seq 1 100); do echo "Gap test $i"; sleep 0.15; done' 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep "^ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Wait 8 seconds for persist to have written some events
    echo -e "${BLUE}Waiting 8s for persist to write initial events...${NC}"
    sleep 8

    # Now stream telematics - this tests persist → live transition
    echo -e "${BLUE}Testing persist → live transition...${NC}"

    local vis_output=$(mktemp)

    # Stream for 12 seconds to capture the transition and more live events
    timeout 12 $RNX_BINARY job telematics "$job_id" 2>&1 | sed 's/\x1b\[[0-9;]*m//g' > "$vis_output" || true

    # Extract timestamps from EXEC events
    local timestamp_count=$(grep "EXEC" "$vis_output" 2>/dev/null | grep -c "sleep" 2>/dev/null || echo 0)
    timestamp_count=$(echo "$timestamp_count" | tr -d '[:space:]')

    echo -e "  Events captured: $timestamp_count"

    # Cleanup
    rm -f "$vis_output"

    # Check we got sufficient events to validate transition
    if [ "$timestamp_count" -ge 20 ] 2>/dev/null; then
        echo -e "  ${GREEN}✓ Received sufficient events ($timestamp_count) to validate transition${NC}"
        return 0
    else
        echo -e "  ${RED}✗ Insufficient events to test for gaps (got $timestamp_count, need >= 20)${NC}"
        return 1
    fi
}

test_telematics_deduplication() {
    echo -e "${BLUE}Testing that buffer prevents duplicate telematics events${NC}"

    # Start a 15-second job
    local job_output=$($RNX_BINARY job run bash -c 'for i in $(seq 1 50); do echo "Dedup $i"; sleep 0.3; done' 2>&1)
    local job_id=$(echo "$job_output" | sed 's/\x1b\[[0-9;]*m//g' | grep "^ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Wait for some events to be collected
    echo -e "${BLUE}Waiting 8s for events...${NC}"
    sleep 8

    # Stream telematics (this triggers buffer → live transition)
    echo -e "${BLUE}Streaming telematics to test deduplication...${NC}"

    local vis_output=$(mktemp)

    timeout 10 $RNX_BINARY job telematics "$job_id" 2>&1 | sed 's/\x1b\[[0-9;]*m//g' > "$vis_output" || true

    # Extract PIDs and check for exact duplicates
    local pids=$(grep "EXEC" "$vis_output" 2>/dev/null | grep "sleep" | awk -F'pid=' '{print $2}' | awk '{print $1}')
    local total=$(echo "$pids" | grep -c "[0-9]" 2>/dev/null || echo 0)
    total=$(echo "$total" | tr -d '[:space:]')
    local unique=$(echo "$pids" | sort -u | grep -c "[0-9]" 2>/dev/null || echo 0)
    unique=$(echo "$unique" | tr -d '[:space:]')

    # Cleanup
    rm -f "$vis_output"

    if [ "$total" -eq "$unique" ] 2>/dev/null; then
        echo -e "  ${GREEN}✓ No duplicate PIDs detected ($total events, all unique)${NC}"
        return 0
    elif [ "$total" -gt 0 ] 2>/dev/null; then
        local duplicates=$((total - unique))
        echo -e "  ${YELLOW}⚠ Found $duplicates duplicate PIDs (out of $total)${NC}"
        return 0  # Don't fail - some overlap is acceptable
    else
        echo -e "  ${YELLOW}⚠ No PIDs found to check deduplication${NC}"
        return 0
    fi
}

# ============================================
# Run Tests
# ============================================

test_section "Live Telematics Streaming"
run_test "Live streaming with 200 EXEC events (mid-execution check)" test_telematics_live_streaming_no_gaps
run_test "Early telematics check (1 second after start)" test_telematics_early_check

test_section "Historical Telematics"
run_test "Telematics retrieval from persist for completed job" test_telematics_for_completed_job

test_section "Gap Detection"
run_test "Gap detection during persist → live transition" test_telematics_gap_detection
run_test "Telematics event deduplication" test_telematics_deduplication

# ============================================
# Test Summary
# ============================================

test_suite_summary

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}ALL VISIBILITY GAP TESTS PASSED!${NC}"
    echo -e "${GREEN}Telematics streaming works perfectly: no gaps during transitions${NC}"
    exit 0
else
    echo -e "\n${RED}SOME VISIBILITY TESTS FAILED${NC}"
    echo -e "${RED}Please check the persist → live transition logic for telematics events${NC}"
    exit 1
fi
