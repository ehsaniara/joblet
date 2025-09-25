# ADR-008: GPU Support Architecture

## Status

**Accepted** - September 2025

**Implementation Status**: ✅ **Complete**
- GPU discovery and management: Implemented
- Security isolation with cgroups device controller: Implemented
- CUDA environment detection and mounting: Implemented
- RNX CLI integration with `--gpu` and `--gpu-memory` flags: Implemented
- JSON API support: Implemented
- Workflow GPU configuration: Implemented
- Comprehensive testing framework: Implemented

## Context

We have received increasing demand from users for GPU support in Joblet. This demand is well-justified - modern machine learning workloads, neural network training, and large-scale data processing require GPU acceleration to achieve reasonable performance and training times that CPUs simply cannot provide.

However, integrating GPU support into Joblet presents significant architectural challenges. Joblet's core strength lies in its robust isolation model built entirely on native Linux kernel features - namespaces and cgroups - without relying on containerization technologies. We cannot compromise this architectural principle by creating security vulnerabilities or abandoning our container-free approach.

GPU hardware introduces several complexities that must be carefully addressed:

**Driver Dependencies**: NVIDIA GPUs require specific kernel modules and driver versions that must be precisely matched with CUDA library versions. Mismatched versions can cause runtime failures or suboptimal performance.

**Resource Economics**: High-end GPUs represent substantial capital investments (often $10,000+ per unit), making dedicated per-job allocation economically inefficient. Multi-tenancy support is essential for cost-effective utilization.

**Memory Architecture**: GPU memory operates independently from system RAM, requiring separate tracking, allocation, and limit enforcement mechanisms that cannot leverage our existing CPU memory management systems.

**Security Isolation**: Multi-tenant GPU sharing creates significant security risks. Without proper isolation, one job could potentially access another job's GPU memory, creating data leakage vulnerabilities that are unacceptable in production environments.

## Decision

After thorough analysis and evaluation, we have decided to implement comprehensive GPU support while maintaining Joblet's core architectural principles. Our approach leverages native Linux kernel features exclusively - no container runtimes, no additional abstraction layers. We will extend our existing namespace and cgroups-based isolation system to include GPU device control using the kernel's native device management capabilities.

We are prioritizing NVIDIA GPU support for this initial implementation. NVIDIA currently dominates the compute GPU market, particularly in machine learning and high-performance computing workloads. While we are designing the architecture to accommodate additional vendors in the future, NVIDIA support addresses the immediate needs of the majority of our user base.

### Design Principles

**User Experience Simplicity**: Our GPU implementation prioritizes ease of use. Users should be able to request GPU resources by simply specifying their requirements (e.g., "1 GPU with 8GB memory") without needing deep expertise in CUDA configuration, driver versions, or device management. The system handles all underlying complexity transparently.

**Security-First Approach**: GPU access follows the same strict security model as our CPU and memory isolation. Jobs receive zero GPU access by default and must explicitly request GPU resources, which are then explicitly granted through controlled allocation. This principle ensures no accidental or unauthorized GPU access.

**Efficient Resource Utilization**: We recognize that GPU hardware represents significant capital investment. Our design supports both exclusive allocation (for compute-intensive tasks like model training) and shared allocation (for lighter workloads like inference) to optimize resource utilization based on workload requirements.

**CUDA Version Management**: The system automatically handles multiple CUDA runtime versions, detecting available installations and providing appropriate environments to jobs based on their requirements. This eliminates version conflicts and simplifies deployment for users with diverse CUDA needs.

### Architecture Components

#### GPU Manager

The GPU Manager serves as the central coordination point for all GPU operations within Joblet. This component is responsible for discovering, tracking, and allocating GPU resources across the system.

**Discovery and Initialization**: During startup, the GPU Manager performs comprehensive GPU discovery by querying `/proc/driver/nvidia/gpus/` and falling back to `nvidia-smi` when necessary. It catalogues each GPU's specifications including memory capacity, compute capability, driver version, and health status.

**Resource Allocation**: The GPU Manager acts as the authoritative gatekeeper for GPU allocation requests. When jobs request GPU resources, the manager evaluates available resources against job requirements and applies configurable allocation strategies. Supported strategies include resource packing (to optimize power efficiency) and resource spreading (to minimize thermal contention).

#### GPU Isolation Implementation

Extending Joblet's isolation system to support GPUs required careful integration with our existing namespace and cgroups architecture. GPUs were not originally designed for the fine-grained isolation that modern multi-tenant systems require, necessitating a sophisticated approach using native Linux kernel features.

**Device Access Control**: We leverage the cgroups v2 device controller to implement precise GPU access control. This is the same kernel feature that containerization technologies use, but we apply it directly without additional runtime overhead. Each job's cgroup receives explicit device permissions only for allocated GPUs, ensuring complete isolation of GPU access.

