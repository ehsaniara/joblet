# Joblet / RNX - Secure Linux Process Execution Platform

[![Tests](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml/badge.svg)](https://github.com/ehsaniara/joblet/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ehsaniara/joblet)](https://goreportcard.com/report/github.com/ehsaniara/joblet)
[![Go Version](https://img.shields.io/github/go-mod/go-version/ehsaniara/joblet)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/ehsaniara/joblet.svg)](https://github.com/ehsaniara/joblet/releases/latest)

<img align="right" width="159px" src="docs/joblet-thumbnail.png">

📝 **Note:** Project recently renamed from _Worker_ to **Joblet** and **RNX** (CLI)

**Joblet** provides **secure, isolated job execution** on Linux systems. Run commands remotely with complete process
isolation, resource limits, real-time monitoring, and **scheduled execution** - protecting your host system from
malicious or resource-hungry
processes.

## 🎯 Why Use Joblet?

### The Problem: Unsafe Remote Execution

```bash
# DANGEROUS: Running commands directly on your server
# SSH into server (dangerous)
ssh production-server "ps aux"

# Output shows ALL system processes (security risk!)
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.0  0.1 225596  8964   ?      Ss   Jan15   0:02 /sbin/init
root         2  0.0  0.0      0     0   ?      S    Jan15   0:00 [kthreadd]
systemd+   534  0.0  0.1  90812  5984   ?      Ssl  Jan15   0:15 /lib/systemd/systemd-resolved
mysql     1234  0.2  2.1 1725932 175184 ?      Ssl  Jan15   2:45 /usr/sbin/mysqld
nginx     1456  0.0  0.1  55180  8932   ?      S    Jan15   0:00 nginx: worker process
postgres  1789  0.1  1.8 394532 151234  ?      Ss   Jan15   1:23 postgres: main process
root      2341  0.0  0.0  72132  3456   ?      S    10:30   0:00 sshd: user [priv]
user      2567  5.2  0.1  21532  2134 pts/0    Rs+  10:42   0:00 ps aux
... (hundreds more system processes)

# ❌ Can see all processes, access sensitive files, consume unlimited resources
# ❌ Can interfere with other applications
# ❌ No resource controls or monitoring
```

### The Solution: Isolated Job Execution

```bash
# Safe execution through Joblet
rnx run ps aux
# Job output (streamed in real-time):
USER       PID %CPU %MEM    VSZ   RSS TTY      STAT START   TIME COMMAND
root         1  0.0  0.0   2304   768 ?        S    10:45   0:00 ps aux

# ✅ Process isolation - can only see its own processes
# ✅ Filesystem isolation - limited access to host files  
# ✅ Resource limits - memory, CPU, I/O controls
# ✅ Real-time monitoring and logs
# ✅ Scheduled execution for automation
```

## 🔧 How It Works

Joblet consists of two components:

1. **Joblet Daemon** - Runs on Linux servers, executes jobs in isolated environments
2. **RNX CLI** - Connects from anywhere (Linux/macOS/Windows) to manage jobs

![joblet-how-it-works.svg](docs/joblet-how-it-works.svg)

### Security & Isolation Features

- **Process Isolation**: Jobs run in separate PID namespaces
- **Filesystem Isolation**: Chroot environments with limited host access
- **Resource Limits**: CPU, memory, I/O bandwidth, and CPU core controls
- **CPU Core Binding**: Limit jobs to specific CPU cores for performance isolation
- **Network Security**: mTLS encryption with certificate-based authentication
- **Role-Based Access**: Admin (full control) and Viewer (read-only) roles
- **Job Scheduling**: Future execution with priority queue management

## 📦 Installation

### Quick Start - Debian Package (Recommended)

Install both daemon and CLI with embedded certificates:

```bash
# Download latest release
wget $(curl -s https://api.github.com/repos/ehsaniara/joblet/releases/latest | grep "browser_download_url.*_amd64\.deb" | cut -d '"' -f 4)

# Interactive installation (prompts for server IP)
sudo dpkg -i joblet_*_amd64.deb

# Start the daemon
sudo systemctl start joblet
sudo systemctl enable joblet

# Test locally 

sudo rnx list
# should return `No jobs found`
# create a job
sudo rnx run ps aux
# check the list of jobs
sudo rnx list
# should see the a COMPLETED job
```

![Joblet Installation Demo](docs/installation.gif)

### Manual Installation

#### 1. Build from Source

```bash
git clone https://github.com/ehsaniara/joblet.git
cd joblet

# Build binaries
make all  # Creates: bin/joblet, bin/rnx

# Or download pre-built binaries from releases
```

#### 2. Install Joblet Daemon (Linux Server)

```bash
# Install daemon binary
sudo mkdir -p /opt/joblet/config /opt/joblet/scripts
sudo cp bin/joblet /opt/joblet/
sudo chmod +x /opt/joblet/joblet

# Copy configuration templates
sudo cp scripts/joblet-config-template.yml /opt/joblet/scripts/
sudo cp scripts/joblet.service /etc/systemd/system/

# Generate certificates with embedded configuration
export JOBLET_SERVER_ADDRESS="192.168.1.100"  # Your server IP
sudo ./scripts/certs_gen_embedded.sh

# Start daemon service
sudo systemctl daemon-reload
sudo systemctl enable joblet
sudo systemctl start joblet
```

#### 3. Install RNX CLI (Client Machine)

##### Linux/macOS CLI Setup

```bash
# Install CLI binary
sudo cp bin/rnx /usr/local/bin/rnx
sudo chmod +x /usr/local/bin/rnx

# Download client configuration from daemon server
mkdir -p ~/.rnx
scp your-server:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Test connection
rnx --config ~/.rnx/rnx-config.yml list
```

##### Windows CLI Setup

```powershell
# Download Windows CLI from releases
Invoke-WebRequest -Uri "https://github.com/ehsaniara/joblet/releases/latest/download/rnx-windows-amd64.exe" -OutFile "rnx.exe"

# Copy to PATH
Move-Item rnx.exe "C:\Program Files\rnx.exe"

# Copy client config from daemon server to:
# %USERPROFILE%\.rnx\rnx-config.yml
```

## 🚀 Usage

### Basic Job Management

```bash
# Run simple commands
rnx run echo "Hello World"
rnx run ps aux
rnx run python3 --version

# Run with resource limits
rnx run --max-cpu=50 --max-memory=256 python3 script.py
rnx run --max-iobps=5242880 dd if=/dev/zero of=/tmp/test bs=1M count=100

# Complex commands
rnx run bash -c "curl -s https://api.github.com/users/octocat"
rnx run python3 -c "import json; print('Python available')"
```

### Job Scheduling

Joblet supports scheduling jobs for future execution with flexible time specifications:

```bash
# Schedule using relative time
rnx run --schedule="1hour" python3 backup_script.py
rnx run --schedule="30min" echo "Scheduled task"
rnx run --schedule="2h30m" data_processing.py
rnx run --schedule="45s" quick_task.sh

# Schedule using absolute time (RFC3339 format)
rnx run --schedule="2025-07-18T20:02:48" backup_script.sh
rnx run --schedule="2025-07-18T20:02:48Z" maintenance.sh
rnx run --schedule="2025-07-18T20:02:48-07:00" deploy.sh

# Combine scheduling with resource limits
rnx run --schedule="1hour" --max-cpu=50 --max-memory=512 heavy_computation.py

# Schedule with file uploads
rnx run --schedule="30min" --upload-dir=dataset python3 process_data.py
```

#### Scheduling Formats

**Relative Time:**

- `"1hour"` - 1 hour from now
- `"30min"` - 30 minutes from now
- `"2h30m"` - 2 hours 30 minutes from now
- `"45s"` - 45 seconds from now

**Absolute Time (RFC3339):**

- `"2025-07-18T20:02:48"` - Local time
- `"2025-07-18T20:02:48Z"` - UTC time
- `"2025-07-18T20:02:48-07:00"` - With timezone

### File & Directory Upload

Joblet supports uploading files and directories to the job workspace for processing. Files are automatically available
in the `/work` directory inside the job:

```bash
# Upload a single file
rnx run --upload=script.py python3 script.py

# Upload multiple files
rnx run --upload=data.csv --upload=process.py python3 process.py

# Upload entire directory with preserved structure
rnx run --upload-dir=dataset python3 analyze.py

# Upload current directory
rnx run --upload-dir=. python3 main.py

# Complex data processing example
rnx run --upload-dir=ml-project bash -c "cd ml-project && python3 train.py"

# Scheduled jobs with uploads
rnx run --schedule="1hour" --upload-dir=. python3 main.py
rnx run --schedule="30min" --upload=data.csv --upload=process.py python3 process.py
```

#### Upload Features

- **Memory-Optimized**: Small files (<1MB) are embedded, large files (≥1MB) are streamed
- **OOM Protection**: Dynamic chunking based on job memory limits prevents out-of-memory kills
- **Progress Feedback**: Real-time upload progress and optimization information
- **Size Limits**: 50MB per file, 100MB total upload size
- **Directory Structure**: Preserves full directory hierarchy in job workspace
- **Security**: Path traversal protection and system directory access prevention
- **Scheduling Support**: File uploads work seamlessly with scheduled jobs

```bash
# Example output with upload optimization
$ rnx run --schedule="1hour" --upload-dir=dataset python3 process_data.py

📤 Upload optimized for memory efficiency:
   Total: 15 files (25.3 MB)
   Small files: 12 (embedded in job)
   Large files: 3 (streamed to job)
   Memory-safe chunking: Enabled

Job started:
ID: 2
Command: python3 process_data.py
Status: SCHEDULED
Schedule Input: 1hour
Scheduled Time: 2025-07-19T15:45:30Z
```

### CPU Core Management

```bash
# Limit to specific CPU cores (range)
rnx run --cpu-cores="0-3" python3 training.py          # Use cores 0,1,2,3

# Limit to specific cores (list)
rnx run --cpu-cores="1,3,5,7" simulation_app           # Use cores 1,3,5,7

# Single core allocation
rnx run --cpu-cores="2" lightweight_task               # Use only core 2

# Combine core limiting with other resource limits
rnx run --cpu-cores="0-1" --max-cpu=150 --max-memory=512 heavy_computation

# Reserve cores for system (use all except core 0)
rnx run --cpu-cores="1-7" user_workload                # Leave core 0 for system

# Schedule with core allocation
rnx run --schedule="30min" --cpu-cores="2-5" --max-memory=1024 batch_job.py
```

### Resource Limiting Examples

```bash
# CPU percentage limiting (can use multiple cores at reduced time)
rnx run --max-cpu=50 stress --cpu 4                    # 50% across all cores

# CPU core limiting (dedicated cores at full speed)
rnx run --cpu-cores="0-1" stress --cpu 2               # Full speed on cores 0-1

# Combine both for precise control
rnx run --cpu-cores="2-3" --max-cpu=150 workload       # 150% across cores 2-3 only

# Memory and I/O limits
rnx run --max-memory=1024 --max-iobps=10485760 data_processor

# Schedule with resource limits
rnx run --schedule="2h" --max-cpu=25 --max-memory=512 overnight_batch.py
```

#### Example

```bash
rnx --cpu-cores="1" --max-cpu=50 run bash -c "while true; do :; done"
Job started:
ID: 2
Command: bash -c while true; do :; done
Status: RUNNING
StartTime: 2025-07-11T06:18:54Z
```

`CPU LIMIT = 50%`

`SELECTED CPU CORE ID = 1`

![cpu-50-core-1.png](docs/cpu-50-core-1.png)

### Job Monitoring

```bash
# List all jobs (including scheduled ones)
rnx list

# Get job details (including scheduled time)
rnx status <job-id>

# Stream live logs
rnx log <job-id>

# Stop running job
rnx stop <job-id>
```

### Remote Server Connection

```bash
# Connect to remote daemon
rnx --server=production:50051 run python3 script.py

# Use different configurations
rnx --config=prod-rnx-config.yml list
rnx --node=production run deploy.sh

# Set default server
export JOBLET_SERVER=production:50051
rnx list  # Uses production server
```

### Real-World Examples

#### Automation & Scheduling

```bash
# Daily backup at 2 AM
rnx run --schedule="2025-07-20T02:00:00" --upload=backup.sh bash backup.sh

# Data processing every 30 minutes
rnx run --schedule="30min" --upload-dir=data --max-memory=2048 python3 process.py

# Weekly maintenance
rnx run --schedule="2025-07-27T03:00:00" --max-cpu=25 maintenance_script.sh

# Resource-limited batch processing
rnx run --schedule="1hour" --cpu-cores="0-3" --max-memory=4096 batch_analysis.py
```

#### High-Performance Computing

```bash
# Parallel processing with dedicated cores
rnx run --cpu-cores="0-3" --max-memory=4096 mpi_simulation

# CPU-intensive tasks with core isolation
rnx run --cpu-cores="4-7" --max-memory=2048 scientific_computation

# Reserve cores for real-time applications
rnx run --cpu-cores="8-11" realtime_data_processor

# Scheduled HPC jobs
rnx run --schedule="2025-07-20T00:00:00" --cpu-cores="0-15" --max-memory=16384 overnight_simulation.py
```

#### Data Processing with Uploads

```bash
# Process large datasets with file upload
rnx run --upload-dir=datasets --cpu-cores="0-7" --max-memory=8192 python3 process_data.py

# Machine learning training with code and data upload
rnx run --upload=train.py --upload-dir=data --cpu-cores="0-3" --max-memory=4096 python3 train.py

# Bioinformatics pipeline with multiple input files
rnx run --upload-dir=genomes --upload=pipeline.sh --max-memory=16384 bash pipeline.sh

# Scheduled data processing
rnx run --schedule="6h" --upload-dir=daily-data --max-memory=8192 python3 daily_analysis.py
```

#### Development Workflow

```bash
# Test on remote development server with limited resources
rnx --server=dev-server:50051 run --upload-dir=. --cpu-cores="0-1" --max-memory=1024 pytest tests/

# Build project with uploaded source code
rnx run --upload-dir=src --cpu-cores="2-5" --max-memory=2048 make build

# Run security scans on uploaded codebase
rnx run --upload-dir=. --cpu-cores="6" --max-memory=512 trivy scan .

# Schedule nightly builds
rnx run --schedule="2025-07-20T01:00:00" --upload-dir=. --max-memory=4096 make build test
```

#### CI/CD Integration

```bash
# In CI pipeline with core allocation and artifact upload
export JOBLET_SERVER=build-server:50051
rnx run --upload-dir=. --cpu-cores="0-3" --max-memory=2048 docker build -t myapp .
rnx run --upload=test-config.yml --cpu-cores="4-5" --max-cpu=50 npm test

# Schedule deployment
rnx run --schedule="2025-07-20T18:00:00" --upload-dir=dist deploy.sh
```

## ⚙️ Configuration

### Daemon Configuration

The daemon uses embedded certificates generated during installation:

```yaml
# /opt/joblet/config/joblet-config.yml (auto-generated)
version: "3.0"
server:
  address: "192.168.1.100"
  port: 50051
  mode: "server"

joblet:
  defaultCpuLimit: 50
  defaultMemoryLimit: 256
  maxConcurrentJobs: 100

cgroup:
  baseDir: "/sys/fs/cgroup/joblet.slice/joblet.service"
  enableControllers: [ "memory", "cpu", "io", "pids", "cpuset" ]  # cpuset for core limiting

security:
  # Certificates embedded here automatically
  serverCert: |
    -----BEGIN CERTIFICATE-----
    ...
  serverKey: |
    -----BEGIN PRIVATE KEY-----
    ...
  caCert: |
    -----BEGIN CERTIFICATE-----
    ...
```

### RNX CLI Configuration

The CLI uses configuration with embedded certificates:

```yaml
# ~/.rnx/rnx-config.yml (auto-generated)
version: "3.0"
nodes:
  default:
    address: "192.168.1.100:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      ...
    key: |
      -----BEGIN PRIVATE KEY-----
      ...
    ca: |
      -----BEGIN CERTIFICATE-----
      ...

  production:
    address: "prod-server:50051"
    # ... additional certificates for production
```

## 🔒 Security Model

### Certificate-Based Authentication

- **Automatic Generation**: Certificates embedded in configuration during installation
- **Role-Based Access**: Admin (full access) vs Viewer (read-only) certificates
- **mTLS Security**: All communication encrypted with TLS 1.3+

### Job Isolation

- **Process Isolation**: Each job runs in separate PID namespace (can only see its own processes)
- **Filesystem Isolation**: Jobs run in chroot with limited host access
- **Resource Isolation**: CPU, memory, and I/O limits prevent resource exhaustion
- **CPU Core Isolation**: Jobs are bound to specific CPU cores for performance isolation
- **Network Isolation**: Optional network namespace isolation
- **Upload Security**: Path traversal protection and system directory access prevention
- **Scheduler Security**: Scheduled jobs maintain same isolation as immediate jobs

## 🛠️ Advanced Configuration

### CPU Core Management

#### System Core Allocation Strategy

```bash
# Check available cores
nproc                           # Show total cores
cat /proc/cpuinfo | grep processor | wc -l  # Alternative method

# View current core usage
htop                           # Interactive view
mpstat -P ALL 1                # Per-core statistics

# Check job core assignments
taskset -cp $(pgrep your_process)  # View process CPU affinity
```

#### Multi-Tenant Core Allocation

```bash
# User 1: Cores 0-3 for development
rnx run --cpu-cores="0-3" dev_workload

# User 2: Cores 4-7 for testing  
rnx run --cpu-cores="4-7" test_suite

# User 3: Cores 8-11 for production tasks
rnx run --cpu-cores="8-11" prod_service

# System: Reserve cores 12-15 for OS and services
# (no job allocation on these cores)

# Schedule different workloads on different cores
rnx run --schedule="2h" --cpu-cores="0-3" batch_dev.py
rnx run --schedule="4h" --cpu-cores="4-7" test_automation.py
```

#### NUMA-Aware Core Allocation

```bash
# Check NUMA topology
numactl --hardware

# Allocate cores from specific NUMA nodes
rnx run --cpu-cores="0-7" numa_node_0_workload     # NUMA node 0
rnx run --cpu-cores="8-15" numa_node_1_workload    # NUMA node 1

# Schedule NUMA-aware jobs
rnx run --schedule="1hour" --cpu-cores="0-7" --max-memory=4096 numa_optimized.py
```

### Regenerate Certificates

```bash
# On daemon server - regenerate with new IP
export JOBLET_SERVER_ADDRESS="new-server-ip"
sudo /usr/local/bin/certs_gen_embedded.sh

# Restart daemon
sudo systemctl restart joblet

# Update clients with new configuration
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

### Multiple Environments

```yaml
# ~/.rnx/rnx-config.yml
nodes:
  development:
    address: "dev.company.com:50051"
    # ... dev certificates

  staging:
    address: "staging.company.com:50051"
    # ... staging certificates

  production:
    address: "prod.company.com:50051"
    # ... production certificates
```

```bash
# Use specific environment
rnx --node=development run pytest
rnx --node=production run deploy.sh

# Schedule jobs on different environments
rnx --node=staging run --schedule="30min" integration_tests.py
rnx --node=production run --schedule="2025-07-20T02:00:00" backup.sh
```

### Service Management

```bash
# Daemon management
sudo systemctl start joblet    # Start daemon
sudo systemctl stop joblet     # Stop daemon
sudo systemctl status joblet   # Check status
sudo journalctl -u joblet -f   # View logs

# Configuration management
sudo dpkg-reconfigure joblet   # Reconfigure package
```

## 🚨 Troubleshooting

### Common Issues

**Connection refused**

```bash
# Check daemon status
sudo systemctl status joblet
sudo journalctl -u joblet --lines=20

# Test connectivity
telnet your-server 50051
```

**Certificate errors**

```bash
# Verify certificates
openssl x509 -in ~/.rnx/rnx-config.yml -noout -text

# Regenerate if needed
sudo /usr/local/bin/certs_gen_embedded.sh
```

**CPU core limiting not working**

```bash
# Check if cpuset controller is enabled
cat /sys/fs/cgroup/cgroup.controllers | grep cpuset

# Verify cgroup configuration
sudo cat /sys/fs/cgroup/joblet.slice/joblet.service/cgroup.subtree_control

# Check job core assignment
sudo find /sys/fs/cgroup -name "job-*" -exec sh -c 'echo "Job: {}"; cat {}/cpuset.cpus 2>/dev/null || echo "no cpuset"' \;
```

**Scheduled jobs not executing**

```bash
# Check scheduler status
sudo journalctl -u joblet | grep scheduler

# View scheduled jobs queue
rnx list | grep SCHEDULED

# Check system time synchronization
timedatectl status
```

**CLI not found**

```bash
# Check CLI installation
which rnx
ls -la /usr/local/bin/rnx

# Install if missing
sudo ln -sf /opt/joblet/rnx /usr/local/bin/rnx
```

### Performance Monitoring

```bash
# Monitor CPU core usage
mpstat -P ALL 2                               # Per-core statistics every 2 seconds

# Check process CPU affinity
ps -eo pid,comm,psr,pcpu | grep your_process  # PSR column shows current core

# Monitor cgroup resource usage
cat /sys/fs/cgroup/joblet.slice/joblet.service/job-*/cpuset.cpus
cat /sys/fs/cgroup/joblet.slice/joblet.service/job-*/memory.current

# Monitor scheduler performance
sudo journalctl -u joblet | grep "scheduled job"
```

### Getting Help

- **Configuration Examples**: `rnx config-help`
- **Node Information**: `rnx nodes`
- **GitHub Issues**: [Report bugs](https://github.com/ehsaniara/joblet/issues)
- **Documentation**: Full docs in `/docs` directory

## 📊 System Requirements

### Joblet Daemon (Linux Server)

- **OS**: Linux kernel 4.6+ (Ubuntu 18.04+, CentOS 8+)
- **CPU**: Multi-core CPU recommended for core limiting features
- **Memory**: 1GB+ RAM
- **Disk**: 5GB+ available space
- **Network**: Port 50051 (configurable)
- **Privileges**: Root access for namespace operations
- **Cgroups**: cgroups v2 with cpuset controller support

### RNX CLI (Client)

- **OS**: Linux, macOS 10.15+, Windows 10+
- **Memory**: 50MB+ RAM
- **Network**: Access to daemon port 50051

## 🚀 Command Reference

### Resource Limiting Flags

| Flag               | Description                | Example                                      |
|--------------------|----------------------------|----------------------------------------------|
| `--max-cpu=N`      | CPU time percentage limit  | `--max-cpu=50` (50% CPU time)                |
| `--max-memory=N`   | Memory limit in MB         | `--max-memory=1024` (1GB)                    |
| `--max-iobps=N`    | I/O bandwidth in bytes/sec | `--max-iobps=10485760` (10MB/s)              |
| `--cpu-cores=SPEC` | CPU cores specification    | `--cpu-cores="0-3"` or `--cpu-cores="1,3,5"` |

### File Upload Flags

| Flag               | Description              | Example                |
|--------------------|--------------------------|------------------------|
| `--upload=FILE`    | Upload single file       | `--upload=script.py`   |
| `--upload-dir=DIR` | Upload entire directory  | `--upload-dir=dataset` |
| `--upload-dir=.`   | Upload current directory | `--upload-dir=.`       |

### Scheduling Flag

| Flag              | Description                       | Example                                                    |
|-------------------|-----------------------------------|------------------------------------------------------------|
| `--schedule=SPEC` | Schedule job for future execution | `--schedule="1hour"` or `--schedule="2025-07-20T15:30:00"` |

### CPU Core Specification Examples

| Format          | Description     | Cores Used          |
|-----------------|-----------------|---------------------|
| `"0"`           | Single core     | Core 0 only         |
| `"0-3"`         | Core range      | Cores 0, 1, 2, 3    |
| `"1,3,5"`       | Specific cores  | Cores 1, 3, 5 only  |
| `"0-1,4-5"`     | Multiple ranges | Cores 0, 1, 4, 5    |
| `""` or omitted | No restriction  | All available cores |

### Schedule Specification Examples

| Format                        | Description            | Execution Time             |
|-------------------------------|------------------------|----------------------------|
| `"30s"`                       | 30 seconds from now    | Relative time              |
| `"5min"`                      | 5 minutes from now     | Relative time              |
| `"1hour"`                     | 1 hour from now        | Relative time              |
| `"2h30m"`                     | 2.5 hours from now     | Relative time              |
| `"2025-07-20T15:30:00"`       | Absolute local time    | Specific date/time         |
| `"2025-07-20T15:30:00Z"`      | Absolute UTC time      | Specific date/time (UTC)   |
| `"2025-07-20T15:30:00-07:00"` | Absolute with timezone | Specific date/time with TZ |

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Commit changes: `git commit -m 'Add amazing feature'`
4. Push to branch: `git push origin feature/amazing-feature`
5. Open Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Quick Links:**

- [Releases](https://github.com/ehsaniara/joblet/releases) - Download latest version
- [Examples](https://github.com/ehsaniara/joblet/tree/main/examples) - Usage examples
- [Contributing](.github/CONTRIBUTING.md) - Development guide

## 💡 Project Name History

This project was previously known as "Worker" but was renamed to:

- **Joblet** (daemon) - Better describes the job execution platform
- **RNX** (CLI) - Remote eXecution, a concise and memorable name

The rename provides clearer branding and eliminates confusion with other "worker" tools in the ecosystem.