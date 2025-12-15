#!/bin/bash
# Simple test to verify no log gaps when using rnx job log
# This test demonstrates the fix for the persist->live transition

set -e

# Source test framework for RNX_BINARY and JOBLET_ROOT
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/../lib/test_framework.sh"

if [ ! -f "$RNX_BINARY" ]; then
    echo "❌ RNX binary not found at $RNX_BINARY"
    echo "Please build it first: make build"
    exit 1
fi

echo "==================================="
echo "Log Gap Prevention Test"
echo "==================================="
echo ""

# Test 1: Completed job - verify all logs, no gaps
echo "Test 1: Completed job (100 lines)"
echo "-----------------------------------"

JOB_OUTPUT=$(mktemp)
trap "rm -f $JOB_OUTPUT" EXIT

echo "Submitting job..."
JOB_ID=$($RNX_BINARY job run bash -c 'for i in {1..100}; do echo "Line $i"; done' --name log-gap-test 2>&1 | grep -oP 'ID: \K[a-f0-9-]+' | head -1)

if [ -z "$JOB_ID" ]; then
    echo "❌ Failed to submit job"
    exit 1
fi

echo "Job ID: $JOB_ID"
echo "Waiting for job to complete..."
sleep 3

# Wait for persist to catch up
echo "Waiting for persist to receive logs..."
sleep 2

echo "Fetching logs..."
$RNX_BINARY job log "$JOB_ID" > "$JOB_OUTPUT" 2>&1 || true

echo ""
echo "Results:"
echo "--------"

# Count total lines
TOTAL=$(grep -c "^Line " "$JOB_OUTPUT" 2>/dev/null || echo "0")
echo "Total lines received: $TOTAL/100"

# Count unique lines
UNIQUE=$(grep "^Line " "$JOB_OUTPUT" 2>/dev/null | sort | uniq | wc -l || echo "0")
echo "Unique lines: $UNIQUE"

# Check for gaps
MISSING=0
for i in {1..100}; do
    if ! grep -q "^Line $i$" "$JOB_OUTPUT"; then
        if [ $MISSING -eq 0 ]; then
            echo "Missing lines:"
        fi
        echo "  - Line $i"
        MISSING=$((MISSING + 1))
    fi
done

# Summary
echo ""
if [ "$TOTAL" -eq 100 ] && [ "$MISSING" -eq 0 ] && [ "$UNIQUE" -eq 100 ]; then
    echo "✅ TEST PASSED: All 100 lines received, no gaps, no duplicates"

    # Cleanup
    echo ""
    echo "Cleaning up job..."
    $RNX_BINARY job delete "$JOB_ID" 2>/dev/null || true

    exit 0
else
    echo "❌ TEST FAILED:"
    if [ "$TOTAL" -ne 100 ]; then
        echo "  - Expected 100 lines, got $TOTAL"
    fi
    if [ "$MISSING" -gt 0 ]; then
        echo "  - Missing $MISSING lines (gaps detected!)"
    fi
    if [ "$UNIQUE" -ne "$TOTAL" ]; then
        DUPS=$((TOTAL - UNIQUE))
        echo "  - Found $DUPS duplicate lines"
    fi

    echo ""
    echo "First 20 lines of output:"
    head -20 "$JOB_OUTPUT"

    # Cleanup
    echo ""
    echo "Cleaning up job..."
    $RNX_BINARY job delete "$JOB_ID" 2>/dev/null || true

    exit 1
fi
