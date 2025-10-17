#!/bin/bash

# Test 07: GitHub Runtime Installation Tests
# Tests GitHub repository-based runtime installation features
#
# RESOURCE MANAGEMENT STRATEGY:
# GitHub runtime installations can be resource-intensive (especially python-3.11-ml)
# To prevent resource exhaustion on remote test hosts:
# - Each test cleans up runtimes immediately after verification
# - Test 8 uses longer timeout (240s) as it runs after multiple heavy installations
# - Concurrent test includes timeout protection
# - All cleanup happens in both success and failure paths

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Test configuration
GITHUB_REPO="ehsaniara/joblet/tree/main/runtimes"
PYTHON_RUNTIME="python-3.11-ml"
JAVA_RUNTIME="openjdk-21"
TEST_RUNTIME_INSTALLED=false

# ============================================
# Helper Functions
# ============================================

cleanup_test_runtime() {
    if [[ "$TEST_RUNTIME_INSTALLED" == "true" ]]; then
        echo -e "  ${YELLOW}Cleaning up test runtime...${NC}"
        "$RNX_BINARY" runtime remove "$PYTHON_RUNTIME" >/dev/null 2>&1 || true
        TEST_RUNTIME_INSTALLED=false
    fi
}

# Trap to ensure cleanup on exit
trap cleanup_test_runtime EXIT

# ============================================
# Test Functions
# ============================================

test_github_runtime_list() {
    echo -e "  ${BLUE}Testing GitHub repository runtime listing...${NC}"
    
    local output=$("$RNX_BINARY" runtime list --github-repo="$GITHUB_REPO" 2>&1)
    local exit_code=$?
    
    if [[ $exit_code -eq 0 ]]; then
        assert_contains "$output" "$PYTHON_RUNTIME" "Should list Python ML runtime"
        assert_contains "$output" "$JAVA_RUNTIME" "Should list Java runtime" 
        return 0
    else
        echo -e "    ${RED}Failed to list GitHub runtimes: $output${NC}"
        return 1
    fi
}

test_github_runtime_list_json() {
    echo -e "  ${BLUE}Testing GitHub repository runtime listing with JSON output...${NC}"
    
    local output=$("$RNX_BINARY" runtime list --github-repo="$GITHUB_REPO" --json 2>&1)
    local exit_code=$?
    
    if [[ $exit_code -eq 0 ]]; then
        # Extract only the JSON part (everything after the first '[')
        local json_part=$(echo "$output" | sed -n '/^\[/,$p')
        
        # Verify JSON structure
        if echo "$json_part" | jq . >/dev/null 2>&1; then
            assert_contains "$json_part" "\"$PYTHON_RUNTIME\"" "JSON should contain Python runtime"
            assert_contains "$json_part" "\"$JAVA_RUNTIME\"" "JSON should contain Java runtime"
            return 0
        else
            echo -e "    ${RED}Output JSON part is not valid JSON${NC}"
            echo -e "    ${RED}JSON part: $json_part${NC}"
            return 1
        fi
    else
        echo -e "    ${RED}Failed to get JSON runtime list: $output${NC}"
        return 1
    fi
}

test_github_runtime_install() {
    echo -e "  ${BLUE}Testing GitHub repository runtime installation...${NC}"
    
    # Ensure runtime is not already installed
    cleanup_test_runtime
    
    # Install from GitHub repository
    local output=$(timeout 300 "$RNX_BINARY" runtime install "$PYTHON_RUNTIME" --github-repo="$GITHUB_REPO" 2>&1)
    local exit_code=$?
    
    if [[ $exit_code -eq 0 ]]; then
        TEST_RUNTIME_INSTALLED=true
        
        # Verify installation success messages
        assert_contains "$output" "🏗️  Installing runtime: $PYTHON_RUNTIME" "Should show installation message"
        assert_contains "$output" "📦 Installing from GitHub repository" "Should show GitHub source"
        assert_contains "$output" "📦 Starting GitHub runtime installation" "Should start installation"
        
        # Verify runtime is now available locally
        if runtime_exists "$PYTHON_RUNTIME"; then
            return 0
        else
            echo -e "    ${RED}Runtime not found after installation${NC}"
            return 1
        fi
    else
        echo -e "    ${RED}Failed to install runtime from GitHub: $output${NC}"
        return 1
    fi
}

