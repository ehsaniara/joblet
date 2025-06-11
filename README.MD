# Job Worker System Design Document

## 1. Overview

The Job Worker is a distributed job execution system that provides secure, resource-controlled execution of arbitrary commands on remote machines. It implements a client-server architecture using gRPC with mutual TLS authentication and fine-grained resource management through Linux cgroups.

### 1.1 Key Features

- **Secure Remote Execution**: Mutual TLS authentication with role-based authorization
- **Resource Management**: CPU, memory, and I/O bandwidth limiting via cgroups v2
- **Real-time Streaming**: Live output streaming of job execution logs
- **Process Isolation**: Containerized execution environment with proper cleanup
- **RESTful API**: gRPC-based API with protobuf serialization

## 2. System Architecture

### 2.1 High-Level Architecture

![high-level.svg](docs/high-level.svg)

### 2.2 Component Overview

| Component | Responsibility | Key Files |
|-----------|---------------|-----------|
| **gRPC Server** | API endpoint, authentication, request routing | `grpc_server.go`, `grpc_service.go` |
| **CLI Client** | Command-line interface for job operations | `root.go`, `create.go`, `get.go`, `stop.go`, `stream.go`, `list.go` |
| **Job Worker** | Core job execution logic and lifecycle management | `service.go` |
| **Store** | In-memory job state management and pub/sub | `store.go`, `task.go` |
| **Resource Manager** | Linux cgroups v2 resource control | `cgroup.go` |
| **Job Initializer** | Process isolation and execution setup | `init.go` |
| **Authorization** | Role-based access control via client certificates | `grpc_authorization.go` |

## 3. Core Components

### 3.1 Job Lifecycle Management

#### Job States
```
INITIALIZING → RUNNING → COMPLETED/FAILED/STOPPED
```

- **INITIALIZING**: Job created, cgroup setup, process starting
- **RUNNING**: Process executing, output streaming available
- **COMPLETED**: Process finished successfully (exit code 0)
- **FAILED**: Process finished with error (exit code != 0)
- **STOPPED**: Process terminated by user request

#### State Transitions
```go
type Job struct {
    Id         string
    Command    string
    Args       []string
    Limits     ResourceLimits
    Status     JobStatus
    Pid        int32
    CgroupPath string
    StartTime  time.Time
    EndTime    *time.Time
    ExitCode   int32
}
```

### 3.2 Resource Management

#### Cgroups v2 Integration
- **CPU Limiting**: Uses `cpu.max` for hard limits (percentage-based)
- **Memory Limiting**: Uses `memory.max` for hard limits and `memory.high` for soft limits
- **I/O Limiting**: Uses `io.max` for bandwidth control (bytes per second)

#### Default Resource Limits
```go
const (
    DefaultCPULimitPercent = 100    // 100% of one core
    DefaultMemoryLimitMB   = 512    // 512 MB
    DefaultIOBPS          = 0       // Unlimited
)
```

### 3.3 Process Isolation

#### Two-Stage Execution
1. **job-init**: Lightweight initialization binary that joins the cgroup
2. **Target Command**: The actual user command executed via `exec()`

This approach ensures:
- Proper cgroup membership before command execution
- Clean process hierarchy
- Reliable resource limit enforcement

### 3.4 Security Model

#### Authentication & Authorization
- **Mutual TLS**: Both client and server certificate validation
- **Role-Based Access**: Admin and Viewer roles extracted from client certificate OU field
- **Operation Permissions**:
    - **Admin**: All operations (create, get, stop, list, stream)
    - **Viewer**: Read-only operations (get, list, stream)

#### Certificate Structure
```
Subject: CN=client-name, OU=admin|viewer, O=organization
```

## 4. API Design

### 4.1 gRPC Service Definition

```protobuf
service JobService {
    rpc CreateJob(CreateJobReq) returns (CreateJobRes);
    rpc GetJob(GetJobReq) returns (GetJobRes);
    rpc StopJob(StopJobReq) returns (StopJobRes);
    rpc GetJobs(EmptyRequest) returns (Jobs);
    rpc GetJobsStream(GetJobsStreamReq) returns (stream DataChunk);
}
```

### 4.2 CLI Interface

```bash

# Job Management
cli create <command> [args...] [--max-cpu=N] [--max-memory=N] [--max-iobps=N]
cli get <job-id>
cli stop <job-id>
cli list

# Real-time Monitoring
cli stream <job-id> [--follow]
```

### 4.3 Request/Response Flow

```
1. Client → Server: CreateJobReq
2. Server: Validate permissions (Admin role required)
3. Server: Create cgroup with resource limits
4. Server: Start job-init process
5. Server: Monitor process execution
6. Server → Client: CreateJobRes (job metadata)
7. Client can stream logs via GetJobsStream
```

## 5. Data Management

### 5.1 In-Memory Store

#### Job Storage
- **Thread-Safe**: Uses `sync.RWMutex` for concurrent access
- **Job State**: Maintains complete job metadata and status
- **Output Buffering**: Captures stdout/stderr in memory buffers

#### Pub/Sub System
```go
type Update struct {
    JobID    string
    LogChunk []byte
    Status   string
}
```

- **Real-time Updates**: Publishes log chunks and status changes
- **Multiple Subscribers**: Supports concurrent log streaming clients
- **Automatic Cleanup**: Removes slow subscribers to prevent memory leaks

