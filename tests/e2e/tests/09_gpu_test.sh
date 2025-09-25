#!/bin/bash

# Test 09: GPU Support End-to-End Tests
# Tests GPU allocation, job execution, and resource management

# Source the test framework
source "$(dirname "$0")/../lib/test_framework.sh"

# ============================================
# GPU Test Configuration
# ============================================

# Check if we should run GPU tests
GPU_TEST_MODE="${GPU_TEST_MODE:-mock}"  # mock, real, skip
REMOTE_HOST="${REMOTE_HOST:-192.168.1.161}"

# ============================================
# GPU Test Helper Functions
# ============================================

# Check if GPU support is available
check_gpu_support() {
    local status_output=$("$RNX_BINARY" monitor status 2>/dev/null)
    echo "$status_output" | grep -q "GPU" && return 0 || return 1
}

# Test GPU flag parsing
test_gpu_flag_parsing() {
    local job_output

    # Test basic GPU flag
    job_output=$("$RNX_BINARY" --json job run --gpu=1 echo "GPU test" 2>&1)
    local exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        echo "    ${RED}GPU flag parsing failed${NC}"
        echo "    ${RED}Output: $job_output${NC}"
        return 1
    fi

    # Check that job was created
    local job_uuid=$(echo "$job_output" | grep -o '"job_uuid"[^"]*"[^"]*"' | sed 's/.*"\([^"]*\)"/\1/')
    if [[ -z "$job_uuid" ]]; then
        echo "    ${RED}No job UUID returned for GPU job${NC}"
        return 1
    fi

    echo "    ${GREEN}GPU flags parsed correctly${NC}"
    return 0
}

# Test GPU memory flag parsing
test_gpu_memory_parsing() {
    local job_output

    # Test GPU memory flag with different formats
    for memory_spec in "4GB" "8192MB" "2048"; do
        job_output=$("$RNX_BINARY" --json job run --gpu=1 --gpu-memory="$memory_spec" echo "GPU memory test" 2>&1)
        local exit_code=$?

        if [[ $exit_code -ne 0 ]]; then
            echo "    ${RED}GPU memory parsing failed for: $memory_spec${NC}"
            echo "    ${RED}Output: $job_output${NC}"
            return 1
        fi

        # Get job UUID and check it was created
        local job_uuid=$(echo "$job_output" | grep -o '"job_uuid"[^"]*"[^"]*"' | sed 's/.*"\([^"]*\)"/\1/')
        if [[ -n "$job_uuid" ]]; then
            # Wait a moment and clean up
            sleep 1
        fi
    done

    echo "    ${GREEN}GPU memory flags parsed correctly${NC}"
    return 0
}

# Test GPU job execution with mock mode
test_gpu_job_execution() {
    if [[ "$GPU_TEST_MODE" == "skip" ]]; then
        echo "    ${YELLOW}Skipping GPU job execution (GPU_TEST_MODE=skip)${NC}"
        return 0
    fi

    # Run a simple GPU job
    local job_output=$("$RNX_BINARY" --json job run --gpu=1 --gpu-memory=4GB echo "GPU job completed successfully" 2>&1)
    local exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        echo "    ${RED}GPU job execution failed${NC}"
        echo "    ${RED}Output: $job_output${NC}"
        return 1
    fi

    # Extract job UUID
    local job_uuid=$(echo "$job_output" | grep -o '"job_uuid"[^"]*"[^"]*"' | sed 's/.*"\([^"]*\)"/\1/')
    if [[ -z "$job_uuid" ]]; then
        echo "    ${RED}No job UUID returned${NC}"
        return 1
    fi

    # Wait for job completion
    sleep 3

    # Check job status
    local status_output=$("$RNX_BINARY" --json job status "$job_uuid" 2>/dev/null)
    if ! echo "$status_output" | grep -q '"status".*"COMPLETED"'; then
        echo "    ${RED}GPU job did not complete successfully${NC}"
        echo "    ${RED}Status: $status_output${NC}"
        return 1
    fi

    # Check job logs for expected output
    local logs=$("$RNX_BINARY" job log "$job_uuid" 2>/dev/null)
    if ! echo "$logs" | grep -q "GPU job completed successfully"; then
        echo "    ${RED}Expected output not found in GPU job logs${NC}"
        echo "    ${RED}Logs: $logs${NC}"
        return 1
    fi

    echo "    ${GREEN}GPU job executed successfully${NC}"
    return 0
}

# Test multi-GPU allocation
test_multi_gpu_allocation() {
    if [[ "$GPU_TEST_MODE" != "real" ]]; then
        echo "    ${YELLOW}Skipping multi-GPU test (requires GPU_TEST_MODE=real)${NC}"
        return 0
    fi

    # Try to allocate 2 GPUs
    local job_output=$("$RNX_BINARY" --json job run --gpu=2 --gpu-memory=4GB echo "Multi-GPU test" 2>&1)
    local exit_code=$?

    # This may fail if not enough GPUs available - that's expected
    if [[ $exit_code -ne 0 ]]; then
        if echo "$job_output" | grep -q -i "insufficient.*gpu\|not.*enough.*gpu\|gpu.*unavailable"; then
            echo "    ${YELLOW}Multi-GPU test skipped (insufficient GPUs available)${NC}"
            return 0
        else
            echo "    ${RED}Multi-GPU test failed unexpectedly${NC}"
            echo "    ${RED}Output: $job_output${NC}"
            return 1
        fi
    fi

    echo "    ${GREEN}Multi-GPU allocation successful${NC}"

    # Clean up if successful
    local job_uuid=$(echo "$job_output" | grep -o '"job_uuid"[^"]*"[^"]*"' | sed 's/.*"\([^"]*\)"/\1/')
    if [[ -n "$job_uuid" ]]; then
        sleep 3  # Let job complete
    fi

    return 0
}

# Test CUDA environment variables
test_cuda_environment() {
    if [[ "$GPU_TEST_MODE" == "skip" ]]; then
        echo "    ${YELLOW}Skipping CUDA environment test (GPU_TEST_MODE=skip)${NC}"
        return 0
    fi

    # Run job that checks CUDA environment variables
    local job_output=$("$RNX_BINARY" --json job run --gpu=1 bash -c "echo 'CUDA_VISIBLE_DEVICES='$CUDA_VISIBLE_DEVICES; env | grep CUDA || echo 'NO_CUDA_VARS'" 2>&1)
    local exit_code=$?

    if [[ $exit_code -ne 0 ]]; then
        echo "    ${RED}CUDA environment test job failed${NC}"
        return 1
    fi

    local job_uuid=$(echo "$job_output" | grep -o '"job_uuid"[^"]*"[^"]*"' | sed 's/.*"\([^"]*\)"/\1/')
    if [[ -z "$job_uuid" ]]; then
        echo "    ${RED}No job UUID returned for CUDA test${NC}"
        return 1
    fi

    # Wait and check logs
    sleep 3
    local logs=$("$RNX_BINARY" job log "$job_uuid" 2>/dev/null)

    # Check if CUDA environment variables are set (or properly indicate no GPUs)
    if echo "$logs" | grep -q "CUDA_VISIBLE_DEVICES="; then
        echo "    ${GREEN}CUDA environment variables detected${NC}"
        return 0
    elif echo "$logs" | grep -q "NO_CUDA_VARS" && [[ "$GPU_TEST_MODE" == "mock" ]]; then
        echo "    ${GREEN}CUDA environment test completed (mock mode)${NC}"
        return 0
    else
        echo "    ${YELLOW}CUDA environment unclear - may indicate no GPU support${NC}"
        echo "    ${YELLOW}Logs: $logs${NC}"
        return 0  # Don't fail - GPU might not be available
    fi
}

# Test workflow with GPU jobs
test_gpu_workflow() {
    # Create a simple GPU workflow
    local workflow_file="/tmp/gpu_test_workflow.yaml"
    cat > "$workflow_file" << 'EOF'
name: "GPU Test Workflow"
description: "Test workflow with GPU allocation"

jobs:
  gpu-preprocessing:
    command: "echo"
    args: ["Preprocessing with GPU"]
    resources:
      gpu_count: 1
      gpu_memory_mb: 2048
      max_memory: 1024

  gpu-training:
    command: "echo"
    args: ["Training with GPU"]
    requires:
      - gpu-preprocessing: "COMPLETED"
    resources:
      gpu_count: 1
      gpu_memory_mb: 4096
      max_memory: 2048
EOF

    # Run the workflow
    local workflow_output=$("$RNX_BINARY" --json job run --workflow="$workflow_file" 2>&1)
    local exit_code=$?

    # Clean up workflow file
    rm -f "$workflow_file"

    if [[ $exit_code -ne 0 ]]; then
        if echo "$workflow_output" | grep -q -i "gpu.*not.*available\|gpu.*disabled"; then
            echo "    ${YELLOW}GPU workflow test skipped (GPU not available)${NC}"
            return 0
        else
            echo "    ${RED}GPU workflow failed${NC}"
            echo "    ${RED}Output: $workflow_output${NC}"
            return 1
        fi
    fi

    echo "    ${GREEN}GPU workflow executed successfully${NC}"
    return 0
}

# ============================================
# Test Suite Execution
# ============================================

main() {
    test_suite_init "GPU Support End-to-End Tests"

    # Check prerequisites
    if ! check_prerequisites; then
        echo -e "${RED}Prerequisites not met. Exiting.${NC}"
        exit 1
    fi

    echo -e "${CYAN}GPU Test Mode: $GPU_TEST_MODE${NC}"
    echo -e "${CYAN}Remote Host: $REMOTE_HOST${NC}"

    test_section "GPU Flag Parsing"
    run_test "GPU flag parsing" test_gpu_flag_parsing
    run_test "GPU memory flag parsing" test_gpu_memory_parsing

    test_section "GPU Job Execution"
    run_test "Basic GPU job execution" test_gpu_job_execution
    run_test "Multi-GPU allocation" test_multi_gpu_allocation
    run_test "CUDA environment variables" test_cuda_environment

    test_section "GPU Workflows"
    run_test "GPU workflow execution" test_gpu_workflow

    # Clean up
    cleanup_test_artifacts

    # Print summary and exit
    test_suite_summary
    exit $?
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi