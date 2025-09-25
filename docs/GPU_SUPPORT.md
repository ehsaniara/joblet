# GPU Support Guide

Joblet provides comprehensive GPU support for machine learning, data processing, and compute-intensive workloads. This guide shows you how to run GPU-accelerated jobs with proper resource isolation and management.

## Overview

With Joblet's GPU support, you can:

- **Allocate specific GPUs** to jobs with automatic discovery
- **Set GPU memory requirements** to ensure sufficient resources
- **Isolate GPU access** between jobs for security and performance
- **Monitor GPU usage** through the web interface and CLI
- **Use CUDA environments** with automatic library mounting

## Quick Start

### Basic GPU Job

```bash
# Run a simple GPU-accelerated Python script
rnx job run --gpu=1 python gpu_hello.py

# Specify minimum GPU memory (8GB in this example)
rnx job run --gpu=1 --gpu-memory=8GB python training_script.py
```

### Multiple GPUs

```bash
# Use 2 GPUs for distributed training
rnx job run --gpu=2 --runtime=python-3.11-ml python distributed_training.py

# Combine with other resource limits
rnx job run --gpu=2 --gpu-memory=16GB --max-memory=32768 --max-cpu=800 \
    python intensive_gpu_workload.py
```

## Command Line Options

### GPU Flags

- `--gpu=N`: Request N number of GPUs for the job
- `--gpu-memory=SIZE`: Minimum GPU memory requirement per GPU
  - Supports units: `8GB`, `4096MB`, or raw numbers (MB)
  - Examples: `--gpu-memory=8GB`, `--gpu-memory=4096`

### Examples

```bash
# Single GPU with 4GB minimum memory
rnx job run --gpu=1 --gpu-memory=4GB python inference.py

# Two GPUs with 8GB each
rnx job run --gpu=2 --gpu-memory=8GB python multi_gpu_training.py

# GPU job with CPU and memory limits
rnx job run --gpu=1 --max-cpu=400 --max-memory=16384 python hybrid_workload.py
```

## Workflow YAML Configuration

You can specify GPU requirements in workflow YAML files:

```yaml
# ml-training-pipeline.yaml
jobs:
  data-preprocessing:
    command: "python3"
    args: ["preprocess.py"]
    runtime: "python-3.11-ml"
    resources:
      max_memory: 4096
      max_cpu: 200

  model-training:
    command: "python3"
    args: ["train.py", "--epochs", "100"]
    runtime: "python-3.11-ml"
    requires:
      - data-preprocessing: "COMPLETED"
    resources:
      max_memory: 16384
      max_cpu: 800
      gpu_count: 2          # Request 2 GPUs
      gpu_memory_mb: 8192   # 8GB minimum per GPU

  model-evaluation:
    command: "python3"
    args: ["evaluate.py"]
    runtime: "python-3.11-ml"
    requires:
      - model-training: "COMPLETED"
    resources:
      max_memory: 8192
      gpu_count: 1
      gpu_memory_mb: 4096
```

```bash
# Run the GPU-enabled workflow
rnx job run --workflow=ml-training-pipeline.yaml
```

## GPU Environment

### CUDA Support

When you request GPUs, Joblet automatically:

