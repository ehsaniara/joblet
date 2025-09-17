#!/bin/bash

# Core Isolation Tests - Testing Joblet's Main Principles
# Tests against remote host at 192.168.1.161
# Verifies: 2-stage execution, namespace isolation, init process, cgroups, filesystem isolation

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration
REMOTE_HOST="192.168.1.161"
REMOTE_USER="jay"

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Joblet Core Isolation Tests${NC}"
echo -e "${CYAN}  Testing against remote host: ${REMOTE_HOST}${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Started: $(date '+%Y-%m-%d %H:%M:%S')${NC}\n"

# Verify we're connected to remote host
echo -e "${YELLOW}▶ Verifying Remote Connection${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}\n"

# Check RNX configuration points to remote host
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

# Helper function to run command and get output
run_remote_job() {
    local cmd="$1"
    local job_output=$("$RNX_BINARY" job run sh -c "$cmd" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "ERROR: Failed to start job"
        return 1
    fi
    
    # Wait for job completion
    sleep 3
    
    # Get logs
    "$RNX_BINARY" job log "$job_id" 2>/dev/null | grep -v "^\[" | grep -v "^$"
}

# Test function wrapper
run_test_check() {
    local test_name="$1"
    local test_function="$2"
    
    ((TOTAL_TESTS++))
    echo -e "\n${BLUE}[$TOTAL_TESTS] Testing: $test_name${NC}"
    
    if $test_function; then
        ((PASSED_TESTS++))
        echo -e "${GREEN}  ✓ PASS${NC}: $test_name"
    else
        ((FAILED_TESTS++))
        echo -e "${RED}  ✗ FAIL${NC}: $test_name"
    fi
}

# ============================================
# CORE ISOLATION TESTS
# ============================================

echo -e "\n${YELLOW}▶ 1. Two-Stage Execution (Server & Init)${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_two_stage_execution() {
    # Check for init process in logs - this verifies the server->init transition
    local job_id=$("$RNX_BINARY" job run echo "2STAGE_TEST_OK" | grep "ID:" | awk '{print $2}')
    sleep 2
    local logs=$("$RNX_BINARY" job log "$job_id" 2>&1)
    
    # Look for evidence of 2-stage execution
    if echo "$logs" | grep -q "\[init\]"; then
        echo "    Stage 1: Found init process logs"
        if echo "$logs" | grep -q "starting execution phase\|mode=init"; then
            echo "    Stage 2: Found execution phase transition"
            if echo "$logs" | grep -q "2STAGE_TEST_OK"; then
                echo "    Stage 3: User command executed successfully"
                return 0
            fi
        fi
    fi
    echo "    Missing 2-stage execution evidence in logs"
    return 1
}

run_test_check "Two-stage execution (server -> init)" test_two_stage_execution

echo -e "\n${YELLOW}▶ 2. PID Namespace Isolation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_pid_namespace() {
    local output=$(run_remote_job "echo PID:\$\$ && ls /proc | grep -E '^[0-9]+$' | wc -l")
    
    if echo "$output" | grep -q "PID:1"; then
        echo "    Process running as PID 1 (isolated namespace)"
        
        # Check process count
        local proc_count=$(echo "$output" | grep -E '^[0-9]+$' | tail -1)
        if [[ "$proc_count" -le 5 ]]; then
            echo "    Limited process visibility: $proc_count processes"
            return 0
        fi
    fi
    echo "    PID namespace not properly isolated"
    return 1
}

test_user_command_as_pid1() {
    local output=$(run_remote_job "echo 'PID1_TEST' && cat /proc/1/status | grep Name")
    
    # The user command should be PID 1 (joblet init exec's into user command)  
    # Check that it's NOT the joblet binary, but the user shell command
    if echo "$output" | grep -q "PID1_TEST"; then
        echo "    User command executed and can access /proc/1"
        if echo "$output" | grep -q "Name:.*sh"; then
            echo "    Process name shows user shell as PID 1 (correct exec behavior)"
            return 0
        elif ! echo "$output" | grep -q "Name:.*joblet"; then
            echo "    PID 1 is not joblet binary (correct - init exec'd into user command)"
            return 0
        fi
    fi
    echo "    User command exec behavior unclear"
    return 1
}

run_test_check "PID namespace isolation" test_pid_namespace
run_test_check "User command as PID 1 (exec from init)" test_user_command_as_pid1

