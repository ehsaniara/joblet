# Worker API Documentation

This document describes the gRPC API for the Worker system, including service definitions, message formats,
authentication, and usage examples.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [Service Definition](#service-definition)
- [API Methods](#api-methods)
- [Message Types](#message-types)
- [Error Handling](#error-handling)
- [Code Examples](#code-examples)
- [CLI Reference](#cli-reference)

## Overview

The Worker API is built on gRPC and uses Protocol Buffers for message serialization. All communication is secured with
mutual TLS authentication and supports role-based authorization.

### API Characteristics

- **Protocol**: gRPC over TLS 1.3
- **Serialization**: Protocol Buffers (protobuf)
- **Authentication**: Mutual TLS with client certificates
- **Authorization**: Role-based (Admin/Viewer)
- **Streaming**: Server-side streaming for real-time log output
- **Job Isolation**: Linux namespaces with host networking

### Base Configuration

```text
Server Address: <host>:50051
TLS: Required (mutual authentication)
Client Certificates: Required for all operations
Platform: Linux server required for job execution
```

## Authentication

### Mutual TLS Authentication

All API calls require valid client certificates signed by the same Certificate Authority (CA) as the server.

#### Certificate Requirements

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
package worker;

service JobService {
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

- Complete job metadata including ID, status, resource limits, and timestamps

**Example**:

```bash
# CLI
worker-cli run --max-cpu=50 --max-memory=512 python3 script.py

# Expected Response
Job started:
ID: 1
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

- `id` (string): Job ID (required)

**Response**:

- Complete job information including current status, execution time, exit code, and resource limits

**Example**:

```bash
# CLI
worker-cli status 1

# Expected Response
Id: 1
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

- `id` (string): Job ID (required)

**Termination Process**:

1. Send `SIGTERM` to process group
2. Wait 100ms for graceful shutdown
3. Send `SIGKILL` if process still alive
4. Clean up cgroup resources and namespaces
5. Update job status to `STOPPED`

**Response**:

- Job ID, final status, end time, and exit code

**Example**:

```bash
# CLI
worker-cli stop 1

# Expected Response
Job stopped successfully:
ID: 1
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
worker-cli list

# Expected Response
1 COMPLETED StartTime: 2024-01-15T10:30:00Z Command: echo hello
2 RUNNING StartTime: 2024-01-15T10:35:00Z Command: python3 script.py
3 FAILED StartTime: 2024-01-15T10:40:00Z Command: invalid-command
```

### GetJobLogs

Streams job output in real-time, including historical logs and live updates. Supports multiple concurrent clients
streaming the same job.

**Authorization**: Admin, Viewer

```protobuf
rpc GetJobLogs(GetJobLogsReq) returns (stream DataChunk);
```

**Request Parameters**:

- `id` (string): Job ID (required)

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
worker-cli log -f 1

# Expected Response (streaming)
Logs for job 1 (Press Ctrl+C to exit if streaming):
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
  string id = 1;                    // Unique job identifier
  string command = 2;               // Command being executed
  repeated string args = 3;         // Command arguments
  int32 maxCPU = 4;                // CPU limit in percent
  int32 maxMemory = 5;             // Memory limit in MB
  int32 maxIOBPS = 6;              // IO limit in bytes per second
  string status = 7;               // Current job status
  string startTime = 8;            // Start time (RFC3339 format)
  string endTime = 9;              // End time (RFC3339 format, empty if running)
  int32 exitCode = 10;             // Process exit code
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

Default values when not specified in configuration (`config.yml`):

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
| `NOT_FOUND`         | Job not found                         | Invalid job ID                    |
| `INTERNAL`          | Server-side error                     | Job creation failed, system error |
| `CANCELED`          | Operation canceled                    | Client disconnected during stream |
| `INVALID_ARGUMENT`  | Invalid request parameters            | Empty command, invalid limits     |

### Error Response Format

```json
{
  "code": "NOT_FOUND",
  "message": "job not found: 999",
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
Error: job not found: 999

# Job not running (for stop operation)
Error: job is not running: 123 (current status: COMPLETED)

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
worker-cli run [flags] <command> [args...]

Flags:
  --max-cpu int      Max CPU percentage (default: from config)
  --max-memory int   Max memory in MB (default: from config)  
  --max-iobps int    Max I/O bytes per second (default: 0=unlimited)

Examples:
  worker-cli run echo "hello world"
  worker-cli run --max-cpu=50 python3 script.py
  worker-cli run --max-memory=1024 java -jar app.jar
  worker-cli run bash -c "sleep 10 && echo done"
```

#### status

Get detailed information about a job by ID.

```bash
worker-cli status <job-id>

Example:
  worker-cli status 1
```

#### list

List all jobs with their current status.

```bash
worker-cli list

Example:
  worker-cli list
```

#### stop

Stop a running job gracefully (SIGTERM) or forcefully (SIGKILL).

```bash
worker-cli stop <job-id>

Example:
  worker-cli stop 1
```

#### log

Stream job output in real-time or view historical logs.

```bash
worker-cli log [flags] <job-id>

Flags:
  --follow, -f   Follow the log stream (default true)

Examples:
  worker-cli log 1              # View all logs
  worker-cli log -f 1           # Follow live output
  worker-cli log --follow=false 1  # Historical logs only
```

### Configuration Examples

#### Remote Server Connection

```bash
# Connect to remote Linux server from any platform
worker-cli --server=prod.example.com:50051 \
  --cert=certs/admin-client-cert.pem \
  --key=certs/admin-client-key.pem \
  run echo "remote execution on Linux"
```

#### Environment Variables

```bash
export WORKER_SERVER="prod.example.com:50051"
export WORKER_CERT_PATH="./certs/admin-client-cert.pem"
export WORKER_KEY_PATH="./certs/admin-client-key.pem"
export WORKER_CA_PATH="./certs/ca-cert.pem"

worker-cli run python3 script.py
```

## Configuration & Limits

### Server Configuration

Resource limits and timeouts are configured in `/opt/worker/config.yml`:

```yaml
worker:
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
[2024-01-15T10:30:00Z] [INFO] job started successfully | jobId=1 pid=12345 command="python3 script.py" duration=50ms
```

### Health Checks

```bash
# Check server health
worker-cli list

# Verify certificate and connection
worker-cli --server=your-server:50051 list

# Monitor service status (systemd)
sudo systemctl status worker
sudo journalctl -u worker -f
```

### Performance Monitoring

- **Concurrent Jobs**: Monitor via `worker-cli list`
- **Resource Usage**: Check cgroup statistics in `/sys/fs/cgroup/worker.slice/`
- **Network**: Monitor gRPC connection count and latency
- **Memory**: Track job output buffer sizes and cleanup efficiency