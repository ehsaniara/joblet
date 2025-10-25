# GPU Testing Guide

This guide covers comprehensive testing approaches for Joblet's GPU support, from unit tests to production validation.

## Testing Levels

### 1. Unit Tests

#### Run GPU Unit Tests

```bash
# Test GPU manager functionality
go test ./internal/joblet/gpu -v

# Test GPU integration components
GPU_TEST_MODE=mock go test ./tests/gpu -v
```

#### What Unit Tests Cover

- GPU allocation/deallocation logic
- Memory requirement validation
- Concurrent access handling
- Error conditions and edge cases

### 2. Integration Tests

#### Mock GPU Testing

```bash
# Set mock mode for testing without real GPUs
export GPU_TEST_MODE=mock
go test ./tests/gpu -v

# Run E2E tests with mock GPUs
./tests/e2e/tests/09_gpu_test.sh
```

#### What Integration Tests Cover

- End-to-end GPU job workflow
- CLI flag parsing (`--gpu`, `--gpu-memory`)
- JSON API responses
- Workflow GPU allocation
- Error handling and resource cleanup

### 3. System Tests (Real GPUs)

#### Prerequisites

- NVIDIA GPU installed
- NVIDIA drivers installed
- CUDA toolkit available (optional)

#### Real GPU Testing

```bash
# Check GPU availability first
nvidia-smi

# Test with real GPUs
export GPU_TEST_MODE=real
./tests/e2e/tests/09_gpu_test.sh

# Test specific GPU scenarios
rnx job run --gpu=1 --gpu-memory=4GB nvidia-smi
rnx job run --gpu=2 --gpu-memory=8GB python gpu_test.py
```

#### What System Tests Cover

- Actual GPU allocation and isolation
- CUDA environment setup
- Device node creation (`/dev/nvidia*`)
- Real GPU memory limits
- Multi-GPU scenarios

## Test Scenarios by Environment

### Development Environment (No GPU Hardware)

```bash
# 1. Unit tests (always run)
go test ./internal/joblet/gpu -v

# 2. Mock integration tests
GPU_TEST_MODE=mock go test ./tests/gpu -v

# 3. E2E with mock mode
GPU_TEST_MODE=mock ./tests/e2e/tests/09_gpu_test.sh

# 4. Verify CLI flags work
rnx job run --gpu=1 --gpu-memory=4GB echo "Mock GPU test"
```

### Staging Environment (With GPU Hardware)

```bash
# 1. All development tests first
GPU_TEST_MODE=mock ./tests/e2e/tests/09_gpu_test.sh

# 2. Real GPU functionality
GPU_TEST_MODE=real ./tests/e2e/tests/09_gpu_test.sh

# 3. Stress testing
for i in {1..5}; do
  rnx job run --gpu=1 --gpu-memory=4GB sleep 10 &
done
wait

# 4. GPU cleanup verification
rnx monitor status  # Should show GPUs as available
```

### Production Environment

```bash
# 1. Smoke tests
rnx job run --gpu=1 echo "Production GPU test"

# 2. Resource monitoring
rnx monitor status  # Check GPU status

# 3. Workflow validation
rnx workflow run examples/workflows/gpu-ml-pipeline.yaml

# 4. Load testing (carefully)
# Run multiple GPU jobs to test resource management
```

## Manual Test Cases

### Test Case 1: Basic GPU Job

```bash
# Expected: Job completes successfully
rnx job run --gpu=1 --gpu-memory=4GB echo "Hello GPU"

# Verify:
# - Job shows COMPLETED status
# - Logs contain expected output
# - GPU is released after completion
```

### Test Case 2: GPU Memory Validation

```bash
# Expected: Job fails with insufficient memory error
rnx job run --gpu=1 --gpu-memory=100GB echo "Too much memory"

# Verify:
# - Job fails with descriptive error
# - No GPU is allocated
# - System remains stable
```

### Test Case 3: Multi-GPU Allocation

```bash
# Expected: Allocates 2 GPUs (if available)
rnx job run --gpu=2 --gpu-memory=4GB python distributed_training.py

# Verify:
# - CUDA_VISIBLE_DEVICES shows 2 GPU indices
# - Both GPUs marked as in-use during execution
# - Both GPUs released after completion
```

### Test Case 4: GPU Workflow

```bash
# Create test workflow
cat > gpu_workflow.yaml << EOF
jobs:
  data-prep:
    command: "echo"
    args: ["Preprocessing data"]
    resources:
      max_memory: 1024

  training:
    command: "echo"
    args: ["Training model"]
    requires:
      - data-prep: "COMPLETED"
    resources:
      gpu_count: 1
      gpu_memory_mb: 4096
      max_memory: 2048
EOF

rnx workflow run gpu_workflow.yaml

# Verify:
# - data-prep runs without GPU
# - training job gets GPU allocation
# - Jobs execute in correct order
```

