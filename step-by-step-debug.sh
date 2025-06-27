#!/bin/bash
# step-by-step-debug.sh - Debug the fork/exec error systematically

echo "🔍 Step-by-Step Debug Process"
echo "=============================="

REMOTE_HOST="192.168.1.161"
REMOTE_USER="jay"

echo "Step 1: Check worker binary basics"
echo "-----------------------------------"
ssh $REMOTE_USER@$REMOTE_HOST '
echo "Binary existence and permissions:"
ls -la /opt/worker/worker 2>/dev/null || echo "❌ Worker binary not found"

echo "Binary type:"
file /opt/worker/worker 2>/dev/null || echo "❌ Cannot check binary type"

echo "Binary execution test:"
timeout 2s /opt/worker/worker --help 2>&1 | head -3 || echo "Basic execution completed"
'

echo -e "\nStep 2: Check user namespace support"
echo "-------------------------------------"
ssh $REMOTE_USER@$REMOTE_HOST '
echo "Kernel namespace support:"
ls -la /proc/self/ns/ | grep -E "(user|pid|mnt|ipc|uts)" || echo "❌ Some namespaces missing"

echo "User namespace limits:"
cat /proc/sys/user/max_user_namespaces 2>/dev/null || echo "❌ max_user_namespaces not found"

echo "Unprivileged clone:"
cat /proc/sys/kernel/unprivileged_userns_clone 2>/dev/null || echo "❌ unprivileged_userns_clone not found"
'

echo -e "\nStep 3: Test simple namespace creation"
echo "---------------------------------------"
ssh $REMOTE_USER@$REMOTE_HOST '
echo "Testing basic unshare:"
unshare --user --pid --mount --fork echo "✅ Basic namespace creation works" 2>&1 || echo "❌ Basic namespace creation failed"

echo "Testing with different flags:"
unshare --pid --mount --fork echo "✅ PID+Mount namespace works" 2>&1 || echo "❌ PID+Mount namespace failed"
'

echo -e "\nStep 4: Check worker-jobs user"
echo "-------------------------------"
ssh $REMOTE_USER@$REMOTE_HOST '
id worker-jobs 2>/dev/null || echo "❌ worker-jobs user not found"
'

echo -e "\nStep 5: Check recent worker logs"
echo "---------------------------------"
ssh $REMOTE_USER@$REMOTE_HOST '
echo "Recent worker service logs:"
sudo journalctl -u worker.service --no-pager -n 10 | grep -E "(error|Error|ERROR|failed|Failed|invalid|Invalid)"
'

echo -e "\nStep 6: Test job execution manually"
echo "------------------------------------"
ssh $REMOTE_USER@$REMOTE_HOST '
echo "Testing manual job-init mode:"
JOB_ID=debug-test WORKER_MODE=job-init JOB_COMMAND=/bin/echo JOB_ARGS_COUNT=1 JOB_ARG_0=test timeout 5s /opt/worker/worker 2>&1 || echo "Manual test completed"
'

echo -e "\nDiagnostic Summary:"
echo "==================="
echo ""
echo "🔍 The error 'fork/exec /opt/worker/worker: invalid argument' typically means:"
echo "   1. Binary architecture mismatch (unlikely)"
echo "   2. User namespace syscall restrictions"
echo "   3. Invalid SysProcAttr configuration"
echo "   4. Missing user/group mappings"
echo ""
echo "🔧 Quick fixes to try:"
echo "   1. Simplify SysProcAttr (remove user namespace temporarily)"
echo "   2. Check if worker-jobs user exists"
echo "   3. Test without complex namespace flags"
echo "   4. Verify binary is correct architecture"
echo ""
echo "📝 Next steps:"
echo "   1. Use simplified worker implementation (remove complex user namespace)"
echo "   2. Test basic namespaces first (PID, mount only)"
echo "   3. Add user namespace back gradually"
echo "   4. Check systemd service logs for detailed errors"

# Quick fix suggestions
echo -e "\n🚀 Quick Fix Commands:"
echo "======================"
echo ""
echo "1. Check if this is an architecture issue:"
echo "   ssh $REMOTE_USER@$REMOTE_HOST 'file /opt/worker/worker'"
echo ""
echo "2. Test worker binary directly:"
echo "   ssh $REMOTE_USER@$REMOTE_HOST '/opt/worker/worker --help'"
echo ""
echo "3. Check worker service status:"
echo "   ssh $REMOTE_USER@$REMOTE_HOST 'sudo systemctl status worker.service'"
echo ""
echo "4. View detailed logs:"
echo "   ssh $REMOTE_USER@$REMOTE_HOST 'sudo journalctl -u worker.service -f'"
echo ""
echo "5. Test simple job execution:"
echo "   ./bin/cli run echo hello"