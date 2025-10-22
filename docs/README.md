# Joblet: Enterprise Linux Job Execution Platform

Joblet is a comprehensive Linux-native job execution platform designed for enterprise workloads. It leverages Linux
namespaces and cgroups v2 to provide robust process isolation, resource management, and secure multi-tenant execution
environments without the overhead of containerization.

## 📚 Documentation Index

### Getting Started

- **[QUICKSTART.md](QUICKSTART.md)** - Quick start guide
- **[INSTALLATION.md](INSTALLATION.md)** - Installation instructions
- **[RNX_CLI_REFERENCE.md](RNX_CLI_REFERENCE.md)** - CLI command reference

### Architecture & Design

- **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture overview
- **[DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md)** - Developer workflows and build instructions
- **[CONFIG_GUIDE.md](CONFIG_GUIDE.md)** - Configuration reference
- **[DESIGN.md](DESIGN.md)** - Design principles
- **[API.md](API.md)** - gRPC API documentation

### Features

- **[GPU_SUPPORT.md](GPU_SUPPORT.md)** - GPU acceleration
- **[WORKFLOWS.md](WORKFLOWS.md)** - Workflow orchestration
- **[NETWORK_MANAGEMENT.md](NETWORK_MANAGEMENT.md)** - Network isolation
- **[VOLUME_MANAGEMENT.md](VOLUME_MANAGEMENT.md)** - Storage management
- **[RUNTIME_SYSTEM.md](RUNTIME_SYSTEM.md)** - Runtime environments
- **[PERSISTENCE.md](PERSISTENCE.md)** - Log and metric persistence (Local, CloudWatch)
- **[MONITORING.md](MONITORING.md)** - Metrics and observability

### Operations

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Deployment guide
- **[SECURITY.md](SECURITY.md)** - Security considerations
- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Common issues and solutions

---

## Executive Summary

Joblet delivers enterprise-grade job execution capabilities by combining native Linux kernel features with modern
orchestration patterns. The platform provides deterministic resource allocation, comprehensive security isolation, and
seamless integration with existing infrastructure through a unified gRPC API and intuitive command-line interface.

### Core Capabilities

- **Process Isolation**: Complete namespace separation (PID, network, mount, IPC, UTS) ensures zero cross-contamination
  between workloads
- **Resource Management**: Granular control over CPU, memory, I/O, and GPU resources through cgroups v2 integration
- **GPU Acceleration**: Native NVIDIA GPU support with automatic device allocation, CUDA environment provisioning, and
  memory isolation
- **Workflow Orchestration**: Sophisticated dependency management with directed acyclic graph (DAG) execution and
  conditional logic
- **Network Virtualization**: Software-defined networking with customizable CIDR blocks, traffic shaping, and inter-job
  communication policies
- **Storage Abstraction**: Flexible volume management supporting persistent and ephemeral storage with quota enforcement
- **Node Identification**: Unique node identification for distributed deployments with automatic UUID generation and job
  tracking
- **Observability**: Real-time metrics collection, structured logging, and comprehensive audit trails for compliance
  requirements
- **Data Persistence**: Dedicated persistence service (`joblet-persist`) with multiple storage backends including local
  filesystem and AWS CloudWatch Logs for cloud-native deployments, featuring multi-node support, high-performance log
  and metric storage with gzip compression, Unix socket IPC, and historical query capabilities

### Security Architecture

- **Mutual TLS (mTLS)**: Certificate-based authentication ensures end-to-end encryption and identity verification
- **Role-Based Access Control (RBAC)**: Fine-grained permission model with administrative, operational, and read-only
  access tiers
- **Privilege Containment**: Kernel-enforced process isolation eliminates privilege escalation vectors
- **Network Segmentation**: Default-deny networking with explicit policy-based connectivity between workloads
- **Audit Compliance**: Comprehensive activity logging with tamper-resistant audit trails for regulatory requirements

### Management Interfaces

- **RNX Command-Line Interface**: Cross-platform client supporting Linux, macOS, and Windows environments with full
  feature parity