### 5.2 Output Management

#### Streaming Strategy
1. **Historical Logs**: Send accumulated output first
2. **Live Updates**: Stream new output chunks in real-time
3. **Completion**: Close stream when job finishes

#### Buffer Management
- **Memory-based**: All output stored in memory (no disk persistence)
- **Thread-safe**: Concurrent writes from process output
- **Size Limits**: No explicit limits (relies on process lifetime)

## 6. Error Handling & Reliability

### 6.1 Process Management

#### Graceful Shutdown
1. Send `SIGTERM` to process group
2. Wait 100ms for graceful termination
3. Send `SIGKILL` if process still alive
4. Clean up cgroup resources

#### Error Recovery
- **Process Group Killing**: Ensures all child processes are terminated
- **Cgroup Cleanup**: Automatic resource cleanup on job completion
- **Emergency Cleanup**: Panic recovery with forced resource cleanup

### 6.2 Resource Cleanup

#### Automatic Cleanup
```go
func (cg *cgroup) CleanupCgroup(jobID string) {
    // 1. Kill remaining processes in cgroup
    // 2. Remove cgroup directory
    // 3. Clean up file descriptors
}
```

#### Timeout Handling
- **Cleanup Timeout**: 30-second limit for cgroup cleanup operations
- **Context Cancellation**: Proper handling of client disconnections
- **Resource Leak Prevention**: Ensures cleanup happens even on failures

## 7. Configuration & Deployment

### 7.1 Server Configuration

#### TLS Certificate Setup
```
./certs/
├── ca-cert.pem      # Certificate Authority
├── server-cert.pem  # Server certificate
├── server-key.pem   # Server private key
└── client-*.pem     # Client certificates
```

#### Network Configuration
- **Default Address**: `0.0.0.0:50051`
- **Protocol**: gRPC over TLS 1.3
- **Message Limits**: Configurable max message sizes

### 7.2 System Requirements

#### Linux Dependencies
- **Cgroups v2**: Must be enabled and mounted
- **Systemd**: Recommended for cgroup management
- **Certificates**: Valid TLS certificates for mutual authentication

#### Resource Requirements
- **Memory**: Scales with number of concurrent jobs and output buffer sizes
- **CPU**: Minimal overhead, primarily limited by job workloads
- **Storage**: No persistent storage required (in-memory only)

## 8. Performance Characteristics

### 8.1 Scalability

#### Concurrent Jobs
- **In-Memory Limits**: Bounded by available system memory
- **Process Limits**: Limited by system `ulimit` settings
- **Cgroup Limits**: Limited by kernel cgroup hierarchy depth

#### Streaming Performance
- **Low Latency**: Real-time output streaming with minimal buffering
- **Multiple Clients**: Concurrent streaming to multiple clients per job
- **Backpressure**: Automatic slow client removal to prevent memory exhaustion

### 8.2 Resource Efficiency

#### Memory Usage
- **Per Job Overhead**: ~1-2KB base overhead per job
- **Output Buffering**: Scales with job output volume
- **Pub/Sub System**: Minimal overhead with automatic cleanup

#### CPU Usage
- **Monitoring Overhead**: One goroutine per running job
- **gRPC Processing**: Efficient binary serialization
- **Resource Management**: Minimal cgroup operation overhead

## 9. Security Considerations

### 9.1 Threat Model

#### Protected Against
- **Unauthorized Access**: Mutual TLS with certificate validation
- **Privilege Escalation**: Process isolation via cgroups
- **Resource Exhaustion**: Configurable resource limits
- **Information Disclosure**: Role-based access control

#### Limitations
- **Root Access**: Requires root privileges for cgroup management
- **Local Attacks**: No protection against local root compromise
- **Certificate Management**: Manual certificate distribution required

### 9.2 Best Practices

#### Operational Security
- **Certificate Rotation**: Regular renewal of TLS certificates
- **Principle of Least Privilege**: Use Viewer role where possible
- **Network Isolation**: Deploy in controlled network environments
- **Audit Logging**: Comprehensive operation logging

## 10. Future Enhancements

### 10.1 Potential Improvements

#### Persistence Layer
- **Job History**: Persistent storage for completed jobs
- **Output Archival**: Long-term log storage
- **Database Integration**: SQL/NoSQL backend options

#### Advanced Features
- **Job Scheduling**: Cron-like scheduling capabilities
- **Job Dependencies**: Chain job execution based on dependencies
- **Resource Quotas**: Per-user or per-tenant resource limits
- **Metrics & Monitoring**: Prometheus integration

#### Scalability Enhancements
- **Horizontal Scaling**: Multi-node job distribution
- **Load Balancing**: Job placement optimization
- **Container Integration**: Docker/Podman support
- **Kubernetes Operator**: Native Kubernetes integration

### 10.2 Technical Debt

#### Known Limitations
- **In-Memory Only**: No persistence across server restarts
- **Single Node**: No distributed execution capability
- **Limited Platforms**: Linux-only due to cgroups dependency
- **Certificate Management**: Manual certificate provisioning

This design document provides a comprehensive overview of the Job Worker system architecture, implementation details, and operational considerations. The system prioritizes security, resource isolation, and real-time monitoring while maintaining simplicity and reliability.