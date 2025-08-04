# Basic Usage Examples

Learn Joblet fundamentals with simple, practical examples covering core features and concepts.

## 📚 Examples Overview

| Example | Files | Description | Complexity | Resources |
|---------|-------|-------------|------------|-----------|
| [Simple Commands](#simple-commands) | `01_simple_commands.sh` | Execute basic shell commands | Beginner | 64MB RAM |
| [File Operations](#file-operations) | `02_file_operations.sh`, `sample_data.txt` | Upload files and workspace usage | Beginner | 128MB RAM |
| [Resource Management](#resource-management) | `03_resource_limits.sh` | CPU, memory, and I/O limits | Beginner | 256MB RAM |
| [Volume Storage](#volume-storage) | `04_volume_storage.sh` | Persistent data storage | Intermediate | 512MB RAM |
| [Job Monitoring](#job-monitoring) | `05_job_monitoring.sh` | Track job status and logs | Intermediate | 128MB RAM |
| [Environment Variables](#environment-variables) | `06_environment.sh` | Configuration with env vars | Beginner | 64MB RAM |
| [Network Basics](#network-basics) | `07_network_basics.sh` | Network isolation concepts | Intermediate | 128MB RAM |
| [Complete Demo Suite](#complete-demo-suite) | `run_demos.sh` | All basic examples in sequence | All Levels | 1GB RAM |

## 🚀 Quick Start

### Run All Basic Examples
```bash
# Execute complete basic usage demo
./run_demos.sh
```

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

### Files Included
- **`01_simple_commands.sh`**: Basic command execution examples

### What It Demonstrates
- Running simple shell commands
- Getting command output
- Understanding job lifecycle
- Basic error handling

### Key Commands
```bash
# Basic command execution
rnx run echo "Hello, Joblet!"

# Command with arguments
rnx run ls -la

# Multiple commands in sequence
rnx run bash -c "pwd && ls && date"

# Check system information
rnx run uname -a
```

### Expected Output
- Command execution results
- Job completion status
- System information from Joblet server environment

## 📁 File Operations

Learn how to upload files and work with the job workspace.

### Files Included
- **`02_file_operations.sh`**: File upload and workspace examples
- **`sample_data.txt`**: Sample data file for demonstrations

### What It Demonstrates
- Uploading single files
- Uploading directories
- Working directory structure
- File processing workflows

### Key Commands
```bash
# Upload and process a file
rnx run --upload=sample_data.txt cat sample_data.txt

# Upload directory and list contents
rnx run --upload-dir=./data ls -la data/

# Process uploaded file
rnx run --upload=sample_data.txt wc -l sample_data.txt

# Working directory exploration
rnx run --upload=sample_data.txt bash -c "pwd && ls -la && cat sample_data.txt"
```

### Expected Output
- File contents and processing results
- Directory structure exploration
- Understanding of `/work` directory layout

## ⚡ Resource Management

Learn how to set and monitor resource limits for jobs.

### Files Included
- **`03_resource_limits.sh`**: Resource management examples

### What It Demonstrates
- CPU percentage limits
- Memory limits in MB
- I/O bandwidth controls
- CPU core affinity
- Resource monitoring

### Key Commands
```bash
# Memory limit
rnx run --max-memory=256 python3 -c "import os; print(f'Memory limit demo')"

# CPU limit (50% of available CPU)
rnx run --max-cpu=50 bash -c "echo 'CPU limited job'; sleep 5"

# I/O bandwidth limit
rnx run --max-iobps=1048576 dd if=/dev/zero of=/tmp/test bs=1M count=10

# CPU core binding (cores 0-1)
rnx run --cpu-cores="0-1" bash -c "echo 'Running on specific CPU cores'"

# Combined resource limits
rnx run --max-cpu=25 --max-memory=128 --max-iobps=524288 \
  bash -c "echo 'Resource constrained job'"
```

### Expected Output
- Jobs running within specified resource constraints
- Understanding of resource limit enforcement
- Performance impact of different limits

## 💾 Volume Storage

Learn persistent data storage with Joblet volumes.

### Files Included
- **`04_volume_storage.sh`**: Volume management and usage examples

### What It Demonstrates
- Creating volumes (filesystem and memory types)
- Mounting volumes in jobs
- Data persistence across job runs
- Volume management lifecycle

### Key Commands
```bash
# Create volumes
rnx volume create my-data --size=100MB --type=filesystem
rnx volume create temp-cache --size=50MB --type=memory

# Use volume in job
rnx run --volume=my-data bash -c "echo 'Hello Volume' > /volumes/my-data/greeting.txt"

# Verify data persistence
rnx run --volume=my-data cat /volumes/my-data/greeting.txt

# List volumes
rnx volume list

# Clean up
rnx volume remove temp-cache
```

### Expected Output
- Data persisting across multiple job runs
- Understanding of volume mount points (`/volumes/<name>/`)
- Differences between filesystem and memory volume types

## 📊 Job Monitoring

Learn how to monitor and manage running jobs.

### Files Included
- **`05_job_monitoring.sh`**: Job monitoring and management examples

### What It Demonstrates
- Listing active and completed jobs
- Checking job status
- Viewing job logs
- Stopping running jobs
- Real-time monitoring

### Key Commands
```bash
# Start a long-running job
rnx run --name="long-task" sleep 60 &

# List all jobs
rnx list

# Check specific job status
rnx status <job-id>

# View job logs
rnx log <job-id>

# Follow logs in real-time
rnx log -f <job-id>

# Stop a running job
rnx stop <job-id>

# Monitor system in real-time
rnx monitor
```

### Expected Output
- Job lifecycle understanding
- Real-time log streaming
- System resource monitoring
- Job management workflows

## 🌍 Environment Variables

Learn how to pass configuration to jobs using environment variables.

### Files Included
- **`06_environment.sh`**: Environment variable examples

### What It Demonstrates
- Setting environment variables
- Using env vars in scripts
- Configuration management
- Secure credential handling concepts

### Key Commands
```bash
# Simple environment variable
rnx run --env=MESSAGE="Hello World" bash -c 'echo $MESSAGE'

# Multiple environment variables
rnx run --env=USER=demo --env=ROLE=admin --env=DEBUG=true \
  bash -c 'echo "User: $USER, Role: $ROLE, Debug: $DEBUG"'

# Environment in script processing
rnx run --env=INPUT_FILE=data.txt --env=OUTPUT_DIR=/tmp \
  bash -c 'echo "Processing $INPUT_FILE to $OUTPUT_DIR"'

# Using environment with uploaded files
rnx run --upload=sample_data.txt --env=LINES_TO_SHOW=5 \
  bash -c 'head -n $LINES_TO_SHOW sample_data.txt'
```

### Expected Output
- Dynamic job configuration using environment variables
- Understanding of environment variable scope and usage
- Best practices for configuration management

## 🌐 Network Basics

Learn network isolation and connectivity concepts.

### Files Included
- **`07_network_basics.sh`**: Network configuration examples

### What It Demonstrates
- Default bridge networking
- Network isolation modes
- Custom networks (if supported)
- Connectivity testing

### Key Commands
```bash
# Default networking (bridge mode)
rnx run ping -c 3 google.com

# No network access
rnx run --network=none ping -c 1 google.com || echo "No network access (expected)"

# Host networking (shares host network)
rnx run --network=host ip addr show

# Check network connectivity
rnx run --network=bridge curl -s https://httpbin.org/ip

# Network information
rnx run ip route show
```

### Expected Output
- Understanding of different network modes
- Network isolation demonstration
- Connectivity testing results

## 🔄 Complete Demo Suite

Execute all basic usage examples with a single command.

### Files Included
- **`run_demos.sh`**: Master basic usage demo script

### What It Runs
1. **Simple Commands**: Basic command execution
2. **File Operations**: Upload and workspace usage
3. **Resource Management**: CPU, memory, and I/O limits
4. **Volume Storage**: Persistent data storage
5. **Job Monitoring**: Status tracking and log viewing
6. **Environment Variables**: Configuration management
7. **Network Basics**: Network connectivity and isolation

### Execution
```bash
# Run complete basic usage demo suite
./run_demos.sh
```

### Demo Flow
1. Demonstrates basic command execution
2. Shows file upload and processing workflows
3. Explains resource limit enforcement
4. Creates volumes and demonstrates data persistence
5. Shows job monitoring and management
6. Demonstrates environment variable usage
7. Explains network isolation concepts
8. Provides guidance for next steps

## 📁 Demo Results

After running the demos, you'll understand:

### Core Concepts
- How Joblet executes commands in isolated environments
- Job lifecycle from creation to completion
- Resource management and monitoring
- Data persistence with volumes

### Practical Skills
- Running basic and complex commands
- Managing files and workspaces
- Setting appropriate resource limits
- Using volumes for data persistence
- Monitoring and managing jobs
- Configuring jobs with environment variables

### Best Practices
- When to use different resource limits
- Choosing between filesystem and memory volumes
- Network security considerations
- Job naming and organization strategies

## 🎯 Next Steps

After mastering these basics, explore more advanced examples:

1. **Python Analytics**: `cd ../python-analytics/`
   - Data science workflows with pandas and scikit-learn
   - Machine learning model training and persistence

2. **Node.js Applications**: `cd ../nodejs/`
   - API testing and microservice deployment
   - Build pipelines and event processing

3. **Agentic AI**: `cd ../agentic-ai/`
   - LLM inference and multi-agent systems
   - RAG implementation and distributed training

4. **Advanced Patterns**: `cd ../advanced/`
   - Multi-node coordination
   - Performance optimization
   - Production deployment strategies

## 📊 Monitoring Your Jobs

```bash
# Real-time system monitoring
rnx monitor

# Check all jobs
rnx list

# Get job details
rnx status <job-id>

# View job logs
rnx log <job-id>

# Check volume usage
rnx volume list
```

## 💡 Tips for Success

- **Start Small**: Begin with simple commands before complex workflows
- **Use Resource Limits**: Always set appropriate limits for production usage
- **Name Your Jobs**: Use `--name` for easy identification
- **Monitor Resources**: Use `rnx monitor` to understand system usage
- **Persist Important Data**: Use volumes for data that must survive job completion
- **Check Logs**: Use `rnx log` to debug issues and understand job behavior

## 📚 Additional Resources

- [RNX CLI Reference](../../docs/RNX_CLI_REFERENCE.md)
- [Volume Management Guide](../../docs/VOLUME_MANAGEMENT.md)
- [Resource Management Best Practices](../../docs/RESOURCE_MANAGEMENT.md)
- [Joblet Configuration](../../docs/CONFIGURATION.md)