# Joblet System Design Document

## 1. Overview

The Joblet is a distributed job execution platform that provides secure, resource-controlled execution of arbitrary
commands on Linux systems. It implements a sophisticated single-binary architecture using gRPC with mutual TLS
authentication, complete process isolation through Linux namespaces, and fine-grained resource management via cgroups
v2.

### 1.1 Key Features

- **Single Binary Architecture**: Same executable operates in server or init mode
- **Complete Process Isolation**: Linux namespaces (PID, mount, IPC, UTS, cgroup)
- **Host Networking**: Shared network namespace for maximum compatibility
- **Resource Management**: CPU, memory, and I/O bandwidth limiting via cgroups v2
- **Real-time Streaming**: Live output streaming with pub/sub architecture
- **Cross-Platform CLI**: Multi-platform client support for remote job management
- **Role-Based Security**: Certificate-based authentication with admin/viewer roles
- **Embedded Certificates**: Configuration files contain all necessary certificates

### 1.2 Design Goals

- **Security First**: Complete process isolation with minimal attack surface
- **Operational Simplicity**: Single binary deployment with embedded configuration
- **Platform Compatibility**: Linux server with cross-platform client support
- **Resource Efficiency**: Minimal overhead with precise resource control
- **Real-time Monitoring**: Live job output streaming and status updates
- **Production Ready**: Comprehensive logging, metrics, and error handling

## 2. Architecture

### 2.1 System Components

![joblet-system-component.svg](joblet-system-component.svg)

### 2.2 Component Responsibilities

#### RNX Client

- **Cross-platform CLI**: Runs on Linux, macOS, Windows
- **gRPC communication**: Secure connection to Joblet server
- **Certificate management**: Embedded TLS certificates in configuration
- **Command interface**: User-friendly job management commands
- **Real-time streaming**: Live log output display

#### Joblet Server

- **Job lifecycle management**: Creation, execution, monitoring, cleanup
- **Resource enforcement**: CPU, memory, I/O limits via cgroups
- **Security isolation**: Process namespaces and filesystem isolation
- **State management**: Job status tracking and persistence
- **API gateway**: gRPC service with authentication and authorization

#### Job Process (Init Mode)

- **Process execution**: Command execution in isolated environment
- **Namespace setup**: PID, mount, IPC, UTS, cgroup isolation
- **Resource assignment**: Automatic cgroup membership
- **Output capture**: stdout/stderr collection and streaming
- **Lifecycle reporting**: Status updates to parent server

### 2.3 Single Binary Architecture

The Joblet implements a unique single-binary architecture where the same executable operates in different modes:

```go
// Mode detection via environment variable
mode := os.Getenv("JOBLET_MODE")
switch mode {
case "server":
// Run as gRPC server and job manager
return modes.RunServer(cfg)
case "init":
// Run as isolated job process
return modes.RunJobInit(cfg)
default:
// Default to server mode
return modes.RunServer(cfg)
}
```

**Benefits:**

- **Simplified deployment**: Single binary to manage
- **Version consistency**: Server and job processes always in sync
- **Reduced complexity**: No separate init system or helper binaries
- **Security**: No external dependencies for job execution

## 3. Security Model

### 3.1 Authentication & Authorization

#### Certificate-Based Roles

- **Admin Certificate** (`OU=admin`): Full job control (run, stop, view)
- **Viewer Certificate** (`OU=viewer`): Read-only access (status, logs, list)
- **Embedded Certificates**: All certificates stored in configuration files

### 3.2 Process Isolation

#### Linux Namespaces

```
Network: Shared (host networking for compatibility)
Mount: Isolated (chroot + bind mounts)
IPC: Isolated (separate IPC namespace)
UTS: Isolated (separate hostname/domain)
Cgroup: Isolated (separate cgroup namespace)
```

#### Resource Limits

```yaml
# Applied per job via cgroups v2
resources:
  cpu: 50%        # CPU percentage limit
  memory: 512MB   # Memory limit
  io: 100MB/s     # I/O bandwidth limit
```

### 3.3 Attack Surface Minimization

#### Server Hardening

- **Root privileges**: Only for namespace/cgroup operations
- **Systemd integration**: Proper service isolation and delegation
- **Certificate validation**: Strict TLS certificate verification
- **Command validation**: Input sanitization and validation
- **Resource boundaries**: Strict cgroup enforcement

#### Client Security

- **Certificate protection**: Secure storage of client certificates
- **Connection validation**: Server certificate verification
- **Input validation**: Command and argument sanitization
- **Error handling**: Information disclosure prevention

## 4. Process Execution Model

### 4.1 Job Lifecycle

![joblet-lifecycle.svg](joblet-lifecycle.svg)

### 4.2 Process Isolation Implementation

#### Namespace Creation

