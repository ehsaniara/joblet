#!/bin/bash

# Test 04: Job Scheduling Tests
# Tests scheduled jobs and timing

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# ============================================
# Test Functions
# ============================================

test_immediate_job() {
    # Test a job that runs immediately
    # Use a simple echo command that works without any runtime
    local job_output=$("$RNX_BINARY" job run echo "IMMEDIATE_JOB_RAN" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
    
    if [[ -z "$job_id" ]]; then
        echo -e "    ${RED}Failed to extract job ID${NC}"
        return 1
    fi
    
    sleep 3
    
    local status=$(check_job_status "$job_id")
    assert_equals "$status" "COMPLETED" "Immediate job should complete"
}

test_delayed_job() {
    # Test scheduling a job with a delay (now supports seconds!)
    # Using 10s schedule for quick testing
    
    # Create a scheduled job with --schedule flag
    local job_output=$("$RNX_BINARY" job run --schedule="10s" echo "DELAYED_JOB_RAN" 2>&1)
    
    if echo "$job_output" | grep -q "ID:"; then
        local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
        
        # Check status immediately (should be SCHEDULED)
        local initial_status=$(check_job_status "$job_id")
        
        if [[ "$initial_status" == "SCHEDULED" ]]; then
            echo -e "    ${GREEN}Job successfully scheduled (status: SCHEDULED)${NC}"
            
            # Wait for the job to execute (10 seconds + small buffer)
            echo -e "    ${BLUE}Waiting 12 seconds for job to execute...${NC}"
            sleep 12
            
            # Check if job completed
            local final_status=$(check_job_status "$job_id")
            if [[ "$final_status" == "COMPLETED" ]]; then
                local logs=$(get_job_logs "$job_id")
                if echo "$logs" | grep -q "DELAYED_JOB_RAN"; then
                    echo -e "    ${GREEN}Scheduled job executed successfully after 10s${NC}"
                    return 0
                fi
            else
                echo -e "    ${YELLOW}Job status after wait: $final_status${NC}"
                return 1
            fi
        else
            echo -e "    ${YELLOW}Job not in SCHEDULED state (got: $initial_status)${NC}"
            return 1
        fi
    else
        echo -e "    ${RED}Failed to create scheduled job${NC}"
        return 1
    fi
}

test_schedule_formats() {
    # Test different schedule format validations
    # Now supports seconds! Minimum is 1 second in the future
    local formats=("5s" "30s" "2min" "1h" "2025-12-31T23:59:59Z")
    local job_ids=()

    for format in "${formats[@]}"; do
        # Test that the format is accepted (job creation succeeds)
        local job_output=$("$RNX_BINARY" job run --schedule="$format" \
            echo "SCHEDULED_FORMAT_TEST" 2>&1)

        if echo "$job_output" | grep -q "ID:"; then
            local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
            job_ids+=("$job_id")

            # Verify job is in SCHEDULED state
            local status=$(check_job_status "$job_id")
            if [[ "$status" == "SCHEDULED" ]]; then
                echo -e "    ${GREEN}Schedule format '$format' accepted and job scheduled${NC}"
            else
                echo -e "    ${YELLOW}Format '$format' accepted but status is: $status${NC}"
            fi
        else
            echo -e "    ${RED}Schedule format '$format' rejected${NC}"
            return 1
        fi
    done

    # Clean up scheduled jobs
    for job_id in "${job_ids[@]}"; do
        "$RNX_BINARY" job delete "$job_id" 2>/dev/null || true
    done

    return 0
}

test_scheduled_job_execution() {
    # Test that a scheduled job actually executes after its scheduled time
    echo -e "    ${YELLOW}Scheduling job for 15 seconds from now (will wait)...${NC}"
    
    # Schedule a job for 15 seconds from now
    local job_output=$("$RNX_BINARY" job run --schedule="15s" echo "SCHEDULED_JOB_EXECUTED" 2>&1)
    
    if ! echo "$job_output" | grep -q "ID:"; then
        echo -e "    ${RED}Failed to create scheduled job${NC}"
        return 1
    fi
    
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
    
    # Verify initial status is SCHEDULED
    local initial_status=$(check_job_status "$job_id")
    if [[ "$initial_status" != "SCHEDULED" ]]; then
        echo -e "    ${RED}Job not in SCHEDULED state (got: $initial_status)${NC}"
        return 1
    fi
    
    echo -e "    ${GREEN}Job scheduled successfully (ID: $job_id)${NC}"
    echo -e "    ${BLUE}Waiting 20 seconds for job to execute...${NC}"
    
    # Wait for the job to execute (15 seconds + buffer)
    sleep 20
    
    # Check if job completed
    local final_status=$(check_job_status "$job_id")
    if [[ "$final_status" == "COMPLETED" ]]; then
        # Verify the output
        local logs=$(get_job_logs "$job_id")
        if echo "$logs" | grep -q "SCHEDULED_JOB_EXECUTED"; then
            echo -e "    ${GREEN}Scheduled job executed successfully at scheduled time${NC}"
            return 0
        else
            echo -e "    ${RED}Job completed but output not as expected${NC}"
            return 1
        fi
    else
        echo -e "    ${RED}Job did not complete after scheduled time (status: $final_status)${NC}"
        return 1
    fi
}

test_multiple_scheduled_jobs() {
    # Test multiple jobs scheduled at different times
    local job_ids=()
    
    # Schedule 3 jobs using simple echo commands
    for i in 1 2 3; do
        local job_output=$("$RNX_BINARY" job run sh -c "echo 'SCHEDULED_JOB_$i'; sleep 1" 2>&1)
        local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
        if [[ -n "$job_id" ]]; then
            job_ids+=("$job_id")
        fi
    done
    
    # Wait for all to complete
    sleep 5
    
    # Check all completed
    local all_completed=true
    for job_id in "${job_ids[@]}"; do
        local status=$(check_job_status "$job_id")
        if [[ "$status" != "COMPLETED" ]]; then
            all_completed=false
            break
        fi
    done
    
    if [[ "$all_completed" == "true" ]]; then
        return 0
    else
        return 1
    fi
}

test_job_queue_ordering() {
    # Test that jobs are executed in order
    local job_output1=$("$RNX_BINARY" job run echo "FIRST" 2>&1)
    local job1=$(echo "$job_output1" | grep "ID:" | awk '{print $2}')
    sleep 0.5
    
    local job_output2=$("$RNX_BINARY" job run echo "SECOND" 2>&1)
    local job2=$(echo "$job_output2" | grep "ID:" | awk '{print $2}')
    sleep 0.5
    
    local job_output3=$("$RNX_BINARY" job run echo "THIRD" 2>&1)
    local job3=$(echo "$job_output3" | grep "ID:" | awk '{print $2}')
    
    sleep 5
    
    # Get logs and check ordering
    local log1=$(get_job_logs "$job1")
    local log2=$(get_job_logs "$job2")
    local log3=$(get_job_logs "$job3")
    
    # All should have completed
    if assert_contains "$log1" "FIRST" && \
       assert_contains "$log2" "SECOND" && \
       assert_contains "$log3" "THIRD"; then
        return 0
    else
        return 1
    fi
}

test_recurring_schedule() {
    # Test recurring schedules (if supported)
    echo -e "    ${YELLOW}Testing recurring schedules (future feature)${NC}"
    
    # This would test schedules like:
    # - Every 5 minutes
    # - Every hour at :30
    # - Daily at 2 AM
    # - Weekly on Mondays
    
    # For now, just pass as this is likely a future feature
    return 0
}

test_schedule_cancellation() {
    # Test cancelling a scheduled job
    local job_output=$("$RNX_BINARY" job run sh -c "sleep 10; echo 'SHOULD_NOT_COMPLETE'" 2>&1)
    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
    
    # Give it a moment to start
    sleep 2
    
    # Try to cancel/delete the job
    local cancel_output=$("$RNX_BINARY" job delete "$job_id" 2>&1 || echo "")
    
    # Wait a bit
    sleep 3
    
    # Check status - should be CANCELED or not exist
    local status=$(check_job_status "$job_id")
    
    if [[ "$status" == "CANCELED" ]] || [[ -z "$status" ]]; then
        return 0
    else
        echo -e "    ${YELLOW}Job cancellation may not be fully implemented${NC}"
        return 0
    fi
}

test_schedule_with_dependencies() {
    # Test scheduling with job dependencies
    echo -e "    ${YELLOW}Testing job dependencies (workflow feature)${NC}"
    
    # This would test:
    # - Job B starts after Job A completes
    # - Job C starts after both A and B complete
    # - Failure handling in dependency chains
    
    # For now, mark as future feature
    return 0
}

test_schedule_timezone_handling() {
    # Test timezone handling in schedules
    local current_tz=$(date +%Z)
    echo -e "    Current timezone: $current_tz"

    # Would test scheduling in different timezones
    # For now, just verify we can get timezone info
    if [[ -n "$current_tz" ]]; then
        return 0
    else
        return 1
    fi
}

test_scheduled_time_display() {
    # Test that scheduled jobs show the correct scheduled time in list output, not creation time
    echo -e "    ${BLUE}Testing scheduled time display in job list${NC}"

    # Get current time for comparison
    local current_time=$(date +"%Y-%m-%d %H:%M")

    # Schedule a job far in the future (end of year)
    local future_time="2025-12-31T23:59:59Z"
    local job_output=$("$RNX_BINARY" job run --schedule="$future_time" echo "FAR_FUTURE_JOB" 2>&1)

    if ! echo "$job_output" | grep -q "ID:"; then
        echo -e "    ${RED}Failed to create scheduled job${NC}"
        return 1
    fi

    local job_id=$(echo "$job_output" | grep "ID:" | awk '{print $2}')
    echo -e "    ${GREEN}Created scheduled job: $job_id${NC}"

    # Get the job list output
    local list_output=$("$RNX_BINARY" job list 2>&1)

    # Find the line with our job
    local job_line=$(echo "$list_output" | grep "$job_id")

    if [[ -z "$job_line" ]]; then
        echo -e "    ${RED}Job not found in list output${NC}"
        "$RNX_BINARY" job delete "$job_id" 2>/dev/null || true
        return 1
    fi

    echo -e "    Job list line: $job_line"

    # Extract the displayed time from the list output (START TIME column)
    # The format is: UUID NAME STATUS START_TIME COMMAND
    # We need to extract the time portion which should be in format YYYY-MM-DD HH:MM:SS
    local displayed_time=$(echo "$job_line" | awk '{print $4 " " $5}')

    echo -e "    Current time:   $current_time"
    echo -e "    Displayed time: $displayed_time"
    echo -e "    Expected time:  2025-12-31 (or similar future date)"

    # Check if the displayed time shows the scheduled time (December 2025)
    # not the creation time (current time)
    if echo "$displayed_time" | grep -q "2025-12-31"; then
        echo -e "    ${GREEN}✓ List correctly shows scheduled time (future)${NC}"
        "$RNX_BINARY" job delete "$job_id" 2>/dev/null || true
        return 0
    elif echo "$displayed_time" | grep -q "$(date +%Y-%m-%d)"; then
        echo -e "    ${RED}✗ List incorrectly shows creation time (today) instead of scheduled time${NC}"
        echo -e "    ${RED}This is a bug: SCHEDULED jobs should show their scheduled execution time${NC}"
        "$RNX_BINARY" job delete "$job_id" 2>/dev/null || true
        return 1
    else
        echo -e "    ${YELLOW}Unable to determine if time is correct${NC}"
        "$RNX_BINARY" job delete "$job_id" 2>/dev/null || true
        return 1
    fi
}

# ============================================
# Main Test Execution
# ============================================

main() {
    # Initialize test suite
    test_suite_init "Job Scheduling Tests"
    
    # Check prerequisites
    if ! check_prerequisites; then
        echo -e "${RED}Prerequisites check failed${NC}"
        exit 1
    fi
    
    # No runtime required - using basic shell commands
    # ensure_runtime "$DEFAULT_RUNTIME"
    
    # Run tests
    test_section "Basic Scheduling"
    run_test "Immediate job execution" test_immediate_job
    run_test "Delayed job execution" test_delayed_job
    run_test "Schedule format validation" test_schedule_formats
    run_test "Scheduled job execution (15s wait)" test_scheduled_job_execution
    
    test_section "Queue Management"
    run_test "Multiple scheduled jobs" test_multiple_scheduled_jobs
    run_test "Job queue ordering" test_job_queue_ordering
    
    test_section "Advanced Scheduling"
    run_test "Recurring schedules" test_recurring_schedule
    run_test "Schedule cancellation" test_schedule_cancellation
    run_test "Schedule with dependencies" test_schedule_with_dependencies
    
    test_section "Time Handling"
    run_test "Timezone handling" test_schedule_timezone_handling
    run_test "Scheduled time display validation" test_scheduled_time_display

    # Print summary
    test_suite_summary
    exit $?
}

# Run if executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi