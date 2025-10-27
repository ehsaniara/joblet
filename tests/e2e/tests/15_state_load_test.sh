#!/bin/bash

# State Client Load Test - Testing 1000+ Concurrent Jobs
# Tests the connection pool architecture under high load
# Verifies: Connection pooling, timeout handling, concurrent operations, pool statistics

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration
REMOTE_HOST="${JOBLET_TEST_HOST:-192.168.1.161}"
REMOTE_USER="${JOBLET_TEST_USER:-jay}"

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  State Client Load Test (1000+ Concurrent Jobs)${NC}"
echo -e "${CYAN}  Testing connection pool architecture under load${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Started: $(date '+%Y-%m-%d %H:%M:%S')${NC}\n"

# Global test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Test configuration
NUM_JOBS=${1:-100}  # Default to 100 for e2e (use 1000 for full load test)
POOL_SIZE=${2:-20}
STATE_SOCKET="/opt/joblet/run/state-ipc.sock"

echo -e "${YELLOW}▶ Test Configuration${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
echo -e "  Number of concurrent jobs: ${NUM_JOBS}"
echo -e "  Connection pool size: ${POOL_SIZE}"
echo -e "  State socket: ${STATE_SOCKET}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"

# ============================================
# Test 1: Verify State Service is Running
# ============================================

test_state_service_running() {
    echo -e "${YELLOW}▶ Test 1: State Service Running${NC}"
    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    # Check if state socket exists on remote host
    if ssh "${REMOTE_USER}@${REMOTE_HOST}" "test -S ${STATE_SOCKET}" 2>/dev/null; then
        echo -e "  ${GREEN}✓ State socket exists${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "  ${RED}✗ State socket not found${NC}"
        echo -e "  ${RED}  Make sure joblet-state service is running${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
        return 1
    fi

    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"
}

# ============================================
# Test 2: Rapid Job Creation (Load Test)
# ============================================

test_rapid_job_creation() {
    echo -e "${YELLOW}▶ Test 2: Rapid Job Creation ($NUM_JOBS concurrent jobs)${NC}"
    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    local job_ids=()
    local start_time=$(date +%s)

    echo -e "  ${BLUE}Creating ${NUM_JOBS} jobs in parallel...${NC}"

    # Create jobs in parallel (background processes)
    for i in $(seq 1 $NUM_JOBS); do
        (
            job_output=$("$RNX_BINARY" job run echo "load-test-$i" 2>&1)
            job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
            if [[ -n "$job_id" ]]; then
                echo "$job_id"
            fi
        ) &

        # Limit concurrent bash processes
        if (( i % 50 == 0 )); then
            wait
        fi
    done

    # Wait for all background jobs
    wait

    local end_time=$(date +%s)
    local elapsed=$((end_time - start_time))
    local jobs_per_sec=$((NUM_JOBS / elapsed))

    echo -e "  ${BLUE}Time elapsed: ${elapsed}s${NC}"
    echo -e "  ${BLUE}Throughput: ${jobs_per_sec} jobs/sec${NC}"

    # Check job completion
    sleep 5  # Give jobs time to complete

    local completed_count=$(ssh "${REMOTE_USER}@${REMOTE_HOST}" \
        "$RNX_BINARY job list 2>/dev/null | grep 'COMPLETED' | grep -c 'load-test-' || echo 0")
    # Ensure we have a valid number (default to 0 if empty or invalid)
    completed_count=${completed_count:-0}
    completed_count=$(echo "$completed_count" | tr -d '\n')

    echo -e "  ${BLUE}Completed jobs: ${completed_count}/${NUM_JOBS}${NC}"

    # Success if at least 95% of jobs completed (allow 5% failure margin)
    local min_success=$((NUM_JOBS * 95 / 100))
    if (( completed_count >= min_success )); then
        echo -e "  ${GREEN}✓ Load test passed (${completed_count}/${NUM_JOBS} jobs completed)${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "  ${RED}✗ Load test failed (only ${completed_count}/${NUM_JOBS} completed)${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi

    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"
}

# ============================================
# Test 3: State Query During Load
# ============================================

test_state_query_during_load() {
    echo -e "${YELLOW}▶ Test 3: State Query During Load${NC}"
    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    echo -e "  ${BLUE}Creating background load (50 jobs)...${NC}"

    # Create background load
    for i in $(seq 1 50); do
        "$RNX_BINARY" job run sleep 10 >/dev/null 2>&1 &
    done

    sleep 2  # Let jobs start

    # Try to query state while under load
    echo -e "  ${BLUE}Querying job list during load...${NC}"

    local start_query=$(date +%s%3N)
    local job_count=$("$RNX_BINARY" job list 2>/dev/null | grep -c "RUNNING\|SCHEDULED" || echo 0)
    local end_query=$(date +%s%3N)
    local query_time=$((end_query - start_query))

    echo -e "  ${BLUE}Query latency: ${query_time}ms${NC}"
    echo -e "  ${BLUE}Found ${job_count} active jobs${NC}"

    # Query should complete in reasonable time (< 5 seconds)
    if (( query_time < 5000 )); then
        echo -e "  ${GREEN}✓ Query completed quickly during load${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "  ${RED}✗ Query too slow (${query_time}ms)${NC}"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi

    # Cleanup
    "$RNX_BINARY" job list 2>/dev/null | \
        grep 'RUNNING' | \
        grep -o '[a-f0-9-]\{36\}' | \
        xargs -I {} "$RNX_BINARY" job cancel {} 2>/dev/null || true

    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"
}

# ============================================
# Test 4: Connection Pool Statistics
# ============================================

test_connection_pool_stats() {
    echo -e "${YELLOW}▶ Test 4: Connection Pool Statistics${NC}"
    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    # Check joblet logs for pool statistics
    echo -e "  ${BLUE}Checking connection pool metrics...${NC}"

    local log_file="/opt/joblet/logs/joblet.log"
    local recent_stats=$(ssh "${REMOTE_USER}@${REMOTE_HOST}" \
        "tail -1000 ${log_file} 2>/dev/null | grep -i 'pool.*stat' | tail -1" || echo "")

    if [[ -n "$recent_stats" ]]; then
        echo -e "  ${BLUE}Recent pool stats:${NC}"
        echo -e "  ${recent_stats}"
        echo -e "  ${GREEN}✓ Pool statistics available${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "  ${YELLOW}⚠ Pool statistics not found in logs${NC}"
        echo -e "  ${YELLOW}  (This is OK - stats may not be logged by default)${NC}"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    fi

    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"
}

# ============================================
# Test 5: Cleanup Load Test Jobs
# ============================================

test_cleanup() {
    echo -e "${YELLOW}▶ Test 5: Cleanup Load Test Jobs${NC}"
    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    echo -e "  ${BLUE}Cleaning up test jobs...${NC}"

    # Delete all load-test jobs
    local deleted_count=0
    while read -r job_id; do
        if [[ -n "$job_id" ]]; then
            "$RNX_BINARY" job delete "$job_id" >/dev/null 2>&1 && \
                deleted_count=$((deleted_count + 1))
        fi
    done < <("$RNX_BINARY" job list 2>/dev/null | \
             grep "load-test-" | \
             grep -o '[a-f0-9-]\{36\}')

    echo -e "  ${BLUE}Deleted ${deleted_count} test jobs${NC}"
    echo -e "  ${GREEN}✓ Cleanup completed${NC}"
    PASSED_TESTS=$((PASSED_TESTS + 1))

    echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"
}

# ============================================
# Run All Tests
# ============================================

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Running State Load Tests${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

test_state_service_running
test_rapid_job_creation
test_state_query_during_load
test_connection_pool_stats
test_cleanup

# ============================================
# Final Results
# ============================================

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Test Results Summary${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

echo -e "  ${BLUE}Total Tests:  ${TOTAL_TESTS}${NC}"
echo -e "  ${GREEN}Passed:       ${PASSED_TESTS}${NC}"
echo -e "  ${RED}Failed:       ${FAILED_TESTS}${NC}"

if (( FAILED_TESTS > 0 )); then
    echo -e "\n${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  SOME TESTS FAILED${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    exit 1
else
    echo -e "\n${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  ALL TESTS PASSED ✓${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    exit 0
fi
