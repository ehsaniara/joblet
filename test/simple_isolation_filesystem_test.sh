#!/bin/bash
# Test script to verify filesystem isolation

set -e

echo "🧪 Testing Filesystem Isolation"

CLI_CMD="${CLI_CMD:-./bin/cli}"

echo "📝 Test 1: Job 1 creates a file in /tmp"
JOB1_ID=$("$CLI_CMD" run bash -c "echo 'Hello from Job 1' > /tmp/job1_file.txt && echo 'Job 1 file created' && ls -la /tmp/" | grep "ID:" | cut -d' ' -f2)
echo "Job 1 ID: $JOB1_ID"
sleep 2

echo "📝 Test 2: Job 2 tries to see Job 1's file"
JOB2_ID=$("$CLI_CMD" run bash -c "echo 'Hello from Job 2' > /tmp/job2_file.txt && echo 'Job 2 file created' && echo 'Looking for job1 file:' && ls -la /tmp/ && echo 'Checking if job1 file exists:' && test -f /tmp/job1_file.txt && echo 'FOUND job1 file (BAD!)' || echo 'job1 file NOT found (GOOD!)'" | grep "ID:" | cut -d' ' -f2)
echo "Job 2 ID: $JOB2_ID"
sleep 2

echo "📝 Test 3: Job 3 checks host filesystem access"
JOB3_ID=$("$CLI_CMD" run bash -c "echo 'Checking host filesystem access:' && ls -la / && echo 'Checking if we can see host /opt:' && ls -la /opt 2>/dev/null || echo 'Cannot see host /opt (GOOD!)' && echo 'Checking if we can see host /home:' && ls -la /home 2>/dev/null || echo 'Cannot see host /home (GOOD!)'" | grep "ID:" | cut -d' ' -f2)
echo "Job 3 ID: $JOB3_ID"
sleep 2

echo "📝 Test 4: Job 4 tries to access sensitive host files"
JOB4_ID=$("$CLI_CMD" run bash -c "echo 'Trying to access sensitive files:' && echo 'Checking /etc/passwd:' && head -n 3 /etc/passwd 2>/dev/null || echo 'Cannot read /etc/passwd (GOOD!)' && echo 'Checking /etc/shadow:' && head -n 1 /etc/shadow 2>/dev/null || echo 'Cannot read /etc/shadow (GOOD!)'" | grep "ID:" | cut -d' ' -f2)
echo "Job 4 ID: $JOB4_ID"
sleep 2

echo "📊 Getting job logs to verify isolation:"
echo ""
echo "=== Job 1 Log ==="
"$CLI_CMD" log $JOB1_ID
echo ""
echo "=== Job 2 Log ==="
"$CLI_CMD" log $JOB2_ID
echo ""
echo "=== Job 3 Log ==="
"$CLI_CMD" log $JOB3_ID
echo ""
echo "=== Job 4 Log ==="
"$CLI_CMD" log $JOB4_ID

echo ""
echo "✅ Test completed!"
echo "Expected results:"
echo "  - Job 1 should create its file successfully"
echo "  - Job 2 should NOT see Job 1's file"
echo "  - Job 3 should NOT see host directories like /opt, /home"
echo "  - Job 4 should NOT be able to read sensitive host files"
echo "  - Each job should have its own isolated /tmp directory"