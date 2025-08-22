# Workflows Guide

Complete guide to creating and managing workflows in Joblet using YAML workflow definitions.

## Table of Contents

- [Overview](#overview)
- [Workflow YAML Format](#workflow-yaml-format)
- [Job Dependencies](#job-dependencies)
- [Network Configuration](#network-configuration)
- [File Uploads](#file-uploads)
- [Resource Management](#resource-management)
- [Workflow Validation](#workflow-validation)
- [Execution and Monitoring](#execution-and-monitoring)
- [Examples](#examples)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

Workflows allow you to define complex job orchestration with dependencies, resource requirements, and network isolation
in YAML format. Joblet provides comprehensive workflow validation and execution capabilities.

### Key Features

- **Job Names**: Human-readable job names for better workflow visibility
- **Job Dependencies**: Define execution order with `requires` clauses
- **Network Isolation**: Specify network for each job (bridge, isolated, none, custom)
- **File Uploads**: Automatically upload required files for each job
- **Resource Limits**: Set CPU, memory, and I/O limits per job
- **Validation**: Comprehensive pre-execution validation prevents runtime failures
- **Monitoring**: Real-time workflow progress tracking with job names and dependencies

## Workflow YAML Format

### Basic Structure

```yaml
jobs:
  job-name:                          # Job name (used for dependencies and monitoring)
    command: "python3"
    args: ["script.py", "--option", "value"]
    runtime: "python:3.11-ml"
    network: "bridge"
    uploads:
      files: ["script.py", "config.json"]
    volumes: ["data-volume"]
    requires:
      - previous-job: "COMPLETED"
    resources:
      max_cpu: 50
      max_memory: 1024
      max_io_bps: 10485760
      cpu_cores: "0-3"
```

**Job Names:**

- Job names are the keys in the `jobs` section (e.g., `job-name`, `previous-job`)
- Names should be descriptive and unique within the workflow
- Used for dependency references and monitoring displays
- Displayed in `rnx status --workflow` and `rnx list --workflow` commands

### Job Specification Fields

| Field       | Description           | Required | Example                                            |
|-------------|-----------------------|----------|----------------------------------------------------|
| `command`   | Executable to run     | Yes      | `"python3"`, `"java"`, `"node"`                    |
| `args`      | Command arguments     | No       | `["script.py", "--verbose"]`                       |
| `runtime`   | Runtime environment   | No       | `"python:3.11-ml"`, `"java:17"`                    |
| `network`   | Network configuration | No       | `"bridge"`, `"isolated"`, `"none"`, `"custom-net"` |
| `uploads`   | Files to upload       | No       | See [File Uploads](#file-uploads)                  |
| `volumes`   | Persistent volumes    | No       | `["data-volume", "logs"]`                          |
| `requires`  | Job dependencies      | No       | See [Job Dependencies](#job-dependencies)          |
| `resources` | Resource limits       | No       | See [Resource Management](#resource-management)    |

## Job Dependencies

### Simple Dependencies

```yaml
jobs:
  extract-data:
    command: "python3"
    args: ["extract.py"]
    runtime: "python:3.11-ml"

  process-data:
    command: "python3"
    args: ["process.py"]
    runtime: "python:3.11-ml"
    requires:
      - extract-data: "COMPLETED"

  generate-report:
    command: "python3"
    args: ["report.py"]
    runtime: "python:3.11-ml"
    requires:
      - process-data: "COMPLETED"
```

### Multiple Dependencies

```yaml
jobs:
  job-a:
    command: "echo"
    args: ["Job A completed"]

  job-b:
    command: "echo"
    args: ["Job B completed"]

  job-c:
    command: "echo"
    args: ["Job C needs both A and B"]
    requires:
      - job-a: "COMPLETED"
      - job-b: "COMPLETED"
```

### Dependency Status Options

- `"COMPLETED"` - Wait for successful completion (exit code 0)
- `"FAILED"` - Wait for job failure (non-zero exit code)
- `"FINISHED"` - Wait for any completion (success or failure)

## Network Configuration

### Built-in Network Types

```yaml
jobs:
  no-network-job:
    command: "echo"
    args: ["No network access"]
    network: "none"

  isolated-job:
    command: "curl"
    args: ["https://api.example.com"]
    network: "isolated"

  bridge-job:
    command: "python3"
    args: ["api_server.py"]
    network: "bridge"
```

### Custom Networks

First create a custom network:

```bash
rnx network create backend --cidr=10.1.0.0/24
```

Then use it in workflows:

```yaml
jobs:
  backend-service:
    command: "python3"
    args: ["backend.py"]
    network: "backend"

  frontend-service:
    command: "node"
    args: ["frontend.js"]
    network: "backend"  # Same network for communication
```

### Network Isolation

Jobs in different networks are completely isolated:

```yaml
jobs:
  service-a:
    command: "python3"
    args: ["service_a.py"]
    network: "network-1"

  service-b:
    command: "python3"  
    args: ["service_b.py"]
    network: "network-2"  # Cannot communicate with service-a
```

## File Uploads

### Basic File Upload

```yaml
jobs:
  process-files:
    command: "python3"
    args: ["processor.py"]
    uploads:
      files: ["processor.py", "config.json", "data.csv"]
```

### Workflow with Multiple File Uploads

```yaml
jobs:
  extract:
    command: "python3"
    args: ["extract.py"]
    uploads:
      files: ["extract.py"]
    
  transform:
    command: "python3"
    args: ["transform.py"]
    uploads:
      files: ["transform.py", "transformations.json"]
    requires:
      - extract: "COMPLETED"
```

## Resource Management

### CPU and Memory Limits

```yaml
jobs:
  memory-intensive:
    command: "python3"
    args: ["ml_training.py"]
    resources:
      max_cpu: 80        # 80% CPU limit
      max_memory: 4096   # 4GB memory limit
      cpu_cores: "0-3"   # Bind to specific cores

  io-intensive:
    command: "python3"
    args: ["data_processing.py"]
    resources:
      max_io_bps: 52428800  # 50MB/s I/O limit
```

### Resource Fields

| Field        | Description                      | Example              |
|--------------|----------------------------------|----------------------|
| `max_cpu`    | CPU percentage limit (0-100)     | `50`                 |
| `max_memory` | Memory limit in MB               | `2048`               |
| `max_io_bps` | I/O bandwidth limit in bytes/sec | `10485760`           |
| `cpu_cores`  | CPU core binding                 | `"0-3"` or `"0,2,4"` |

## Workflow Validation

Joblet performs comprehensive validation before executing workflows:

### Validation Checks

1. **Circular Dependencies**: Detects dependency loops using DFS algorithm
2. **Volume Validation**: Verifies all referenced volumes exist
3. **Network Validation**: Confirms all specified networks exist
4. **Runtime Validation**: Checks runtime availability with name normalization
5. **Job Dependencies**: Ensures all dependencies reference existing jobs

### Validation Output

```bash
$ rnx run --workflow=my-workflow.yaml
🔍 Validating workflow prerequisites...
✅ No circular dependencies found
✅ All required volumes exist
✅ All required networks exist
✅ All required runtimes exist
✅ All job dependencies are valid
🎉 Workflow validation completed successfully!
```

### Validation Errors

```bash
$ rnx run --workflow=broken-workflow.yaml
Error: workflow validation failed: network validation failed: missing networks: [non-existent-network]. Available networks: [bridge isolated none custom-net]
```

## Execution and Monitoring

### Starting Workflows

```bash
# Execute workflow
rnx run --workflow=data-pipeline.yaml

# Execute with file uploads
rnx run --workflow=ml-workflow.yaml  # Automatically uploads files specified in YAML
```

### Monitoring Progress

```bash
# List all workflows
rnx list --workflow

# Check specific workflow status (enhanced with job names and dependencies)
rnx status --workflow <workflow-uuid>

# Monitor job logs
rnx log <job-uuid>
```

### Workflow Status

**List View:**

```bash
ID   NAME                 STATUS      PROGRESS
---- -------------------- ----------- ---------
20   client-workflow-1... COMPLETED   6/6
21   client-workflow-1... RUNNING     3/5
22   client-workflow-1... PENDING     0/4
```

**Detailed Workflow Status:**

```bash
# rnx status --workflow a1b2c3d4-e5f6-7890-1234-567890abcdef
Workflow UUID: a1b2c3d4-e5f6-7890-1234-567890abcdef
Workflow: data-pipeline.yaml
Status: RUNNING
Progress: 2/4 jobs completed

Jobs in Workflow:
-----------------------------------------------------------------------------------------
JOB UUID                             JOB NAME             STATUS       EXIT CODE  DEPENDENCIES        
-----------------------------------------------------------------------------------------
f47ac10b-58cc-4372-a567-0e02b2c3d479 setup-data           COMPLETED    0          -                   
a1b2c3d4-e5f6-7890-abcd-ef1234567890 process-data         RUNNING      -          setup-data          
00000000-0000-0000-0000-000000000000 validate-results     PENDING      -          process-data        
00000000-0000-0000-0000-000000000000 generate-report      PENDING      -          validate-results    
```

**Features:**

- **Job UUID Display**: Started jobs show actual job UUIDs (e.g., "f47ac10b-58cc-4372-a567-0e02b2c3d479"), non-started
  jobs show "00000000-0000-0000-0000-000000000000"
- Job names clearly displayed for easy identification
- Dependency relationships shown (e.g., process-data depends on setup-data)
- Real-time status updates with color coding
- Exit codes for completed jobs

## Examples

### Data Pipeline

```yaml
# data-pipeline.yaml
jobs:
  extract-data:
    command: "python3"
    args: ["extract.py"]
    runtime: "python:3.11-ml"
    uploads:
      files: ["extract.py"]
    volumes: ["data-pipeline"]
    resources:
      max_memory: 1024

  validate-data:
    command: "python3"
    args: ["validate.py"]
    runtime: "python:3.11-ml"
    uploads:
      files: ["validate.py"]
    volumes: ["data-pipeline"]
    requires:
      - extract-data: "COMPLETED"

  transform-data:
    command: "python3"
    args: ["transform.py"]
    runtime: "python:3.11-ml"
    uploads:
      files: ["transform.py"]
    volumes: ["data-pipeline"]
    requires:
      - validate-data: "COMPLETED"
    resources:
      max_cpu: 50
      max_memory: 2048
  
  load-to-warehouse:
    command: "python3"
    args: ["load.py"]
    runtime: "python:3.11-ml"
    uploads:
      files: ["load.py"]
    volumes: ["data-pipeline"]
    requires:
      - transform-data: "COMPLETED"

  generate-report:
    command: "python3"
    args: ["report.py"]
    runtime: "python:3.11-ml"
    uploads:
      files: ["report.py"]
    volumes: ["data-pipeline"]
    requires:
      - load-to-warehouse: "COMPLETED"

  cleanup:
    command: "rm"
    args: ["-rf", "data/", "*.pyc"]
    volumes: ["data-pipeline"]
    requires:
      - generate-report: "COMPLETED"
```

### Microservices with Network Isolation

```yaml
# microservices.yaml
jobs:
  database:
    command: "postgres"
    args: ["--config=/config/postgresql.conf"]
    network: "backend"
    volumes: ["db-data"]
    
  api-service:
    command: "python3"
    args: ["api.py"]
    runtime: "python:3.11-ml"
    network: "backend"
    uploads:
      files: ["api.py", "requirements.txt"]
    requires:
      - database: "COMPLETED"
    
  web-service:
    command: "java"
    args: ["-jar", "web-service.jar"]
    runtime: "java:17"
    network: "frontend"
    uploads:
      files: ["web-service.jar", "application.properties"]
    requires:
      - api-service: "COMPLETED"
```

## Best Practices

### Workflow Design

1. **Use Descriptive Names**: Choose clear, descriptive job names
2. **Minimize Dependencies**: Avoid unnecessary dependencies to maximize parallelism
3. **Resource Planning**: Set appropriate resource limits for each job
4. **Network Segmentation**: Use different networks for different service tiers
5. **Volume Management**: Use persistent volumes for data that needs to survive job completion

### File Management

1. **Upload Only Required Files**: Include only necessary files in uploads
2. **Use Shared Volumes**: For large datasets, use volumes instead of uploads
3. **Organize Files**: Keep related files in the same directory structure

### Resource Optimization

1. **Set Realistic Limits**: Don't over-allocate resources
2. **Use CPU Binding**: Bind CPU-intensive jobs to specific cores
3. **Monitor Usage**: Check actual resource usage and adjust limits

### Security

1. **Network Isolation**: Use appropriate network modes for security requirements
2. **Runtime Selection**: Use minimal runtime environments
3. **Volume Permissions**: Set appropriate volume permissions

## Troubleshooting

### Common Issues

#### Validation Failures

```bash
# Missing network
Error: missing networks: [custom-network]
Solution: Create the network or use an existing one

# Circular dependencies
Error: circular dependency detected: job 'a' depends on itself
Solution: Review and fix dependency chain

# Missing volumes
Error: missing volumes: [data-volume]
Solution: Create the volume with: rnx volume create data-volume
```

#### Runtime Issues

```bash
# Job fails to start
Check: Runtime exists and is properly configured
Check: Command and arguments are correct
Check: Required files are uploaded

# Network connectivity issues
Check: Jobs are in the same network if communication is needed
Check: Network exists and is properly configured
Check: Firewall rules allow required traffic
```

#### Performance Issues

```bash
# Slow job execution
Check: Resource limits are appropriate
Check: CPU binding configuration
Check: I/O bandwidth limits

# Jobs not starting
Check: Dependencies are satisfied
Check: Required resources are available
Check: Workflow validation passed
```

### Debug Commands

```bash
# Check workflow validation
rnx run --workflow=my-workflow.yaml  # Shows validation details

# Check available resources
rnx runtime list
rnx volume list
rnx network list

# Monitor system resources
rnx monitor status
```

### Getting Help

- Check logs: `rnx log <job-uuid>`
- Validate workflows: Run without execution to see validation results
- Review examples: See `/examples/workflows/` directory
- Check documentation: Review relevant docs for specific features