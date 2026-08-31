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

# Clean-room setup: purge any joblet/rnx install, install this tree's .deb,
# wait for readiness. Tests then run against the same artifact users install.
fresh_install() {
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Clean Install (purge + packaged install)${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

    cd "$JOBLET_ROOT"

    echo -e "${BLUE}Removing previous build artifacts (bin/, dist/, packages)...${NC}"
    rm -rf bin/ dist/ joblet-deb-*/ rpmbuild/ joblet_*.deb joblet-*.rpm

    echo -e "${BLUE}Building joblet binaries and resolving rnx...${NC}"
    if ! make all rnx >/dev/null 2>&1; then
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi

    echo -e "${BLUE}Purging existing joblet installation...${NC}"
    if ! sudo ./scripts/uninstall.sh --purge; then
        echo -e "${RED}Purge failed!${NC}"
        exit 1
    fi

    echo -e "${BLUE}Building .deb from working tree...${NC}"
    local ARCH VERSION DEB
    ARCH=$(go env GOARCH)
    VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "0.0.0-dev")
    if ! ./scripts/build-deb.sh "$ARCH" "$VERSION" >/dev/null; then
        echo -e "${RED}Package build failed!${NC}"
        exit 1
    fi
    DEB=$(ls -t "$JOBLET_ROOT"/joblet_*_"$ARCH".deb | head -1)

    echo -e "${BLUE}Installing: $DEB${NC}"
    if ! sudo DEBIAN_FRONTEND=noninteractive dpkg -i "$DEB"; then
        echo -e "${RED}Package install failed!${NC}"
        exit 1
    fi
    sudo systemctl start joblet.service

    echo -e "${BLUE}Waiting for service readiness (persist socket + gRPC)...${NC}"
    local ready=0 i
    for i in $(seq 1 60); do
        if sudo test -S /opt/joblet/run/persist-ipc.sock && "$RNX_BINARY" job list >/dev/null 2>&1; then
            ready=1
            break
        fi
        sleep 0.5
    done
    if [[ "$ready" -ne 1 ]]; then
        echo -e "${RED}Service not ready after 30s - check: journalctl -u joblet${NC}"
        exit 1
    fi
    if [[ ! -f "$HOME/.rnx/rnx-config.yml" ]]; then
        echo -e "${RED}~/.rnx/rnx-config.yml not created by installer${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Clean packaged install ready${NC}\n"
    cd "$SCRIPT_DIR"
}

# Quick iteration mode (QUICK_DEPLOY=1): swap binaries onto the existing
# install instead of the full purge + packaged install
build_and_deploy() {
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Build and Deployment (quick mode)${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"

    cd "$JOBLET_ROOT"

    echo -e "${BLUE}Building joblet binaries and resolving rnx...${NC}"
    if ! make all rnx >/dev/null 2>&1; then
        echo -e "${RED}Build failed!${NC}"
        exit 1
    fi

    echo -e "${BLUE}Deploying to joblet service...${NC}"
    if ! make deploy; then
        echo -e "${RED}Deployment failed!${NC}"
        exit 1
    fi

    echo -e "${GREEN}✓ Build and deployment successful${NC}"
    echo -e "${BLUE}Waiting for service to stabilize...${NC}"
    sleep 5
    echo -e "${GREEN}✓ Service ready${NC}\n"
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
  1. Remove previous build artifacts (bin/, dist/, packages)
  2. Build the Joblet codebase and resolve rnx (make all rnx)
  3. Purge any existing joblet/rnx install (/opt/joblet, symlinks, configs)
  4. Build a .deb from the working tree and install it (needs sudo)
  5. Run all E2E test suites against the clean packaged install

This validates the same artifact a user would install. Modes:
  QUICK_DEPLOY=1  swap binaries onto the existing install (fast iteration)
  SKIP_DEPLOY=1   test the already-running service as-is

OPTIONS:
    -h, --help          Show this help message
    -v, --verbose       Enable verbose output
    -t, --test PATTERN  Run only tests matching pattern
    -x, --exclude PAT   Skip tests matching pattern (comma-separated, e.g. "02_,09_")
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
    local exclude_patterns=""
    local list_only=false

    # One run at a time: a second run would purge the install and remove bin/
    # underneath the first
    exec 9>/tmp/joblet-e2e.lock
    if ! flock -n 9; then
        echo -e "${RED}Another e2e/pre-pr run is already in progress (lock: /tmp/joblet-e2e.lock)${NC}"
        exit 1
    fi
    
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
            -x|--exclude)
                exclude_patterns="$2"
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
    
    # Apply exclusion patterns
    if [[ -n "$exclude_patterns" ]]; then
        local kept=()
        for test in "${TESTS_TO_RUN[@]}"; do
            local excluded=false
            IFS=',' read -ra patterns <<< "$exclude_patterns"
            for pat in "${patterns[@]}"; do
                if [[ "$(basename "$test")" == *"$pat"* ]]; then
                    excluded=true
                    echo -e "${YELLOW}⊘ Excluding: $(basename "$test" .sh) (matched '$pat')${NC}"
                    break
                fi
            done
            [[ "$excluded" == "false" ]] && kept+=("$test")
        done
        TESTS_TO_RUN=("${kept[@]}")
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

    # Default: purge + packaged install. QUICK_DEPLOY=1 swaps binaries onto the
    # existing install; SKIP_DEPLOY=1 tests the running service as-is
    if [[ "${SKIP_DEPLOY:-}" == "1" ]]; then
        echo -e "${YELLOW}⊘ Skipping install (SKIP_DEPLOY=1) - testing the running service${NC}\n"
    elif [[ "${QUICK_DEPLOY:-}" == "1" ]]; then
        build_and_deploy
    else
        fresh_install
    fi

    # Start from a clean slate: remove jobs/networks/volumes left by prior runs
    cleanup_previous_test_state

    # Run tests
    run_all_tests
    exit $?
}

# Make test scripts executable
chmod +x "$SCRIPT_DIR/lib/test_framework.sh" 2>/dev/null || true
chmod +x "$SCRIPT_DIR/tests"/*.sh 2>/dev/null || true

# Run main
main "$@"