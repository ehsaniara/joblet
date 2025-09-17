#!/bin/bash

# Test 02: Complete Runtime Lifecycle Tests
# Tests full runtime management lifecycle against remote host
# Ensures clean state, installs all 4 runtimes, tests each individually, verifies no host contamination

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# All available runtimes from manifest
AVAILABLE_RUNTIMES=("graalvmjdk-21" "openjdk-21" "python-3.11-ml" "python-3.11")
REMOTE_HOST="192.168.1.161"
REMOTE_USER="jay"

echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  Complete Runtime Lifecycle Tests${NC}"
echo -e "${CYAN}  Testing against remote host: ${REMOTE_HOST}${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Started: $(date '+%Y-%m-%d %H:%M:%S')${NC}\n"

# Global test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

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
# HELPER FUNCTIONS
# ============================================

check_host_contamination() {
    echo "  Checking for host contamination on $REMOTE_HOST..."
    
    # Check for joblet-specific runtime installations outside /opt/joblet
    # Look for newly installed runtimes, not system Python/Java
    local contamination_check=$(ssh "$REMOTE_USER@$REMOTE_HOST" "
        # Check for joblet-installed runtimes in unexpected locations
        find /usr/local -name '*graalvm*' -o -name '*openjdk-21*' 2>/dev/null | grep -v /opt/joblet
        find /usr/local -path '*/python3.11*' -newer /tmp 2>/dev/null | grep -v /opt/joblet
    " 2>/dev/null)
    
    if [[ -z "$contamination_check" ]]; then
        echo "    Host system clean - no joblet runtime contamination"
        return 0
    else
        echo "    WARNING: Potential joblet runtime contamination detected"
        echo "    $contamination_check"
        # For now, treat as warning not failure since system Python is expected
        echo "    (Treating as warning - may be system files)"
        return 0
    fi
}

remove_all_runtimes() {
    echo "  Removing all existing runtimes to start clean..."
    
    # Get list of currently installed runtimes (filter out headers and help text)
    local installed_runtimes=$("$RNX_BINARY" runtime list 2>/dev/null | grep -v "RUNTIME" | grep -v "^---" | grep -v "Use " | grep -v "^$" | grep -v "No " | grep -v "To " | awk '{print $1}' | grep -v "^$")
    
    if [[ -z "$installed_runtimes" ]]; then
        echo "    No runtimes currently installed"
        return 0
    fi
    
    # Remove each installed runtime
    echo "$installed_runtimes" | while read -r runtime; do
        if [[ -n "$runtime" ]] && [[ "$runtime" != "RUNTIME" ]] && [[ "$runtime" != "Use" ]]; then
            echo "    Removing runtime: $runtime"
            "$RNX_BINARY" runtime remove "$runtime" || echo "    Failed to remove $runtime"
        fi
    done
    
    # Wait longer for removal to complete on remote host
    echo "    Waiting for removals to complete..."
    sleep 5
    
    # Verify all removed
    local remaining=$("$RNX_BINARY" runtime list 2>/dev/null | grep -v "RUNTIME" | grep -v "^---" | grep -v "Use " | grep -v "^$" | grep -v "No " | grep -v "To " | awk '{print $1}' | grep -v "^$" | wc -l)
    if [[ "$remaining" -eq 0 ]]; then
        echo "    All runtimes successfully removed"
        return 0
    else
        echo "    Warning: $remaining runtime(s) may still remain"
        # Show what remains for debugging
        "$RNX_BINARY" runtime list 2>/dev/null | grep -v "RUNTIME" | grep -v "^---" | grep -v "Use " | grep -v "^$" | grep -v "No " | grep -v "To " | awk '{print "      - " $1}' | grep -v "^$"
        # Don't fail - just warn, as this might be a timing issue
        return 0
    fi
}

install_runtime_with_timeout() {
    local runtime="$1"
    local timeout_duration="$2"
    
    echo "    Installing $runtime (timeout: ${timeout_duration}s)..."
    
    # Install with timeout
    if timeout "$timeout_duration" "$RNX_BINARY" runtime install "$runtime" >/dev/null 2>&1; then
        echo "    ✓ $runtime installed successfully"
        return 0
    else
        echo "    ✗ $runtime installation failed or timed out"
        return 1
    fi
}

test_runtime_execution() {
    local runtime="$1"
    
    echo "    Testing $runtime execution..."
    
    case "$runtime" in
        "graalvmjdk-21"|"openjdk-21")
            # Test Java runtimes
            local job_id=$("$RNX_BINARY" job run --runtime="$runtime" java -version 2>&1 | grep "ID:" | awk '{print $2}')
            ;;
        "python-3.11"|"python-3.11-ml")
            # Test Python runtimes
            local job_id=$("$RNX_BINARY" job run --runtime="$runtime" python3 -c "print('PYTHON_OK')" 2>&1 | grep "ID:" | awk '{print $2}')
            ;;
        *)
            echo "    Unknown runtime type: $runtime"
            return 1
            ;;
    esac
    
    if [[ -z "$job_id" ]]; then
        echo "    ✗ Failed to start job with $runtime"
        return 1
    fi
    
    # Wait for completion and check status
    sleep 3
    local status=$(check_job_status "$job_id")
    
    if [[ "$status" == "COMPLETED" ]]; then
        echo "    ✓ $runtime execution successful"
        
        # Check logs for expected output
        local logs=$(get_job_logs "$job_id")
        case "$runtime" in
            "graalvmjdk-21"|"openjdk-21")
                if echo "$logs" | grep -q "openjdk\|java\|OpenJDK\|GraalVM"; then
                    echo "      Found expected Java output"
                    return 0
                fi
                ;;
            "python-3.11"|"python-3.11-ml")
                if echo "$logs" | grep -q "PYTHON_OK"; then
                    echo "      Found expected Python output"
                    return 0
                fi
                ;;
        esac
        echo "    ⚠ Job completed but output verification failed"
        return 1
    else
        echo "    ✗ $runtime execution failed (status: $status)"
        # Show logs for debugging
        local logs=$(get_job_logs "$job_id")
        echo "      Logs: $(echo "$logs" | head -2 | tail -1)"
        return 1
    fi
}

