# Worker System Design Document

## 1. Overview

The Worker is a distributed job execution platform that provides secure, resource-controlled execution of arbitrary commands on Linux systems. It implements a sophisticated single-binary architecture using gRPC with mutual TLS authentication, complete process isolation through Linux namespaces, and fine-grained resource management via cgroups v2.

### 1.1 Key Features

- **Single Binary Architecture**: Same executable operates in server or init mode
- **Complete Process Isolation**: Linux namespaces (PID, mount, IPC, UTS, cgroup)
- **Host Networking**: Shared network namespace for maximum compatibility
- **Resource Management**: CPU, memory, and I/O bandwidth limiting via cgroups v2
- **Real-time Streaming**: Live output streaming with pub/sub architecture
- **Cross-Platform CLI**: Multi-platform client support for remote job management
- **Role-Based Security**: Certificate-based authentication with admin/