#!/bin/bash
# Corrected User Namespace Isolation Test Suite
# Uses single commands or bash -c wrappers to avoid dangerous characters

set -e

# Configuration
CLI="./bin/cli"

echo "🧪 Corrected User Namespace Isolation Test Suite"
echo "================================================="
echo ""

# Test 1: Basic UID/GID Tests
echo "📋 Test 1: UID/GID Isolation"
echo "----------------------------"

echo "Creating UID test jobs..."
JOB1=$($CLI create id | grep "ID:" | cut -d' ' -f2)
JOB2=$($CLI create whoami | grep "ID:" | cut -d' ' -f2)
JOB3=$($CLI create bash -c 'echo UID: \$(id -u) GID: \$(id -g)' | grep "ID:" | cut -d' ' -f2)

echo "✅ Created UID test jobs: $JOB1, $JOB2, $JOB3"
sleep 3
echo ""

# Test 2: Process Isolation
echo "📋 Test 2: Process Isolation"
echo "----------------------------"

echo "Testing process visibility..."
PROC_JOB1=$($CLI create ps aux | grep "ID:" | cut -d' ' -f2)
PROC_JOB2=$($CLI create bash -c 'ps aux | wc -l' | grep "ID:" | cut -d' ' -f2)
PROC_JOB3=$($CLI create bash -c 'echo Processes: \$(ps aux | wc -l)' | grep "ID:" | cut -d' ' -f2)

echo "✅ Created process test jobs: $PROC_JOB1, $PROC_JOB2, $PROC_JOB3"
echo "✅ Each job should only see its own processes (small number)"
sleep 3
echo ""

# Test 3: File System Isolation
echo "📋 Test 3: File System Isolation"
echo "--------------------------------"

echo "Testing file access isolation..."
FILE_JOB1=$($CLI create bash -c 'echo secret-data > /tmp/job-secret.txt && echo File created && ls -la /tmp/job-secret.txt' | grep "ID:" | cut -d' ' -f2)
sleep 2
FILE_JOB2=$($CLI create bash -c 'cat /tmp/job-secret.txt 2>&1 || echo GOOD: Cannot access other job files' | grep "ID:" | cut -d' ' -f2)

echo "✅ Created file isolation test jobs: $FILE_JOB1, $FILE_JOB2"
echo "✅ Job2 should NOT be able to access job1's files"
sleep 3
echo ""

# Test 4: Root Capabilities Inside Namespace
echo "📋 Test 4: Root Inside Namespace"
echo "--------------------------------"

echo "Testing root capabilities..."
ROOT_JOB1=$($CLI create bash -c 'echo Namespace UID: \$(id -u) - Should be 0' | grep "ID:" | cut -d' ' -f2)
ROOT_JOB2=$($CLI create bash -c 'whoami && echo Effective user inside namespace' | grep "ID:" | cut -d' ' -f2)

echo "✅ Created root test jobs: $ROOT_JOB1, $ROOT_JOB2"
echo "✅ Should show UID=0 (root) inside namespace"
sleep 3
echo ""

# Test 5: Network Access
echo "📋 Test 5: Network Access"
echo "-------------------------"

echo "Testing network connectivity..."
NET_JOB=$($CLI create bash -c 'curl -s --connect-timeout 3 httpbin.org/ip || echo Network test completed' | grep "ID:" | cut -d' ' -f2)

echo "✅ Created network test job: $NET_JOB"
echo "✅ Should have network access (host networking)"
sleep 5
echo ""

# Test 6: Resource Limits
echo "📋 Test 6: Resource Limits"
echo "--------------------------"

echo "Testing resource limits..."
LIMIT_JOB=$($CLI create --max-memory=100 --max-cpu=50 bash -c 'echo Resource test && cat /proc/self/cgroup | head -3' | grep "ID:" | cut -d' ' -f2)

echo "✅ Created resource limit test job: $LIMIT_JOB"
echo "✅ Should show cgroup assignment"
sleep 3
echo ""

# Test 7: Concurrent Jobs
echo "📋 Test 7: Concurrent Execution"
echo "-------------------------------"

echo "Starting concurrent jobs..."
CONCURRENT_JOBS=()
for i in {1..3}; do
    JOB_ID=$($CLI create bash -c 'echo Concurrent job $i starting && sleep 2 && echo Job $i UID: \$(id -u) && echo Job $i finished' | grep "ID:" | cut -d' ' -f2)
    CONCURRENT_JOBS+=($JOB_ID)
    echo "Started concurrent job $i: $JOB_ID"
done

echo "✅ Started ${#CONCURRENT_JOBS[@]} concurrent jobs"
echo "✅ Each should run with different UID"
sleep 5
echo ""

# Test 8: Security Boundaries
echo "📋 Test 8: Security Boundaries"
echo "------------------------------"

echo "Testing security isolation..."
SEC_JOB1=$($CLI create bash -c 'echo Security test && ls -la / | head -5' | grep "ID:" | cut -d' ' -f2)
SEC_JOB2=$($CLI create bash -c 'mount 2>&1 | head -3 || echo Mount command limited' | grep "ID:" | cut -d' ' -f2)

echo "✅ Created security test jobs: $SEC_JOB1, $SEC_JOB2"
echo "✅ Should show limited system access"
sleep 3
echo ""

# Test 9: Command Validation (Test the Security)
echo "📋 Test 9: Command Validation Security"
echo "--------------------------------------"

echo "Testing command validation (these should fail)..."
echo "Trying dangerous command with semicolons..."
$CLI create bash "echo test; rm -rf /" 2>&1 | grep -q "dangerous characters" && echo "✅ GOOD: Dangerous semicolon command blocked" || echo "❌ BAD: Dangerous command allowed"

echo "Trying command with pipes (should work)..."
PIPE_JOB=$($CLI create bash -c 'echo test | wc -l' | grep "ID:" | cut -d' ' -f2 2>/dev/null) && echo "✅ GOOD: Safe pipe command allowed: $PIPE_JOB" || echo "Note: Pipe command blocked"

echo ""

# Test Summary
echo "🎯 Test Results Summary"
echo "======================="
echo ""
echo "📊 Job List:"
$CLI list | head -10
echo ""
echo "🔍 What to Verify in Logs:"
echo "• Each job should show UID=0 inside namespace"
echo "• Jobs should see limited processes (not host processes)"
echo "• Jobs should not access each other's files"
echo "• Network connectivity should work"
echo "• Resource limits should be visible in cgroups"
echo "• Dangerous commands should be blocked"
echo ""
echo "📋 Quick verification commands:"
echo "  ./bin/cli stream $JOB1  # Check UID output"
echo "  ./bin/cli stream $PROC_JOB2  # Check process count"
echo "  ./bin/cli stream $FILE_JOB2  # Check file isolation"
echo ""
echo "📊 Monitor live: make live-log"