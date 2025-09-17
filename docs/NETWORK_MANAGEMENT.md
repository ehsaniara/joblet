# Network Management Guide

Complete guide to managing network isolation and custom networks in Joblet.

## Table of Contents

- [Network Overview](#network-overview)
- [Network Modes](#network-modes)
- [Creating Custom Networks](#creating-custom-networks)
- [Network Isolation](#network-isolation)
- [Inter-Job Communication](#inter-job-communication)
- [Network Security](#network-security)
- [Performance and Bandwidth](#performance-and-bandwidth)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Network Overview

Joblet provides comprehensive network management capabilities:

- **Network isolation** between jobs
- **Custom networks** with defined CIDR ranges
- **Resource limits** (memory, CPU, I/O) with network isolation
- **DNS configuration** per network via /etc/hosts
- **Inter-job communication** within same network

### Network Architecture

```
┌─────────────────────────────────────────────┐
│                Joblet Host                  │
│                                             │
│  ┌──────────────┐  ┌─────────────┐          │
│  │   Bridge     │  │   Custom    │          │
│  │   Network    │  │   Network   │          │
│  │ 172.20.0.0/16│  │ 10.10.0.0/24│          │
│  └──────┬───────┘  └──────┬──────┘          │
│         │                 │                 │
│    ┌────┴───┐         ┌───┴────┐            │
│    │ Job A  │         │ Job C  │            │
│    └────────┘         └────────┘            │
│    ┌────────┐         ┌────────┐            │
│    │ Job B  │         │ Job D  │            │
│    └────────┘         └────────┘            │
└─────────────────────────────────────────────┘
```

## Network Implementation: 2-Phase Setup

Joblet uses a sophisticated 2-phase network setup to ensure proper network namespace configuration without race
conditions.

### The Problem

Network namespace configuration requires:

- A process PID to attach the namespace to
- Network configuration (IP, routes, interfaces)

However, there's a timing challenge:

- Network namespaces can only be configured after the process exists
- But the process needs to wait for network configuration before executing

### The Solution: 2-Phase Network Setup

#### Phase 1: Resource Allocation (Pre-Launch)

Before the job process is created:

```go
// Allocate IP address from network pool
ip := networkStore.AllocateIP("bridge") // e.g., 172.20.0.19

// Store allocation for later retrieval
networkStore.AssignJobToNetwork(jobID, "bridge", allocation)

// Create synchronization file path
networkReadyFile := "/tmp/joblet-network-ready-{jobID}"

// Pass to job via environment
environment["NETWORK_READY_FILE"] = networkReadyFile
```

#### Phase 2: Namespace Configuration (Post-Launch)

After the job process is created with PID:

```go
// Now we have the process PID
pid := process.Pid()

// Create veth pair interfaces
ip link add veth-h-{jobID} type veth peer name veth-p-{jobID}

// Move peer interface into job's network namespace
ip link set veth-p-{jobID} netns {pid}

// Configure IP and routes inside namespace
nsenter --net = /proc/{pid}/ns/net ip addr add {ip}/16 dev veth-p-{jobID}
nsenter --net = /proc/{pid}/ns/net ip link set veth-p-{jobID} up
nsenter --net = /proc/{pid}/ns/net ip route add default via {gateway}

// Signal to job that network is ready
echo "ready" > {networkReadyFile}
```

### Synchronization Mechanism

The job process waits for network setup before continuing:

```go
// Job init process (server.go)
func waitForNetworkReady() {
networkFile := os.Getenv("NETWORK_READY_FILE")
if networkFile == "" {
return // No network setup needed (e.g., "none" network)
}

// Wait for signal file
for i := 0; i < 100; i++ {
if _, err := os.Stat(networkFile); err == nil {
os.Remove(networkFile) // Clean up
return                 // Network ready!
}
time.Sleep(100 * time.Millisecond)
}
panic("timeout waiting for network setup")
}
```

### Sequence Diagram

```
Server (Coordinator)          Job Process              NetworkService
        |                          |                         |
        |-- Phase 1: Allocate IP --|------------------------>|
        |<-------------------------|-- IP: 172.20.0.19 ------|
        |                          |                         |
        |-- Launch Process ------->|                         |
        |                          |-- Start (PID: 12345) -->|
        |                          |                         |
        |-- Phase 2: Setup NS -----|------------------------>|
        |                          |                         |
        |                          |<- Create veth pairs ----|
        |                          |<- Configure namespace --|
        |                          |                         |
        |-- Write Ready File ------|------------------------>|
        |                          |                         |
        |                          |-- Check Ready File ---->|
        |                          |-- Network Ready! ------>|
        |                          |-- Continue Execution -->|
```

### Benefits

1. **No Race Conditions**: Network is guaranteed to be ready before job executes
2. **Clean Separation**: Resource allocation separate from namespace configuration
3. **Flexible**: Works with all network types (bridge, isolated, custom)
4. **Simple Synchronization**: File-based signaling avoids complex IPC
5. **Robust**: Handles timeouts and cleanup gracefully

### Testing the 2-Phase Setup

The network test suite verifies this implementation:

```bash
# Test output showing 2-phase operation
[DEBUG] [init] waiting for network setup | file=/tmp/joblet-network-ready-abc123
[DEBUG] [init] blocking on network ready signal file...
[DEBUG] [server] configuring network namespace with process PID | pid=12345
[DEBUG] [init] network setup signal received, proceeding with initialization
```

Tests verify:

- Phase 1 allocates IP before launch
- Phase 2 configures namespace after PID exists
- Job waits for network before continuing
- "none" network skips both phases
- Proper cleanup of synchronization files

## Network Modes

### 1. Bridge Network (Default)

The default network mode with NAT and internet access.

```bash
# Uses bridge network by default
rnx job run ping google.com

# Explicitly specify bridge
rnx job run --network=bridge curl https://api.example.com
```

**Characteristics:**

- Internet access via NAT
- Isolated from other networks
- DHCP IP assignment
- Default CIDR: 172.20.0.0/16

### 2. Custom Networks

User-defined networks with specific CIDR ranges.

```bash
# Create custom network
rnx network create myapp --cidr=10.10.0.0/24

# Use custom network
rnx job run --network=myapp ip addr show
```

**Characteristics:**

- Isolated from other networks
- Custom CIDR range
- Inter-job communication within network
- External internet access via NAT

### 3. Isolated Network

External-only network access with no inter-job communication.

```bash
# Run with isolated network
rnx job run --network=isolated wget https://example.com

# No access to other jobs, but external access works
rnx job run --network=isolated ping google.com
```

**Characteristics:**

- Complete isolation from other jobs
- External internet access via NAT
- Point-to-point veth connection
- Maximum security for external operations

### 4. None Network

Complete network isolation - no network access.

```bash
# No network access
rnx job run --network=none ip addr show

# Useful for secure processing
rnx job run --network=none --upload=sensitive.data process_offline.sh
```

**Characteristics:**

- Complete isolation
- No network interfaces (except loopback)
- Maximum security
- No external communication

## Creating Custom Networks

### Basic Network Creation

```bash
# Create network with CIDR
rnx network create development --cidr=10.1.0.0/24

# Create multiple networks for different environments
rnx network create testing --cidr=10.2.0.0/24
rnx network create staging --cidr=10.3.0.0/24
rnx network create production --cidr=10.4.0.0/24
```

### CIDR Range Selection

Choose non-overlapping private IP ranges:

```bash
# Class A private (large networks)
rnx network create large-net --cidr=10.0.0.0/8

# Class B private (medium networks)
rnx network create medium-net --cidr=172.16.0.0/12

# Class C private (small networks)
rnx network create small-net --cidr=192.168.1.0/24

# Custom subnets
rnx network create app-tier --cidr=10.10.1.0/24
rnx network create db-tier --cidr=10.10.2.0/24
rnx network create cache-tier --cidr=10.10.3.0/24
```

### Network Information

```bash
# List all networks
rnx network list

# Output format:
# NAME          CIDR            BRIDGE
# bridge        172.20.0.0/16   joblet0
# development   10.1.0.0/24     joblet-dev
# production    10.4.0.0/24     joblet-prod

# JSON output
rnx network list --json

# Output format:
# {
#   "networks": [
#     {"name": "bridge", "cidr": "172.20.0.0/16", "bridge": "joblet0", "builtin": true},
#     {"name": "development", "cidr": "10.1.0.0/24", "bridge": "joblet-dev", "builtin": false}
#   ]
# }
```

## Network Isolation

### Isolated Workloads

```bash
# Create isolated networks for different tenants
rnx network create tenant-a --cidr=10.100.0.0/24
rnx network create tenant-b --cidr=10.101.0.0/24

# Run jobs in isolated networks
rnx job run --network=tenant-a nginx
rnx job run --network=tenant-b nginx

# Jobs cannot communicate across networks
rnx job run --network=tenant-a ping 10.101.0.2  # Will fail
```

### Security Groups Simulation

```bash
# Database network (no internet)
rnx network create db-net --cidr=10.20.0.0/24

# Application network (with internet)
rnx network create app-net --cidr=10.21.0.0/24

# Run database in isolated network
rnx job run --network=db-net --volume=pgdata postgres:latest

# Run app with access to both networks (if supported)
# Note: Joblet typically supports one network per job
```

## Inter-Job Communication

### Distributed Training Example

```bash
# Start parameter server for distributed training
rnx job run \
  --network=training-net \
  --name=ps-0 \
  python3 parameter_server.py --port=2222

# Get parameter server IP
PS_IP=$(rnx job run --network=training-net --json hostname -I | jq -r .output | awk '{print $1}')

# Start worker nodes
rnx job run \
  --network=training-net \
  python3 worker.py --ps-host=$PS_IP --worker-id=0
```

### Multi-Service Application

```bash
# Create application network
rnx network create microservices --cidr=10.30.0.0/24

# Start backend service
BACKEND_JOB=$(rnx job run --json \
  --network=microservices \
  python -m http.server 8000 | jq -r .id)

# Get backend IP
sleep 2
BACKEND_IP=$(rnx job run --network=microservices ip addr show | grep "inet 10.30" | awk '{print $2}' | cut -d/ -f1)

# Start frontend connecting to backend
rnx job run --network=microservices node frontend.js
```

### Load Balancing Pattern

```bash
# Start multiple backend instances
for i in {1..3}; do
  rnx job run --network=app-net python app.py --port=$((8000 + i)) &
done

# Simple load balancer
rnx job run --network=app-net --upload=nginx.conf nginx -c /work/nginx.conf
```

### Workflow Network Configuration

Workflows support per-job network configuration with comprehensive validation:

```yaml
# microservices-workflow.yaml
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

**Workflow Network Validation:**

```bash
$ rnx job run --workflow=microservices-workflow.yaml
🔍 Validating workflow prerequisites...
✅ All required networks exist
```

**Benefits:**

- **Network Validation**: Confirms all networks exist before execution
- **Per-Job Networks**: Each job can use different networks
- **Dependency Coordination**: Network setup happens before job dependencies
- **Error Prevention**: Catches network misconfigurations early

## Network Security

### Firewall Rules (Conceptual)

While Joblet doesn't have built-in firewall rules, you can implement security patterns:

```bash
# Secure database pattern
# 1. Create isolated network
rnx network create secure-db --cidr=10.50.0.0/24

# 2. Run database with no external access
rnx job run \
  --network=secure-db \
  --volume=secure-data \
  --env=POSTGRES_PASSWORD_FILE=/volumes/secure-data/password \
  postgres

# 3. Access only through application
rnx job run \
  --network=secure-db \
  --upload=db-client.py \
  python db-client.py
```

### Network Policies

```bash
# Development network with full access
rnx network create dev-full --cidr=10.60.0.0/24

# Production network with restrictions
rnx network create prod-restricted --cidr=10.61.0.0/24

# Use network=none for maximum security
rnx job run \
  --network=none \
  --volume=offline-data \
  python process_sensitive.py
```

## Performance and Resource Management

### Resource Limits

```bash
# Limit total I/O operations (affects network, disk, all I/O)
# Note: --max-iobps controls all I/O operations, not network-specific bandwidth
rnx job run \
  --max-iobps=10485760 \
  --network=bridge \
  wget https://example.com/large-file.zip

# Set memory and CPU limits along with network isolation
rnx job run \
  --max-memory=512M \
  --max-cpu=50 \
  --network=isolated \
  python data-processing.py

# Test network performance within resource constraints
rnx job run --network=bridge iperf3 -c iperf.server.com
```

### Network Performance Testing

```bash
# Test latency
rnx job run --network=mynet ping -c 10 8.8.8.8

# Test bandwidth
rnx job run --network=mynet \
  curl -o /dev/null -w "Download Speed: %{speed_download} bytes/sec\n" \
  https://speed.cloudflare.com/__down?bytes=10000000

# DNS performance
rnx job run --network=mynet \
  dig @8.8.8.8 google.com
```

### Optimizing Network Performance

```bash
# Minimize network hops by placing related services in same network
rnx network create fast-local --cidr=10.70.0.0/24

# Run communicating services together in same network
rnx job run --network=fast-local data-producer
rnx job run --network=fast-local data-consumer

# Use bridge network for balanced performance and isolation
rnx job run --network=bridge iperf3 -c remote-server
```

## Best Practices

### 1. Network Planning

```bash
# Use consistent CIDR allocation
# Development: 10.1.0.0/16
# Testing:     10.2.0.0/16  
# Staging:     10.3.0.0/16
# Production:  10.4.0.0/16

# Document network usage
cat > networks.md << EOF
# Network Allocation

| Network | CIDR | Purpose |
|---------|------|---------|
| dev-web | 10.1.1.0/24 | Development web tier |
| dev-db  | 10.1.2.0/24 | Development database |
| test-integration | 10.2.1.0/24 | Integration testing |
EOF
```

### 2. Naming Conventions

```bash
# Environment-based naming
rnx network create prod-frontend --cidr=10.4.1.0/24
rnx network create prod-backend --cidr=10.4.2.0/24
rnx network create prod-database --cidr=10.4.3.0/24

# Service-based naming
rnx network create cache-cluster --cidr=10.5.1.0/24
rnx network create kafka-cluster --cidr=10.5.2.0/24

# Project-based naming
rnx network create project-alpha --cidr=10.6.0.0/24
rnx network create project-beta --cidr=10.7.0.0/24
```

### 3. Security Patterns

```bash
# Three-tier architecture
rnx network create dmz --cidr=10.10.1.0/24         # Public facing
rnx network create app-tier --cidr=10.10.2.0/24    # Application
rnx network create data-tier --cidr=10.10.3.0/24   # Database

# Run services in appropriate tiers
rnx job run --network=dmz nginx
rnx job run --network=app-tier python app.py
rnx job run --network=data-tier postgres
```

### 4. Network Cleanup

```bash
# Regular cleanup script
#!/bin/bash
# List unused networks (no running jobs)
for network in $(rnx network list --json | jq -r '.networks[].name'); do
  if [ "$network" != "bridge" ]; then
    JOBS=$(rnx job list --json | jq --arg net "$network" '.[] | select(.network == $net) | .id')
    if [ -z "$JOBS" ]; then
      echo "Network $network appears unused"
      # rnx network remove $network
    fi
  fi
done

# Remove test networks
rnx network list --json | \
  jq -r '.networks[] | select(.name | startswith("test-")) | .name' | \
  xargs -I {} rnx network remove {}
```

### 5. Monitoring

```bash
# Network usage monitoring
rnx job run --network=myapp ifstat 1 10

# Connection tracking
rnx job run --network=myapp netstat -an | grep ESTABLISHED

# DNS resolution testing
rnx job run --network=myapp nslookup google.com
```

## Troubleshooting

### Common Issues

**1. Network Creation Fails**

```bash
# Error: "failed to create network: CIDR overlaps with existing network"
# Solution: Use different CIDR range
rnx network list  # Check existing networks
rnx network create mynet --cidr=10.99.0.0/24  # Use unique range
```

**2. No Internet Access**

```bash
# Check network mode
rnx job run --network=custom-net ping 8.8.8.8

# If fails, check if network has NAT enabled
# Bridge network should have internet access
rnx job run --network=bridge ping google.com
```

**3. Jobs Cannot Communicate**

```bash
# Ensure jobs are in same network
rnx job run --network=app-net nc -l 8080
rnx job run --network=app-net nc <job-ip> 8080  # Should connect

# Different networks cannot communicate
rnx job run --network=net1 nc -l 8080
rnx job run --network=net2 nc <job-ip> 8080  # Will fail
```

**4. DNS Resolution Issues**

```bash
# Test DNS
rnx job run --network=mynet nslookup google.com

# Check resolv.conf
rnx job run --network=mynet cat /etc/resolv.conf

# Use specific DNS
rnx job run --network=mynet dig @1.1.1.1 cloudflare.com
```

**5. Network Performance Issues**

```bash
# Check network interface
rnx job run --network=slow-net ip link show

# Test bandwidth
rnx job run --network=slow-net \
  dd if=/dev/zero bs=1M count=100 | nc remote-host 9999

# Check for packet loss
rnx job run --network=slow-net ping -c 100 target-host
```

### Debugging Commands

```bash
# Show network interfaces
rnx job run --network=debug-net ip addr show

# Show routing table
rnx job run --network=debug-net ip route show

# Show network statistics
rnx job run --network=debug-net netstat -i

# Trace network path
rnx job run --network=debug-net traceroute google.com

# Check connectivity
rnx job run --network=debug-net nc -zv target-host 80
```

## Examples

### Microservices Architecture

```bash
# Create networks for distributed ML pipeline
rnx network create training-net --cidr=10.80.1.0/24
rnx network create inference-net --cidr=10.80.2.0/24
rnx network create data-processing-net --cidr=10.80.3.0/24

# Deploy ML pipeline components
# Training job with GPUs
rnx job run \
  --network=training-net \
  --upload-dir=./models \
  python3 train_model.py --epochs=100 --batch-size=128

# Inference/evaluation job
rnx job run \
  --network=inference-net \
  --upload-dir=./models \
  python3 evaluate.py --model-path=/volumes/models/latest

# Data preprocessing
rnx job run \
  --network=data-processing-net \
  python3 preprocess.py --input=/volumes/raw --output=/volumes/processed
```

### Testing Environment

```bash
# Create isolated test network
rnx network create test-env --cidr=10.90.0.0/24

# Run test database
rnx job run \
  --network=test-env \
  postgres

# Run integration tests
rnx job run \
  --network=test-env \
  --upload-dir=./tests \
  pytest integration_tests/
```

### Development Workflow

```bash
# Create development network for ML experimentation
rnx network create ml-dev --cidr=10.100.0.0/24

# Start supporting services for ML development
# Jupyter notebook server for experimentation
rnx job run --network=ml-dev --name=jupyter python3 -m jupyter notebook --ip=0.0.0.0
# TensorBoard for monitoring training
rnx job run --network=ml-dev --name=tensorboard tensorboard --logdir=/volumes/logs

# Run training experiment with live monitoring
rnx job run \
  --network=ml-dev \
  --volume=experiments \
  --upload-dir=./experiments \
  python3 experiment.py --tensorboard-host=tensorboard
```

## See Also

- [Job Execution Guide](./JOB_EXECUTION.md)
- [Volume Management](./VOLUME_MANAGEMENT.md)
- [Security Guide](./SECURITY.md)
- [Configuration Guide](./CONFIGURATION.md)