#!/bin/bash

# Unified Test Framework for Joblet E2E Tests
# Provides consistent functions and formatting for all tests

# Colors
export RED='\033[0;31m'
export GREEN='\033[0;32m'
export YELLOW='\033[1;33m'
export BLUE='\033[0;34m'
export CYAN='\033[0;36m'
export NC='\033[0m' # No Color

# Test counters
export TOTAL_TESTS=0
export PASSED_TESTS=0
export FAILED_TESTS=0
export SKIPPED_TESTS=0

# Paths for developer
export JOBLET_ROOT="${JOBLET_ROOT:-/home/jay/joblet/joblet}"
export RNX_BINARY="${RNX_BINARY:-$JOBLET_ROOT/bin/rnx}"
export TESTS_DIR="$JOBLET_ROOT/tests/e2e"

# Runtime configuration
export DEFAULT_RUNTIME="openjdk-21"
export RUNTIME_TIMEOUT=60
export JOB_TIMEOUT=15

# Remote host configuration (for e2e tests that need remote joblet)
# Set these environment variables to test against a remote joblet instance
# If not set, tests will use local joblet instance without SSH
export JOBLET_TEST_HOST="${JOBLET_TEST_HOST:-}"
export JOBLET_TEST_USER="${JOBLET_TEST_USER:-$USER}"
export JOBLET_TEST_USE_SSH="${JOBLET_TEST_USE_SSH:-false}"

# Auto-detect SSH requirement based on host
if [[ -n "$JOBLET_TEST_HOST" && "$JOBLET_TEST_HOST" != "localhost" && "$JOBLET_TEST_HOST" != "127.0.0.1" ]]; then
    JOBLET_TEST_USE_SSH="true"
fi

# ============================================
# Core Test Functions
# ============================================

