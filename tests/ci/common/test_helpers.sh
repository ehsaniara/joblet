#!/bin/bash

# Common test helper functions for CI environment
# Provides utilities and setup for CI-compatible E2E tests

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration - use environment variables or defaults
export TEST_DIR="${TEST_DIR:-$(dirname "$(dirname "$0")")}"
export JOBLET_TEST_MODE="${JOBLET_TEST_MODE:-1}"
export JOBLET_CONFIG="${JOBLET_CONFIG:-/tmp/joblet/config/joblet-config.yml}"
export RNX_CONFIG="${RNX_CONFIG:-/tmp/joblet/config/rnx-config.yml}"

# Function to find RNX binary
find_rnx_binary() {
    if command -v rnx >/dev/null 2>&1; then
        echo "rnx"
    elif [[ -x "$(dirname "$(dirname "$(dirname "$0")")")/rnx" ]]; then
        echo "$(dirname "$(dirname "$(dirname "$0")")")/rnx"
    else
        echo "rnx"  # fallback
    fi
}

# Set RNX binary path
export RNX_BINARY="${RNX_BINARY:-$(find_rnx_binary)}"

# Test results tracking
TEST_COUNT=0
PASSED_COUNT=0
FAILED_COUNT=0

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to run a test with error handling
run_test() {
    local test_name="$1"
    local test_function="$2"
    
    TEST_COUNT=$((TEST_COUNT + 1))
    
    echo -e "\n${YELLOW}Running test: $test_name${NC}"
    
    if $test_function; then
        print_success "$test_name passed"
        PASSED_COUNT=$((PASSED_COUNT + 1))
        return 0
    else
        print_error "$test_name failed"
        FAILED_COUNT=$((FAILED_COUNT + 1))
        return 1
    fi
}

# Function to wait for joblet server to be ready
wait_for_server() {
    local max_attempts="${1:-30}"
    local attempt=0
    
    print_info "Waiting for joblet server to be ready..."
    
    while ! "$RNX_BINARY" --config "$RNX_CONFIG" list >/dev/null 2>&1; do
        attempt=$((attempt + 1))
        if [[ $attempt -ge $max_attempts ]]; then
            print_error "Joblet server failed to start within $max_attempts seconds"
            return 1
        fi
        sleep 1
    done
    
    print_success "Joblet server is ready"
    return 0
}

