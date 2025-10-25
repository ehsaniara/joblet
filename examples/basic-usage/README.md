# Basic Usage Examples

Basic examples of Joblet features.

## 📚 Examples Overview

| Example                                     | Files                                      | Description                      | Complexity   | Resources |
|---------------------------------------------|--------------------------------------------|----------------------------------|--------------|-----------|
| [Simple Commands](#simple-commands)         | `01_simple_commands.sh`                    | Execute basic shell commands     | Beginner     | 64MB RAM  |
| [File Operations](#file-operations)         | `02_file_operations.sh`, `sample_data.txt` | Upload files and workspace usage | Beginner     | 128MB RAM |
| [Resource Management](#resource-management) | `03_resource_limits.sh`                    | CPU, memory, and I/O limits      | Beginner     | 256MB RAM |
| [Volume Storage](#volume-storage)           | `04_volume_storage.sh`                     | Persistent data storage          | Intermediate | 512MB RAM |
| [Job Monitoring](#job-monitoring)           | `05_job_monitoring.sh`                     | Track job status and logs        | Intermediate | 128MB RAM |
| [Network Basics](#network-basics)           | `07_network_basics.sh`                     | Network isolation concepts       | Intermediate | 128MB RAM |
| [Complete Demo Suite](#complete-demo-suite) | `run_demos.sh`                             | All basic examples in sequence   | All Levels   | 1GB RAM   |

## 🚀 Quick Start

### Using YAML Workflows (NEW - Recommended)

```bash
# Run specific example using the workflow
rnx workflow run jobs.yaml      # Basic hello world
rnx workflow run jobs.yaml         # File operations demo
rnx workflow run jobs.yaml    # Resource limits testing
rnx workflow run jobs.yaml      # Volume storage demo
rnx workflow run jobs.yaml     # Network isolation test
rnx workflow run jobs.yaml         # Multi-step workflow

# Run all demos using template
rnx workflow run jobs.yaml
```

### Run All Basic Examples (Traditional Method)

```bash
# Execute complete basic usage demo
./run_demos.sh
```

This will run all examples in sequence with explanations and pauses for learning.

### Run Individual Examples

```bash
# Simple commands
./01_simple_commands.sh

# File operations
./02_file_operations.sh

# Resource management
./03_resource_limits.sh
```

## 💻 Simple Commands

Learn the basics of running commands with Joblet.

### File Included

- **`01_simple_commands.sh`**: Basic command execution examples

### What It Demonstrates

- Running shell commands in isolated environments
- Job submission and execution
- Basic `rnx run` syntax
- Job output and status

### Key Concepts

- **Job Submission**: How commands are sent to the Joblet server
- **Isolation**: Each job runs in its own isolated environment
- **Command Syntax**: Basic `rnx job run <command>` usage
- **Output Handling**: How to see results from executed jobs

### Usage

```bash
./01_simple_commands.sh
```

This example will demonstrate basic command execution patterns and help you understand the Joblet workflow.

## 📁 File Operations

Learn how to upload files and work with job workspaces.

### Files Included

- **`02_file_operations.sh`**: File upload and workspace examples
- **`sample_data.txt`**: Sample data file for demonstrations

### What It Demonstrates

- Uploading files to job workspaces
- Accessing uploaded files within jobs
- Understanding the job workspace directory structure
- File processing patterns

### Key Concepts

- **File Upload**: Using `--upload` to send files to jobs
- **Workspace**: Each job gets its own isolated workspace
- **File Access**: How to reference uploaded files in commands
- **Data Processing**: Common patterns for processing uploaded data

### Usage

```bash
./02_file_operations.sh
```

## ⚡ Resource Management

Learn how to control CPU, memory, and I/O resources for jobs.

### File Included

- **`03_resource_limits.sh`**: Resource limit examples

### What It Demonstrates

- Setting memory limits with `--max-memory`
- Controlling CPU usage with `--max-cpu`
- Understanding resource allocation and limits
- Preventing resource exhaustion

### Key Concepts

- **Memory Limits**: Controlling maximum memory usage per job
- **CPU Limits**: Setting CPU percentage limits
- **Resource Isolation**: How limits protect system resources
- **Performance Tuning**: Choosing appropriate resource limits

### Usage

```bash
./03_resource_limits.sh
```

## 💾 Volume Storage

Learn about persistent data storage with volumes.

### File Included

- **`04_volume_storage.sh`**: Volume creation and usage examples

### What It Demonstrates

- Creating persistent volumes for data storage
- Using volumes to share data between jobs
- Understanding volume types (filesystem vs memory)
- Data persistence beyond job lifecycle

### Key Concepts

- **Volume Creation**: Creating named storage volumes
- **Volume Types**: Filesystem (persistent) vs memory (temporary)
- **Data Sharing**: Using volumes to pass data between jobs
- **Persistence**: How data survives job completion

### Usage

```bash
./04_volume_storage.sh
```

## 📊 Job Monitoring

Learn how to track job status and view logs.

### File Included

- **`05_job_monitoring.sh`**: Job monitoring and logging examples

### What It Demonstrates

- Checking job status with `rnx job list`
- Viewing job logs with `rnx log`
- Understanding job lifecycle states
- Monitoring long-running jobs

### Key Concepts

- **Job Status**: Understanding RUNNING, COMPLETED, FAILED states
- **Log Viewing**: Accessing job output and error logs
- **Job Management**: Tracking multiple concurrent jobs
- **Debugging**: Using logs to troubleshoot job issues

### Usage

```bash
./05_job_monitoring.sh
```

## 🌐 Network Basics

Learn network isolation and connectivity concepts.

### File Included

- **`07_network_basics.sh`**: Network configuration examples

### What It Demonstrates

- Default network behavior for jobs
- Network isolation between jobs
- Understanding job connectivity limitations
- Basic networking concepts in isolated environments

### Key Concepts

- **Network Isolation**: How jobs are isolated from each other
- **Default Networking**: Standard network configuration for jobs
- **Connectivity**: What jobs can and cannot access
- **Security**: Network-based security in job execution

### Usage

```bash
./07_network_basics.sh
```

## 🎬 Complete Demo Suite

Run all basic usage examples in sequence with guided explanations.

### File Included

- **`run_demos.sh`**: Master script that runs all examples

### What It Demonstrates

All basic Joblet concepts in a structured learning path:

1. **Simple Commands**: Basic command execution
2. **File Operations**: Upload and workspace usage
3. **Resource Management**: CPU, memory, and I/O limits
4. **Volume Storage**: Persistent data storage
5. **Job Monitoring**: Status tracking and log viewing
6. **Network Basics**: Network connectivity and isolation

### Key Features

- **Interactive**: Pauses between sections
- **Coverage**: All basic concepts in one script
- **Progression**: Builds from simple to advanced
- **Examples**: Usage patterns

### Usage

```bash
./run_demos.sh
```

The script runs through examples for each concept.

## 💡 Best Practices Demonstrated

### Command Execution

- **Start Simple**: Begin with basic commands
- **Test Connectivity**: Verify server connection first
- **Resources**: Set memory and CPU limits based on needs

### File Management

- **Upload Strategy**: Only upload files that jobs actually need
- **Workspace Organization**: Keep job workspaces clean and organized
- **Data Validation**: Verify uploaded files exist before processing

### Resource Planning

- **Size Jobs**: Set resource limits for each job
- **Monitor Usage**: Track resource consumption
- **Set Limits**: Protect system resources

### Data Persistence

- **Volumes**: Create for persistent data
- **Types**: Filesystem for persistence, memory for speed
- **Clean Up**: Remove unused volumes

### Job Management

- **Monitor Progress**: Regularly check job status and logs
- **Handle Failures**: Plan for job failures and implement recovery
- **Log Analysis**: Use job logs for debugging and optimization

## 🚀 Next Steps

1. **Basics**: Work through examples in order
2. **Experiment**: Try variations with your data
3. **Scale Up**: Move to advanced examples
4. **Production**: Apply to workflows

## 📚 Additional Resources

- [Advanced Examples](../advanced/) - Complex job coordination patterns
- [Python Analytics](../python-analytics/) - Data processing workflows
- [Joblet Documentation](../../docs/) - Complete feature documentation