1. **Discovers NVIDIA GPUs** using `/proc/driver/nvidia/gpus/` and `nvidia-smi`
2. **Allocates specific GPUs** to your job (you'll see which ones in job status)
3. **Sets up device permissions** so your job can access the GPU devices
4. **Mounts CUDA libraries** from common installation paths
5. **Sets CUDA_VISIBLE_DEVICES** environment variable

### Supported CUDA Paths

Joblet automatically detects and mounts CUDA from these common locations:
- `/usr/local/cuda`
- `/opt/cuda`
- `/usr/lib/cuda`
- Custom paths configured in joblet configuration

### Environment Variables

Your GPU jobs automatically get these environment variables:

```bash
CUDA_VISIBLE_DEVICES=0,1    # Specific GPU indices allocated to your job
NVIDIA_VISIBLE_DEVICES=0,1  # Alternative naming for some frameworks
```

## Monitoring and Status

### Check Job Status

```bash
# View job status including GPU allocation
rnx job status abc123de-f456-7890-1234-567890abcdef

# Sample output:
# UUID: abc123de-f456-7890-1234-567890abcdef
# Status: RUNNING
# GPUs: [0, 1] (2 GPUs allocated)
# GPU Memory: 8192 MB required per GPU
```

### Web Interface

The Joblet web interface shows:
- GPU allocation status for running jobs
- Which specific GPUs are assigned to each job
- GPU memory requirements and usage
- Historical GPU job information

## Real-World Examples

### Machine Learning Training

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

### Data Processing

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

### AI Inference

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

## Best Practices

### Resource Planning

1. **Check GPU memory requirements** for your models beforehand
2. **Request appropriate GPU count** - don't over-allocate
3. **Combine with CPU/memory limits** for balanced resource usage
4. **Use workflows** to chain GPU and non-GPU jobs efficiently

### Example Resource Planning

```bash
# Good: Balanced resource allocation
rnx job run --gpu=2 --gpu-memory=8GB --max-memory=16384 --max-cpu=800 \
    python training.py

# Avoid: Over-requesting resources
# rnx job run --gpu=8 --gpu-memory=32GB --max-memory=128000 \
#     python simple_inference.py
```

### Job Organization

```bash
# Separate preprocessing (CPU-only) from training (GPU)
rnx job run --max-cpu=400 --max-memory=8192 python preprocess.py
rnx job run --gpu=1 --gpu-memory=8GB python train.py

# Use workflows for complex pipelines
rnx job run --workflow=ml-pipeline.yaml
```

### Development vs Production

```bash
# Development: Single GPU for testing
rnx job run --gpu=1 --gpu-memory=4GB python test_model.py

# Production: Multiple GPUs for performance
rnx job run --gpu=4 --gpu-memory=16GB python production_training.py
```

## Troubleshooting

### Common Issues

**1. "No GPUs available"**
- Check if NVIDIA drivers are installed: `nvidia-smi`
- Verify Joblet can detect GPUs: Check server logs
- Ensure other jobs aren't using all available GPUs

**2. "Insufficient GPU memory"**
- Reduce `--gpu-memory` requirement
- Check actual GPU memory with `nvidia-smi`
- Consider using multiple smaller GPUs instead

**3. "CUDA not found in job"**
- Verify CUDA is installed on the host system
- Check Joblet configuration for CUDA paths
- Ensure CUDA libraries are accessible

### Debug Commands

```bash
# Check GPU availability on the system
nvidia-smi

# View job details including GPU allocation
rnx job status --detail <job-uuid>

# Check server logs for GPU-related errors
# (Server-side debugging - contact your administrator)
```

## Configuration

### Server Configuration

Your Joblet administrator can configure GPU support in the server config:

```yaml
# joblet-config.yml
gpu:
  enabled: true
  cuda_paths:
    - "/usr/local/cuda"
    - "/opt/cuda"
    - "/usr/lib/cuda"
```

### Runtime Environments

The `python-3.11-ml` runtime includes:
- CUDA-compatible Python packages
- Common ML frameworks (PyTorch, TensorFlow, etc.)
- GPU utilities and monitoring tools

## Migration from Docker

If you're coming from Docker-based GPU workflows:

### Docker Equivalent

```bash
# Old Docker approach
docker run --gpus=2 --shm-size=16g nvidia/cuda:11.8-devel python train.py

# New Joblet approach
rnx job run --gpu=2 --max-memory=16384 --runtime=python-3.11-ml python train.py
```

### Benefits

- **No container images** to build or manage
- **Direct process access** for debugging
- **Better resource isolation** with cgroups
- **Integrated monitoring** and job management
- **Native Linux performance** without container overhead

---

## Next Steps

- See [RNX CLI Reference](./RNX_CLI_REFERENCE.md) for complete command options
- Check out [Workflows Guide](./WORKFLOWS.md) for complex GPU pipelines
- Review [Runtime System](./RUNTIME_SYSTEM.md) for pre-built ML environments
- Explore [API Reference](./API.md) for programmatic GPU job submission