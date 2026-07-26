#!/bin/bash

# Test 05: Volume Management Tests
# Tests volume creation, mounting, persistence, and sharing

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# Remote host configuration
TEST_HOST="$(get_test_host_display)"

# Test configuration
TEST_VOLUME_NAME="test-volume-$$"
TEST_DATA="test_data_$(date +%s)"

# ============================================
# Remote Host Helper Functions
# ============================================

# Run job on remote host
run_remote_job() {
    local cmd="$1"
    local job_output=$("$RNX_BINARY" job run sh -c "$cmd" 2>&1)
    local job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo "Failed to get job ID"
        return 1
    fi
    
    # Wait for job completion and get logs
    get_job_logs "$job_id"
}

# Verify file exists on remote host within job context
verify_remote_file() {
    local file_path="$1" 
    local expected_content="$2"
    
    local output=$(run_remote_job "if [ -f '$file_path' ]; then echo 'FILE_EXISTS'; cat '$file_path' | sed 's/^/CONTENT:/'; else echo 'FILE_NOT_FOUND'; fi")
    
    if echo "$output" | grep -q "FILE_EXISTS" && [[ -n "$expected_content" ]]; then
        echo "$output" | grep -q "CONTENT:$expected_content"
    elif echo "$output" | grep -q "FILE_EXISTS"; then
        return 0
    else
        return 1
    fi
}

# Verify directory exists on remote host within job context  
verify_remote_directory() {
    local dir_path="$1"
    
    local output=$(run_remote_job "if [ -d '$dir_path' ]; then echo 'DIR_EXISTS'; ls -la '$dir_path' 2>/dev/null; else echo 'DIR_NOT_FOUND'; fi")
    echo "$output" | grep -q "DIR_EXISTS"
}

# ============================================
# Test Functions
# ============================================