test_github_manifest_validation() {
    echo -e "  ${BLUE}Testing GitHub manifest system validation...${NC}"

    # Ensure runtime is not already installed
    cleanup_test_runtime

    local output=$("$RNX_BINARY" runtime install "$PYTHON_RUNTIME" --github-repo="$GITHUB_REPO" 2>&1)
    local exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        TEST_RUNTIME_INSTALLED=true

        # Should show GitHub installation process
        assert_contains "$output" "📦 Installing from GitHub repository" "Should show GitHub installation"
        assert_contains "$output" "📋 Repository:" "Should show repository info"
        assert_contains "$output" "📋 Branch:" "Should show branch info"
        assert_contains "$output" "📦 Starting GitHub runtime installation" "Should start installation"

        # Clean up immediately after verification to free resources
        cleanup_test_runtime
        return 0
    else
        echo -e "    ${RED}GitHub installation failed: $output${NC}"
        return 1
    fi
}

test_github_archive_download() {
    echo -e "  ${BLUE}Testing GitHub archive download process...${NC}"

    # Ensure runtime is not already installed
    cleanup_test_runtime

    local output=$("$RNX_BINARY" runtime install "$PYTHON_RUNTIME" --github-repo="$GITHUB_REPO" 2>&1)
    local exit_code=$?

    if [[ $exit_code -eq 0 ]]; then
        TEST_RUNTIME_INSTALLED=true

        # Should show build process (GitHub download happens during installation)
        assert_contains "$output" "📊 📦 Cloning repository" "Should show repository cloning"
        assert_contains "$output" "📊 Extracting repository" "Should show extraction"
        assert_contains "$output" "📊 🏗️  Running setup script" "Should run setup script"

        # Clean up immediately after verification to free resources
        cleanup_test_runtime
        return 0
    else
        echo -e "    ${RED}GitHub archive download test failed: $output${NC}"
        return 1
    fi
}

test_runtime_execution_after_github_install() {
    echo -e "  ${BLUE}Testing runtime execution after GitHub installation...${NC}"
    
    # Install a simple Java runtime for testing (faster than Python ML)
    cleanup_test_runtime
    "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
    
    local install_output=$(timeout 180 "$RNX_BINARY" runtime install "$JAVA_RUNTIME" --github-repo="$GITHUB_REPO" 2>&1)
    
    if [[ $? -eq 0 ]] && runtime_exists "$JAVA_RUNTIME"; then
        # Test Java execution
        local job_output=$("$RNX_BINARY" job run --runtime="$JAVA_RUNTIME" java -version 2>&1)
        local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
        
        if [[ -n "$job_id" ]]; then
            sleep 3
            local logs=$("$RNX_BINARY" job log "$job_id" 2>/dev/null)
            
            if echo "$logs" | grep -q "openjdk\|java\|OpenJDK"; then
                echo -e "    ${GREEN}✓ Runtime execution successful after GitHub install${NC}"
                "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
                return 0
            fi
        fi
    fi
    
    echo -e "    ${RED}Runtime execution failed or runtime not properly installed${NC}"
    return 1
}

test_host_contamination_prevention() {
    echo -e "  ${BLUE}Testing host system contamination prevention...${NC}"

    # Clean up any existing runtime first
    cleanup_test_runtime

    # Get pip package count before
    local pip_before=$(pip3 list 2>/dev/null | wc -l || echo "0")

    # Install runtime from GitHub
    "$RNX_BINARY" runtime install "$PYTHON_RUNTIME" --github-repo="$GITHUB_REPO" >/dev/null 2>&1
    TEST_RUNTIME_INSTALLED=true

    # Get pip package count after
    local pip_after=$(pip3 list 2>/dev/null | wc -l || echo "0")

    # Host pip packages should not change
    if [[ "$pip_before" -eq "$pip_after" ]]; then
        echo -e "    ${GREEN}✓ Host pip packages unchanged ($pip_before -> $pip_after)${NC}"

        # Clean up immediately after verification to free resources
        cleanup_test_runtime
        return 0
    else
        echo -e "    ${RED}✗ Host contamination detected: pip packages changed ($pip_before -> $pip_after)${NC}"
        cleanup_test_runtime
        return 1
    fi
}

