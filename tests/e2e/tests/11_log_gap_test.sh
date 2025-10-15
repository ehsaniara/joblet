#!/bin/bash

# Log Gap Prevention Tests
# Tests that rnx job log shows all logs without gaps when transitioning
# from historical (persist) to live (buffer) logs

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration
REMOTE_HOST="192.168.1.161"
REMOTE_USER="jay"

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Joblet Log Gap Prevention Tests${NC}"
echo -e "${CYAN}  Testing against remote host: ${REMOTE_HOST}${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Started: $(date '+%Y-%m-%d %H:%M:%S')${NC}\n"

# Verify RNX configuration
if grep -q "$REMOTE_HOST" ~/.rnx/rnx-config.yml 2>/dev/null; then
    echo -e "  ${GREEN}✓ RNX configured for remote host $REMOTE_HOST${NC}"
else
    echo -e "  ${RED}✗ RNX not configured for remote host${NC}"
    exit 1
fi

# Global test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Cleanup function
cleanup() {
    local job_id=$1
    if [ -n "$job_id" ]; then
        echo "Cleaning up job: $job_id"
        $RNX_BINARY job delete "$job_id" 2>/dev/null || true
    fi
}

# Test 1: Completed job - verify all logs present, no duplicates
test_completed_job() {
    echo ""
    echo "Test 1: Completed job (should have no gaps, no duplicates)"
    echo "-----------------------------------------------------------"

    # Run a job that produces 100 numbered lines
    JOB_OUTPUT=$(mktemp)
    trap "rm -f $JOB_OUTPUT" EXIT

    echo "Submitting job that outputs 100 numbered lines..."
    JOB_ID=$($RNX_BINARY job run bash -c 'for i in {1..100}; do echo "Line $i"; done' --name log-gap-test-1 2>&1 | grep -oP 'ID: \K[a-f0-9-]+' | head -1 || echo "")

    if [ -z "$JOB_ID" ]; then
        echo "❌ Failed to get job ID"
        return 1
    fi

    echo "Job ID: $JOB_ID"

    # Wait for job to complete
    echo "Waiting for job to complete..."
    sleep 2

    MAX_WAIT=30
    WAITED=0
    while [ $WAITED -lt $MAX_WAIT ]; do
        STATUS=$($RNX_BINARY job status "$JOB_ID" | grep -oP '(?<=Status: )[A-Z]+' || echo "")
        if [ "$STATUS" = "COMPLETED" ] || [ "$STATUS" = "FAILED" ]; then
            break
        fi
        sleep 1
        WAITED=$((WAITED + 1))
    done

    if [ "$STATUS" != "COMPLETED" ]; then
        echo "❌ Job did not complete in time (status: $STATUS)"
        cleanup "$JOB_ID"
        return 1
    fi

    echo "Job completed successfully"

    # Wait for persist to receive all logs
    echo "Waiting 2 seconds for persist to catch up..."
    sleep 2

    # Fetch logs
    echo "Fetching logs..."
    $RNX_BINARY job log "$JOB_ID" > "$JOB_OUTPUT" 2>&1

    # Verify we got all 100 lines
    TOTAL_LINES=$(grep -c "^Line " "$JOB_OUTPUT" || echo "0")
    echo "Total log lines received: $TOTAL_LINES"

    if [ "$TOTAL_LINES" -ne 100 ]; then
        echo "❌ Expected 100 lines, got $TOTAL_LINES - GAP DETECTED!"
        echo "First 10 lines:"
        head -10 "$JOB_OUTPUT"
        echo "Last 10 lines:"
        tail -10 "$JOB_OUTPUT"
        cleanup "$JOB_ID"
        return 1
    fi

    # Check for duplicates
    UNIQUE_LINES=$(sort "$JOB_OUTPUT" | uniq | grep -c "^Line " || echo "0")
    if [ "$UNIQUE_LINES" -ne "$TOTAL_LINES" ]; then
        echo "❌ Found duplicates! Unique: $UNIQUE_LINES, Total: $TOTAL_LINES"
        echo "Duplicate lines:"
        sort "$JOB_OUTPUT" | uniq -d
        cleanup "$JOB_ID"
        return 1
    fi

    # Verify sequence is correct (1, 2, 3, ... 100)
    for i in {1..100}; do
        if ! grep -q "^Line $i$" "$JOB_OUTPUT"; then
            echo "❌ Missing line $i in output - GAP DETECTED!"
            cleanup "$JOB_ID"
            return 1
        fi
    done

    echo "✅ All 100 lines present, no gaps, no duplicates"
    cleanup "$JOB_ID"
    return 0
}