- **Joblet Admin UI**: [Standalone React-based management dashboard](https://github.com/ehsaniara/joblet-admin)
  providing real-time system monitoring and job
  orchestration capabilities via direct gRPC connectivity
- **Log Aggregation**: Streaming log infrastructure with advanced filtering, pattern matching, and retention policies
- **Runtime Catalog**: Curated collection of production-ready runtime environments including Python, Python ML with CUDA
  libraries, and JVM-based platforms

## Enterprise Use Cases

### Continuous Integration and Deployment

```bash
# Run jobs with pre-built runtime environments
rnx job run --runtime=python-3.11-ml pytest tests/
rnx job run --runtime=openjdk-21 --upload=pom.xml --upload=src/ mvn clean install
```

### Data Engineering and Analytics Workloads

```bash
# Isolated data processing with resource limits
rnx job run --max-memory=8192 --max-cpu=400 \
        --volume=data-lake \
        --runtime=python-3.11-ml \
        python process_big_data.py

# GPU-accelerated data processing
rnx job run --gpu=2 --gpu-memory=16GB \
        --max-memory=16384 \
        --runtime=python-3.11-ml \
        python gpu_analysis.py
```

### Microservices Testing and Validation

```bash
# Network-isolated service testing
rnx network create test-env --cidr=10.10.0.0/24
rnx job run --network=test-env --runtime=openjdk-21 ./service-a
rnx job run --network=test-env --runtime=python-3.11-ml ./service-b
```

### Complex Workflow Orchestration

```yaml
# ml-pipeline.yaml
jobs:
  data-extraction:
    command: "python3"
    args: [ "extract.py" ]
    runtime: "python-3.11-ml"
    resources:
      max_memory: 2048
      max_cpu: 100

  model-training:
    command: "python3"
    args: [ "train.py" ]
    runtime: "python-3.11-ml"
    requires:
      - data-extraction: "COMPLETED"
    resources:
      max_memory: 8192
      max_cpu: 400
      gpu_count: 1
      gpu_memory_mb: 8192
```

```bash
# Execute and monitor workflow with job names
rnx job run --workflow=ml-pipeline.yaml
rnx job status --workflow a1b2c3d4-e5f6-7890-1234-567890abcdef

# View workflow status with original YAML content (available from any workstation)
rnx job status --workflow --detail a1b2c3d4-e5f6-7890-1234-567890abcdef

# Output shows job names, node identification, and dependencies:
# JOB UUID        JOB NAME             NODE ID         STATUS       EXIT CODE  DEPENDENCIES
# -----------------------------------------------------------------------------------------
# f47ac10b-...    data-extraction      8f94c5b2-...    COMPLETED    0          -
# a1b2c3d4-...    model-training       8f94c5b2-...    RUNNING      -          data-extraction     
```

### Site Reliability Engineering Operations

```bash
# Resource-bounded health checks with timeout
rnx job run --max-cpu=10 --max-memory=64 \
        --runtime=python-3.11 \
        python health_check.py

# Isolated incident response tooling
rnx job run --network=isolated \
        --volume=incident-logs \
        ./debug-analyzer.sh
```

### Artificial Intelligence and Machine Learning Workloads

```bash
# Multi-agent system with isolation
rnx job run --max-memory=4096 --runtime=python-3.11-ml \
        python agent_coordinator.py

# GPU-powered ML agents
rnx job run --gpu=1 --gpu-memory=8GB \
        --max-memory=2048 --runtime=python-3.11-ml \
        --network=agent-net \
        python inference_agent.py

rnx job run --max-memory=1024 --runtime=python-3.11-ml \
        --network=agent-net \
        python monitoring_agent.py
```

## Technical Architecture

### Linux Kernel Integration

- **Control Groups v2**: Hierarchical resource management with unified accounting and deterministic allocation
- **Namespace Isolation**: Complete process separation across all kernel subsystems (PID, network, mount, IPC, UTS)
- **Native Process Execution**: Direct process spawning eliminates virtualization overhead while maintaining security
  boundaries
- **Kernel API Integration**: Leverages standard Linux system calls and interfaces for maximum compatibility and
  performance

### Security Framework

- **Transport Security**: Mutual TLS encryption with certificate pinning for all inter-component communication
- **Access Control Model**: Hierarchical RBAC implementation with principle of least privilege enforcement
- **Isolation Boundaries**: Kernel-enforced namespace separation prevents lateral movement and data leakage
- **Resource Quotas**: Hard limits on compute, memory, and I/O resources prevent denial-of-service conditions

### Scalability and Performance

- **Stateless Architecture**: Horizontally scalable design supports elastic capacity expansion
- **Event-Driven Processing**: Asynchronous job state management with sub-second latency
- **API-First Design**: Comprehensive gRPC API enables seamless integration with existing toolchains
- **Modern Management Console**: React-based interface optimized for operational efficiency and real-time monitoring

## Documentation Resources

### Getting Started

- [**Quick Start Guide**](./QUICKSTART.md) - Get up and running in 5 minutes
- [**Installation Guide**](./INSTALLATION.md) - Detailed installation for all platforms
- [**Configuration**](./CONFIGURATION.md) - Complete configuration reference

### User Guides

- [**RNX CLI Reference**](./RNX_CLI_REFERENCE.md) - Complete command reference with examples
- [**Job Execution Guide**](./JOB_EXECUTION.md) - Running jobs with resource limits and isolation
- [**GPU Support Guide**](./GPU_SUPPORT.md) - NVIDIA GPU acceleration, CUDA environments, and resource management
- [**Workflows Guide**](./WORKFLOWS.md) - YAML workflows with dependencies and orchestration
- [**Runtime System**](./RUNTIME_SYSTEM.md) - Pre-built environments for instant execution (start here)
- [**Runtime Registry Guide**](./RUNTIME_REGISTRY_GUIDE.md) - Using and managing runtime registries
- [**Runtime Design & Examples**](./RUNTIME_DESIGN.md) - Technical design with practical examples
- [**Runtime Advanced**](./RUNTIME_ADVANCED.md) - Implementation, security, and enterprise patterns
- [**Volume Management**](./VOLUME_MANAGEMENT.md) - Persistent and temporary storage
- [**Network Management**](./NETWORK_MANAGEMENT.md) - Network isolation and custom networks
- [**Joblet Admin UI**](./ADMIN_UI.md) - [Standalone React-based interface](https://github.com/ehsaniara/joblet-admin)
  for visual management with direct gRPC connectivity

### Advanced Topics

- [**Security Guide**](./SECURITY.md) - mTLS, authentication, and best practices
- [**Deployment Guide**](./DEPLOYMENT.md) - Production deployment strategies
- [**Troubleshooting**](./TROUBLESHOOTING.md) - Common issues and solutions

### Reference

- [**API Reference**](./API.md) - Complete gRPC API documentation
- [**Architecture**](./DESIGN.md) - System design and architecture deep-dive
- [**Storage Guide**](./STORAGE.md) - Data persistence and storage management
- [**Process Isolation**](./PROCESS_ISOLATION.md) - Complete guide to process isolation and multi-process jobs
- [**Security Analysis**](./ISOLATION_SECURITY_ANALYSIS.md) - Service-based isolation security analysis

## Quick Start Example

```bash
# Install Joblet Server on Linux (see Installation Guide for details)
# Download from GitHub releases and run installation script

# Run your first job
rnx job run echo "Hello, Joblet!"

# Create a workflow
cat > ml-pipeline.yaml << EOF
jobs:
  analyze:
    command: "python3"
    args: ["analyze.py", "--data", "/data/input.csv"]
    runtime: "python-3.11-ml"
    volumes: ["data-volume"]
EOF

# Execute the workflow
rnx job run --workflow=ml-pipeline.yaml
```

## Command Reference

### Job Execution

```bash
# Run basic commands
rnx job run echo "Hello World"
rnx job run --runtime=python-3.11-ml python script.py
rnx job run --runtime=openjdk-21 java MyApp

# Resource limits
rnx job run --max-memory=2048 --max-cpu=200 intensive-task

# GPU-accelerated jobs
rnx job run --gpu=1 --gpu-memory=4GB python ml_training.py
rnx job run --gpu=2 --runtime=python-3.11-ml python distributed_inference.py

# Multi-process jobs (see PROCESS_ISOLATION.md for details)
rnx job run --runtime=python-3.11-ml bash -c "sleep 30 & sleep 40 & ps aux"
rnx job run --runtime=python-3.11-ml bash -c "task1 & task2 & wait"
```

### Node Identification

```bash
# View jobs with node identification for distributed tracking
rnx job list

# Example output showing node IDs:
# UUID                                 NAME         NODE ID                              STATUS
# ------------------------------------  ------------ ------------------------------------ ----------
# f47ac10b-58cc-4372-a567-0e02b2c3d479  setup-data   8f94c5b2-1234-5678-9abc-def012345678 COMPLETED
# a1b2c3d4-e5f6-7890-abcd-ef1234567890  process-data 8f94c5b2-1234-5678-9abc-def012345678 RUNNING

# View detailed job status including node information
rnx job status f47ac10b-58cc-4372-a567-0e02b2c3d479

# Node ID information helps identify which Joblet instance executed each job
# Useful for debugging and tracking in multi-node distributed deployments
```

### Runtime Management

```bash
# List available runtimes (Python, Python ML, Java)
rnx runtime list

# Get runtime information
rnx runtime info python-3.11-ml

# Install runtimes
rnx runtime install python-3.11-ml
rnx runtime install python-3.11
rnx runtime install openjdk-21
rnx runtime install graalvmjdk-21

# Remove runtimes
rnx runtime remove python-3.11-ml

# Test runtime functionality
rnx runtime test openjdk-21
```

### Network & Storage

```bash
# Create isolated networks
rnx network create my-network --cidr=10.0.0.0/24

# Create persistent volumes
rnx volume create data-vol --size=10GB

# Use in jobs
rnx job run --network=my-network --volume=data-vol app
```

## Business Value Proposition

### DevOps and Platform Teams

- **Infrastructure Simplification**: Eliminates container registry management and image versioning complexity
- **Enhanced Security Posture**: Kernel-level isolation without container runtime vulnerabilities
- **Operational Cost Reduction**: Minimal resource overhead compared to container orchestration platforms
- **Seamless Integration**: Native compatibility with existing Linux infrastructure and tooling

### Development Teams

- **Rapid Iteration**: Immediate job execution without container build cycles
- **Enhanced Debugging**: Direct process visibility and filesystem access for troubleshooting
- **Curated Runtimes**: Production-ready environments for Python, Java, and machine learning workloads
- **Developer-Friendly Tooling**: Intuitive CLI and web interfaces designed for productivity

### Operations Teams

- **Comprehensive Observability**: Built-in metrics, monitoring, and alerting capabilities
- **Enterprise Security**: mTLS authentication with fine-grained RBAC policies
- **Centralized Management**: Web-based console for job orchestration and system administration
- **Resource Governance**: Enforced quotas and limits ensure fair resource allocation

### Site Reliability Engineering

- **Fault Isolation**: Process boundaries prevent cascading failures across workloads
- **Resource Predictability**: Deterministic resource allocation ensures consistent performance
- **Monitoring Integration**: Native support for Prometheus, Grafana, and enterprise monitoring solutions
- **Diagnostic Access**: Direct process introspection capabilities for incident response

### Machine Learning and AI Platforms

- **GPU Acceleration**: Native NVIDIA CUDA support with automatic driver management
- **Multi-Agent Isolation**: Secure execution environments for distributed AI systems
- **Resource Optimization**: Fine-grained control over CPU, memory, and GPU allocation
- **ML-Ready Environments**: Pre-configured runtimes with TensorFlow, PyTorch, and CUDA libraries
- **Pipeline Orchestration**: DAG-based workflow execution for complex ML training pipelines

---

## Getting Started

For detailed installation instructions and initial configuration, please refer to
the [Quick Start Guide](./QUICKSTART.md). For production deployment considerations, consult
the [Deployment Guide](./DEPLOYMENT.md).