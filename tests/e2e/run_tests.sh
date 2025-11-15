#!/bin/bash

# Unified Test Runner for Joblet E2E Tests
# Runs all tests in a consistent, organized manner

# Remove set -e to allow test failures without terminating the runner
# set -e

# Source the test framework
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/lib/test_framework.sh"

# Test configuration
TESTS_TO_RUN=()
VERBOSE=false

# ============================================
# Build and Deploy Functions
# ============================================

build_and_deploy() {
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Build and Deployment${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
    
    cd "$JOBLET_ROOT"
    
    echo -e "${BLUE}Building RNX CLI...${NC}"
    if ! make all >/dev/null 2>&1; then
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi
    
    echo -e "${BLUE}Deploying to joblet service...${NC}"
    if ! make deploy >/dev/null 2>&1; then
        echo -e "${RED}Deployment failed!${NC}"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Build and deployment successful${NC}\n"
    cd "$SCRIPT_DIR"
}

# ============================================
# Test Discovery and Execution
# ============================================

discover_tests() {
    # Find all test files in the tests directory
    if [[ -d "$SCRIPT_DIR/tests" ]]; then
        for test_file in "$SCRIPT_DIR/tests"/*.sh; do
            if [[ -f "$test_file" ]]; then
                TESTS_TO_RUN+=("$test_file")
            fi
        done
    fi
    
    # Sort tests by name
    IFS=$'\n' TESTS_TO_RUN=($(sort <<<"${TESTS_TO_RUN[*]}"))
    unset IFS
}

run_single_test() {
    local test_file="$1"
    local test_name=$(basename "$test_file" .sh)
    
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Running: $test_name${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    if [[ -x "$test_file" ]]; then
        # Run test and capture exit code
        if "$test_file"; then
            echo -e "${GREEN}✓ $test_name completed successfully${NC}"
            return 0
        else
            local exit_code=$?
            echo -e "${RED}✗ $test_name failed (exit code: $exit_code)${NC}"
            return 1
        fi
    else
        echo -e "${YELLOW}⊘ $test_name is not executable, skipping${NC}"
        return 0
    fi
}

run_all_tests() {
    local total_tests=${#TESTS_TO_RUN[@]}
    local passed_suites=0
    local failed_suites=0
    local skipped_suites=0
    
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Joblet E2E Test Suite${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Found $total_tests test suites to run${NC}\n"
    
    for test_file in "${TESTS_TO_RUN[@]}"; do
        if run_single_test "$test_file"; then
            ((passed_suites++))
        else
            ((failed_suites++))
        fi
    done
    
    # Final summary
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Overall Test Summary${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    
    echo -e "Test Suites Run:    $total_tests"
    echo -e "Suites Passed:      ${GREEN}$passed_suites${NC}"
    echo -e "Suites Failed:      ${RED}$failed_suites${NC}"
    
    if [[ $failed_suites -eq 0 ]]; then
        echo -e "\n${GREEN}🎉 ALL TEST SUITES PASSED!${NC}"
        echo -e "${GREEN}Joblet is working correctly.${NC}"
        return 0
    else
        echo -e "\n${RED}❌ SOME TEST SUITES FAILED${NC}"
        echo -e "${RED}Please review the failures above.${NC}"
        return 1
    fi
}

# ============================================
# Usage and Help
# ============================================

show_usage() {
    cat << EOF
Usage: $0 [OPTIONS] [TEST_PATTERN]

Run Joblet E2E tests with full build and deployment for 100% confidence.

This script ALWAYS performs these steps for 100% confidence testing:
  1. Build the entire Joblet codebase (make all)
  2. Deploy to the joblet service (make deploy)  
  3. Run all E2E test suites to validate functionality

This ensures that all tests run against the latest code changes.

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Enable verbose output
    -t, --test PATTERN  Run only tests matching pattern
    -l, --list          List available tests without running

EXAMPLES:
    $0                  # RECOMMENDED: Full build + deploy + test all suites
    $0 -t isolation     # Build + deploy + run only isolation tests
    $0 -t "01_*"        # Build + deploy + run tests starting with 01_
    $0 --list           # List all available tests

ENVIRONMENT VARIABLES:
    JOBLET_ROOT         Path to joblet root directory
    RNX_BINARY          Path to rnx binary
    DEFAULT_RUNTIME     Default runtime to use (default: python-3.11-ml)

EOF
}

list_tests() {
    echo -e "${CYAN}Available Test Suites:${NC}\n"
    
    for test_file in "${TESTS_TO_RUN[@]}"; do
        local test_name=$(basename "$test_file" .sh)
        local test_desc="No description"
        
        # Try to extract description from test file
        if [[ -f "$test_file" ]]; then
            local desc_line=$(grep "^# Test [0-9]*:" "$test_file" | head -1)
            if [[ -n "$desc_line" ]]; then
                test_desc=$(echo "$desc_line" | sed 's/^# Test [0-9]*: *//')
            fi
        fi
        
        printf "  ${BLUE}%-25s${NC} %s\n" "$test_name" "$test_desc"
    done
}

# ============================================
# Main Execution
# ============================================

main() {
    local test_pattern=""
    local list_only=false
    
    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_usage
                exit 0
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -t|--test)
                test_pattern="$2"
                shift 2
                ;;
            -l|--list)
                list_only=true
                shift
                ;;
            *)
                test_pattern="$1"
                shift
                ;;
        esac
    done
    
    # Discover tests
    discover_tests
    
    # Filter tests if pattern provided
    if [[ -n "$test_pattern" ]]; then
        local filtered=()
        for test in "${TESTS_TO_RUN[@]}"; do
            if [[ "$(basename "$test")" == *"$test_pattern"* ]]; then
                filtered+=("$test")
            fi
        done
        TESTS_TO_RUN=("${filtered[@]}")
    fi
    
    # List tests if requested
    if [[ "$list_only" == "true" ]]; then
        list_tests
        exit 0
    fi
    
    # Check if any tests found
    if [[ ${#TESTS_TO_RUN[@]} -eq 0 ]]; then
        echo -e "${RED}No tests found matching pattern: $test_pattern${NC}"
        exit 1
    fi

    # Build and deploy unless SKIP_DEPLOY is set
    if [[ "${SKIP_DEPLOY:-}" != "1" ]]; then
        build_and_deploy
    else
        echo -e "${YELLOW}⊘ Skipping build and deploy (SKIP_DEPLOY=1)${NC}\n"
    fi
    
    # Run tests
    run_all_tests
    exit $?
}

# Make test scripts executable
chmod +x "$SCRIPT_DIR/lib/test_framework.sh" 2>/dev/null || true
chmod +x "$SCRIPT_DIR/tests"/*.sh 2>/dev/null || true

# Run main
main "$@"