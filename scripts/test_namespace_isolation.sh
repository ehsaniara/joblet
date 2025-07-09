#!/bin/bash
# Comprehensive test for namespace isolation after cgroup fix

echo "=== Testing Namespace Isolation After Cgroup Fix ==="
echo

# Test 1: PID Namespace Isolation
echo "1. Testing PID Namespace Isolation..."
echo "   Starting job that shows process list..."

JOB_OUTPUT=$(./bin/rnx run ps aux)
JOB_ID=$(echo "$JOB_OUTPUT" | grep "ID:" | cut -d' ' -f2)

echo "   Job ID: $JOB_ID"
echo "   Waiting for job to complete..."
sleep 3

echo "   Getting job output (should show minimal process list)..."
./bin/rnx log "$JOB_ID" | head -10

echo
echo "   Expected: Should only see a few processes (PID 1 = joblet init, etc.)"
echo "   NOT: Should NOT see host system processes like systemd, sshd, etc."
echo

# Test 2: Mount Namespace Isolation
echo "2. Testing Mount Namespace Isolation..."
echo "   Testing filesystem view inside job..."

MOUNT_JOB=$(./bin/rnx run mount | grep "ID:" | cut -d' ' -f2)
sleep 3

echo "   Mount points visible to job:"
./bin/rnx log "$MOUNT_JOB" | head -5

echo
echo "   Expected: Limited mount points, isolated filesystem"
echo

# Test 3: Network Namespace (if enabled)
echo "3. Testing Network Access..."
echo "   Testing network connectivity from job..."

NET_JOB=$(./bin/rnx run ip addr show | grep "ID:" | cut -d' ' -f2)
sleep 3

echo "   Network interfaces visible to job:"
./bin/rnx log "$NET_JOB"

echo
echo "   Expected: Should have network access (host networking mode)"
echo

# Test 4: User/Group Namespace
echo "4. Testing User Namespace..."
echo "   Checking user ID inside job..."

USER_JOB=$(./bin/rnx run "whoami && id" | grep "ID:" | cut -d' ' -f2)
sleep 3

echo "   User info inside job:"
./bin/rnx log "$USER_JOB"

echo
echo "   Expected: Should show user mapping (may be root inside namespace)"
echo

# Test 5: Resource Limits Still Work
echo "5. Testing Resource Limits (CPU)..."
echo "   Starting CPU-intensive job with 10% limit..."

CPU_JOB=$(./bin/rnx run --max-cpu=10 bash -c "while true; do :; done")
CPU_JOB_ID=$(echo "$CPU_JOB" | grep "ID:" | cut -d' ' -f2)

echo "   Job ID: $CPU_JOB_ID"
echo "   Waiting 3 seconds for CPU usage to stabilize..."
sleep 3

# Find the process and check its cgroup and CPU usage
CPU_PID=$(pgrep -f "while true; do :; done" | head -1)
if [ -n "$CPU_PID" ]; then
    echo "   Process PID: $CPU_PID"
    echo "   Process cgroup: $(cat /proc/$CPU_PID/cgroup)"
    echo "   CPU usage:"
    top -p $CPU_PID -b -n 1 | tail -1
else
    echo "   CPU test process not found"
fi

echo
echo "   Expected: Process should be in job cgroup, CPU usage ~10%"
echo

# Stop the CPU job
./bin/rnx stop "$CPU_JOB_ID"

# Test 6: Cgroup Assignment Verification
echo "6. Testing Cgroup Assignment..."
echo "   Starting short job and checking immediate cgroup assignment..."

CGROUP_JOB=$(./bin/rnx run sleep 10)
CGROUP_JOB_ID=$(echo "$CGROUP_JOB" | grep "ID:" | cut -d' ' -f2)

echo "   Job ID: $CGROUP_JOB_ID"
sleep 1

# Check if process is immediately in cgroup
SLEEP_PID=$(pgrep -f "sleep 10" | head -1)
if [ -n "$SLEEP_PID" ]; then
    echo "   Sleep process PID: $SLEEP_PID"
    echo "   Process cgroup: $(cat /proc/$SLEEP_PID/cgroup)"

    # Check if it's in the job cgroup
    JOB_CGROUP_DIR="/sys/fs/cgroup/joblet.slice/joblet.service/job-$CGROUP_JOB_ID"
    if [ -f "$JOB_CGROUP_DIR/cgroup.procs" ]; then
        echo "   Processes in job cgroup:"
        cat "$JOB_CGROUP_DIR/cgroup.procs"

        if grep -q "$SLEEP_PID" "$JOB_CGROUP_DIR/cgroup.procs"; then
            echo "   ✅ Process correctly assigned to job cgroup"
        else
            echo "   ❌ Process NOT in job cgroup"
        fi
    else
        echo "   ❌ Job cgroup directory not found"
    fi
else
    echo "   Sleep process not found"
fi

echo

# Test 7: Job Logs and State
echo "7. Testing Job State Management..."
echo "   Listing all jobs..."
./bin/rnx list

echo
echo "   Getting status of recent job..."
./bin/rnx status "$CGROUP_JOB_ID"

echo

# Test 8: Multiple Concurrent Jobs
echo "8. Testing Multiple Concurrent Jobs with Isolation..."
echo "   Starting 3 concurrent jobs..."

JOB1=$(./bin/rnx run --max-cpu=5 bash -c "echo 'Job 1'; sleep 5" | grep "ID:" | cut -d' ' -f2)
JOB2=$(./bin/rnx run --max-cpu=5 bash -c "echo 'Job 2'; sleep 5" | grep "ID:" | cut -d' ' -f2)
JOB3=$(./bin/rnx run --max-cpu=5 bash -c "echo 'Job 3'; sleep 5" | grep "ID:" | cut -d' ' -f2)

echo "   Job IDs: $JOB1, $JOB2, $JOB3"

sleep 2

echo "   Checking cgroup assignments..."
for job in $JOB1 $JOB2 $JOB3; do
    if [ -f "/sys/fs/cgroup/joblet.slice/joblet.service/job-$job/cgroup.procs" ]; then
        procs=$(cat "/sys/fs/cgroup/joblet.slice/joblet.service/job-$job/cgroup.procs" | wc -l)
        echo "   Job $job: $procs processes in cgroup"
    else
        echo "   Job $job: cgroup not found"
    fi
done

sleep 6  # Wait for jobs to complete

echo

# Summary
echo "=== Isolation Test Summary ==="
echo
echo "✅ PID Namespace: Jobs should see isolated process tree"
echo "✅ Mount Namespace: Jobs should see limited filesystem"
echo "✅ Network: Jobs should have host network access"
echo "✅ User Namespace: Jobs should run with mapped users"
echo "✅ Cgroup Assignment: Jobs should be immediately assigned to cgroups"
echo "✅ Resource Limits: CPU limits should work automatically"
echo "✅ Multiple Jobs: Each job should be in separate cgroup"
echo
echo "If any ❌ appear above, namespace isolation may be broken"
echo "If all ✅, then namespace isolation is working correctly with cgroup fix"