# Function to check prerequisites
check_prerequisites() {
    local missing_deps=()
    
    # Check for required commands - check for rnx binary in project root first
    if ! command -v rnx >/dev/null 2>&1 && ! [[ -x "$(dirname "$(dirname "$(dirname "$0")")")/rnx" ]]; then
        missing_deps+=("rnx")
    fi
    command -v jq >/dev/null 2>&1 || missing_deps+=("jq")
    
    if [[ ${#missing_deps[@]} -ne 0 ]]; then
        print_error "Missing dependencies: ${missing_deps[*]}"
        print_info "Please install missing dependencies before running tests"
        return 1
    fi
    
    # Check for config files
    if [[ ! -f "$RNX_CONFIG" ]]; then
        print_warning "RNX config file not found at: $RNX_CONFIG"
        print_info "Using default configuration or environment variables"
    fi
    
    return 0
}

# Function to generate unique test ID
generate_test_id() {
    echo "test_$(date +%s)_$$"
}

# Function to cleanup test jobs
cleanup_test_jobs() {
    local pattern="${1:-test_}"
    
    print_info "Cleaning up test jobs matching pattern: $pattern"
    
    # Get list of jobs and stop any that match our test pattern
    local job_list
    if job_list=$("$RNX_BINARY" --config "$RNX_CONFIG" list 2>/dev/null); then
        local job_ids
        job_ids=$(echo "$job_list" | jq -r '.[] | select(.command | contains("'$pattern'")) | .id' 2>/dev/null || echo "")
        
        for job_id in $job_ids; do
            if [[ -n "$job_id" && "$job_id" != "null" ]]; then
                print_info "Stopping test job: $job_id"
                "$RNX_BINARY" --config "$RNX_CONFIG" stop "$job_id" >/dev/null 2>&1 || true
            fi
        done
    fi
}

# Function to validate JSON output
validate_json() {
    local json_string="$1"
    local description="${2:-JSON}"
    
    if ! echo "$json_string" | jq . >/dev/null 2>&1; then
        print_error "$description is not valid JSON: $json_string"
        return 1
    fi
    
    return 0
}

# Function to wait for job completion
wait_for_job_completion() {
    local job_id="$1"
    local timeout="${2:-30}"
    local check_interval="${3:-1}"
    
    local elapsed=0
    
    while [[ $elapsed -lt $timeout ]]; do
        local status
        if status=$("$RNX_BINARY" --config "$RNX_CONFIG" status "$job_id" 2>/dev/null | jq -r '.status' 2>/dev/null); then
            if [[ "$status" == "COMPLETED" || "$status" == "STOPPED" || "$status" == "FAILED" ]]; then
                return 0
            fi
        fi
        
        sleep "$check_interval"
        elapsed=$((elapsed + check_interval))
    done
    
    print_warning "Job $job_id did not complete within $timeout seconds"
    return 1
}

# Function to get job logs with CI environment error handling
get_job_logs_safe() {
    local job_id="$1"
    local description="${2:-job}"
    
    local job_logs
    job_logs=$("$RNX_BINARY" --config "$RNX_CONFIG" log "$job_id" 2>&1)
    
    # Check for log streaming errors (common in CI)
    if [[ "$job_logs" == *"buffer is closed"* ]] || [[ "$job_logs" == *"failed to stream logs"* ]]; then
        print_warning "Log streaming failed for $description - likely CI environment limitation"
        print_info "This is expected in containerized CI environments"
        echo "CI_LOG_STREAMING_ERROR"
        return 1
    fi
    
    # Clean logs output
    job_logs=$(echo "$job_logs" | grep -v "^\[" | grep -v "^$" | grep -v "Usage:" | grep -v "Flags:" | grep -v "Global Flags:" | grep -v "help for log")
    
    if [[ -z "$job_logs" ]]; then
        print_warning "No job output received for $description - likely CI environment limitation"
        print_info "This is expected in containerized CI environments"
        echo "CI_NO_OUTPUT"
        return 1
    fi
    
    echo "$job_logs"
    return 0
}

# Function to create temporary test file
create_temp_test_file() {
    local content="$1"
    local suffix="${2:-.txt}"
    
    local temp_file
    temp_file=$(mktemp "/tmp/joblet_test_XXXXXX$suffix")
    echo "$content" > "$temp_file"
    echo "$temp_file"
}

# Function to cleanup temporary files
cleanup_temp_files() {
    rm -f /tmp/joblet_test_* 2>/dev/null || true
}

# Function to print test summary (legacy - for individual test functions)
print_test_summary() {
    echo -e "\n${YELLOW}================================${NC}"
    echo -e "${YELLOW}Test Summary${NC}"
    echo -e "${YELLOW}================================${NC}"
    echo "Total Tests: $TEST_COUNT"
    echo -e "Passed: ${GREEN}$PASSED_COUNT${NC}"
    echo -e "Failed: ${RED}$FAILED_COUNT${NC}"
    
    if [[ $FAILED_COUNT -eq 0 ]]; then
        echo -e "\n${GREEN}All tests passed!${NC}"
        return 0
    else
        echo -e "\n${RED}Some tests failed!${NC}"
        return 1
    fi
}

# Function to print test suite summary
print_suite_summary() {
    echo -e "\n${YELLOW}================================${NC}"
    echo -e "${YELLOW}Test Summary${NC}"
    echo -e "${YELLOW}================================${NC}"
    echo "Total Test Suites: $SUITE_COUNT"
    echo -e "Passed: ${GREEN}$SUITE_PASSED${NC}"
    echo -e "Failed: ${RED}$SUITE_FAILED${NC}"
    
    if [[ $SUITE_FAILED -eq 0 ]]; then
        echo -e "\n${GREEN}All tests passed!${NC}"
        return 0
    else
        echo -e "\n${RED}Some tests failed!${NC}"
        return 1
    fi
}

# Function to setup test environment
setup_test_environment() {
    print_info "Setting up CI test environment..."
    
    # Check prerequisites
    if ! check_prerequisites; then
        return 1
    fi
    
    # Wait for server to be ready
    if ! wait_for_server; then
        return 1
    fi
    
    # Cleanup any existing test jobs
    cleanup_test_jobs
    
    print_success "CI test environment ready"
    return 0
}

# Function to cleanup test environment
cleanup_test_environment() {
    print_info "Cleaning up CI test environment..."
    
    # Cleanup test jobs
    cleanup_test_jobs
    
    # Cleanup temporary files
    cleanup_temp_files
    
    print_success "CI test environment cleaned up"
}

# Trap to ensure cleanup on exit
trap cleanup_test_environment EXIT

# Function to get joblet version info (for debugging)
get_joblet_info() {
    print_info "Joblet environment information:"
    echo "  JOBLET_CONFIG: $JOBLET_CONFIG"
    echo "  RNX_CONFIG: $RNX_CONFIG"
    echo "  TEST_DIR: $TEST_DIR"
    
    # Try to get version info if available
    if command -v joblet >/dev/null 2>&1; then
        echo "  Joblet binary: $(which joblet)"
    fi
    
    if command -v rnx >/dev/null 2>&1; then
        echo "  RNX binary: $(which rnx)"
    fi
}

# Export functions for use in test scripts
export -f print_info print_success print_warning print_error
export -f run_test wait_for_server check_prerequisites
export -f generate_test_id cleanup_test_jobs validate_json
export -f wait_for_job_completion get_job_logs_safe create_temp_test_file cleanup_temp_files
export -f print_test_summary print_suite_summary setup_test_environment cleanup_test_environment
export -f get_joblet_info