```go
// Create isolated process with namespaces
cmd := exec.Command("/opt/joblet/joblet") // Same binary in init mode
cmd.SysProcAttr = &syscall.SysProcAttr{
Cloneflags: syscall.CLONE_NEWPID | // PID isolation
syscall.CLONE_NEWNS |    // Mount isolation  
syscall.CLONE_NEWIPC |   // IPC isolation
syscall.CLONE_NEWUTS |   // UTS isolation
syscall.CLONE_NEWCGROUP, // Cgroup isolation
// Note: No CLONE_NEWNET (host networking)
}
```

#### Resource Assignment

```go
// Assign process to cgroup for resource control
cgroupPath := fmt.Sprintf("/sys/fs/cgroup/joblet.slice/joblet.service/job-%s", jobID)
procFile := filepath.Join(cgroupPath, "cgroup.procs")
ioutil.WriteFile(procFile, []byte(fmt.Sprintf("%d", pid)), 0644)
```

### 4.3 Host Networking Strategy

Unlike traditional container solutions, Joblet uses **host networking** for maximum compatibility:

**Benefits:**

- **No network configuration**: Jobs have full network access
- **Service discovery**: Can connect to localhost services
- **Port binding**: Can bind to specific ports if needed
- **DNS resolution**: Uses host DNS configuration
- **Firewall compatibility**: Works with existing network security

**Security Considerations:**

- **Network isolation**: Compensated by other namespace isolations
- **Monitoring**: Network activity visible in host monitoring
- **Firewall rules**: Host-level network security applies

## 5. State Management

### 5.1 Job State Store

```go
type Job struct {
Id         string // Unique identifier
Command    string // Command to execute
Args       []string               // Command arguments
Limits     ResourceLimits         // CPU/memory/IO limits
Status     JobStatus              // Current state
Pid        int32                  // Process ID
CgroupPath string    // Resource control path
StartTime  time.Time // Creation time
EndTime    *time.Time // Completion time
ExitCode   int32      // Process exit status
}

type JobStatus string
const (
StatusInitializing JobStatus = "INITIALIZING"
StatusRunning     JobStatus = "RUNNING"
StatusCompleted   JobStatus = "COMPLETED"
StatusFailed      JobStatus = "FAILED"
StatusStopped     JobStatus = "STOPPED"
)
```

### 5.2 Real-time Streaming

#### Pub/Sub Architecture

![joblet-pub-sub.svg](joblet-pub-sub.svg)

#### Stream Management

- **Multiple subscribers**: Many clients can stream same job
- **Backpressure handling**: Slow clients automatically removed
- **Historical + live**: Past output plus real-time updates
- **Memory efficient**: Bounded buffers with cleanup

## 6. Resource Management

### 6.1 Cgroups v2 Integration

```
Linux Kernel Cgroups v2 Hierarchy:
/sys/fs/cgroup/
├── joblet.slice/                    # Systemd slice
│   └── joblet.service/              # Main service cgroup
│       ├── cgroup.controllers       # Available controllers
│       ├── cgroup.subtree_control   # Enabled controllers  
│       ├── job-1/                   # Individual job cgroup
│       │   ├── memory.max           # Memory limit
│       │   ├── cpu.max              # CPU limit
│       │   ├── io.max               # I/O limit
│       │   └── cgroup.procs         # Process list
│       └── job-2/
│           └── ...
```

### 6.2 Resource Enforcement

#### CPU Limiting

```bash
# Set CPU quota: 50% of one core
echo "50000 100000" > /sys/fs/cgroup/joblet.slice/joblet.service/job-1/cpu.max
# Format: quota_microseconds period_microseconds
```

#### Memory Limiting

```bash
# Set memory limit: 512MB
echo "536870912" > /sys/fs/cgroup/joblet.slice/joblet.service/job-1/memory.max
```

#### I/O Limiting

```bash
# Set I/O bandwidth: 10MB/s read, 5MB/s write
echo "8:0 rbps=10485760 wbps=5242880" > /sys/fs/cgroup/joblet.slice/joblet.service/job-1/io.max
```

### 6.3 Resource Monitoring

```go
// Real-time resource usage collection
type ResourceUsage struct {
CPUUsage    time.Duration // Total CPU time
MemoryUsage int64         // Current memory bytes
IORead      int64          // Total bytes read
IOWrite     int64          // Total bytes written
}

// Collected via cgroup statistics files
func (r *ResourceManager) GetUsage(jobID string) (*ResourceUsage, error) {
cgroupPath := r.getCgroupPath(jobID)

// Read CPU usage
cpuStat := filepath.Join(cgroupPath, "cpu.stat")

// Read memory usage  
memoryCurrent := filepath.Join(cgroupPath, "memory.current")

// Read I/O usage
ioStat := filepath.Join(cgroupPath, "io.stat")

// Parse and return aggregated usage
}
```

## 7. Configuration Management

