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

The Worker API is built on gRPC and uses Protocol Buffers for message serialization. All communication is secured
with mutual TLS authentication and supports role-based authorization.

### API Characteristics

- **Protocol**: gRPC over TLS 1.3
- **Serialization**: Protocol Buffers (protobuf)
- **Authentication**: Mutual TLS with client certificates
- **Authorization**: Role-based (Admin/Viewer)
- **Streaming**: Server-side streaming for real-time log output

### Base Configuration

```text
Server Address: <host>:50051
TLS: Required (mutual authentication)
Client Certificates: Required for all operations
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
├── client-cert.pem       # Client certificate  
└── client-key.pem        # Client private key
```

### Role-Based Authorization

| Role       | CreateJob | GetJob | StopJob | GetJobs | GetJobsStream |
|------------|-----------|--------|---------|---------|---------------|
| **admin**  | ✅         | ✅      | ✅       | ✅       | ✅             |
| **viewer** | ❌         | ✅      | ❌       | ✅       | ✅             |

## Service Definition

```protobuf
syntax = "proto3";
package job_worker;

service JobService {
  // Create and start a new job
  rpc CreateJob(CreateJobReq) returns (CreateJobRes);

  // Get job information by ID
  rpc GetJob(GetJobReq) returns (GetJobRes);

  // Stop a running job
  rpc StopJob(StopJobReq) returns (StopJobRes);

  // List all jobs
  rpc GetJobs(EmptyRequest) returns (Jobs);

  // Stream job output in real-time
  rpc GetJobsStream(GetJobsStreamReq) returns (stream DataChunk);
}
```

## API Methods

### CreateJob

Creates and starts a new job with specified command and resource limits.

**Authorization**: Admin only

```protobuf
rpc CreateJob(CreateJobReq) returns (CreateJobRes);
```

**Request Parameters**:

- `command` (string): Command to execute
- `args` (repeated string): Command arguments
- `maxCPU` (int32): CPU limit percentage (optional)
- `maxMemory` (int32): Memory limit in MB (optional)
- `maxIOBPS` (int32): I/O bandwidth limit in bytes/sec (optional)

**Response**:

- Complete job metadata including ID, status, and resource limits

**Example**:

```bash

# CLI
./bin/cli create --max-cpu=50 --max-memory=512 python3 script.py

# Expected Response
Job created:
ID: 1
Command: python3 script.py
Status: INITIALIZING
StartTime: 2024-01-15T10:30:00Z
```

### GetJob

Retrieves detailed information about a specific job.

**Authorization**: Admin, Viewer

```protobuf
rpc GetJob(GetJobReq) returns (GetJobRes);
```

**Request Parameters**:

- `id` (string): Job ID

**Response**:

- Complete job information including current status, execution time, and exit code

**Example**:

```bash

# CLI
./bin/cli get 1

# Expected Response
Id: 1
Command: python3 script.py
Status: RUNNING
Started At: 2024-01-15T10:30:00Z
Ended At: 
MaxCPU: 50
MaxMemory: 512
MaxIOBPS: 0
```

### StopJob

Terminates a running job gracefully (SIGTERM) or forcefully (SIGKILL).

**Authorization**: Admin only

```protobuf
rpc StopJob(StopJobReq) returns (StopJobRes);
```

**Request Parameters**:

- `id` (string): Job ID

**Response**:

- Job ID, final status, end time, and exit code

**Termination Process**:

1. Send SIGTERM to process group
2. Wait 100ms for graceful shutdown
3. Send SIGKILL if process still alive
4. Clean up cgroup resources

**Example**:

```bash

# CLI
./bin/cli stop 1

# Expected Response
Job stopped successfully:
ID: 1
Status: STOPPED
```

### GetJobs

Lists all jobs with their current status and metadata.

**Authorization**: Admin, Viewer

```protobuf
rpc GetJobs(EmptyRequest) returns (Jobs);
```

**Request Parameters**: None

**Response**:

- Array of all jobs with complete metadata

**Example**:

```bash

# CLI
./bin/cli list

# Expected Response
1 COMPLETED StartTime: 2024-01-15T10:30:00Z Command: echo hello
2 RUNNING StartTime: 2024-01-15T10:35:00Z Command: python3 script.py
3 FAILED StartTime: 2024-01-15T10:40:00Z Command: invalid-command
```

### GetJobsStream

Streams job output in real-time, including historical logs and live updates.

**Authorization**: Admin, Viewer

```protobuf
rpc GetJobsStream(GetJobsStreamReq) returns (stream DataChunk);
```

**Request Parameters**:

- `id` (string): Job ID

**Response**:

- Stream of `DataChunk` messages containing log output

**Streaming Behavior**:

1. Send all historical output immediately
2. Stream live output as it's generated
3. Close stream when job completes
4. Handle client disconnections gracefully

**Example**:

```bash

# CLI
./bin/cli stream 1 --follow

# Expected Response (streaming)
Streaming logs for job 1 (Press Ctrl+C to exit):
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
  string endTime = 9;              // End time (RFC3339 format)
  int32 exitCode = 10;             // Process exit code
}
```

### Job Status Values

```
INITIALIZING  - Job created, setting up resources
RUNNING       - Process executing
COMPLETED     - Process finished successfully (exit code 0)
FAILED        - Process finished with error (exit code != 0)
STOPPED       - Process terminated by user request
```

### Resource Limits

Default values when not specified:

```go
DefaultCPULimitPercent = 100 // 100% of one core
DefaultMemoryLimitMB = 512   // 512 MB  
DefaultIOBPS = 0 // Unlimited
```

### CreateJobReq