**Device Node Management**: Within each job's isolated filesystem namespace, we create only the necessary device nodes (`/dev/nvidia0`, `/dev/nvidiactl`, `/dev/nvidia-uvm`, etc.) corresponding to allocated GPUs. Device nodes are created using `mknod` system calls with appropriate permissions, ensuring jobs cannot access unallocated GPU hardware.

**CUDA Runtime Integration**: CUDA libraries are integrated using read-only bind mounts into each job's mount namespace. This approach provides necessary runtime access while preventing modification of system libraries. Environment variables (such as `CUDA_VISIBLE_DEVICES`) are configured to enforce memory limits and device visibility, providing an additional layer of resource control.

#### API Extensions

The GPU implementation introduces minimal, well-designed API extensions that maintain backward compatibility while providing comprehensive GPU functionality.

**Job Submission Extensions**: Job submission requests now support GPU resource specifications including GPU count, memory requirements, and minimum compute capability. These fields are optional, ensuring existing non-GPU workloads continue to function without modification.

**Enhanced Status Reporting**: Job status responses include detailed GPU allocation information, providing visibility into assigned GPU indices, memory allocation, and utilization metrics. This enables users and monitoring systems to track GPU resource usage effectively.

#### Enhanced Scheduling Logic

The job scheduler incorporates sophisticated GPU-aware scheduling algorithms that optimize both resource utilization and job performance.

**Workload Compatibility**: The scheduler recognizes that different GPU workload types have varying resource usage patterns and thermal characteristics. Training workloads with high sustained GPU utilization are scheduled to minimize thermal contention, while inference workloads can be co-located more densely.

**Thermal Management**: GPU scheduling considers thermal distribution across available hardware, spreading high-utilization jobs across different GPUs when possible to prevent thermal throttling and maintain consistent performance.

## Consequences

### Benefits and Advantages

This implementation delivers enterprise-grade GPU support that matches capabilities found in systems like Kubernetes or Slurm, while maintaining Joblet's core philosophy of using native Linux kernel features without containerization overhead. Users continue to benefit from Joblet's simplicity and directness, now extended to GPU workloads.

**Robust Security Model**: Our GPU isolation maintains the same rigorous security standards as our existing CPU and memory isolation. GPU memory remains completely isolated between jobs using kernel-level features. The cgroups device controller ensures jobs cannot access unauthorized GPU resources, providing security equivalent to containerized solutions without the associated overhead.

**User Experience Excellence**: The system eliminates the complexity traditionally associated with GPU computing. Users can request resources using simple, intuitive syntax ("2 GPUs with 16GB each") while the system automatically handles driver detection, CUDA version matching, and environment configuration.

**Flexible Resource Management**: The architecture supports multiple optimization strategies from day one. Administrators can configure resource allocation to prioritize power efficiency through job packing, thermal management through resource spreading, or performance isolation through exclusive GPU access. The system is also prepared for future technologies like NVIDIA's MIG (Multi-Instance GPU) and additional vendor support.

### Implementation Challenges

This implementation introduces significant complexity that must be carefully managed and acknowledged.

**Hardware Complexity**: GPU ecosystems present inherent complexity with driver version dependencies, CUDA runtime variations, and generation-specific features. Each GPU generation introduces new capabilities and potential incompatibilities that increase codebase maintenance requirements.

**Vendor Dependency**: While our architecture is designed for multi-vendor support, the initial focus on NVIDIA creates a temporary dependency. Adding support for AMD or Intel GPUs will require substantial development effort beyond simple configuration changes, including vendor-specific discovery mechanisms, driver interfaces, and feature support.

**Performance Overhead**: GPU management introduces measurable system overhead through status polling (`nvidia-smi`), state tracking, and allocation management. Our performance analysis indicates this overhead should remain below 2% of system resources, but it represents a non-zero cost for GPU-enabled systems.

**Testing Complexity**: Comprehensive testing requires access to GPU hardware, which is expensive and limited in CI/CD environments. This necessitates sophisticated mocking strategies and careful abstraction design to ensure reliable testing without physical hardware dependencies.

### Implementation Phases

The implementation follows a structured, incremental approach to minimize risk and ensure system stability throughout the rollout process.

**Phase 1 - Foundation**: Establish core GPU discovery and basic allocation capabilities. This phase focuses on reliable GPU detection, state tracking, and fundamental allocation logic without advanced features. The goal is achieving basic "GPU available, job requests GPU, allocation succeeds" functionality.

**Phase 2 - Isolation Integration**: Integrate GPU support with Joblet's existing isolation mechanisms. This critical phase involves configuring cgroups device controllers, implementing device node creation within job namespaces, and ensuring allocated GPUs are accessible to jobs while maintaining security isolation.