# Initialize test suite
test_suite_init() {
    local suite_name="$1"
    TOTAL_TESTS=0
    PASSED_TESTS=0
    FAILED_TESTS=0
    SKIPPED_TESTS=0
    
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  $suite_name${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Started: $(date '+%Y-%m-%d %H:%M:%S')${NC}\n"
}

# Start a test section
test_section() {
    local section_name="$1"
    echo -e "\n${YELLOW}▶ $section_name${NC}"
    echo -e "${BLUE}$(printf '─%.0s' {1..65})${NC}"
}

# Run a single test
run_test() {
    local test_name="$1"
    local test_function="$2"
    
    ((TOTAL_TESTS++))
    echo -e "\n${BLUE}[$TOTAL_TESTS] Testing: $test_name${NC}"
    
    # Run the test function and capture result
    if $test_function; then
        ((PASSED_TESTS++))
        echo -e "${GREEN}  ✓ PASS${NC}: $test_name"
        return 0
    else
        ((FAILED_TESTS++))
        echo -e "${RED}  ✗ FAIL${NC}: $test_name"
        return 1
    fi
}

# Skip a test
skip_test() {
    local test_name="$1"
    local reason="$2"
    
    ((TOTAL_TESTS++))
    ((SKIPPED_TESTS++))
    echo -e "\n${BLUE}[$TOTAL_TESTS] Testing: $test_name${NC}"
    echo -e "${YELLOW}  ⊘ SKIP${NC}: $reason"
}

# Assert functions
assert_equals() {
    local actual="$1"
    local expected="$2"
    local message="${3:-Values should be equal}"
    
    if [[ "$actual" == "$expected" ]]; then
        return 0
    else
        echo -e "    ${RED}Expected: '$expected', Got: '$actual'${NC}"
        return 1
    fi
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="${3:-Should contain substring}"
    
    if echo "$haystack" | grep -q "$needle"; then
        return 0
    else
        echo -e "    ${RED}Output does not contain: '$needle'${NC}"
        return 1
    fi
}

assert_numeric_le() {
    local value="$1"
    local max="$2"
    local message="${3:-Value should be less than or equal to $max}"
    
    if [[ "$value" =~ ^[0-9]+$ ]] && [[ $value -le $max ]]; then
        return 0
    else
        echo -e "    ${RED}Value '$value' is not numeric or exceeds $max${NC}"
        return 1
    fi
}

assert_file_exists() {
    local file="$1"
    local message="${2:-File should exist}"
    
    if [[ -f "$file" ]]; then
        return 0
    else
        echo -e "    ${RED}File not found: '$file'${NC}"
        return 1
    fi
}

# ============================================
# Job Execution Helpers
# ============================================

# Run a job and get its ID
run_job() {
    local command="$1"
    local runtime="${2:-}"
    
    local job_output
    if [[ -n "$runtime" ]]; then
        job_output=$("$RNX_BINARY" job run --runtime="$runtime" "$command" 2>&1)
    else
        job_output=$("$RNX_BINARY" job run "$command" 2>&1)
    fi
    
    echo "$job_output" | grep "ID:" | awk '{print $2}'
}

# Run a job with Python runtime
run_python_job() {
    local python_code="$1"
    # Run python with separate arguments instead of one quoted command
    local job_output
    if [[ -n "$DEFAULT_RUNTIME" ]]; then
        job_output=$("$RNX_BINARY" job run --runtime="$DEFAULT_RUNTIME" python3 -c "$python_code" 2>&1)
    else
        job_output=$("$RNX_BINARY" job run python3 -c "$python_code" 2>&1)
    fi
    echo "$job_output" | grep "ID:" | awk '{print $2}'
}

# Wait for job and get logs
get_job_logs() {
    local job_id="$1"
    local wait_time="${2:-3}"
    local max_attempts=15
    
    # Wait for job to complete with exponential backoff
    for i in $(seq 1 $max_attempts); do
        local status=$(check_job_status "$job_id")
        if [[ "$status" == "COMPLETED" || "$status" == "FAILED" ]]; then
            break
        fi
        # Start with shorter waits, increase gradually
        if [[ $i -le 5 ]]; then
            sleep 0.5
        elif [[ $i -le 10 ]]; then
            sleep 1
        else
            sleep 2
        fi
    done
    
    # Brief wait to ensure logs are fully written
    sleep 0.2
    "$RNX_BINARY" job log "$job_id" 2>/dev/null
}

# Get clean output (no debug/info logs)
get_clean_output() {
    local logs="$1"
    # Extract only the actual job output by filtering out all log lines
    local clean_lines
    clean_lines=$(echo "$logs" | grep -v '^\[20[0-9][0-9]-' | grep -v '\[DEBUG\]' | grep -v '\[INFO\]' | grep -v '\[ERROR\]' | grep -v '\[WARN\]' | grep -v '^[[:space:]]*$')
    echo "$clean_lines" | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//'
}

# Check job status
check_job_status() {
    local job_id="$1"
    local status_output=$("$RNX_BINARY" job status "$job_id" 2>/dev/null)
    if [[ -z "$status_output" ]]; then
        echo "UNKNOWN"
        return
    fi
    echo "$status_output" | grep "Status:" | sed 's/\x1b\[[0-9;]*m//g' | awk '{print $2}'
}

# ============================================
# Runtime Management Helpers
# ============================================

# Check if runtime exists
runtime_exists() {
    local runtime="$1"
    "$RNX_BINARY" runtime list 2>/dev/null | grep -q "$runtime"
}

# Install runtime if not exists
ensure_runtime() {
    local runtime="$1"
    
    if runtime_exists "$runtime"; then
        return 0
    else
        echo -e "  ${YELLOW}Installing runtime: $runtime${NC}"
        # Run from project root to find runtime sources
        timeout "$RUNTIME_TIMEOUT" bash -c "cd '$JOBLET_ROOT' && '$RNX_BINARY' runtime install '$runtime'" >/dev/null 2>&1
    fi
}

# Get runtime info
get_runtime_info() {
    local runtime="$1"
    "$RNX_BINARY" runtime info "$runtime" 2>/dev/null
}

# ============================================
# Test Summary Functions
# ============================================

# Print test summary
test_suite_summary() {
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Test Summary${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    echo -e "Total Tests:    $TOTAL_TESTS"
    echo -e "Passed:         ${GREEN}$PASSED_TESTS${NC}"
    echo -e "Failed:         ${RED}$FAILED_TESTS${NC}"
    echo -e "Skipped:        ${YELLOW}$SKIPPED_TESTS${NC}"
    
    if [[ $TOTAL_TESTS -gt 0 ]]; then
        local pass_rate=$((PASSED_TESTS * 100 / TOTAL_TESTS))
        echo -e "Pass Rate:      ${GREEN}${pass_rate}%${NC}"
    fi
    
    echo -e "\n${BLUE}Completed: $(date '+%Y-%m-%d %H:%M:%S')${NC}"
    
    if [[ $FAILED_TESTS -eq 0 && $TOTAL_TESTS -gt 0 ]]; then
        echo -e "\n${GREEN}✅ ALL TESTS PASSED!${NC}"
        return 0
    elif [[ $FAILED_TESTS -gt 0 ]]; then
        echo -e "\n${RED}❌ SOME TESTS FAILED${NC}"
        return 1
    else
        echo -e "\n${YELLOW}⚠ NO TESTS EXECUTED${NC}"
        return 2
    fi
}

# ============================================
# Utility Functions
# ============================================

# Check prerequisites
check_prerequisites() {
    local prereqs_met=true
    
    # Check RNX binary
    if [[ ! -x "$RNX_BINARY" ]]; then
        echo -e "${RED}Error: RNX binary not found or not executable: $RNX_BINARY${NC}"
        prereqs_met=false
    fi
    
    # Check joblet service
    if ! "$RNX_BINARY" runtime list &>/dev/null; then
        echo -e "${RED}Error: Cannot connect to joblet service${NC}"
        prereqs_met=false
    fi
    
    if [[ "$prereqs_met" == "false" ]]; then
        return 1
    fi
    
    return 0
}

# Cleanup function
cleanup_test_artifacts() {
    # Remove temporary test files
    rm -f /tmp/test_*.txt /tmp/test_*.yaml /tmp/test_*.log 2>/dev/null

    # Clean up test jobs if needed
    # Note: Add job cleanup logic here if needed
}

# ============================================
# Remote Execution Helpers
# ============================================

# Execute command on remote host via SSH or locally
# Usage: run_remote_command "command" [timeout]
run_remote_command() {
    local command="$1"
    local timeout="${2:-10}"

    if [[ "$JOBLET_TEST_USE_SSH" == "true" ]]; then
        # Remote execution via SSH
        timeout "$timeout" ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no \
            "$JOBLET_TEST_USER@$JOBLET_TEST_HOST" "$command" 2>/dev/null || echo "SSH_FAILED"
    else
        # Local execution
        timeout "$timeout" bash -c "$command" 2>/dev/null || true
    fi
}

# Execute RNX command (remote or local)
# Usage: run_rnx_command "job run echo test"
run_rnx_command() {
    local rnx_args="$1"

    if [[ "$JOBLET_TEST_USE_SSH" == "true" ]]; then
        # Remote RNX via SSH
        ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no \
            "$JOBLET_TEST_USER@$JOBLET_TEST_HOST" "rnx $rnx_args" 2>&1 || true
    else
        # Local RNX
        "$RNX_BINARY" $rnx_args 2>&1 || true
    fi
}

# Get test host display name for logging
get_test_host_display() {
    if [[ "$JOBLET_TEST_USE_SSH" == "true" ]]; then
        echo "$JOBLET_TEST_HOST"
    else
        echo "localhost"
    fi
}

# Export all functions
export -f test_suite_init test_section run_test skip_test
export -f assert_equals assert_contains assert_numeric_le assert_file_exists
export -f run_job run_python_job get_job_logs get_clean_output check_job_status
export -f runtime_exists ensure_runtime get_runtime_info
export -f test_suite_summary check_prerequisites cleanup_test_artifacts
export -f run_remote_command run_rnx_command get_test_host_display