### Test Case 5: GPU Isolation

```bash
# Run two GPU jobs simultaneously
rnx job run --gpu=1 sleep 30 &
JOB1_PID=$!
rnx job run --gpu=1 sleep 30 &
JOB2_PID=$!

# Verify:
# - If 2+ GPUs: both jobs get different GPUs
# - If 1 GPU: second job waits or fails appropriately
# - No GPU conflicts or shared access

wait $JOB1_PID $JOB2_PID
```

## Debugging GPU Issues

### Check GPU Discovery

```bash
# On the Joblet server
nvidia-smi                    # Verify GPUs are detected by system
ls /proc/driver/nvidia/gpus/  # Check GPU proc entries
cat /proc/driver/nvidia/gpus/*/information  # GPU details
```

### Check Joblet GPU Status

```bash
# Monitor GPU status
rnx monitor status

# Check server logs for GPU messages
journalctl -u joblet -f | grep -i gpu

# Verify job GPU allocation
rnx job status <job-uuid>  # Should show GPU info if allocated
```

### Check Job GPU Environment

```bash
# Create debug job
rnx job run --gpu=1 bash -c "
  echo 'CUDA_VISIBLE_DEVICES='$CUDA_VISIBLE_DEVICES
  ls -la /dev/nvidia*
  nvidia-smi || echo 'nvidia-smi not available'
  env | grep CUDA
"
```

### Common Issues and Solutions

**Issue**: "No GPUs available"

- **Check**: `nvidia-smi` output
- **Solution**: Verify NVIDIA drivers installed

**Issue**: "Insufficient GPU memory"

- **Check**: GPU memory with `nvidia-smi`
- **Solution**: Reduce `--gpu-memory` requirement

**Issue**: "GPU allocation failed"

- **Check**: Server logs for specific error
- **Solution**: Check if other jobs are using GPUs

**Issue**: Jobs hang waiting for GPU

- **Check**: Current GPU allocations
- **Solution**: Release stuck GPU allocations or increase timeout

## Performance Testing

### GPU Throughput Test

```bash
# Test multiple sequential GPU jobs
time for i in {1..10}; do
  rnx job run --gpu=1 sleep 5
done
```

### GPU Memory Stress Test

```bash
# Test with various memory requirements
for memory in 1GB 4GB 8GB 16GB; do
  rnx job run --gpu=1 --gpu-memory=$memory echo "Memory test: $memory"
done
```

### Concurrent GPU Jobs

```bash
# Test concurrent job handling
for i in {1..5}; do
  rnx job run --gpu=1 sleep 10 &
done
wait
```

## Continuous Integration

### CI Pipeline GPU Tests

```yaml
# .github/workflows/gpu-tests.yml
- name: Run GPU Unit Tests
  run: go test ./internal/joblet/gpu -v

- name: Run GPU Integration Tests (Mock)
  run: GPU_TEST_MODE=mock go test ./tests/gpu -v

- name: Run GPU E2E Tests (Mock)
  run: GPU_TEST_MODE=mock ./tests/e2e/tests/09_gpu_test.sh
```

### GPU Test Matrix

Test different configurations:

- Mock GPU vs Real GPU
- Single GPU vs Multi-GPU
- Various memory requirements
- Different CUDA versions (if applicable)

## Test Data and Fixtures

### Sample GPU Workloads

Create test Python scripts:

**gpu_test.py** (basic CUDA test):

```python
try:
    import torch

    print(f"PyTorch version: {torch.__version__}")
    print(f"CUDA available: {torch.cuda.is_available()}")
    if torch.cuda.is_available():
        print(f"GPU count: {torch.cuda.device_count()}")
        print(f"Current GPU: {torch.cuda.current_device()}")
        print(f"GPU name: {torch.cuda.get_device_name()}")
except ImportError:
    print("PyTorch not available - basic GPU test")
    import os

    print(f"CUDA_VISIBLE_DEVICES: {os.environ.get('CUDA_VISIBLE_DEVICES', 'not set')}")
```

**gpu_memory_test.py** (memory usage test):

```python
import os
import time

try:
    import torch

    device = torch.cuda.current_device()
    # Allocate some GPU memory
    tensor = torch.randn(1000, 1000, device=device)
    print(f"Allocated tensor on GPU {device}")
    time.sleep(5)  # Hold memory for 5 seconds
    del tensor
    torch.cuda.empty_cache()
    print("Memory freed")
except ImportError:
    print("PyTorch not available")
```

This comprehensive testing approach ensures GPU support works reliably across all environments and use cases.