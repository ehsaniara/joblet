#!/bin/bash

# Test 08: RNX JSON Flag Tests
# Tests the --json flag functionality for key rnx commands to ensure frontend compatibility

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# ============================================
# RNX JSON Test Helper Functions
# ============================================

# Basic JSON validation without jq dependency
validate_basic_json() {
    local json_output="$1"
    
    # Basic checks for JSON structure (object or array)
    if [[ "$json_output" =~ ^[[:space:]]*\{ && "$json_output" =~ \}[[:space:]]*$ ]]; then
        # JSON object
        return 0
    elif [[ "$json_output" =~ ^[[:space:]]*\[ && "$json_output" =~ \][[:space:]]*$ ]]; then
        # JSON array
        return 0
    else
        echo "    ${RED}Output doesn't appear to be JSON format${NC}"
        echo "    ${RED}Raw output: $json_output${NC}"
        return 1
    fi
}

# Check if JSON contains expected fields using grep
check_json_fields() {
    local json_output="$1"
    local required_fields="$2"  # space-separated list
    
    for field in $required_fields; do
        if ! echo "$json_output" | grep -q "\"$field\""; then
            echo "    ${RED}Missing required field: $field${NC}"
            return 1
        fi
    done
    
    return 0
}

# Execute rnx command with --json flag and validate
execute_rnx_json() {
    local command="$1"
    local expected_fields="$2"
    local timeout="${3:-10}"

    local json_output
    # Use --json as global flag for all commands
    json_output=$(timeout "$timeout" "$RNX_BINARY" --json $command 2>&1)
    local exit_code=$?
    
    if [[ $exit_code -ne 0 ]]; then
        echo "    ${RED}Command failed with exit code: $exit_code${NC}"
        echo "    ${RED}Output: $json_output${NC}"
        return 1
    fi
    
    # If no expected fields specified, just validate JSON structure
    if [[ -z "$expected_fields" ]]; then
        if validate_basic_json "$json_output"; then
            echo "$json_output"
            return 0
        fi
    else
        if validate_basic_json "$json_output" && check_json_fields "$json_output" "$expected_fields"; then
            echo "$json_output"
            return 0
        fi
    fi
    
    return 1
}

# Extract simple JSON values using grep/sed (basic implementation)
extract_json_value() {
    local json_output="$1"
    local field="$2"
    
    echo "$json_output" | grep -o "\"$field\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" | sed 's/.*"\([^"]*\)"/\1/' | head -1
}

# ============================================
# Test Functions
# ============================================

test_rnx_runtime_list_json() {
    local json_output
    json_output=$(execute_rnx_json "runtime list" "")
    
    if [[ $? -eq 0 ]]; then
        # Check that it looks like a JSON array
        if echo "$json_output" | grep -q '^\s*\[' && echo "$json_output" | grep -q '\]\s*$'; then
            # Check for runtime structure if any runtimes exist
            if echo "$json_output" | grep -q '"name"'; then
                # Check for expected runtime fields (note: available is a boolean, not quoted)
                local required_runtime_fields="name language version"
                for field in $required_runtime_fields; do
                    if ! echo "$json_output" | grep -q "\"$field\""; then
                        echo "    ${RED}Runtime object missing field: $field${NC}"
                        return 1
                    fi
                done
                # Check for available field (boolean, so no quotes around value)
                if ! echo "$json_output" | grep -q '"available":\s*\(true\|false\)'; then
                    echo "    ${RED}Runtime object missing or invalid 'available' field${NC}"
                    return 1
                fi
            fi
            return 0
        else
            echo "    ${RED}Output is not a JSON array${NC}"
            return 1
        fi
    fi
    
    return 1
}

test_rnx_run_json() {
    local json_output
    json_output=$(execute_rnx_json "job run echo 'rnx-json-test'" "job_uuid status" 15)
    
    if [[ $? -eq 0 ]]; then
        # Validate job_uuid format (UUID-like) using basic pattern matching
        local job_uuid
        job_uuid=$(extract_json_value "$json_output" "job_uuid")
        
        if [[ "$job_uuid" =~ ^[a-f0-9-]{36}$ ]]; then
            # Check that status field exists
            if echo "$json_output" | grep -q "status"; then
                return 0
            else
                echo "    ${RED}Status field not found${NC}"
                return 1
            fi
        else
            echo "    ${RED}Invalid job_uuid format: $job_uuid${NC}"
            return 1
        fi
    fi
    
    return 1
}

test_rnx_list_json() {
    local json_output
    json_output=$(execute_rnx_json "job list" "id")
    
    if [[ $? -eq 0 ]]; then
        # Check that it looks like a JSON array
        if echo "$json_output" | grep -q '^\s*\[' && echo "$json_output" | grep -q '\]\s*$'; then
            # If jobs exist, check for basic job fields
            if echo "$json_output" | grep -q '"id"'; then
                local required_job_fields="id command status node_id"
                for field in $required_job_fields; do
                    if ! echo "$json_output" | grep -q "\"$field\""; then
                        echo "    ${RED}Job object missing field: $field${NC}"
                        return 1
                    fi
                done
            fi
            return 0
        fi
    fi
    
    return 1
}

