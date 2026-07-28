# GPU Acceleration and CUDA Support Guide

This comprehensive guide details Joblet's GPU acceleration capabilities for high-performance computing, machine
learning, and data-intensive workloads. Joblet provides enterprise-grade GPU resource management with deterministic
allocation, memory isolation, and CUDA environment integration.

## Executive Overview

> ⚠️ **Status:** GPU support is currently experimental. GPU discovery, allocation bookkeeping,
> request validation, and `CUDA_VISIBLE_DEVICES` / `NVIDIA_VISIBLE_DEVICES` env injection work today.
> Per-job device-node creation (`/dev/nvidia*`) and automatic CUDA library provisioning are now wired
> into the execution path, but this path has **not yet been validated on a host with an NVIDIA GPU**.
> Runtime GPU memory enforcement is still **not implemented** — see the notes in the sections below.

Joblet's GPU orchestration framework is designed to enable organizations to:

- **Automated GPU Discovery and Allocation**: Assignment of GPU resources based on job requirements *(working)*
- **Memory Quota Management** *(not yet enforced)*: `--gpu-memory` filters candidate GPUs at selection time only; it is
  not enforced at runtime
- **Hardware-Level Isolation** *(implemented, pending GPU-hardware validation)*: Per-job `/dev/nvidia*` device nodes are
  now created inside the job; cgroup device rules are still not applied
- **Comprehensive Observability**: GPU allocation metadata through multiple interfaces
- **CUDA Integration**: `CUDA_VISIBLE_DEVICES` env configuration *(working)*; automatic CUDA library provisioning is
  *implemented, pending GPU-hardware validation*

## Getting Started with GPU Workloads

### Basic GPU Job Submission

```bash
# Run a simple GPU-accelerated Python script
rnx job run --gpu=1 python gpu_hello.py

# Specify minimum GPU memory (8GB in this example)
rnx job run --gpu=1 --gpu-memory=8GB python training_script.py
```

### Multi-GPU Configuration

```bash
# Use 2 GPUs for distributed training
rnx job run --gpu=2 --runtime=python-3.11-ml python distributed_training.py

# Combine with other resource limits
rnx job run --gpu=2 --gpu-memory=16GB --max-memory=32768 --max-cpu=800 \
    python intensive_gpu_workload.py
```

## Command-Line Interface Options

### GPU Parameter Reference

- `--gpu=N`: Specifies the number of GPU devices to allocate for job execution
- `--gpu-memory=SIZE`: Defines minimum GPU memory requirement per device
    - Supported formats: `8GB`, `4096MB`, or numeric values in megabytes
    - Usage examples: `--gpu-memory=8GB`, `--gpu-memory=4096`

### Practical Examples

```bash
# Single GPU with 4GB minimum memory
rnx job run --gpu=1 --gpu-memory=4GB python inference.py

# Two GPUs with 8GB each
rnx job run --gpu=2 --gpu-memory=8GB python multi_gpu_training.py

# GPU job with CPU and memory limits
rnx job run --gpu=1 --max-cpu=400 --max-memory=16384 python hybrid_workload.py
```

## GPU Runtime Environment

### CUDA Runtime Integration

> ⚠️ **Current state:** Per-job device-node passthrough (`/dev/nvidia*`) and CUDA library
> bind-mounting are now wired into the execution path: the server forwards the allocated GPU indices
> and detected CUDA paths to the init process (via the `JOB_GPU_INDICES` / `JOB_GPU_CUDA_MOUNTS` env
> vars), which creates the device nodes post-chroot and bind-mounts the CUDA libraries read-only
> pre-chroot. This path has **not yet been validated on a host with an NVIDIA GPU**. cgroup device
> access controls are still **not applied**.

Upon GPU allocation request, Joblet performs the following operations:

1. **GPU Discovery**: Enumerates available NVIDIA devices via `/proc/driver/nvidia/gpus/` and `nvidia-smi` interfaces
2. **Device Allocation**: Assigns specific GPU indices to job instances with tracking in job metadata (in-memory bookkeeping)
3. **Device Node Creation** *(implemented, pending GPU-hardware validation)*: Creates the per-job `/dev/nvidia*`,
   `/dev/nvidiactl`, and `/dev/nvidia-uvm` nodes inside the job (post-chroot) for the allocated GPU indices. Note:
   cgroup device-access rules are still not applied
4. **Library Provisioning** *(implemented, pending GPU-hardware validation)*: Bind-mounts the detected host CUDA
   directories read-only into the job (pre-chroot)
5. **Environment Setup**: Configures `CUDA_VISIBLE_DEVICES` and `NVIDIA_VISIBLE_DEVICES` for framework compatibility

### CUDA Library Path Resolution

> ⚠️ **Implemented, pending GPU-hardware validation:** CUDA library bind-mounting into the job
> namespace is now wired into the execution path; the detected CUDA directories are bind-mounted
> read-only into the job. This has not yet been validated on a host with an NVIDIA GPU. The paths
> below describe where Joblet discovers CUDA installations.

Joblet discovers CUDA installations from standard locations:

- `/usr/local/cuda`
- `/opt/cuda`
- `/usr/lib/cuda`
- Custom paths configured in joblet configuration

### Runtime Environment Variables

GPU-enabled jobs receive the following environment configuration:

```bash
CUDA_VISIBLE_DEVICES=0,1    # Specific GPU indices allocated to your job
NVIDIA_VISIBLE_DEVICES=0,1  # Alternative naming for some frameworks
```

## Monitoring and Observability

### Job Status Inspection

```bash
# View job status including GPU allocation
rnx job status abc123de-f456-7890-1234-567890abcdef

# Sample output:
# UUID: abc123de-f456-7890-1234-567890abcdef
# Status: RUNNING
# GPUs: [0, 1] (2 GPUs allocated)
# GPU Memory: 8192 MB required per GPU
```