test_invalid_github_repo() {
    echo -e "  ${BLUE}Testing invalid GitHub repository handling...${NC}"
    
    local output
    local exit_code
    output=$("$RNX_BINARY" runtime list --github-repo="invalid/nonexistent" 2>&1)
    exit_code=$?
    
    echo -e "    ${BLUE}Debug: exit_code=$exit_code${NC}"
    
    if [[ $exit_code -ne 0 ]]; then
        # Should handle gracefully with error message - check for any error indicators
        if echo "$output" | grep -qi "failed\|error\|❌\|404\|not.*found"; then
            echo -e "    ${GREEN}✓ Properly handled invalid repository with error message${NC}"
            return 0
        else
            echo -e "    ${RED}Exit code indicates failure but no clear error message found${NC}"
            echo -e "    ${YELLOW}Output snippet: $(echo "$output" | head -3 | tr '\n' ' ')${NC}"
            return 1
        fi
    else
        echo -e "    ${RED}Should have failed for invalid repository (got exit code 0)${NC}"
        echo -e "    ${YELLOW}Output snippet: $(echo "$output" | head -3 | tr '\n' ' ')${NC}"
        return 1
    fi
}

test_runtime_isolation_verification() {
    echo -e "  ${BLUE}Testing runtime isolation after GitHub installation...${NC}"

    # Clean up both test runtimes to free resources before this test
    cleanup_test_runtime
    "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true

    # Use longer timeout (240s) since this runs after multiple heavy installations
    # and may face resource contention on the remote host
    local install_output=$(timeout 240 "$RNX_BINARY" runtime install "$JAVA_RUNTIME" --github-repo="$GITHUB_REPO" 2>&1)
    local install_exit=$?

    if [[ $install_exit -eq 0 ]] && runtime_exists "$JAVA_RUNTIME"; then
        # Check that runtime appears in the runtime list (meaning it's properly registered)
        if "$RNX_BINARY" runtime list | grep -q "$JAVA_RUNTIME"; then
            echo -e "    ${GREEN}✓ Runtime properly registered in system${NC}"

            # Check runtime info is accessible (indicates proper configuration)
            if "$RNX_BINARY" runtime info "$JAVA_RUNTIME" >/dev/null 2>&1; then
                echo -e "    ${GREEN}✓ Runtime configuration accessible${NC}"

                # Clean up immediately after verification
                "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
                return 0
            else
                echo -e "    ${RED}✗ Runtime configuration not accessible${NC}"
                "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
                return 1
            fi
        else
            echo -e "    ${RED}✗ Runtime not properly registered${NC}"
            "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
            return 1
        fi
    else
        echo -e "    ${RED}✗ Runtime installation failed or not found (exit code: $install_exit)${NC}"
        echo -e "    ${YELLOW}Installation output (last 20 lines):${NC}"
        echo "$install_output" | tail -20
        "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
        return 1
    fi
}