**Phase 3 - CUDA Runtime Support**: Implement comprehensive CUDA library detection, version management, and environment setup. This phase addresses the complexity of multiple CUDA installations, library mounting into job namespaces, and graceful handling of version mismatches.

**Phase 4 - Advanced Features**: Introduce sophisticated resource management features including memory limits, enhanced scheduling algorithms, and preparation for technologies like NVIDIA MIG. This phase focuses on optimizing user experience and resource utilization.

**Phase 5 - Production Hardening**: Comprehensive testing, performance optimization, documentation completion, and regression prevention for existing functionality. This phase ensures production readiness and maintains system reliability for both GPU and non-GPU workloads.

### Backward Compatibility

Existing Joblet deployments remain completely unaffected by GPU support implementation. GPU functionality is disabled by default, ensuring zero impact on current workloads. Users can enable GPU support through configuration when ready, and can disable it again if issues arise. Non-GPU jobs continue to operate normally regardless of GPU support status.

### Security Considerations

GPU hardware was not originally designed with multi-tenancy security as a primary concern, requiring additional security measures to ensure safe operation in shared environments:

**Memory Sanitization**: GPU memory is completely cleared between job allocations to prevent data leakage. While this process introduces latency, it is essential for maintaining security in multi-tenant environments.

**Execution Time Limits**: GPU kernel execution time limits prevent resource abuse, including unauthorized cryptocurrency mining or other malicious activities that could monopolize expensive GPU resources.

**Rate Limiting**: GPU allocation requests are rate-limited to prevent denial-of-service attacks through rapid allocation and deallocation cycles that could destabilize the system.

**Comprehensive Auditing**: All GPU operations are logged with detailed information including user identity, GPU usage duration, memory allocation, and resource utilization metrics to support security auditing and compliance requirements.

## Alternative Approaches Considered

We evaluated several alternative implementation approaches before settling on our current design:

**NVIDIA Container Toolkit Integration**: The nvidia-container-toolkit provides established GPU support used by Docker and containerd. However, adopting this approach would require introducing container runtime dependencies, fundamentally contradicting Joblet's architectural philosophy of using native Linux kernel features directly. This approach would undermine our core value proposition of demonstrating that sophisticated isolation is achievable without containerization overhead.

**NVIDIA Docker Wrapper**: Leveraging nvidia-docker would have provided immediate GPU support but at the cost of introducing Docker as a hard dependency specifically for GPU functionality. This approach represents an even greater departure from our principles, essentially admitting that containerization is necessary for advanced features.

**Simple Device Passthrough**: The most straightforward approach would involve directly exposing GPU device nodes to jobs without sophisticated isolation. While this aligns with our simplicity philosophy, it fails to provide adequate security, resource limits, or multi-tenancy support. Modern production environments require more sophisticated resource management than simple device passthrough can provide.

**Kubernetes Device Plugin Adaptation**: NVIDIA's Kubernetes device plugin has solved similar GPU management challenges, but their solution is tightly coupled to the Kubernetes ecosystem. Adopting this approach would contradict our goal of proving that sophisticated resource management is possible using only kernel features without requiring full orchestration platforms.

Our chosen approach maintains consistency with Joblet's core philosophy: leverage native Linux kernel capabilities directly. We utilize cgroups v2 for device control, mount namespaces for library isolation, and the /proc filesystem for hardware discovery. This approach requires more development effort than adopting existing solutions, but it demonstrates that powerful isolation and resource management are achievable without traditional containerization overhead.

## Reference Materials

The following technical resources provided essential guidance during the design and implementation process:

- **[NVIDIA Driver Documentation](https://docs.nvidia.com/cuda/cuda-installation-guide-linux/)**: Comprehensive reference for NVIDIA driver installation, configuration, and troubleshooting. Particularly valuable for understanding driver version compatibility and system requirements.

- **[Kubernetes GPU Device Plugin](https://github.com/NVIDIA/k8s-device-plugin)**: Source code analysis revealed numerous edge cases and implementation details not covered in official documentation, particularly around device discovery and error handling.

- **[Linux Cgroups v2 Documentation](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html)**: Essential technical reference for understanding device controller capabilities, permission models, and integration patterns for our isolation implementation.

- **[NVIDIA MIG User Guide](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/)**: Comprehensive documentation for Multi-Instance GPU technology, which informed our architectural decisions for future expandability and advanced resource partitioning.

- **[CUDA Compatibility Documentation](https://docs.nvidia.com/deploy/cuda-compatibility/)**: Critical reference for understanding version compatibility matrices, forward compatibility guarantees, and runtime library dependencies that prevent version conflicts in multi-tenant environments.