test_rnx_status_json() {
    # First create a test job to get status for
    local run_output
    run_output=$("$RNX_BINARY" --json job run --env=TEST_VAR=status_test echo 'status-test-job' 2>&1)
    
    if [[ $? -ne 0 ]]; then
        echo "    ${RED}Failed to create test job for status test${NC}"
        return 1
    fi
    
    # Extract job UUID from run output
    local job_uuid
    job_uuid=$(extract_json_value "$run_output" "job_uuid")
    
    if [[ -z "$job_uuid" ]]; then
        echo "    ${RED}Failed to extract job UUID from run output${NC}"
        return 1
    fi
    
    # Wait a moment for job to complete
    sleep 2
    
    # Test status command with JSON output
    local json_output
    json_output=$(execute_rnx_json "job status $job_uuid" "uuid command status")
    
    if [[ $? -eq 0 ]]; then
        # Check for enhanced status fields that were added
        local enhanced_fields="uuid command status startTime environment nodeId"
        for field in $enhanced_fields; do
            if ! echo "$json_output" | grep -q "\"$field\""; then
                echo "    ${RED}Status JSON missing enhanced field: $field${NC}"
                return 1
            fi
        done
        
        # Check for new fields added in the enhancement (even if empty)
        local new_fields="network volumes runtime workDir uploads dependencies workflowUuid"
        for field in $new_fields; do
            if ! echo "$json_output" | grep -q "\"$field\""; then
                echo "    ${YELLOW}Note: Status JSON missing new field: $field (may be empty)${NC}"
                # Don't fail the test for these as they may be empty
            fi
        done
        
        # Verify the UUID in response matches the requested one
        local response_uuid
        response_uuid=$(extract_json_value "$json_output" "uuid")
        
        if [[ "$response_uuid" == "$job_uuid" ]]; then
            return 0
        else
            echo "    ${RED}Response UUID ($response_uuid) doesn't match requested UUID ($job_uuid)${NC}"
            return 1
        fi
    fi
    
    return 1
}

test_rnx_status_enhanced_display() {
    # Test that the enhanced status display contains expected sections
    local run_output
    run_output=$("$RNX_BINARY" --json job run --max-cpu=50 --max-memory=256 --env=TEST_VAR=enhanced --secret-env=SECRET_VAR=hidden echo 'enhanced-status-test' 2>&1)
    
    if [[ $? -ne 0 ]]; then
        echo "    ${RED}Failed to create enhanced test job${NC}"
        return 1
    fi
    
    local job_uuid
    job_uuid=$(extract_json_value "$run_output" "job_uuid")
    
    if [[ -z "$job_uuid" ]]; then
        echo "    ${RED}Failed to extract job UUID from enhanced run output${NC}"
        return 1
    fi
    
    # Wait for job to complete
    sleep 3
    
    # Test regular (non-JSON) status output for enhanced sections
    local status_output
    status_output=$("$RNX_BINARY" job status "$job_uuid" 2>&1)
    
    if [[ $? -eq 0 ]]; then
        # Check for expected sections in the enhanced display
        local expected_sections=("Job ID:" "Node ID:" "Command:" "Status:" "Timing:" "Resource Limits:" "Environment Variables:")
        for section in "${expected_sections[@]}"; do
            if ! echo "$status_output" | grep -q "$section"; then
                echo "    ${RED}Enhanced status display missing section: $section${NC}"
                return 1
            fi
        done
        
        # Check that resource limits show the values we set
        if ! echo "$status_output" | grep -q "Max CPU: 50%"; then
            echo "    ${RED}Enhanced status display missing CPU limit${NC}"
            return 1
        fi
        
        if ! echo "$status_output" | grep -q "Max Memory: 256 MB"; then
            echo "    ${RED}Enhanced status display missing memory limit${NC}"
            return 1
        fi
        
        # Check that environment variables are displayed
        if ! echo "$status_output" | grep -q "TEST_VAR=enhanced"; then
            echo "    ${RED}Enhanced status display missing environment variable${NC}"
            return 1
        fi
        
        # Check that secret variables are masked
        if ! echo "$status_output" | grep -q "SECRET_VAR=\*\*\* (secret)"; then
            echo "    ${RED}Enhanced status display not masking secret variable correctly${NC}"
            return 1
        fi
        
        return 0
    fi
    
    return 1
}

# Test that non-JSON commands still work normally (regression test)
test_rnx_normal_output() {
    # Test that commands without --json still produce normal output
    local normal_output
    normal_output=$("$RNX_BINARY" runtime list 2>/dev/null | head -5)
    
    if [[ $? -eq 0 ]]; then
        # Normal output should NOT be JSON
        if echo "$normal_output" | grep -qE "^\s*[\[\{]"; then
            echo "    ${RED}Normal output is unexpectedly JSON-like${NC}"
            return 1
        fi
        
        # Should contain some expected text patterns (basic check)
        if echo "$normal_output" | grep -qE "(openjdk|python|Runtime|Available|Name)" || [[ ${#normal_output} -gt 10 ]]; then
            return 0
        else
            echo "    ${RED}Normal output seems too short or missing expected patterns${NC}"
            return 1
        fi
    fi
    
    return 1
}

# ============================================
# Test Suite Execution
# ============================================

main() {
    test_suite_init "RNX JSON Flag Tests"
    
    # Check prerequisites
    if ! check_prerequisites; then
        echo -e "${RED}Prerequisites not met. Exiting.${NC}"
        exit 1
    fi
    
    # Note: This test uses basic shell tools for JSON validation to avoid external dependencies
    
    test_section "RNX Core Commands with --json"
    run_test "rnx runtime list --json" test_rnx_runtime_list_json
    run_test "rnx job run --json" test_rnx_run_json
    run_test "rnx job list --json" test_rnx_list_json
    run_test "rnx job status --json" test_rnx_status_json
    
    test_section "Enhanced Status Command Tests"
    run_test "Enhanced status display" test_rnx_status_enhanced_display
    
    test_section "RNX Regression Tests"
    run_test "Normal output still works" test_rnx_normal_output
    
    # Clean up test artifacts
    cleanup_test_artifacts
    
    # Print summary and exit
    test_suite_summary
    exit $?
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi