# Joblet gRPC API Reference Documentation

This comprehensive API reference provides detailed technical documentation for the Joblet gRPC interface, including
complete service definitions, message schemas, authentication protocols, authorization frameworks, and practical
implementation examples for client development.

## API Reference Structure

- [Overview](#overview)
- [Main Joblet Service API](#main-joblet-service-api)
    - [Authentication](#authentication)
    - [Service Definition](#service-definition)
    - [API Methods](#api-methods)
    - [Message Types](#message-types)
    - [Error Handling](#error-handling)
    - [Code Examples](#code-examples)
    - [CLI Reference](#cli-reference)
- [Persist Service API](#persist-service-api)
    - [Overview](#persist-service-overview)
    - [Communication Architecture](#communication-architecture)
    - [Query API](#persist-query-api)
- [Recent Updates](#recent-updates)

## API Architecture Overview

Joblet provides two gRPC API services:

1. **Main Joblet Service** (port 50051): Job execution, management, and live log streaming
2. **Persist Service** (port 50052): Historical log and metric queries

Both services utilize gRPC as their communication protocol with Protocol Buffers for efficient message serialization.
The API implements enterprise-grade security through mutual TLS authentication and provides comprehensive role-based
access control for organizational deployment scenarios.

### Technical Specifications

**Main Joblet Service:**

- **Communication Protocol**: gRPC over TLS 1.3 with HTTP/2 multiplexing
- **Message Serialization**: Protocol Buffers (protobuf) for efficient binary encoding
- **Security Framework**: Mutual TLS authentication with X.509 client certificate validation
- **Access Control**: Role-based authorization system (Administrative/Viewer permissions)
- **Real-time Capabilities**: Server-side streaming for live log aggregation and job monitoring
- **Isolation Architecture**: Linux kernel namespaces with configurable network policies
- **Logging Infrastructure**: Asynchronous log persistence system supporting 5M+ writes per second

**Persist Service:**

- **Communication Protocol**: gRPC (TLS optional, Unix socket IPC for internal communication)
- **Message Serialization**: Protocol Buffers (internal proto definitions)
- **Storage Backend**: Local filesystem with gzip compression (cloud backends planned)
- **Query Capabilities**: Historical log retrieval, metric queries, streaming support
- **IPC Communication**: Unix domain socket at `/opt/joblet/run/persist-ipc.sock`

### Base Configuration Parameters

**Main Service:**

```text
Server Address: <host>:50051
TLS: Required (mutual authentication)
Client Certificates: Required for all operations
Platform: Linux server required for job execution
```

**Persist Service:**

```text
Unix Socket: /opt/joblet/run/persist-grpc.sock (optional, gRPC queries)
IPC Socket: /opt/joblet/run/persist-ipc.sock (internal communication)
TLS: Optional (disabled by default for localhost)
Platform: Linux server required
```

---

## Main Joblet Service API

## Authentication

### Mutual TLS Authentication Protocol

The Joblet API enforces mutual TLS authentication for all client connections, requiring valid X.509 client certificates
issued by the same Certificate Authority (CA) that signed the server certificate.

#### Client Certificate Requirements

```text
Client Certificate Subject Format:
CN=<client-name>, OU=<role>, O=<organization>

Supported Roles:
- OU=admin  → Full access (all operations)
- OU=viewer → Read-only access (get, list, stream)
```

#### Certificate Files Required

```text
certs/
├── ca-cert.pem           # Certificate Authority
├── client-cert.pem       # Client certificate (admin or viewer)
└── client-key.pem        # Client private key
```

### Role-Based Authorization

| Role       | RunJob | GetJobStatus | StopJob | ListJobs | GetJobLogs |
|------------|--------|--------------|---------|----------|------------|
| **admin**  | ✅      | ✅            | ✅       | ✅        | ✅          |
| **viewer** | ❌      | ✅            | ❌       | ✅        | ✅          |

## Service Definition

```protobuf
syntax = "proto3";
package joblet;

service JobletService {
  // Create and start a new job
  rpc RunJob(RunJobReq) returns (RunJobRes);

  // Get job information by ID
  rpc GetJobStatus(GetJobStatusReq) returns (GetJobStatusRes);

  // Stop a running job
  rpc StopJob(StopJobReq) returns (StopJobRes);

  // List all jobs
  rpc ListJobs(EmptyRequest) returns (Jobs);

  // Stream job output in real-time
  rpc GetJobLogs(GetJobLogsReq) returns (stream DataChunk);
}
```

## API Methods

### RunJob

Creates and starts a new job with specified command and resource limits. Jobs execute on the Linux server with complete
process isolation.

**Authorization**: Admin only

```protobuf
rpc RunJob(RunJobReq) returns (RunJobRes);
```

**Request Parameters**:

- `command` (string): Command to execute (required)
- `args` (repeated string): Command arguments (optional)
- `maxCPU` (int32): CPU limit percentage (optional, default: 100)
- `maxMemory` (int32): Memory limit in MB (optional, default: 512)
- `maxIOBPS` (int32): I/O bandwidth limit in bytes/sec (optional, default: 0=unlimited)

**Job Execution Environment**:

- **Process Isolation**: PID, mount, IPC, UTS namespaces
- **Network**: Host networking (shared with server)
- **Resource Limits**: Enforced via Linux cgroups v2
- **Security**: Runs in isolated environment on Linux server

**Response**:

- Complete job metadata including UUID, status, resource limits, and timestamps

**Example**:

```bash
# CLI
rnx job run --max-cpu=50 --max-memory=512 python3 script.py

# Expected Response
Job started:
ID: f47ac10b-58cc-4372-a567-0e02b2c3d479
Command: python3 script.py
Status: INITIALIZING
StartTime: 2024-01-15T10:30:00Z
MaxCPU: 50
MaxMemory: 512
Network: host (shared with system)
```

### GetJobStatus

Retrieves detailed information about a specific job, including current status, resource usage, and execution metadata.

**Authorization**: Admin, Viewer

```protobuf
rpc GetJobStatus(GetJobStatusReq) returns (GetJobStatusRes);
```

**Request Parameters**:

- `id` (string): Job UUID (required)

**Response**:

- Complete job information including current status, execution time, exit code, resource limits, and node identification

**Example**:

```bash
# CLI
rnx job status f47ac10b-58cc-4372-a567-0e02b2c3d479

# Expected Response
Id: f47ac10b-58cc-4372-a567-0e02b2c3d479
Command: python3 script.py
Status: RUNNING
Started At: 2024-01-15T10:30:00Z
Ended At: 
MaxCPU: 50
MaxMemory: 512
MaxIOBPS: 0
ExitCode: 0
```

### StopJob

Terminates a running job using graceful shutdown (SIGTERM) followed by force termination (SIGKILL) if necessary.

**Authorization**: Admin only

```protobuf
rpc StopJob(StopJobReq) returns (StopJobRes);
```

**Request Parameters**:

- `id` (string): Job UUID (required)

**Termination Process**:

1. Send `SIGTERM` to process group
2. Wait 100ms for graceful shutdown
3. Send `SIGKILL` if process still alive
4. Clean up cgroup resources and namespaces
5. Update job status to `STOPPED`

**Response**:

- Job UUID, final status, end time, and exit code

**Example**:

```bash
# CLI
rnx job stop f47ac10b-58cc-4372-a567-0e02b2c3d479

# Expected Response
Job stopped successfully:
ID: f47ac10b-58cc-4372-a567-0e02b2c3d479
Status: STOPPED
ExitCode: -1
EndTime: 2024-01-15T10:45:00Z
```

### ListJobs

Lists all jobs with their current status and metadata. Useful for monitoring overall system activity.

**Authorization**: Admin, Viewer

```protobuf
rpc ListJobs(EmptyRequest) returns (Jobs);
```

**Request Parameters**: None

**Response**:

- Array of all jobs with complete metadata including status breakdown

**Example**:

```bash
# CLI
rnx job list

# Expected Response
f47ac10b-58cc-4372-a567-0e02b2c3d479 COMPLETED StartTime: 2024-01-15T10:30:00Z Command: echo hello
6ba7b810-9dad-11d1-80b4-00c04fd430c8 RUNNING StartTime: 2024-01-15T10:35:00Z Command: python3 script.py
6ba7b811-9dad-11d1-80b4-00c04fd430c8 FAILED StartTime: 2024-01-15T10:40:00Z Command: invalid-command
```

### GetJobLogs

Streams job output in real-time, including historical logs and live updates. Supports multiple concurrent clients
streaming the same job.

**Authorization**: Admin, Viewer

```protobuf
rpc GetJobLogs(GetJobLogsReq) returns (stream DataChunk);
```

**Request Parameters**:

- `id` (string): Job UUID (required)

**Streaming Behavior**:

1. **Historical Data**: Send all accumulated output immediately
2. **Live Updates**: Stream new output chunks as they're generated
3. **Multiple Clients**: Support concurrent streaming to multiple clients
4. **Backpressure**: Remove slow clients automatically to prevent memory leaks
5. **Completion**: Close stream when job completes or fails

**Response**:

- Stream of `DataChunk` messages containing raw stdout/stderr output

**Example**:

```bash
# CLI
rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479

# Expected Response (streaming)
Logs for job f47ac10b-58cc-4372-a567-0e02b2c3d479 (Press Ctrl+C to exit if streaming):
Starting script...
Processing item 1
Processing item 2
...
Script completed successfully
```

## Message Types

### Job

Core job representation used across all API responses.

```protobuf
message Job {
  string id = 1;                    // Unique job UUID identifier
  string name = 2;                  // Readable job name (from workflows, empty for individual jobs)
  string command = 3;               // Command being executed
  repeated string args = 4;         // Command arguments
  int32 maxCPU = 5;                // CPU limit in percent
  string cpuCores = 6;             // CPU core binding specification
  int32 maxMemory = 7;             // Memory limit in MB
  int32 maxIOBPS = 8;              // IO limit in bytes per second
  string status = 9;               // Current job status
  string startTime = 10;           // Start time (RFC3339 format)
  string endTime = 11;             // End time (RFC3339 format, empty if running)
  int32 exitCode = 12;             // Process exit code
  string scheduledTime = 13;       // Scheduled execution time (RFC3339 format)
  string runtime = 14;             // Runtime specification used
  map<string, string> environment = 15;       // Regular environment variables (visible)
  map<string, string> secret_environment = 16; // Secret environment variables (masked)
  // Additional fields
  string nodeId = 20;              // Unique identifier of the Joblet node that executed this job
}
```

### Job Status Values

```
INITIALIZING  - Job created, setting up isolation and resources
RUNNING       - Process executing in isolated namespace
COMPLETED     - Process finished successfully (exit code 0)
FAILED        - Process finished with error (exit code != 0)
STOPPED       - Process terminated by user request or timeout
```

### Resource Limits

Default values when not specified in configuration (`joblet-config.yml`):

```go
DefaultCPULimitPercent = 100 // 100% of one core
DefaultMemoryLimitMB = 512   // 512 MB  
DefaultIOBPS = 0 // Unlimited I/O
```

### RunJobReq

```protobuf
message RunJobReq {
  string command = 1;              // Required: command to execute
  repeated string args = 2;        // Optional: command arguments
  int32 maxCPU = 3;               // Optional: CPU limit percentage
  int32 maxMemory = 4;            // Optional: memory limit in MB
  int32 maxIOBPS = 5;             // Optional: I/O bandwidth limit
}
```

### GetJobStatusRes

Response message for job status requests, including node identification.

```protobuf
message GetJobStatusRes {
  string uuid = 1;                  // Job UUID
  string name = 2;                  // Job name (from workflows, empty for individual jobs)
  string command = 3;               // Command being executed
  repeated string args = 4;         // Command arguments
  int32 maxCPU = 5;                // CPU limit in percent
  string cpuCores = 6;             // CPU core binding specification
  int32 maxMemory = 7;             // Memory limit in MB
  int64 maxIOBPS = 8;              // IO limit in bytes per second
  string status = 9;               // Current job status
  string startTime = 10;           // Start time (RFC3339 format)
  string endTime = 11;             // End time (RFC3339 format, empty if running)
  int32 exitCode = 12;             // Process exit code
  string scheduledTime = 13;       // Scheduled execution time (RFC3339 format)
  string runtime = 14;             // Runtime specification used
  map<string, string> environment = 15;       // Regular environment variables (visible)
  map<string, string> secret_environment = 16; // Secret environment variables (masked)
  string network = 17;             // Network configuration
  repeated string volumes = 18;     // Volume names
  string workDir = 19;             // Working directory
  repeated FileUpload uploads = 20; // File uploads
  repeated string dependencies = 21; // Job dependencies
  string workflowUuid = 22;        // Workflow UUID if part of workflow
  int32 gpuCount = 23;             // Number of GPUs allocated
  repeated int32 gpuIndices = 24;   // GPU indices allocated
  int64 gpuMemoryMB = 25;          // GPU memory in MB
  string nodeId = 26;              // Unique identifier of the Joblet node that executed this job
}
```

### DataChunk

Used for streaming job output with efficient binary transport.

```protobuf
message DataChunk {
  bytes payload = 1;               // Raw output data (stdout/stderr merged)
}
```

## Error Handling

### gRPC Status Codes

| Code                | Description                           | Common Causes                     |
|---------------------|---------------------------------------|-----------------------------------|
| `UNAUTHENTICATED`   | Invalid or missing client certificate | Certificate expired, wrong CA     |
| `PERMISSION_DENIED` | Insufficient role permissions         | Viewer trying admin operation     |
| `NOT_FOUND`         | Job not found                         | Invalid job UUID                  |
| `INTERNAL`          | Server-side error                     | Job creation failed, system error |
| `CANCELED`          | Operation canceled                    | Client disconnected during stream |
| `INVALID_ARGUMENT`  | Invalid request parameters            | Empty command, invalid limits     |

### Error Response Format

```json
{
  "code": "NOT_FOUND",
  "message": "job not found: f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "details": []
}
```

### Common Error Scenarios

#### Authentication Errors

```text
# Missing certificate
Error: failed to extract client role: no TLS information found

# Wrong role (viewer trying to run job)
Error: role viewer is not allowed to perform operation run_job

# Invalid certificate
Error: certificate verify failed: certificate has expired
```

#### Job Operation Errors

```text
# Job not found
Error: job not found: f47ac10b-58cc-4372-a567-0e02b2c3d479

# Job not running (for stop operation)
Error: job is not running: 6ba7b810-9dad-11d1-80b4-00c04fd430c8 (current status: COMPLETED)

# Command validation failed
Error: invalid command: command contains dangerous characters

# Resource limits exceeded
Error: job creation failed: maxMemory exceeds system limits
```

#### Platform Errors

```text
# Linux platform required
Error: job execution requires Linux server (current: darwin)

# Cgroup setup failed
Error: cgroup setup failed: permission denied

# Namespace creation failed
Error: failed to create isolated environment: operation not permitted
```

## CLI Reference

### Global Flags

```bash
--server string     Server address (default "localhost:50051")
--cert string      Client certificate path (default "certs/client-cert.pem")
--key string       Client private key path (default "certs/client-key.pem")
--ca string        CA certificate path (default "certs/ca-cert.pem")
```

### Commands

#### run

Create and start a new job with optional resource limits.

```bash
rnx job run [flags] <command> [args...]

Flags:
  --max-cpu int      Max CPU percentage (default: from config)
  --max-memory int   Max memory in MB (default: from config)  
  --max-iobps int    Max I/O bytes per second (default: 0=unlimited)

Examples:
  rnx job run echo "hello world"
  rnx job run --max-cpu=50 python3 script.py
  rnx job run --max-memory=1024 java -jar app.jar
  rnx job run bash -c "sleep 10 && echo done"
```

#### status

Get detailed information about a job by UUID.

```bash
rnx job status <job-uuid>

Example:
  rnx job status f47ac10b-58cc-4372-a567-0e02b2c3d479
```

#### list

List all jobs with their current status.

```bash
rnx job list

Example:
  rnx job list
```

#### stop

Stop a running job gracefully (SIGTERM) or forcefully (SIGKILL).

```bash
rnx job stop <job-uuid>

Example:
  rnx job stop f47ac10b-58cc-4372-a567-0e02b2c3d479
```

#### log

Stream job output in real-time or view historical logs.

```bash
rnx job log <job-uuid>

Streams logs from running or completed jobs. Use Ctrl+C to stop following.

Examples:
  rnx job log f47ac10b-58cc-4372-a567-0e02b2c3d479              # Stream logs
  rnx job log f47ac10b | grep ERROR                             # Filter output
```

### Configuration Examples

#### Remote Server Connection

```bash
# Connect to remote Linux server from any platform
rnx --server=prod.example.com:50051 \
  --cert=certs/admin-client-cert.pem \
  --key=certs/admin-client-key.pem \
  run echo "remote execution on Linux"
```

#### Environment Variables

```bash
export JOBLET_SERVER="prod.example.com:50051"
export JOBLET_CERT_PATH="./certs/admin-client-cert.pem"
export JOBLET_KEY_PATH="./certs/admin-client-key.pem"
export JOBLET_CA_PATH="./certs/ca-cert.pem"

rnx job run python3 script.py
```

## Configuration & Limits

### Server Configuration

Resource limits and timeouts are configured in `/opt/joblet/joblet-config.yml`:

```yaml
joblet:
  defaultCpuLimit: 100        # Default CPU percentage
  defaultMemoryLimit: 512     # Default memory in MB
  defaultIoLimit: 0           # Default I/O limit (0=unlimited)
  maxConcurrentJobs: 100      # Maximum concurrent jobs
  jobTimeout: "1h"            # Maximum job runtime
  cleanupTimeout: "5s"        # Resource cleanup timeout

grpc:
  maxRecvMsgSize: 524288      # 512KB max receive message
  maxSendMsgSize: 4194304     # 4MB max send message
  keepAliveTime: "30s"        # Connection keep-alive
```

### Client Limits

- **Message Size**: Limited by server configuration (default 4MB)
- **Connection Timeout**: 30-second default keep-alive
- **Certificate Expiration**: Validate certificate validity before connections

## Monitoring and Observability

### Server-Side Metrics

The server provides detailed logging for:

- **Job Lifecycle**: Creation, execution, completion events
- **Resource Usage**: CPU, memory, I/O utilization per job
- **Client Connections**: Authentication attempts and role validation
- **Performance**: Request latency, stream throughput
- **Error Conditions**: Failed jobs, resource limit violations

### Log Levels and Format

```bash
# Structured logging with fields
DEBUG - Detailed execution flow and debugging info
INFO  - Job lifecycle events and normal operations
WARN  - Resource limit violations, slow clients, recoverable errors
ERROR - Job failures, system errors, authentication failures

# Example log entry
[2024-01-15T10:30:00Z] [INFO] job started successfully | jobId=f47ac10b-58cc-4372-a567-0e02b2c3d479 pid=12345 command="python3 script.py" duration=50ms
```

### Health Checks

```bash
# Check server health
rnx job list

# Verify certificate and connection
rnx --server=your-server:50051 list

# Monitor service status (systemd)
sudo systemctl status joblet
sudo journalctl -u joblet -f
```

### Performance Monitoring

- **Concurrent Jobs**: Monitor via `rnx job list`
- **Resource Usage**: Check cgroup statistics in `/sys/fs/cgroup/joblet.slice/`
- **Network**: Monitor gRPC connection count and latency
- **Memory**: Track job output buffer sizes and cleanup efficiency

## Workflow API

### Overview

Joblet provides comprehensive workflow orchestration through YAML-defined job dependencies. Workflows enable complex
multi-job execution with dependency management, resource isolation, and comprehensive monitoring.

### Key Workflow Features

- **Job Names**: Job names derived from YAML job keys
- **Dependency Management**: Define job execution order with `requires` clauses
- **Resource Isolation**: Per-job resource limits and network configuration
- **Real-time Monitoring**: Track workflow progress with job-level status updates
- **Validation**: Pre-execution validation prevents runtime failures

### Service Architecture

The API provides multiple services with distinct responsibilities:

#### JobService (Production Operations)

Handles regular user jobs with production isolation:

```protobuf
service JobService {
  // Job execution with production isolation
  rpc RunJob(RunJobReq) returns (RunJobRes);
  rpc GetJobStatus(GetJobStatusReq) returns (GetJobStatusRes);
  rpc StopJob(StopJobReq) returns (StopJobRes);
  rpc ListJobs(EmptyRequest) returns (Jobs);
  rpc GetJobLogs(GetJobLogsReq) returns (stream DataChunk);
  
  // Workflow execution
  rpc RunWorkflow(RunWorkflowRequest) returns (RunWorkflowResponse);
  rpc GetWorkflowStatus(GetWorkflowStatusRequest) returns (GetWorkflowStatusResponse);
  rpc ListWorkflows(ListWorkflowsRequest) returns (ListWorkflowsResponse);
  rpc GetWorkflowJobs(GetWorkflowJobsRequest) returns (GetWorkflowJobsResponse);
}
```

#### RuntimeService (Administrative Operations)

Handles runtime building with builder chroot access:

```protobuf
service RuntimeService {
  // Runtime installation and management
  rpc InstallRuntime(InstallRuntimeRequest) returns (InstallRuntimeResponse);
  rpc ListRuntimes(ListRuntimesRequest) returns (ListRuntimesResponse);
  rpc GetRuntimeInfo(GetRuntimeInfoRequest) returns (GetRuntimeInfoResponse);
  rpc TestRuntime(TestRuntimeRequest) returns (TestRuntimeResponse);
}
```

**Key Differences:**

- **JobService**: Sets `JobType: "standard"` → minimal chroot with production isolation
- **RuntimeService**: Sets `JobType: "runtime-build"` → builder chroot with host OS access

### Workflow Messages

#### WorkflowJob

Represents a job within a workflow with dependency information.

```protobuf
message WorkflowJob {
  string jobId = 1;                      // Actual job UUID for started jobs, "0" for non-started jobs
  string jobName = 2;                    // Job name from workflow YAML
  string status = 3;                     // Current job status
  repeated string dependencies = 4;       // List of job names this job depends on
  Timestamp startTime = 5;               // Job start time
  Timestamp endTime = 6;                 // Job completion time
  int32 exitCode = 7;                    // Process exit code
}
```

**Job ID Behavior:**

- **Started jobs**: `jobId` contains actual job UUID assigned by joblet (e.g., "f47ac10b-58cc-4372-a567-0e02b2c3d479", "
  6ba7b810-9dad-11d1-80b4-00c04fd430c8")
- **Non-started jobs**: `jobId` shows "0" to indicate the job hasn't been started yet

#### GetWorkflowStatusResponse

Provides comprehensive workflow status with job details.

```protobuf
message GetWorkflowStatusResponse {
  WorkflowInfo workflow = 1;             // Overall workflow information
  repeated WorkflowJob jobs = 2;         // Detailed job information with dependencies
}
```

### Job Names in Workflows

Workflow jobs have **Job names** derived from YAML job keys:

```yaml
# workflow.yaml
jobs:
  setup-data:        # Job name: "setup-data"
    command: "python3"
    args: ["setup.py"]
    
  process-data:      # Job name: "process-data" 
    command: "python3"
    args: ["process.py"]
    requires:
      - setup-data: "COMPLETED"
```

**Job ID vs Job Name:**

- **Job ID**: Unique UUID identifier assigned by joblet (e.g., "f47ac10b-58cc-4372-a567-0e02b2c3d479", "
  6ba7b810-9dad-11d1-80b4-00c04fd430c8")
- **Job Name**: Job name from workflow YAML (e.g., "setup-data", "process-data")

**Status Display:**

```
JOB ID                                   JOB NAME             STATUS       EXIT CODE  DEPENDENCIES        
---------------------------------------------------------------------------------------------------------------------
f47ac10b-58cc-4372-a567-0e02b2c3d479     setup-data           COMPLETED    0          -                   
6ba7b810-9dad-11d1-80b4-00c04fd430c8     process-data         RUNNING      -          setup-data          
```

### CLI Integration

Workflow status commands automatically display job names for better visibility:

```bash
# Get workflow status with job names and dependencies
rnx job status --workflow a1b2c3d4-e5f6-7890-1234-567890abcdef

# List workflows
rnx job list --workflow

# Execute workflow
rnx job run --workflow=pipeline.yaml
```

---

## Persist Service API

The Persist Service provides historical data storage and querying capabilities for logs and metrics. It operates
alongside the main Joblet service to provide durable storage and efficient historical queries.

### Persist Service Overview

**Purpose**: Store and query historical job logs and metrics

**Architecture**:

- **Internal Communication**: Unix domain socket IPC for high-throughput writes from main service
- **External Queries**: Optional gRPC API for historical data retrieval
- **Storage**: Local filesystem with gzip compression (cloud backends planned)
- **Performance**: Non-blocking async writes, 5M+ log lines/second

**Deployment**:

```bash
# Service runs alongside main joblet service
systemctl status joblet-persist

# Configured via unified config file
/opt/joblet/config/joblet-config.yml
```

### Communication Architecture

The persist service uses two communication channels:

#### 1. Unix Socket IPC (Internal)

**Purpose**: High-throughput log and metric writes from main service

```text
Joblet Service → Unix Socket → Persist Service
                 (/opt/joblet/run/persist-ipc.sock)
```

**Protocol**: Custom IPC protocol (defined in `internal/proto/ipc.proto`)

**Message Types**:

- Log entries (stdout/stderr with timestamps)
- Metric samples (CPU, memory, I/O, GPU)
- Batch writes for efficiency

**Configuration**:

```yaml
persist:
  ipc:
    socket: "/opt/joblet/run/persist-ipc.sock"
    max_message_size: 10485760  # 10MB
```

#### 2. gRPC API (External)

**Purpose**: Historical queries from RNX clients

```text
RNX Client → gRPC (Unix socket) → Persist Service
```

**Protocol**: gRPC (defined in `internal/proto/persist.proto`)

**Security**: Optional TLS (disabled by default for localhost)

**Configuration**:

```yaml
persist:
  server:
    grpc_socket: "/opt/joblet/run/persist-grpc.sock"
    tls:
      enabled: false  # Optional for external access
```

### Persist Query API

#### QueryLogs

Retrieves historical logs for a completed or running job.

**Request**:

```protobuf
message QueryLogsRequest {
  string job_id = 1;           // Job UUID (required)
  string stream = 2;           // "stdout" or "stderr" (optional, default: both)
  int64 start_time = 3;        // Unix timestamp (optional)
  int64 end_time = 4;          // Unix timestamp (optional)
  int32 limit = 5;             // Max lines to return (optional)
}
```

**Response**:

```protobuf
message QueryLogsResponse {
  repeated LogEntry entries = 1;
}

message LogEntry {
  int64 timestamp = 1;         // Unix timestamp in nanoseconds
  string stream = 2;           // "stdout" or "stderr"
  string content = 3;          // Log line content
}
```

**Storage Location**:

```
/opt/joblet/logs/{job_id}/
├── stdout.log.gz    # Compressed stdout
└── stderr.log.gz    # Compressed stderr
```

**Example Usage**:

```bash
# RNX automatically queries persist service for completed jobs
rnx job log <job-id>

# Internally calls persist service QueryLogs API
```

#### QueryMetrics

Retrieves historical metrics for a job.

**Request**:

```protobuf
message QueryMetricsRequest {
  string job_id = 1;           // Job UUID (required)
  string metric_type = 2;      // "cpu", "memory", "io", "gpu" (optional)
  int64 start_time = 3;        // Unix timestamp (optional)
  int64 end_time = 4;          // Unix timestamp (optional)
  int32 limit = 5;             // Max samples to return (optional)
}
```

**Response**:

```protobuf
message QueryMetricsResponse {
  repeated MetricSample samples = 1;
}

message MetricSample {
  int64 timestamp = 1;         // Unix timestamp in nanoseconds
  string metric_type = 2;      // Metric category
  map<string, double> values = 3;  // Metric key-value pairs
}
```

**Storage Location**:

```
/opt/joblet/metrics/{job_id}/
└── metrics.jsonl.gz    # Compressed JSON Lines metrics
```

**Metric Types**:

- **CPU**: `cpu_user`, `cpu_system`, `cpu_total`, `cpu_percent`
- **Memory**: `memory_rss`, `memory_vms`, `memory_percent`
- **I/O**: `io_read_bytes`, `io_write_bytes`, `io_read_ops`, `io_write_ops`
- **GPU**: `gpu_utilization`, `gpu_memory_used`, `gpu_temperature` (if available)

**Example Metric Sample**:

```json
{
  "timestamp": 1704451200000000000,
  "metric_type": "cpu",
  "values": {
    "cpu_user": 45.2,
    "cpu_system": 12.8,
    "cpu_total": 58.0,
    "cpu_percent": 58.0
  }
}
```

### Data Retention and Cleanup

**Storage Management**:

```yaml
persist:
  storage:
    retention:
      logs_days: 30        # Keep logs for 30 days
      metrics_days: 90     # Keep metrics for 90 days
      cleanup_interval: "24h"  # Run cleanup daily
```

**Manual Cleanup**:

```bash
# Clean up old job data
find /opt/joblet/logs -type d -mtime +30 -exec rm -rf {} \;
find /opt/joblet/metrics -type d -mtime +90 -exec rm -rf {} \;
```

### Performance Characteristics

**Write Performance**:

- **Throughput**: 5M+ log lines per second
- **Latency**: <1ms average IPC write latency
- **Concurrency**: Handles 1000+ concurrent jobs
- **Buffering**: Async writes with overflow protection

**Query Performance**:

- **Log Queries**: ~50-500ms depending on file size
- **Metric Queries**: ~10-100ms for typical job metrics
- **Compression**: ~80% storage reduction with gzip
- **Caching**: Query result caching (configurable TTL)

### Future Enhancements

**Planned Features**:

- Cloud storage backends (S3, CloudWatch Logs)
- Advanced query filters (regex, time ranges)
- Metric aggregation and rollups
- Web UI for log browsing
- Prometheus metrics export
- Data archival to cold storage

**Configuration (Future)**:

```yaml
persist:
  storage:
    type: "cloud"  # "local" or "cloud"
    cloud:
      provider: "aws"
      s3:
        bucket: "joblet-logs"
        region: "us-west-2"
      cloudwatch:
        log_group: "/joblet/jobs"
```

---

## Recent Updates

### Version 2.11.0 (January 2025) - Async Log System

#### Log Persistence Enhancements

- **Async Log System**: Rate-decoupled async log persistence optimized for HPC workloads
    - Producer-consumer pattern with microsecond write latency
    - 5M+ writes/second sustained throughput capability
    - Four overflow strategies: compress, spill, sample, alert
    - Configurable queue size (100k default) and memory limits (1GB default)
    - Background batching for optimal disk I/O efficiency

- **Adapter Architecture**: Comprehensive documentation added for all adapter methods
    - AsyncLogSystem with overflow protection and metrics
    - JobStoreAdapter with UUID prefix resolution and real-time streaming
    - NetworkStoreAdapter with IP pool management and allocation tracking
    - VolumeStoreAdapter with usage tracking and validation
    - Factory methods with configuration validation and resource management

- **HPC Optimization**: System designed for high-performance computing workloads
    - Non-blocking writes ensure jobs never wait for disk I/O
    - Handles 1000+ concurrent jobs with GB-scale logs
    - Rate mismatch resilience between log production and disk write speed
    - Complete data integrity with multiple overflow protection strategies

#### Testing & Quality

- **Comprehensive Test Suite**: 11 async system tests plus integration and benchmark tests
- **Performance Validation**: Confirmed 5M+ writes/second under load testing
- **Memory Management**: Bounded memory usage with configurable limits and monitoring
- **Documentation**: Complete method documentation across entire adapter layer

### Version 2.10.0 (August 2025)

#### Workflow Enhancements

- **Job Names Support**: Added job names for workflow jobs
    - Job names derived from YAML job keys (e.g., "setup-data", "process-data")
    - Enhanced CLI display with separate JOB ID and JOB NAME columns
    - Updated protobuf messages to include name field
    - Improved workflow monitoring and dependency visualization

#### API Implementations

- **GetJobLogs**: Fully implemented streaming job logs functionality
    - Real-time log streaming for running jobs
    - Historical log retrieval for completed jobs
    - Support for multiple concurrent stream clients
    - Automatic cleanup and backpressure handling

- **ListJobs**: Fully implemented job listing functionality
    - Returns all jobs with complete metadata
    - Includes job status, resource limits, and timestamps
    - Proper authorization checks for admin/viewer roles

#### Monitoring Enhancements

- **Enhanced CPU Metrics**: Added detailed CPU breakdown (user, system, idle, I/O wait)
- **Top Processes Display**: Shows top 10 processes by CPU usage in monitor commands
- **Improved Table Formatting**: Optimized column widths for better readability
- **Network Interface Monitoring**: Limited to 10 active interfaces for clarity

#### Code Improvements

- **Simplified Resource Limits**: Removed complex builder pattern in favor of simple constructors
- **File Upload Enhancements**: Removed artificial size limits (previously 50MB per file, 100MB total)
- **CI/CD Compatibility**: Enhanced test suite to handle various CI/CD environments gracefully

### Migration Notes

- No breaking changes to existing API contracts
- All new implementations follow established patterns
- Backward compatibility maintained for all client versions