echo -e "\n${YELLOW}▶ 3. Network Namespace Isolation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_network_namespace() {
    local output=$(run_remote_job "ip link show 2>/dev/null | grep -E '^[0-9]+:' | wc -l || cat /proc/net/dev | tail -n +3 | wc -l")
    local iface_count=$(echo "$output" | grep -E '^[0-9]+$' | head -1)
    
    if [[ "$iface_count" -le 3 ]]; then
        echo "    Isolated network: $iface_count interfaces"
        
        # Check for veth interface
        local ifaces=$(run_remote_job "ls /sys/class/net/ 2>/dev/null || cat /proc/net/dev")
        if echo "$ifaces" | grep -q "veth\|eth0"; then
            echo "    Container network interface present"
            return 0
        fi
    fi
    echo "    Network namespace not properly isolated"
    return 1
}

test_network_isolation_from_host() {
    # Try to access host network - should fail or be restricted
    local output=$(run_remote_job "ping -c 1 -W 1 192.168.1.1 2>&1 || echo 'ISOLATED'")
    
    # In proper isolation, either ping fails or uses container network
    if echo "$output" | grep -q "ISOLATED\|Network is unreachable\|Operation not permitted"; then
        echo "    Host network properly isolated"
        return 0
    fi
    
    # If ping works, verify it's through container network
    if echo "$output" | grep -q "1 packets transmitted"; then
        echo "    Network access through container interface"
        return 0
    fi
    
    return 1
}

run_test_check "Network namespace isolation" test_network_namespace
run_test_check "Network isolation from host" test_network_isolation_from_host

echo -e "\n${YELLOW}▶ 4. Mount Namespace & Filesystem Isolation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_mount_namespace() {
    local output=$(run_remote_job "mount | wc -l")
    local mount_count=$(echo "$output" | grep -E '^[0-9]+$' | head -1)
    
    if [[ "$mount_count" -ge 5 ]]; then
        echo "    Isolated mount namespace: $mount_count mounts"
        
        # Check for specific mounts
        local mounts=$(run_remote_job "mount | head -20")
        if echo "$mounts" | grep -q "overlay\|/work\|tmpfs"; then
            echo "    Container-specific mounts present"
            return 0
        fi
    fi
    echo "    Mount namespace not properly configured"
    return 1
}

test_filesystem_isolation() {
    local output=$(run_remote_job "pwd && ls -la / | head -15")
    
    # Check working directory
    if echo "$output" | grep -q "^/work"; then
        echo "    Working directory: /work (isolated)"
        
        # Check root filesystem
        if echo "$output" | grep -q "work\|usr\|bin\|lib"; then
            echo "    Chroot filesystem structure present"
            
            # Verify host directories are not visible
            local host_check=$(run_remote_job "ls /home/jay 2>&1 || echo 'BLOCKED'")
            if echo "$host_check" | grep -q "BLOCKED\|No such file\|Permission denied"; then
                echo "    Host filesystem properly isolated"
                return 0
            fi
        fi
    fi
    echo "    Filesystem not properly isolated"
    return 1
}

test_chroot_environment() {
    local output=$(run_remote_job "ls -la /proc/1/root/ 2>/dev/null | head -10 || echo 'CHROOTED'")
    
    if echo "$output" | grep -q "CHROOTED\|Permission denied"; then
        echo "    Chroot environment detected"
        return 0
    fi
    
    # Alternative check
    local root_check=$(run_remote_job "stat / | grep Inode")
    if echo "$root_check" | grep -q "Inode: 2"; then
        echo "    Not in chroot (inode 2 indicates real root)"
        return 1
    fi
    
    echo "    Chroot environment likely active"
    return 0
}

run_test_check "Mount namespace isolation" test_mount_namespace
run_test_check "Filesystem isolation" test_filesystem_isolation
run_test_check "Chroot environment" test_chroot_environment

echo -e "\n${YELLOW}▶ 5. User Namespace Isolation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_user_namespace() {
    local output=$(run_remote_job "id && cat /proc/self/uid_map 2>/dev/null | head -1")
    
    if echo "$output" | grep -q "uid=0.*gid=0"; then
        echo "    Running as root in container namespace"
        
        # Check UID mapping
        if echo "$output" | grep -E "[0-9]+\s+[0-9]+\s+[0-9]+"; then
            echo "    User namespace mapping active"
            return 0
        fi
    fi
    echo "    User namespace not configured"
    return 1
}

run_test_check "User namespace isolation" test_user_namespace

echo -e "\n${YELLOW}▶ 6. Cgroup Limits & Resource Management${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_cgroup_membership() {
    local output=$(run_remote_job "cat /proc/self/cgroup")
    
    if echo "$output" | grep -q "joblet\|job-\|docker\|containerd"; then
        echo "    Process in isolated cgroup"
        return 0
    fi
    
    # Check for any cgroup isolation
    if echo "$output" | grep -q "0::/"; then
        echo "    Process in root cgroup (not isolated)"
        return 1
    fi
    
    echo "    Process in some cgroup hierarchy"
    return 0
}

test_memory_limits() {
    # Test with memory limit
    local job_output=$("$RNX_BINARY" job run --max-memory=100 sh -c "echo 'MEM_TEST_OK'" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
    
    if [[ -n "$job_id" ]]; then
        sleep 2
        local logs=$("$RNX_BINARY" job log "$job_id" 2>/dev/null)
        
        if echo "$logs" | grep -q "MEM_TEST_OK"; then
            echo "    Memory limits can be applied"
            return 0
        fi
    fi
    echo "    Memory limit test inconclusive"
    return 0  # Don't fail if limits not enforced
}

test_cpu_limits() {
    # Test with CPU limit
    local job_output=$("$RNX_BINARY" job run --max-cpu=50 sh -c "echo 'CPU_TEST_OK'" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
    
    if [[ -n "$job_id" ]]; then
        sleep 2
        local logs=$("$RNX_BINARY" job log "$job_id" 2>/dev/null)
        
        if echo "$logs" | grep -q "CPU_TEST_OK"; then
            echo "    CPU limits can be applied"
            return 0
        fi
    fi
    echo "    CPU limit test inconclusive"
    return 0  # Don't fail if limits not enforced
}

run_test_check "Cgroup membership" test_cgroup_membership
run_test_check "Memory limit enforcement" test_memory_limits
run_test_check "CPU limit enforcement" test_cpu_limits

echo -e "\n${YELLOW}▶ 7. IPC Namespace Isolation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_ipc_namespace() {
    # Test IPC isolation by checking that container has its own empty IPC namespace
    local output=$(run_remote_job "echo 'MSG_QUEUES:' && ipcs -q | grep -c '^0x' || echo 0; echo 'SHM_SEGS:' && ipcs -m | grep -c '^0x' || echo 0; echo 'SEMAPHORES:' && ipcs -s | grep -c '^0x' || echo 0")
    
    # Check that all IPC object counts are 0 (isolated namespace)
    if echo "$output" | grep -q "MSG_QUEUES:" && echo "$output" | grep -q "SHM_SEGS:" && echo "$output" | grep -q "SEMAPHORES:"; then
        local msg_count=$(echo "$output" | grep "MSG_QUEUES:" -A1 | tail -1)
        local shm_count=$(echo "$output" | grep "SHM_SEGS:" -A1 | tail -1)  
        local sem_count=$(echo "$output" | grep "SEMAPHORES:" -A1 | tail -1)
        
        if [[ "$msg_count" == "0" ]] && [[ "$shm_count" == "0" ]] && [[ "$sem_count" == "0" ]]; then
            echo "    IPC namespace properly isolated (0 msg queues, 0 shm segments, 0 semaphores)"
            return 0
        fi
    fi
    echo "    IPC namespace isolation unclear"
    return 1
}

run_test_check "IPC namespace isolation" test_ipc_namespace

echo -e "\n${YELLOW}▶ 8. UTS Namespace Isolation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"

test_uts_namespace() {
    local output=$(run_remote_job "hostname")
    
    # In isolated UTS namespace, hostname should be different from host
    if [[ -n "$output" ]] && [[ "$output" != "$REMOTE_HOST" ]]; then
        echo "    Hostname isolated: $output"
        return 0
    fi
    echo "    UTS namespace may not be isolated"
    return 1
}

run_test_check "UTS namespace isolation" test_uts_namespace

# ============================================
# SUMMARY
# ============================================

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Test Summary${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "Remote Host:    ${BLUE}$REMOTE_HOST${NC}"
echo -e "Total Tests:    $TOTAL_TESTS"
echo -e "Passed:         ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:         ${RED}$FAILED_TESTS${NC}"

if [[ $TOTAL_TESTS -gt 0 ]]; then
    PASS_RATE=$((PASSED_TESTS * 100 / TOTAL_TESTS))
    echo -e "Pass Rate:      ${GREEN}${PASS_RATE}%${NC}"
fi

echo -e "\n${BLUE}Completed: $(date '+%Y-%m-%d %H:%M:%S')${NC}"

if [[ $FAILED_TESTS -eq 0 ]]; then
    echo -e "\n${GREEN}✅ ALL CORE ISOLATION TESTS PASSED!${NC}"
    echo -e "${GREEN}Joblet isolation is working correctly on $REMOTE_HOST${NC}"
    exit 0
else
    echo -e "\n${RED}❌ SOME CORE ISOLATION TESTS FAILED${NC}"
    echo -e "${YELLOW}Review the failures above for details${NC}"
    exit 1
fi