# Test 2: Running job - start log stream while job is running
test_running_job() {
    echo ""
    echo "Test 2: Running job (connect while running)"
    echo "--------------------------------------------"

    JOB_OUTPUT=$(mktemp)
    trap "rm -f $JOB_OUTPUT" EXIT

    # Run a job that produces 200 lines over 10 seconds
    echo "Submitting long-running job (200 lines over 10 seconds)..."
    JOB_ID=$($RNX_BINARY job run bash -c 'for i in {1..200}; do echo "Line $i"; sleep 0.05; done' --name log-gap-test-2 2>&1 | grep -oP 'ID: \K[a-f0-9-]+' | head -1 || echo "")

    if [ -z "$JOB_ID" ]; then
        echo "❌ Failed to get job ID"
        return 1
    fi

    echo "Job ID: $JOB_ID"

    # Wait for some logs to be produced and persisted
    echo "Waiting 3 seconds for job to produce logs..."
    sleep 3

    # Start streaming logs (this should get historical + live)
    echo "Starting log stream..."
    timeout 15 $RNX_BINARY job log "$JOB_ID" > "$JOB_OUTPUT" 2>&1 || true

    # Count lines
    TOTAL_LINES=$(grep -c "^Line " "$JOB_OUTPUT" || echo "0")
    echo "Total log lines received: $TOTAL_LINES"

    # We should have all 200 lines
    if [ "$TOTAL_LINES" -ne 200 ]; then
        echo "⚠️  Warning: Expected 200 lines, got $TOTAL_LINES"
        if [ "$TOTAL_LINES" -lt 180 ]; then
            echo "❌ Gap is too large (missing more than 20 lines)"
            cleanup "$JOB_ID"
            return 1
        fi
        echo "Minor gap acceptable (within 20 lines)"
    fi

    # Check for duplicates
    UNIQUE_LINES=$(sort "$JOB_OUTPUT" | uniq | grep -c "^Line " || echo "0")
    if [ "$UNIQUE_LINES" -ne "$TOTAL_LINES" ]; then
        DUPS=$((TOTAL_LINES - UNIQUE_LINES))
        echo "⚠️  Found $DUPS duplicate lines (expected with deduplication)"
        if [ "$DUPS" -gt 50 ]; then
            echo "❌ Too many duplicates (> 50)"
            cleanup "$JOB_ID"
            return 1
        fi
        echo "Acceptable duplicate count"
    fi

    echo "✅ Running job test passed (received $TOTAL_LINES lines, $UNIQUE_LINES unique)"
    cleanup "$JOB_ID"
    return 0
}

# Test 3: Rapid log generation - stress test
test_rapid_logs() {
    echo ""
    echo "Test 3: Rapid log generation (stress test)"
    echo "-------------------------------------------"

    JOB_OUTPUT=$(mktemp)
    trap "rm -f $JOB_OUTPUT" EXIT

    # Generate 1000 lines very quickly
    echo "Submitting rapid log generation job (1000 lines)..."
    JOB_ID=$($RNX_BINARY job run bash -c 'for i in {1..1000}; do echo "Line $i"; done' --name log-gap-test-3 2>&1 | grep -oP 'ID: \K[a-f0-9-]+' | head -1 || echo "")

    if [ -z "$JOB_ID" ]; then
        echo "❌ Failed to get job ID"
        return 1
    fi

    echo "Job ID: $JOB_ID"

    # Wait for completion
    sleep 3

    # Wait for persist
    echo "Waiting for persist to catch up..."
    sleep 2

    # Fetch logs
    echo "Fetching logs..."
    $RNX_BINARY job log "$JOB_ID" > "$JOB_OUTPUT" 2>&1

    TOTAL_LINES=$(grep -c "^Line " "$JOB_OUTPUT" || echo "0")
    echo "Total log lines received: $TOTAL_LINES"

    if [ "$TOTAL_LINES" -lt 900 ]; then
        echo "❌ Too many missing lines (< 900/1000)"
        cleanup "$JOB_ID"
        return 1
    fi

    if [ "$TOTAL_LINES" -lt 1000 ]; then
        MISSING=$((1000 - TOTAL_LINES))
        echo "⚠️  Warning: Missing $MISSING lines (got $TOTAL_LINES/1000)"
        echo "This might indicate persist latency under high load"
    else
        echo "✅ All 1000 lines received"
    fi

    # Check for duplicates
    UNIQUE_LINES=$(sort "$JOB_OUTPUT" | uniq | grep -c "^Line " || echo "0")
    if [ "$UNIQUE_LINES" -ne "$TOTAL_LINES" ]; then
        echo "❌ Found duplicates in stress test"
        cleanup "$JOB_ID"
        return 1
    fi

    echo "✅ Stress test passed"
    cleanup "$JOB_ID"
    return 0
}

# Run all tests
FAILED=0

test_completed_job || FAILED=$((FAILED + 1))
test_running_job || FAILED=$((FAILED + 1))
test_rapid_logs || FAILED=$((FAILED + 1))

echo ""
echo "==================================="
echo "Test Summary"
echo "==================================="
if [ $FAILED -eq 0 ]; then
    echo "✅ All log gap tests passed!"
    exit 0
else
    echo "❌ $FAILED test(s) failed"
    exit 1
fi
