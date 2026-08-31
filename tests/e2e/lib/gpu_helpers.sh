#!/bin/bash

# GPU e2e helpers: toggle joblet GPU support (real or simulated) through a
# systemd env drop-in + restart. Needs cached/passwordless sudo (sudo -n).
# Requires test_framework.sh to be sourced first (RNX_BINARY, get_job_logs).

DROPIN_DIR="/etc/systemd/system/joblet.service.d"
DROPIN="$DROPIN_DIR/gpu-e2e.conf"
GPU_ENABLED=0

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
# warm; confirm a canary job's logs are retrievable before asserting anything.
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

# Revert the drop-in and restart so GPU support is off again.
gpu_cleanup() {
    if [[ $GPU_ENABLED -eq 1 ]]; then
        sudo -n rm -f "$DROPIN" 2>/dev/null
        sudo -n systemctl daemon-reload 2>/dev/null
        sudo -n systemctl restart joblet 2>/dev/null
        wait_ready || true
        GPU_ENABLED=0
    fi
}

# enable_gpu "Environment=JOBLET_GPU_ENABLED=1"  (real)
# enable_gpu "Environment=JOBLET_GPU_SIMULATE=2" (simulation, 2 fake GPUs)
enable_gpu() {
    local env_line="$1"
    sudo -n mkdir -p "$DROPIN_DIR" 2>/dev/null || return 1
    printf '[Service]\n%s\n' "$env_line" | sudo -n tee "$DROPIN" >/dev/null 2>&1 || return 1
    sudo -n systemctl daemon-reload 2>/dev/null || return 1
    sudo -n systemctl restart joblet 2>/dev/null || return 1
    GPU_ENABLED=1
    wait_ready
}