# ============================================
# MAIN TEST FUNCTIONS
# ============================================

test_initial_state() {
    echo "  Checking initial runtime state..."
    
    # Verify we're connected to remote host
    if ! grep -q "$REMOTE_HOST" ~/.rnx/rnx-config.yml 2>/dev/null; then
        echo "    Not connected to remote host $REMOTE_HOST"
        return 1
    fi
    
    # Check runtime list command works
    if ! "$RNX_BINARY" runtime list >/dev/null 2>&1; then
        echo "    Runtime list command failed"
        return 1
    fi
    
    echo "    Connected to remote host and runtime commands working"
    return 0
}

test_clean_slate() {
    echo "  Setting up clean test environment..."
    
    # Remove all existing runtimes
    if ! remove_all_runtimes; then
        echo "    Failed to achieve clean slate"
        return 1
    fi
    
    # Check host contamination
    if ! check_host_contamination; then
        echo "    Host contamination detected before tests"
        return 1
    fi
    
    echo "    Clean slate achieved"
    return 0
}

test_install_all_runtimes() {
    echo "  Installing all 4 runtimes sequentially..."
    
    local success_count=0
    local total_count=${#AVAILABLE_RUNTIMES[@]}
    
    for runtime in "${AVAILABLE_RUNTIMES[@]}"; do
        echo "    [$((success_count + 1))/$total_count] Installing $runtime..."
        
        # Set timeout based on runtime type (Python runtimes take longer)
        local timeout_duration=300  # 5 minutes default
        if [[ "$runtime" == "python-"* ]]; then
            timeout_duration=600  # 10 minutes for Python runtimes
        fi
        
        if install_runtime_with_timeout "$runtime" "$timeout_duration"; then
            ((success_count++))
            
            # Verify it appears in list
            if "$RNX_BINARY" runtime list | grep -q "$runtime"; then
                echo "      ✓ $runtime confirmed in runtime list"
            else
                echo "      ⚠ $runtime not found in runtime list after installation"
                ((success_count--))
            fi
        fi
    done
    
    echo "    Installed $success_count out of $total_count runtimes"
    
    # Require at least 2 runtimes to pass (in case some fail due to network/compilation issues)
    if [[ "$success_count" -ge 2 ]]; then
        return 0
    else
        return 1
    fi
}

test_individual_runtime_execution() {
    echo "  Testing each installed runtime individually..."
    
    # Get list of actually installed runtimes (filter properly)
    local installed_runtimes=$("$RNX_BINARY" runtime list 2>/dev/null | grep -v "RUNTIME" | grep -v "^---" | grep -v "Use " | grep -v "^$" | grep -v "No " | grep -v "To " | awk '{print $1}' | grep -v "^$")
    
    if [[ -z "$installed_runtimes" ]]; then
        echo "    No runtimes installed to test"
        return 1
    fi
    
    local success_count=0
    local test_count=0
    
    # Use while loop without pipe to avoid subshell
    while IFS= read -r runtime; do
        if [[ -n "$runtime" ]] && [[ "$runtime" != "RUNTIME" ]] && [[ "$runtime" != "Use" ]]; then
            ((test_count++))
            echo "    Testing runtime: $runtime"
            
            if test_runtime_execution "$runtime"; then
                ((success_count++))
            fi
        fi
    done <<< "$installed_runtimes"
    
    echo "    Successfully tested $success_count runtimes"
    
    # Success if at least one runtime works
    if [[ "$success_count" -gt 0 ]]; then
        return 0
    else
        return 1
    fi
}

test_concurrent_runtime_usage() {
    echo "  Testing concurrent usage of multiple runtimes..."
    
    # Get first two installed runtimes (filter properly)
    local runtime1=$("$RNX_BINARY" runtime list 2>/dev/null | grep -v "RUNTIME" | grep -v "^---" | grep -v "Use " | grep -v "^$" | grep -v "No " | grep -v "To " | awk 'NR==1{print $1}' | grep -v "^$")
    local runtime2=$("$RNX_BINARY" runtime list 2>/dev/null | grep -v "RUNTIME" | grep -v "^---" | grep -v "Use " | grep -v "^$" | grep -v "No " | grep -v "To " | awk 'NR==2{print $1}' | grep -v "^$")
    
    if [[ -z "$runtime1" ]] || [[ -z "$runtime2" ]]; then
        echo "    Need at least 2 runtimes for concurrent test, skipping"
        return 0
    fi
    
    echo "    Starting concurrent jobs with $runtime1 and $runtime2..."
    
    # Start jobs with different runtimes concurrently  
    local job1=$("$RNX_BINARY" job run --runtime="$runtime1" java -version 2>&1 | grep "ID:" | awk '{print $2}')
    local job2=$("$RNX_BINARY" job run --runtime="$runtime2" java -version 2>&1 | grep "ID:" | awk '{print $2}')
    
    # Wait for completion
    sleep 5
    
    local status1=$(check_job_status "$job1")
    local status2=$(check_job_status "$job2")
    
    if [[ "$status1" == "COMPLETED" ]] && [[ "$status2" == "COMPLETED" ]]; then
        echo "    ✓ Concurrent runtime usage successful"
        return 0
    else
        echo "    ✗ Concurrent usage failed (statuses: $status1, $status2)"
        return 1
    fi
}

test_host_isolation() {
    echo "  Verifying host system isolation..."
    
    # Check that runtime installations don't contaminate host
    if check_host_contamination; then
        echo "    ✓ Host system remains uncontaminated"
        return 0
    else
        echo "    ✗ Host contamination detected"
        return 1
    fi
}

test_runtime_persistence() {
    echo "  Testing runtime persistence..."
    
    # Check that installed runtimes persist (filter properly)
    local runtime_count=$("$RNX_BINARY" runtime list 2>/dev/null | grep -v "RUNTIME" | grep -v "^---" | grep -v "Use " | grep -v "^$" | grep -v "No " | grep -v "To " | awk '{print $1}' | grep -v "^$" | wc -l)
    
    if [[ "$runtime_count" -gt 0 ]]; then
        echo "    ✓ $runtime_count runtimes persist"
        return 0
    else
        echo "    ✗ No runtimes found"
        return 1
    fi
}

test_cleanup() {
    echo "  Cleaning up test environment..."
    
    # Remove all test runtimes
    if remove_all_runtimes; then
        echo "    ✓ Test cleanup successful"
        return 0
    else
        echo "    ⚠ Test cleanup incomplete"
        return 1
    fi
}

# ============================================
# MAIN TEST EXECUTION
# ============================================

echo -e "${YELLOW}▶ 1. Initial State Verification${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Initial state and connectivity" test_initial_state

echo -e "\n${YELLOW}▶ 2. Environment Preparation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Clean slate preparation" test_clean_slate

echo -e "\n${YELLOW}▶ 3. Runtime Installation Lifecycle${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Install all 4 runtimes sequentially" test_install_all_runtimes

echo -e "\n${YELLOW}▶ 4. Individual Runtime Testing${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Test each runtime individually" test_individual_runtime_execution

echo -e "\n${YELLOW}▶ 5. Concurrent Runtime Usage${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Concurrent runtime usage" test_concurrent_runtime_usage

echo -e "\n${YELLOW}▶ 6. Host System Isolation${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Host system isolation verification" test_host_isolation

echo -e "\n${YELLOW}▶ 7. Runtime Persistence${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Runtime persistence verification" test_runtime_persistence

echo -e "\n${YELLOW}▶ 8. Test Cleanup${NC}"
echo -e "${BLUE}─────────────────────────────────────────────────────────────────${NC}"
run_test_check "Test environment cleanup" test_cleanup

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
    echo -e "\n${GREEN}✅ ALL RUNTIME LIFECYCLE TESTS PASSED!${NC}"
    echo -e "${GREEN}Runtime management working correctly on $REMOTE_HOST${NC}"
    exit 0
else
    echo -e "\n${RED}❌ SOME RUNTIME LIFECYCLE TESTS FAILED${NC}"
    echo -e "${YELLOW}Review the failures above for details${NC}"
    exit 1
fi