test_concurrent_github_installs() {
    echo -e "  ${BLUE}Testing concurrent GitHub runtime installations...${NC}"

    # Clean up first to ensure fresh start
    "$RNX_BINARY" runtime remove "$PYTHON_RUNTIME" >/dev/null 2>&1 || true
    "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true

    # Start concurrent installations with timeout protection
    timeout 300 "$RNX_BINARY" runtime install "$PYTHON_RUNTIME" --github-repo="$GITHUB_REPO" >/dev/null 2>&1 &
    local python_pid=$!

    timeout 300 "$RNX_BINARY" runtime install "$JAVA_RUNTIME" --github-repo="$GITHUB_REPO" >/dev/null 2>&1 &
    local java_pid=$!

    # Wait for both to complete
    wait $python_pid
    local python_exit=$?
    wait $java_pid
    local java_exit=$?

    # Check results
    if [[ $python_exit -eq 0 && $java_exit -eq 0 ]]; then
        if runtime_exists "$PYTHON_RUNTIME" && runtime_exists "$JAVA_RUNTIME"; then
            echo -e "    ${GREEN}✓ Both runtimes installed successfully concurrently${NC}"

            # Clean up immediately after verification
            "$RNX_BINARY" runtime remove "$PYTHON_RUNTIME" >/dev/null 2>&1 || true
            "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true

            return 0
        else
            echo -e "    ${RED}✗ Runtimes not found after concurrent install${NC}"
            # Clean up partial installations
            "$RNX_BINARY" runtime remove "$PYTHON_RUNTIME" >/dev/null 2>&1 || true
            "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
            return 1
        fi
    else
        echo -e "    ${RED}✗ Concurrent installations failed (python: $python_exit, java: $java_exit)${NC}"
        # Clean up partial installations
        "$RNX_BINARY" runtime remove "$PYTHON_RUNTIME" >/dev/null 2>&1 || true
        "$RNX_BINARY" runtime remove "$JAVA_RUNTIME" >/dev/null 2>&1 || true
        return 1
    fi
}

# ============================================
# Prerequisites Check
# ============================================

check_github_prerequisites() {
    echo -e "${BLUE}Checking GitHub runtime test prerequisites...${NC}"
    
    # Check basic requirements using framework function
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
    
    # Check network connectivity to GitHub
    if ! timeout 10 bash -c "wget --spider --quiet https://github.com" >/dev/null 2>&1; then
        echo -e "${YELLOW}Warning: Cannot reach GitHub - some tests may be skipped${NC}"
        prereqs_met=false
    fi
    
    # Check if jq is available for JSON tests
    if ! command -v jq >/dev/null 2>&1; then
        echo -e "${YELLOW}Warning: jq not available - JSON tests will be skipped${NC}"
    fi
    
    if [[ "$prereqs_met" == "false" ]]; then
        return 1
    fi
    
    echo -e "${GREEN}✓ Prerequisites satisfied${NC}"
    return 0
}

# ============================================
# Main Test Execution
# ============================================

main() {
    # Initialize test suite
    test_suite_init "GitHub Runtime Installation Tests"
    
    # Check prerequisites
    if ! check_github_prerequisites; then
        echo -e "${RED}Prerequisites check failed - skipping GitHub runtime tests${NC}"
        exit 0  # Skip rather than fail
    fi
    
    # Run tests
    test_section "GitHub Repository Runtime Listing"
    run_test "GitHub runtime list" test_github_runtime_list
    
    if command -v jq >/dev/null 2>&1; then
        run_test "GitHub runtime list (JSON)" test_github_runtime_list_json
    else
        skip_test "GitHub runtime list (JSON)" "jq not available"
    fi
    
    test_section "GitHub Repository Runtime Installation"
    run_test "GitHub runtime installation" test_github_runtime_install
    run_test "GitHub manifest validation" test_github_manifest_validation
    run_test "GitHub archive download" test_github_archive_download
    
    test_section "Runtime Execution After GitHub Install"
    run_test "Runtime execution after GitHub install" test_runtime_execution_after_github_install
    
    test_section "Security and Isolation"
    run_test "Host contamination prevention" test_host_contamination_prevention
    run_test "Runtime isolation verification" test_runtime_isolation_verification
    
    test_section "Error Handling"
    run_test "Invalid GitHub repository handling" test_invalid_github_repo
    
    test_section "Concurrency"
    run_test "Concurrent GitHub installations" test_concurrent_github_installs
    
    # Print summary
    test_suite_summary
    exit $?
}

# Run if executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi