#!/bin/bash

# Quick nodeId verification test
set -e

echo "Testing nodeId functionality..."

# Build the CLI
cd /home/jay/joblet/joblet
go build -o ./rnx ./cmd/rnx/main.go
go build -o ./joblet ./cmd/joblet/main.go

# Check if joblet is running
if ! pgrep -f joblet > /dev/null; then
    echo "Starting joblet server..."
    sudo systemctl start joblet || echo "Could not start via systemctl, joblet may not be installed"
    sleep 3
fi

echo "Running basic job..."
# Run a simple job
JOB_OUTPUT=$(./rnx job run echo "test" 2>&1 || echo "Job failed")
echo "Job output: $JOB_OUTPUT"

# Extract job ID
JOB_ID=$(echo "$JOB_OUTPUT" | grep "ID:" | awk '{print $2}')
if [[ -z "$JOB_ID" ]]; then
    echo "Failed to get job ID"
    exit 1
fi

echo "Job ID: $JOB_ID"

# Wait a bit
sleep 2

echo "Testing job status..."
# Check job status
STATUS_OUTPUT=$(./rnx job status "$JOB_ID" 2>&1 || echo "Status failed")
echo "Status output: $STATUS_OUTPUT"

# Check if Node ID appears in status
if echo "$STATUS_OUTPUT" | grep -q "Node ID:"; then
    echo "✓ Node ID found in job status!"
else
    echo "✗ Node ID NOT found in job status"
fi

echo "Testing job list..."
# Check job list
LIST_OUTPUT=$(./rnx job list 2>&1 || echo "List failed")
echo "List output: $LIST_OUTPUT"

# Check if NODE ID column appears in list
if echo "$LIST_OUTPUT" | grep -q "NODE ID"; then
    echo "✓ Node ID column found in job list!"
else
    echo "✗ Node ID column NOT found in job list"
fi

echo "Testing JSON output..."
# Check JSON status
JSON_STATUS=$(./rnx --json job status "$JOB_ID" 2>&1 || echo "JSON status failed")
echo "JSON status: $JSON_STATUS"

# Check if nodeId field appears in JSON
if echo "$JSON_STATUS" | grep -q "nodeId"; then
    echo "✓ nodeId field found in JSON status!"
else
    echo "✗ nodeId field NOT found in JSON status"
fi

# Check JSON list
JSON_LIST=$(./rnx --json job list 2>&1 || echo "JSON list failed")
echo "JSON list: $JSON_LIST"

# Check if node_id field appears in JSON
if echo "$JSON_LIST" | grep -q "node_id"; then
    echo "✓ node_id field found in JSON list!"
else
    echo "✗ node_id field NOT found in JSON list"
fi

echo "nodeId test completed!"