### Administrative Web Console

The Joblet management interface provides:

- Real-time GPU allocation status for active workloads
- Device-specific assignment mapping for job instances
- Memory utilization metrics and quota enforcement status
- Historical GPU job execution analytics and trends

## Production Use Cases

### Deep Learning Model Training

```bash
# PyTorch distributed training
rnx job run --gpu=4 --gpu-memory=16GB --max-memory=65536 \
    --runtime=python-3.11-ml \
    --upload=model.py --upload=dataset/ \
    python -m torch.distributed.launch --nproc_per_node=4 model.py

# TensorFlow model training
rnx job run --gpu=2 --gpu-memory=8GB \
    --runtime=python-3.11-ml \
    python tensorflow_training.py --batch-size=64
```

### GPU-Accelerated Data Processing

```bash
# RAPIDS GPU-accelerated data processing
rnx job run --gpu=1 --gpu-memory=12GB \
    --volume=data-lake \
    --runtime=python-3.11-ml \
    python rapids_etl.py

# GPU-accelerated analytics
rnx job run --gpu=1 --max-memory=16384 \
    --runtime=python-3.11-ml \
    python cupy_analytics.py
```

### Artificial Intelligence Inference

```bash
# Large language model inference
rnx job run --gpu=1 --gpu-memory=24GB \
    --runtime=python-3.11-ml \
    python llm_inference.py --model=llama2-70b

# Computer vision inference pipeline
rnx job run --gpu=1 --gpu-memory=8GB \
    --volume=images \
    --runtime=python-3.11-ml \
    python vision_pipeline.py
```

## Best Practices and Recommendations

### Resource Planning Guidelines

1. **Pre-validate Memory Requirements**: Profile model memory consumption before production deployment
2. **Right-size GPU Allocation**: Request only necessary GPU resources to maximize cluster efficiency
3. **Balance Resource Profiles**: Configure complementary CPU and memory limits for optimal performance

### Resource Allocation Patterns

```bash
# Good: Balanced resource allocation
rnx job run --gpu=2 --gpu-memory=8GB --max-memory=16384 --max-cpu=800 \
    python training.py

# Avoid: Over-requesting resources
# rnx job run --gpu=8 --gpu-memory=32GB --max-memory=128000 \
#     python simple_inference.py
```

### Workload Organization Strategies

```bash
# Separate preprocessing (CPU-only) from training (GPU)
rnx job run --max-cpu=400 --max-memory=8192 python preprocess.py
rnx job run --gpu=1 --gpu-memory=8GB python train.py
```

### Environment-Specific Configuration

```bash
# Development: Single GPU for testing
rnx job run --gpu=1 --gpu-memory=4GB python test_model.py

# Production: Multiple GPUs for performance
rnx job run --gpu=4 --gpu-memory=16GB python production_training.py
```

## Troubleshooting and Diagnostics

### Common Issues and Resolution

**Issue: "No GPUs available"**

- Verify NVIDIA driver installation: Execute `nvidia-smi` on the host system
- Confirm Joblet GPU detection: Review server logs for device enumeration
- Check resource availability: Ensure GPUs are not fully allocated to other workloads

**Issue: "Insufficient GPU memory"**

- Adjust memory requirements: Lower `--gpu-memory` parameter value
- Validate available memory: Query device capacity using `nvidia-smi`
- Consider distributed approach: Utilize multiple GPUs with smaller memory footprints

**Issue: "CUDA libraries not accessible"**

- Confirm CUDA installation: Verify CUDA toolkit presence on host system
- Review path configuration: Check Joblet server configuration for CUDA library paths
- Validate library accessibility: Ensure proper permissions and mount points

### Diagnostic Commands

```bash
# Check GPU availability on the system
nvidia-smi

# View job details including GPU allocation
rnx job status --detail <job-uuid>

# Check server logs for GPU-related errors
# (Server-side debugging - contact your administrator)
```

## System Configuration

### Server-Side GPU Configuration

System administrators configure GPU support through the Joblet server configuration file:

```yaml
# joblet-config.yml
gpu:
  enabled: true
  cuda_paths:
    - "/usr/local/cuda"
    - "/opt/cuda"
    - "/usr/lib/cuda"
```

### Pre-Configured Runtime Environments

The `python-3.11-ml` runtime environment provides:

- CUDA-optimized Python packages and libraries
- Production-ready ML frameworks (PyTorch, TensorFlow, JAX)
- GPU profiling and diagnostic utilities

## Migration from Container-Based GPU Workloads

For teams transitioning from Docker or Kubernetes GPU deployments:

### Container Command Equivalents

```bash
# Old Docker approach
docker run --gpus=2 --shm-size=16g nvidia/cuda:11.8-devel python train.py

# New Joblet approach
rnx job run --gpu=2 --max-memory=16384 --runtime=python-3.11-ml python train.py
```

### Operational Advantages

- **Eliminated Container Overhead**: No image build cycles or registry management
- **Enhanced Debugging Capabilities**: Direct process introspection and profiling
- **Superior Resource Isolation**: Kernel-level cgroups v2 enforcement
- **Unified Observability**: Integrated metrics and monitoring infrastructure
- **Optimal Performance**: Native Linux execution without virtualization layers

---

## Additional Resources

- See [RNX CLI Reference](./RNX_CLI_REFERENCE.md) for complete command options
- Review [Runtime System](./RUNTIME_SYSTEM.md) for pre-built ML environments
- Explore [API Reference](./API.md) for programmatic GPU job submission
- Check [Job Execution Guide](./JOB_EXECUTION.md) for complex GPU job patterns