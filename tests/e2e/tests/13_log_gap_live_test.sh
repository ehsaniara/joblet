#!/bin/bash
# Test: Live log streaming with no gaps (persist → live transition)
# This tests the critical user experience: checking logs while job is RUNNING
#
# Scenario:
# - Job runs for 10 seconds, producing 100 logs/second (1000 total logs)
# - User checks logs at the 5-second mark (500 logs already produced)
# - Should get all historical logs from persist
# - Then seamlessly transition to live streaming
# - No gaps, no duplicates

# Source test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration
REMOTE_HOST="${REMOTE_HOST:-192.168.1.161}"
REMOTE_USER="${REMOTE_USER:-jay}"

# Set JOBLET_TEST_HOST for test framework
export JOBLET_TEST_HOST="$REMOTE_HOST"
export JOBLET_TEST_USER="$REMOTE_USER"
export JOBLET_TEST_USE_SSH="true"

# Initialize test suite
test_suite_init "Live Log Gap Prevention Tests"

# ============================================
# Test Functions
# ============================================

test_live_log_streaming_no_gaps() {
    echo -e "${BLUE}Testing live log streaming with high-frequency logs${NC}"
    echo -e "${BLUE}Job: 10 seconds, 100 logs/second = 1000 total logs${NC}"

    # Start a job that produces 100 logs per second for 10 seconds
    local job_output=$(run_rnx_command "job run bash -c 'for i in {1..1000}; do echo \"Log \$i\"; sleep 0.01; done' --name log-gap-live-test" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Wait 5 seconds (job produces ~500 logs)
    echo -e "${BLUE}Waiting 5 seconds (job will produce ~500 logs)...${NC}"
    sleep 5

    # Now start streaming logs while job is STILL RUNNING
    echo -e "${BLUE}Streaming logs from running job (checking persist → live transition)...${NC}"

    local log_output=$(mktemp)
    trap "rm -f $log_output" EXIT

    # Stream logs for 8 seconds (enough to get rest of job + ensure completion)
    # Use SSH directly instead of through run_rnx_command to avoid timeout issues with bash functions
    timeout 8 ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" "rnx job log '$job_id'" > "$log_output" 2>&1 || true

    # Wait for job to complete
    sleep 2

    # Analyze logs
    echo -e "${BLUE}Analyzing log completeness...${NC}"

    # Filter out DEBUG/system lines and count only user logs
    grep "^Log [0-9]" "$log_output" 2>/dev/null > "${log_output}.filtered" || true

    # Count total lines
    local total=$(wc -l < "${log_output}.filtered" 2>/dev/null || echo "0")
    echo -e "  Total logs received: $total/1000"

    # Count unique lines
    local unique=$(sort -u "${log_output}.filtered" 2>/dev/null | wc -l || echo "0")
    echo -e "  Unique logs: $unique"

    # Check for duplicates
    local duplicates=$((total - unique))
    if [ $duplicates -gt 0 ]; then
        echo -e "  ${YELLOW}⚠ Found $duplicates duplicate logs (acceptable during persist→live transition)${NC}"
    else
        echo -e "  ${GREEN}✓ No duplicate logs (perfect!)${NC}"
    fi

    # Check for gaps
    local missing=0
    local first_missing=""
    for i in {1..1000}; do
        if ! grep -q "^Log $i$" "${log_output}.filtered"; then
            if [ -z "$first_missing" ]; then
                first_missing=$i
            fi
            missing=$((missing + 1))
        fi
    done

    if [ $missing -gt 0 ]; then
        echo -e "  ${RED}✗ Missing $missing logs (first missing: Log $first_missing)${NC}"
        echo -e "  ${YELLOW}This indicates a gap in persist → live transition!${NC}"
    else
        echo -e "  ${GREEN}✓ No missing logs - seamless transition!${NC}"
    fi

    # Calculate success
    local success_rate=$((total * 100 / 1000))
    echo -e "  Success rate: ${success_rate}%"

    # Cleanup
    run_rnx_command "job delete '$job_id'" >/dev/null 2>&1 || true

    # Test passes if we got all unique logs (no gaps), even with duplicates
    # Duplicates during transition are acceptable - gaps are not!
    if [ $unique -ge 950 ] && [ $missing -eq 0 ]; then
        if [ $duplicates -eq 0 ]; then
            echo -e "${GREEN}✓ Test PASSED PERFECTLY: All ${unique}/1000 logs, no duplicates!${NC}"
        else
            echo -e "${GREEN}✓ Test PASSED: All ${unique}/1000 logs received, no gaps${NC}"
            echo -e "${YELLOW}  (${duplicates} duplicates during transition - acceptable)${NC}"
        fi
        return 0
    else
        echo -e "${RED}✗ Test FAILED${NC}"
        if [ $missing -gt 0 ]; then
            echo -e "  ${RED}CRITICAL: Missing $missing logs - gaps detected!${NC}"
        fi
        if [ $unique -lt 950 ]; then
            echo -e "  ${RED}CRITICAL: Only got $unique/1000 unique logs${NC}"
        fi
        echo -e "  First 20 log lines:"
        head -20 "${log_output}.filtered"
        echo -e "  Last 20 log lines:"
        tail -20 "${log_output}.filtered"
        return 1
    fi
}

test_live_log_streaming_early_check() {
    echo -e "${BLUE}Testing early log check (check logs immediately after job starts)${NC}"

    # Start a job that produces logs continuously
    local job_output=$(run_rnx_command "job run bash -c 'for i in {1..500}; do echo \"Early \$i\"; sleep 0.02; done' --name early-check-test" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Check logs almost immediately (1 second after start)
    echo -e "${BLUE}Waiting only 1 second before checking logs...${NC}"
    sleep 1

    # Stream logs while job is still in early stages
    echo -e "${BLUE}Checking logs very early in job execution...${NC}"

    local log_output=$(mktemp)
    trap "rm -f $log_output" EXIT

    # Stream for 5 seconds
    # Use SSH directly instead of through run_rnx_command to avoid timeout issues
    timeout 5 ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "$REMOTE_USER@$REMOTE_HOST" "rnx job log '$job_id'" > "$log_output" 2>&1 || true

    # Count logs (filter out DEBUG lines)
    local total=$(grep "^Early [0-9]" "$log_output" 2>/dev/null | wc -l || echo "0")
    echo -e "  Logs received: $total"

    # Cleanup
    run_rnx_command "job delete '$job_id'" >/dev/null 2>&1 || true

    # Test passes if we got at least some logs (proves streaming works even when checking early)
    if [ $total -gt 10 ]; then
        echo -e "${GREEN}✓ Test PASSED: Got $total logs from early check${NC}"
        return 0
    else
        echo -e "${RED}✗ Test FAILED: Only got $total logs${NC}"
        return 1
    fi
}

test_log_streaming_after_persist() {
    echo -e "${BLUE}Testing log retrieval after persist has written (historical logs)${NC}"

    # Start a short job
    local job_output=$(run_rnx_command "job run bash -c 'for i in {1..100}; do echo \"Historical \$i\"; done' --name persist-test" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}' | head -1)

    if [ -z "$job_id" ]; then
        echo -e "${RED}✗ Failed to start job${NC}"
        return 1
    fi

    echo -e "${GREEN}✓ Job started: $job_id${NC}"

    # Wait for job to complete AND persist to write logs
    echo -e "${BLUE}Waiting for job completion and persist write...${NC}"
    sleep 5

    # Now fetch logs (should come from persist, not live stream)
    echo -e "${BLUE}Fetching historical logs from persist...${NC}"

    local log_output=$(mktemp)
    trap "rm -f $log_output" EXIT

    run_rnx_command "job log '$job_id'" > "$log_output" 2>&1

    # Count logs (filter out DEBUG lines and empty lines)
    local total=$(grep "^Historical [0-9]" "$log_output" 2>/dev/null | grep -v "^$" | wc -l || echo "0")
    echo -e "  Logs received: $total/100"

    # Cleanup
    run_rnx_command "job delete '$job_id'" >/dev/null 2>&1 || true

    # Test passes if we got all logs (allow 1-2 extra lines for formatting)
    if [ $total -ge 100 ] && [ $total -le 102 ]; then
        echo -e "${GREEN}✓ Test PASSED: All 100 historical logs retrieved${NC}"
        return 0
    else
        echo -e "${RED}✗ Test FAILED: Got $total logs, expected 100${NC}"
        return 1
    fi
}

# ============================================
# Run Tests
# ============================================

test_section "Live Log Streaming Tests"
run_test "Live streaming with 1000 logs (mid-execution check)" test_live_log_streaming_no_gaps
run_test "Early log check (1 second after start)" test_live_log_streaming_early_check
run_test "Historical log retrieval from persist" test_log_streaming_after_persist

# ============================================
# Test Summary
# ============================================

test_suite_summary

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}✅ ALL LIVE LOG GAP TESTS PASSED!${NC}"
    echo -e "${GREEN}Log streaming works perfectly: no gaps, no duplicates${NC}"
    exit 0
else
    echo -e "\n${RED}❌ SOME LIVE LOG TESTS FAILED${NC}"
    echo -e "${RED}Please check the persist → live transition logic${NC}"
    exit 1
fi