```protobuf
message CreateJobReq {
  string command = 1;              // Required: command to execute
  repeated string args = 2;        // Optional: command arguments
  int32 maxCPU = 3;               // Optional: CPU limit percentage
  int32 maxMemory = 4;            // Optional: memory limit in MB
  int32 maxIOBPS = 5;             // Optional: I/O bandwidth limit
}
```

### DataChunk

Used for streaming job output.

```protobuf
message DataChunk {
  bytes payload = 1;               // Raw output data (stdout/stderr)
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

# Wrong role
Error: role viewer is not allowed to perform operation create_job

# Invalid certificate
Error: certificate verify failed: certificate has expired
```

#### Job Operation Errors

```text
# Job not found
Error: job not found: 999

# Job not running (for stop operation)
Error: job is not running: 123 (current status: COMPLETED)

# Command not found
Error: command not found in PATH: invalid-command
```

## Code Examples

### Go Client Example

```go
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"
	"log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	pb "your-module/api/gen"
)

func main() {
	// Load client certificate
	cert, err := tls.LoadX509KeyPair("certs/client-cert.pem", "certs/client-key.pem")
	if err != nil {
		log.Fatal(err)
	}

	// Load CA certificate
	caCert, err := ioutil.ReadFile("certs/ca-cert.pem")
	if err != nil {
		log.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	// Configure TLS
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		ServerName:   "your-server-host",
	}

	// Connect to server
	conn, err := grpc.Dial("your-server:50051",
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewJobServiceClient(conn)

	// Create a job
	resp, err := client.CreateJob(context.Background(), &pb.CreateJobReq{
		Command:   "echo",
		Args:      []string{"Hello", "World"},
		MaxCPU:    50,
		MaxMemory: 256,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Job created: ID=%s, Status=%s", resp.Id, resp.Status)

	// Stream job output
	stream, err := client.GetJobsStream(context.Background(),
		&pb.GetJobsStreamReq{Id: resp.Id})
	if err != nil {
		log.Fatal(err)
	}

	for {
		chunk, err := stream.Recv()
		if err != nil {
			break
		}
		log.Printf("Output: %s", string(chunk.Payload))
	}
}
```

### Python Client Example

```python
import grpc
import ssl
from api import worker_pb2_grpc, worker_pb2


def create_secure_channel(server_addr, cert_path, key_path, ca_path):
    # Load client certificate
    with open(cert_path, 'rb') as f:
        client_cert = f.read()
    with open(key_path, 'rb') as f:
        client_key = f.read()
    with open(ca_path, 'rb') as f:
        ca_cert = f.read()

    # Create credentials
    credentials = grpc.ssl_channel_credentials(
        root_certificates=ca_cert,
        private_key=client_key,
        certificate_chain=client_cert
    )

    return grpc.secure_channel(server_addr, credentials)


def main():
    channel = create_secure_channel(
        'your-server:50051',
        'certs/client-cert.pem',
        'certs/client-key.pem',
        'certs/ca-cert.pem'
    )

    client = worker_pb2_grpc.JobServiceStub(channel)

    # Create job
    request = worker_pb2.CreateJobReq(
        command='python3',
        args=['-c', 'print("Hello from Python job")'],
        maxCPU=30,
        maxMemory=128
    )

    response = client.CreateJob(request)
    print(f"Job created: ID={response.id}, Status={response.status}")

    # Stream output
    stream_request = worker_pb2.GetJobsStreamReq(id=response.id)
    for chunk in client.GetJobsStream(stream_request):
        print(f"Output: {chunk.payload.decode('utf-8')}", end='')


if __name__ == '__main__':
    main()
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

#### create

Create and start a new job.

```bash
./bin/cli create [flags] <command> [args...]

Flags:
  --max-cpu int      Max CPU percentage (default 100)
  --max-memory int   Max memory in MB (default 512)  
  --max-iobps int    Max I/O bytes per second (default 0)

Examples:
  ./bin/cli create echo "hello world"
  ./bin/cli create --max-cpu=50 python3 script.py
  ./bin/cli create bash -c "sleep 10 && echo done"
```

#### get

Get detailed information about a job.

```bash
./bin/cli get <job-id>

Example:
  ./bin/cli get 1
```

#### list

List all jobs.

```bash
./bin/cli list

Example:
  ./bin/cli list
```

#### stop

Stop a running job.

```bash
./bin/cli stop <job-id>

Example:
  ./bin/cli stop 1
```

#### stream

Stream job output in real-time.

```bash
./bin/cli stream [flags] <job-id>

Flags:
  --follow, -f   Follow the log stream (default true)

Examples:
  ./bin/cli stream 1
  ./bin/cli stream --follow=false 1
```

### Configuration Examples

#### Remote Server

```bash
# Connect to remote server
./bin/cli --server=prod.example.com:50051 \
  --cert=certs/admin-client-cert.pem \
  --key=certs/admin-client-key.pem \
  create echo "remote job"
```

#### Environment Variables

```bash
export WORKER_SERVER="prod.example.com:50051"
export WORKER_CERT_PATH="./certs"

./bin/cli create echo "hello"
```

## Rate Limits and Quotas

Currently, the Worker API does not enforce rate limits or quotas. Consider implementing the following in production:

- Maximum concurrent jobs per client
- Maximum job execution time
- Resource usage quotas per role
- API request rate limiting

## Monitoring and Observability

### Server-Side Metrics

The server provides detailed logging for:

- Job creation and lifecycle events
- Resource usage and limits
- Client connections and authentication
- Error conditions and performance metrics

### Log Levels

```bash

DEBUG - Detailed execution flow
INFO  - Job lifecycle events  
WARN  - Resource limit violations, slow clients
ERROR - Job failures, system errors
```

### Health Checks

```bash

# Check server health
./bin/cli list

# Monitor service status (via Makefile)
make service-status
make live-log
```