### 7.1 Embedded Certificate Architecture

Instead of separate certificate files, Joblet uses embedded certificates in YAML configuration:

```yaml
# /opt/joblet/config/joblet-config.yml (Server)
version: "3.0"
server:
  address: "0.0.0.0"
  port: 50051
  mode: "server"

security:
  serverCert: |
    -----BEGIN CERTIFICATE-----
    MIIDXTCCAkWgAwIBAgIJAKoK/heBjcO...
    -----END CERTIFICATE-----
  serverKey: |
    -----BEGIN PRIVATE KEY-----
    MIIEvgIBADANBgkqhkiG9w0BAQEFAA...
    -----END PRIVATE KEY-----
  caCert: |
    -----BEGIN CERTIFICATE-----
    MIIDQTCCAimgAwIBAgITBmyfz5m/jA...
    -----END CERTIFICATE-----

# /opt/joblet/config/rnx-config.yml (Client)
version: "3.0"
nodes:
  default:
    address: "192.168.1.100:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      # Admin client certificate
    key: |
      -----BEGIN PRIVATE KEY-----
      # Admin client key
    ca: |
      -----BEGIN CERTIFICATE-----
      # CA certificate
  viewer:
    address: "192.168.1.100:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      # Viewer client certificate (OU=viewer)
```

### 7.2 Configuration Benefits

- **Simplified deployment**: No separate certificate files
- **Atomic updates**: Configuration and certificates update together
- **Reduced complexity**: Single file per component
- **Security**: Proper file permissions protect embedded keys
- **Portability**: Self-contained configuration files

## 8. Error Handling & Reliability

### 8.1 Failure Modes

#### Server Failures

- **Process crashes**: Systemd restart with job recovery
- **Certificate expiry**: Automatic rotation with backup/restore
- **Resource exhaustion**: Job limiting and cleanup
- **Network issues**: Connection retry and timeout handling

#### Job Failures

- **Command not found**: Pre-execution validation
- **Resource limits**: Graceful limit enforcement
- **Process hangs**: Timeout-based termination
- **Namespace failures**: Fallback and error reporting

#### Client Failures

- **Network disconnection**: Automatic reconnection
- **Certificate issues**: Clear error messages and recovery steps
- **Streaming interruption**: Resume from last position

### 8.2 Recovery Mechanisms

#### Graceful Degradation

```go
// Job cleanup with multiple fallback strategies
func (j *Joblet) cleanupJob(jobID string) error {
// 1. Try graceful shutdown (SIGTERM)
if err := j.terminateGracefully(jobID); err == nil {
return nil
}

// 2. Force termination (SIGKILL)
if err := j.forceTerminate(jobID); err == nil {
return nil
}

// 3. Cgroup cleanup
if err := j.cleanupCgroup(jobID); err != nil {
log.Warn("cgroup cleanup failed", "jobId", jobID, "error", err)
}

// 4. Resource cleanup
j.cleanupResources(jobID)

return nil // Always succeed to prevent state inconsistency
}
```

## 9. Performance Characteristics

### 9.1 Scalability Limits

- **Concurrent jobs**: Limited by system resources and configuration
- **Memory usage**: ~2MB overhead per job process
- **CPU overhead**: Minimal for job management, scales with job count
- **I/O performance**: Depends on storage and cgroup configuration
- **Network throughput**: Limited by gRPC and TLS overhead

### 9.2 Optimization Strategies

#### Resource Pooling

- **Cgroup reuse**: Reuse cgroup directories when possible
- **Namespace optimization**: Efficient namespace creation/cleanup
- **Buffer management**: Bounded buffers with adaptive sizing

#### Performance Monitoring

```go
// Built-in performance metrics
type Metrics struct {
JobsCreated     int64
JobsCompleted   int64
JobsFailed      int64
AvgJobDuration  time.Duration
ConcurrentJobs  int64
MemoryUsage     int64
CPUUsage        float64
}
```

## 10. Future Considerations

### 10.1 Potential Enhancements

- **Job scheduling**: Queue management and priority scheduling
- **Multi-node support**: Distributed job execution across nodes
- **GPU support**: NVIDIA GPU isolation and resource management
- **Workflow management**: Job dependencies and conditional execution
- **Audit logging**: Comprehensive security and compliance logging
- **Metrics export**: Prometheus/InfluxDB integration
- **Web UI**: Browser-based job management interface

### 10.2 Architecture Evolution

The current design provides a solid foundation for future enhancements while maintaining:

- **Backward compatibility**: API and configuration stability
- **Security first**: No compromise on isolation and security
- **Operational simplicity**: Minimal complexity increase
- **Platform consistency**: Linux-focused with cross-platform clients

---

This design document represents the current state of the Joblet system and serves as a reference for developers,
operators, and users seeking to understand the system's architecture, security model, and operational characteristics.