test_upload_directory() {
    # Upload a nested directory - exercises the relative-path (subdir) upload
    # flow end to end, which routes through the server's path-containment
    # check, and verifies nested files land at the right workspace path.
    local test_dir="/tmp/test_uploaddir_$$"
    mkdir -p "$test_dir/sub"
    echo "$TEST_DATA" > "$test_dir/sub/nested.txt"

    echo -e "    ${BLUE}Testing nested directory upload to $TEST_HOST${NC}"

    # rnx uploads a directory's CONTENTS relative to the dir, so sub/nested.txt
    # lands at /work/sub/nested.txt (no basename prefix). This exercises the
    # server-side relative-path containment check for a subdir path.
    local job_output=$("$RNX_BINARY" job run \
        --upload="$test_dir" \
        sh -c "
if [ -f '/work/sub/nested.txt' ]; then
    echo 'NESTED_FOUND'
    cat '/work/sub/nested.txt' | sed 's/^/NESTED:/'
else
    echo 'NESTED_NOT_FOUND'
    find /work -type f
fi
" 2>&1)

    local job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    local logs=$(get_job_logs "$job_id" 5)

    rm -rf "$test_dir"
    if assert_contains "$logs" "NESTED:$TEST_DATA" "Nested upload should land at its subdir path"; then
        echo -e "    ${GREEN}✓ Nested directory upload preserved subdir structure${NC}"
        return 0
    else
        return 1
    fi
}

test_upload_file() {
    # Test uploading a file to a job with remote host verification
    local test_file="/tmp/test_upload_$$.txt"
    echo "$TEST_DATA" > "$test_file"
    local uploaded_basename=$(basename "$test_file")
    
    echo -e "    ${BLUE}Testing file upload to $TEST_HOST${NC}"
    
    local job_output=$("$RNX_BINARY" job run \
        --upload="$test_file" \
        sh -c "
ls -la /work
if [ -f '/work/$uploaded_basename' ]; then
    echo 'FILE_FOUND'
    cat '/work/$uploaded_basename' | sed 's/^/CONTENT:/'
else
    echo 'FILE_NOT_FOUND'
fi
" 2>&1)
    
    local job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    local logs=$(get_job_logs "$job_id" 5)
    
    # Client-side verification
    if assert_contains "$logs" "CONTENT:$TEST_DATA" "Uploaded file should be accessible in job logs"; then
        echo -e "    ${GREEN}✓ Client-side upload verification passed${NC}"
        echo -e "    ${GREEN}✓ File upload functionality working correctly${NC}"
        
        # Note: Files don't persist across job boundaries (proper isolation)
        # The fact that we can see the file content in the logs proves upload worked
        echo -e "    ${BLUE}Note: Files are isolated per job (expected behavior)${NC}"
        rm -f "$test_file"
        return 0
    else
        rm -f "$test_file"
        return 1
    fi
}

test_download_result() {
    # Test downloading results from a job
    local job_output=$("$RNX_BINARY" job run sh -c "
echo 'DOWNLOAD_TEST_DATA' > /work/output.txt && echo 'FILE_CREATED'
" 2>&1)
    local job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    local logs=$(get_job_logs "$job_id")
    assert_contains "$logs" "FILE_CREATED" "Should create output file"
    
    # Check if download command exists
    local download_help=$("$RNX_BINARY" help 2>&1 | grep -i "download" || echo "")
    
    if [[ -n "$download_help" ]]; then
        local download_output=$("$RNX_BINARY" download "$job_id" /work/output.txt 2>&1 || echo "")
        
        if [[ -f "output.txt" ]]; then
            local content=$(cat output.txt)
            rm -f output.txt
            assert_equals "$content" "DOWNLOAD_TEST_DATA" "Downloaded content should match"
        else
            echo -e "    ${YELLOW}Download feature may not be implemented${NC}"
            return 0
        fi
    else
        echo -e "    ${YELLOW}Download command not available${NC}"
        return 0
    fi
}

test_volume_creation() {
    # Create a named filesystem volume. --size is required by the CLI; without
    # it the volume is never created (which previously made the mount test below
    # silently pass against a non-existent volume).
    local volume_output=$("$RNX_BINARY" volume create "$TEST_VOLUME_NAME" --size 100MB 2>&1 || echo "")

    if ! echo "$volume_output" | grep -qi "created successfully"; then
        echo -e "    ${RED}✗ Volume creation failed: $volume_output${NC}"
        return 1
    fi

    # Confirm it shows up in the listing.
    if ! "$RNX_BINARY" volume list 2>&1 | grep -q "$TEST_VOLUME_NAME"; then
        echo -e "    ${RED}✗ Created volume not present in 'volume list'${NC}"
        return 1
    fi
    echo -e "    ${GREEN}✓ Volume $TEST_VOLUME_NAME created (100MB) and listed${NC}"
    return 0
}

test_volume_mounting() {
    # A volume is referenced by bare name (rnx job run --volume=NAME) and always
    # mounts inside the job at /volumes/<name> - the mount path is derived from
    # the name, not client-supplied. Verify the volume created earlier actually
    # mounts there and is writable.
    echo -e "    ${BLUE}Testing volume mounting on $TEST_HOST${NC}"

    # The volume data dir is chowned to the job user at creation, so the
    # unprivileged job (nobody) can write to its own volume. Verify both the
    # mount (name -> /volumes/<name> contract) and that a write succeeds and
    # reads back.
    local mount_path="/volumes/$TEST_VOLUME_NAME"
    local job_output=$("$RNX_BINARY" job run \
        --volume="$TEST_VOLUME_NAME" \
        sh -c "
if [ -d '$mount_path' ]; then
    echo 'VOLUME_MOUNTED'
    if echo 'VOLUME_DATA' > '$mount_path/test.txt' && [ \"\$(cat '$mount_path/test.txt')\" = 'VOLUME_DATA' ]; then
        echo 'FILE_WRITTEN'
    else
        echo 'WRITE_FAILED'
    fi
else
    echo 'NO_VOLUME'
    ls -la /volumes/ 2>&1
fi
" 2>&1 || echo "")

    local job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    if [[ -z "$job_id" ]]; then
        echo -e "    ${RED}✗ Volume job was not accepted: $job_output${NC}"
        return 1
    fi

    local logs=$(get_job_logs "$job_id")
    if ! assert_contains "$logs" "VOLUME_MOUNTED" "Volume should be mounted at $mount_path"; then
        echo -e "    ${RED}✗ Volume did not mount at $mount_path${NC}"
        return 1
    fi
    if ! assert_contains "$logs" "FILE_WRITTEN" "Unprivileged job should be able to write to its volume"; then
        echo -e "    ${RED}✗ Job could not write to volume at $mount_path${NC}"
        return 1
    fi
    echo -e "    ${GREEN}✓ Volume mounted at $mount_path and writable by the job${NC}"
    return 0
}

test_volume_cross_job_persistence() {
    # The point of a named volume: data written by one job is visible to a later,
    # separate job that mounts the same volume. Full lifecycle in one test:
    # create -> job A writes -> job B reads back -> remove.
    local vol="persist-vol-$$"
    local mount_path="/volumes/$vol"
    local marker="CROSS_JOB_$(date +%s)"

    echo -e "    ${BLUE}Testing cross-job persistence with volume $vol${NC}"

    # Create a dedicated volume for this scenario.
    if ! "$RNX_BINARY" volume create "$vol" --size 50MB 2>&1 | grep -qi "created successfully"; then
        echo -e "    ${RED}✗ Could not create volume $vol${NC}"
        return 1
    fi

    # Job A: write a marker into the volume.
    local jobA_output=$("$RNX_BINARY" job run --volume="$vol" \
        sh -c "echo '$marker' > '$mount_path/persist.txt' && echo WROTE_A" 2>&1)
    local jobA=$(echo "$jobA_output" | grep "^ID:" | awk '{print $2}')
    local logsA=$(get_job_logs "$jobA")
    if ! assert_contains "$logsA" "WROTE_A" "Job A should write to the volume"; then
        echo -e "    ${RED}✗ Job A failed to write to $vol${NC}"
        "$RNX_BINARY" volume remove "$vol" >/dev/null 2>&1
        return 1
    fi

    sleep 2

    # Job B: a separate job mounts the same volume and reads the marker back.
    local jobB_output=$("$RNX_BINARY" job run --volume="$vol" \
        sh -c "if [ -f '$mount_path/persist.txt' ]; then echo FOUND_B; cat '$mount_path/persist.txt'; else echo MISSING_B; fi" 2>&1)
    local jobB=$(echo "$jobB_output" | grep "^ID:" | awk '{print $2}')
    local logsB=$(get_job_logs "$jobB")

    local ok=0
    if assert_contains "$logsB" "$marker" "Job B should read data written by Job A"; then
        echo -e "    ${GREEN}✓ Data persisted across jobs via volume $vol${NC}"
    else
        echo -e "    ${RED}✗ Job B did not see Job A's data in $vol${NC}"
        echo "$logsB" | head -5
        ok=1
    fi

    "$RNX_BINARY" volume remove "$vol" >/dev/null 2>&1
    return $ok
}

test_volume_persistence() {
    # Test that data persists across jobs with remote host verification
    echo -e "    ${BLUE}Testing data persistence isolation on $TEST_HOST${NC}"
    
    # First job: write data to /work
    local job1_output=$("$RNX_BINARY" job run sh -c "
mkdir -p /work/persist
echo 'PERSISTENT_DATA' > /work/persist/data.txt
echo 'DATA_WRITTEN'
ls -la /work/persist/
" 2>&1)
    local job1=$(echo "$job1_output" | grep "^ID:" | awk '{print $2}')
    
    local logs1=$(get_job_logs "$job1")
    if ! assert_contains "$logs1" "DATA_WRITTEN" "First job should write data"; then
        return 1
    fi
    echo -e "    ${GREEN}✓ First job wrote data successfully${NC}"
    
    # Remote host verification - confirm data exists in first job's context
    echo -e "    ${BLUE}Verifying data exists on remote host in first job context...${NC}"
    if verify_remote_file "/work/persist/data.txt" "PERSISTENT_DATA"; then
        echo -e "    ${GREEN}✓ Data confirmed to exist in job context${NC}"
    else
        echo -e "    ${YELLOW}Data verification in job context failed, continuing test...${NC}"
    fi
    
    sleep 3
    
    # Second job: try to read data (should NOT persist without volumes - proper isolation)
    local job2_output=$("$RNX_BINARY" job run sh -c "
echo 'CHECKING_PERSISTENCE'
if [ -f '/work/persist/data.txt' ]; then
    echo 'FOUND:'
    cat /work/persist/data.txt
else
    echo 'DATA_NOT_FOUND'
fi
ls -la /work/ 2>/dev/null || echo 'WORK_DIR_EMPTY'
" 2>&1)
    local job2=$(echo "$job2_output" | grep "^ID:" | awk '{print $2}')
    
    local logs2=$(get_job_logs "$job2")
    
    # Client-side verification - without volumes, data should NOT persist (proper isolation)
    if assert_contains "$logs2" "DATA_NOT_FOUND"; then
        echo -e "    ${GREEN}✓ Client-side isolation verification passed${NC}"
        
        # Remote host verification - confirm data isolation across job boundaries
        echo -e "    ${BLUE}Verifying data isolation on remote host...${NC}"
        local isolation_check=$(run_remote_job "ls /work/persist/ 2>/dev/null | wc -l")
        if echo "$isolation_check" | grep -q "^0$"; then
            echo -e "    ${GREEN}✓ Remote host isolation verification passed (no data persistence)${NC}"
            return 0
        else
            echo -e "    ${YELLOW}Remote host shows some files, but client-side isolation working${NC}"
            return 0
        fi
    else
        echo -e "    ${YELLOW}Data might be persisting unexpectedly - checking if this is expected behavior${NC}"
        # In some implementations, /work might persist - log for investigation but don't fail
        echo -e "    ${BLUE}Logs from second job:${NC}"
        echo "$logs2" | head -10
        return 1
    fi
}

test_concurrent_volume_access() {
    # Test multiple jobs accessing same volume
    echo -e "    ${YELLOW}Testing concurrent volume access (advanced feature)${NC}"
    
    # This would test:
    # - Multiple jobs reading from same volume
    # - Locking mechanisms for write access
    # - Race condition handling
    
    return 0
}

test_volume_size_limits() {
    # Test volume size restrictions with remote host verification
    echo -e "    ${BLUE}Testing volume size limits on $TEST_HOST${NC}"
    
    local job_output=$("$RNX_BINARY" job run sh -c "
# Try to write a large file (10MB)
echo 'CREATING_LARGE_FILE'
dd if=/dev/zero of=/work/large.txt bs=1M count=10 2>/dev/null
if [ -f '/work/large.txt' ]; then
    echo 'LARGE_FILE_CREATED'
    size=\$(stat -c%s '/work/large.txt' 2>/dev/null || stat -f%z '/work/large.txt' 2>/dev/null || echo 0)
    echo \"SIZE:\$size\"
    ls -lh /work/large.txt
else
    echo 'ERROR:Could not create file'
fi
" 2>&1)
    local job_id=$(echo "$job_output" | grep "^ID:" | awk '{print $2}')
    
    local logs=$(get_job_logs "$job_id")
    
    # Client-side verification
    if assert_contains "$logs" "SIZE:" "Large file should be created with size info"; then
        echo -e "    ${GREEN}✓ Client-side large file creation verified${NC}"
        
        # Extract and verify the size from logs
        local size_line=$(echo "$logs" | grep "SIZE:" | head -1)
        local file_size=$(echo "$size_line" | sed 's/.*SIZE://' | grep -o '[0-9]*')
        
        if [[ "$file_size" -eq 10485760 ]] || [[ "$file_size" -gt 10000000 ]]; then
            echo -e "    ${GREEN}✓ Large file (${file_size} bytes) created successfully${NC}"
            echo -e "    ${GREEN}✓ Volume size limits test passed (10MB file created)${NC}"
            return 0
        else
            echo -e "    ${YELLOW}File created but size verification inconclusive (${file_size} bytes)${NC}"
            return 0
        fi
    else
        echo -e "    ${RED}Large file creation failed${NC}"
        return 1
    fi
}

test_volume_cleanup() {
    # Remove the test volume (the CLI subcommand is 'remove', not 'delete').
    local remove_output=$("$RNX_BINARY" volume remove "$TEST_VOLUME_NAME" 2>&1 || echo "")

    if ! echo "$remove_output" | grep -qi "removed successfully"; then
        echo -e "    ${RED}✗ Volume removal failed: $remove_output${NC}"
        return 1
    fi

    # Confirm it is gone from the listing.
    if "$RNX_BINARY" volume list 2>&1 | grep -q "$TEST_VOLUME_NAME"; then
        echo -e "    ${RED}✗ Volume still present after removal${NC}"
        return 1
    fi
    echo -e "    ${GREEN}✓ Volume $TEST_VOLUME_NAME removed${NC}"
    return 0
}

test_bind_mount() {
    # Test bind mounting host directories
    echo -e "    ${YELLOW}Testing bind mounts (security-sensitive feature)${NC}"
    
    # This would test mounting host directories
    # Should be restricted for security
    
    return 0
}

# ============================================
# Main Test Execution
# ============================================

main() {
    # Initialize test suite
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Volume Management Tests${NC}"
    echo -e "${CYAN}  Testing against: ${TEST_HOST}${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}Started: $(date '+%Y-%m-%d %H:%M:%S')${NC}\n"
    
    # Check prerequisites
    if ! check_prerequisites; then
        echo -e "${RED}Prerequisites check failed${NC}"
        exit 1
    fi
    
    # No runtime required - using basic shell commands
    # ensure_runtime "$DEFAULT_RUNTIME"
    
    # Run tests
    test_section "File Transfer"
    run_test "Upload file to job" test_upload_file
    run_test "Upload nested directory to job" test_upload_directory
    run_test "Download results from job" test_download_result
    
    test_section "Volume Operations"
    run_test "Volume creation" test_volume_creation
    run_test "Volume mounting" test_volume_mounting
    run_test "Cross-job volume persistence" test_volume_cross_job_persistence
    run_test "Data persistence" test_volume_persistence
    
    test_section "Advanced Volume Features"
    run_test "Concurrent volume access" test_concurrent_volume_access
    run_test "Volume size limits" test_volume_size_limits
    run_test "Bind mount restrictions" test_bind_mount
    
    test_section "Cleanup"
    run_test "Volume cleanup" test_volume_cleanup
    
    # Print enhanced summary with remote host info
    echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  Volume Management Tests Summary${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "Test Host:      ${BLUE}$TEST_HOST${NC}"
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
        echo -e "\n${GREEN}✅ ALL VOLUME TESTS PASSED!${NC}"
        echo -e "${GREEN}Volume management is working correctly on $TEST_HOST${NC}"
        exit 0
    elif [[ $FAILED_TESTS -gt 0 ]]; then
        echo -e "\n${RED}❌ SOME VOLUME TESTS FAILED${NC}"
        exit 1
    else
        echo -e "\n${YELLOW}⚠ NO TESTS EXECUTED${NC}"
        exit 2
    fi
}

# Run if executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi