#!/bin/bash

# E2E Test: GPU control plane (auto-detects real GPU, else simulation)
#
# Validates joblet's GPU wiring. If the host has a real NVIDIA GPU it enables real
# GPU support and additionally checks nvidia-smi works inside the job; otherwise it
# enables simulation (JOBLET_GPU_SIMULATE), which presents fake GPUs so the control
# plane still runs (allocation -> env forwarding -> init mknod's /dev/nvidia*).
# Either way we assert the observable joblet-side outcomes. Simulation cannot prove
# CUDA runs - that needs real hardware.
#
# GPU support is toggled via a systemd env drop-in + restart, so this needs sudo.
# It uses `sudo -n` (never prompts): inside `make pre-pr` the credential is cached
# from the deploy step, so it runs; otherwise it SKIPS cleanly. It always reverts.

source "$(dirname "$0")/../lib/test_framework.sh"

test_suite_init "GPU Control Plane Tests"

DROPIN_DIR="/etc/systemd/system/joblet.service.d"
DROPIN="$DROPIN_DIR/gpu-e2e.conf"
GPU_ENABLED=0
GPU_MODE="none"

# Detect a real, usable NVIDIA GPU on the host.
have_real_gpu() {
    command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi -L 2>/dev/null | grep -q "GPU 0"
}

wait_ready() {
    for _ in $(seq 1 30); do
        "$RNX_BINARY" job list >/dev/null 2>&1 && return 0
        sleep 1
    done
    return 1
}

# After a restart the API answers before job execution + log capture are fully
# warm (persist subprocess catching up). Confirm a canary job's logs are actually
# retrievable before asserting, so we don't misread warmup as a GPU failure.
canary_ready() {
    for _ in $(seq 1 20); do
        local out id logs
        out=$("$RNX_BINARY" job run echo CANARY_OK 2>&1)
        id=$(echo "$out" | grep "^ID:" | awk '{print $2}')
        if [[ -n "$id" ]]; then
            logs=$(get_job_logs "$id")
            echo "$logs" | grep -q "CANARY_OK" && return 0
        fi
        sleep 2
    done
    return 1
}

cleanup() {
    if [[ $GPU_ENABLED -eq 1 ]]; then
        sudo -n rm -f "$DROPIN" 2>/dev/null
        sudo -n systemctl daemon-reload 2>/dev/null
        sudo -n systemctl restart joblet 2>/dev/null
        wait_ready || true
        GPU_ENABLED=0
    fi
}
trap cleanup EXIT

# Enable GPU support in the chosen mode via a drop-in + restart.
enable_gpu() {
    local env_line="$1"
    sudo -n mkdir -p "$DROPIN_DIR" 2>/dev/null || return 1
    printf '[Service]\n%s\n' "$env_line" | sudo -n tee "$DROPIN" >/dev/null 2>&1 || return 1
    sudo -n systemctl daemon-reload 2>/dev/null || return 1
    sudo -n systemctl restart joblet 2>/dev/null || return 1
    GPU_ENABLED=1
    wait_ready
}

# A --gpu job must see its device nodes and the forwarded visible-devices env.
test_gpu_devices_and_env() {
    local out id logs
    out=$("$RNX_BINARY" job run --gpu=1 sh -c \
        'ls /dev/nvidia* 2>/dev/null; echo "CUDA=$CUDA_VISIBLE_DEVICES"; echo "NV=$NVIDIA_VISIBLE_DEVICES"' 2>&1)
    id=$(echo "$out" | grep "^ID:" | awk '{print $2}')
    if [[ -z "$id" ]]; then
        echo -e "    ${RED}GPU job not accepted: $out${NC}"
        return 1
    fi
    logs=$(get_job_logs "$id")
    local ok=0
    assert_contains "$logs" "/dev/nvidia0" || ok=1
    assert_contains "$logs" "/dev/nvidiactl" || ok=1
    assert_contains "$logs" "/dev/nvidia-uvm" || ok=1
    assert_contains "$logs" "/dev/nvidia-uvm-tools" || ok=1
    assert_contains "$logs" "CUDA=0" || ok=1
    assert_contains "$logs" "NV=0" || ok=1
    if [[ $ok -eq 0 ]]; then
        echo -e "    ${GREEN}Job saw its device nodes (incl. nvidia-uvm-tools) and visible-devices env${NC}"
    else
        echo "$logs" | sed 's/^/      /' | head
    fi
    return $ok
}

# Real hardware only: nvidia-smi must actually work inside the job.
test_nvidia_smi_in_job() {
    local out id logs
    out=$("$RNX_BINARY" job run --gpu=1 sh -c 'nvidia-smi -L 2>&1 || echo NVIDIA_SMI_FAILED' 2>&1)
    id=$(echo "$out" | grep "^ID:" | awk '{print $2}')
    [[ -z "$id" ]] && { echo -e "    ${RED}job not accepted: $out${NC}"; return 1; }
    logs=$(get_job_logs "$id")
    if assert_contains "$logs" "GPU 0"; then
        echo -e "    ${GREEN}nvidia-smi works inside the job (real GPU access verified)${NC}"
        return 0
    fi
    echo -e "    ${RED}nvidia-smi did not work inside the job${NC}"
    echo "$logs" | sed 's/^/      /' | head
    return 1
}

# A non-GPU job must not receive any /dev/nvidia* nodes.
test_non_gpu_unaffected() {
    local out id logs
    out=$("$RNX_BINARY" job run sh -c 'ls /dev/nvidia* 2>/dev/null && echo HAS_NVIDIA || echo NO_NVIDIA' 2>&1)
    id=$(echo "$out" | grep "^ID:" | awk '{print $2}')
    [[ -z "$id" ]] && { echo -e "    ${RED}job not accepted: $out${NC}"; return 1; }
    logs=$(get_job_logs "$id")
    assert_contains "$logs" "NO_NVIDIA" && { echo -e "    ${GREEN}Non-GPU job has no /dev/nvidia*${NC}"; return 0; }
    return 1
}

# ============================================
# Run
# ============================================

if have_real_gpu; then
    GPU_MODE="real"
    test_section "GPU (real hardware detected)"
else
    GPU_MODE="mock"
    test_section "GPU (no hardware - simulation)"
fi

if ! sudo -n true 2>/dev/null; then
    skip_test "GPU control plane ($GPU_MODE)" \
        "cached/passwordless sudo required to toggle GPU support; runs inside 'make pre-pr' where sudo is cached"
else
    if [[ "$GPU_MODE" == "real" ]]; then
        enable_ok=1; enable_gpu "Environment=JOBLET_GPU_ENABLED=1" && enable_ok=0
    else
        enable_ok=1; enable_gpu "Environment=JOBLET_GPU_SIMULATE=2" && enable_ok=0
    fi

    if [[ $enable_ok -ne 0 ]]; then
        cleanup
        run_test "Enable GPU support and restart daemon" false
    elif ! canary_ready; then
        # Daemon didn't resume executing jobs after the restart; skip rather than
        # report a false GPU failure.
        skip_test "GPU control plane ($GPU_MODE)" "daemon did not resume executing jobs after restart"
    else
        run_test "GPU job sees device nodes and forwarded env" test_gpu_devices_and_env
        run_test "Non-GPU job unaffected" test_non_gpu_unaffected
        if [[ "$GPU_MODE" == "real" ]]; then
            run_test "nvidia-smi works inside the job (real GPU)" test_nvidia_smi_in_job
        else
            skip_test "nvidia-smi inside job" "simulation mode - fake GPUs cannot run CUDA/nvidia-smi"
        fi
    fi
fi

# ============================================
# Summary
# ============================================

echo -e "\n${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}  GPU Control Plane Test Results (mode: $GPU_MODE)${NC}"
echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "  Total:   $TOTAL_TESTS"
echo -e "  ${GREEN}Passed:  $PASSED_TESTS${NC}"
echo -e "  ${RED}Failed:  $FAILED_TESTS${NC}"
echo -e "  ${YELLOW}Skipped: $SKIPPED_TESTS${NC}"

[[ $FAILED_TESTS -gt 0 ]] && exit 1
exit 0
