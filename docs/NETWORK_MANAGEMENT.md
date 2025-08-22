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
- **Traffic control** and bandwidth limiting
- **DNS configuration** per network
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

## Network Modes

### 1. Bridge Network (Default)

The default network mode with NAT and internet access.

```bash
# Uses bridge network by default
rnx run ping google.com

# Explicitly specify bridge
rnx run --network=bridge curl https://api.example.com
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
rnx run --network=myapp ip addr show
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
rnx run --network=isolated wget https://example.com

# No access to other jobs, but external access works
rnx run --network=isolated ping google.com
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
rnx run --network=none ip addr show

# Useful for secure processing
rnx run --network=none --upload=sensitive.data process_offline.sh
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
```

## Network Isolation

### Isolated Workloads

```bash
# Create isolated networks for different tenants
rnx network create tenant-a --cidr=10.100.0.0/24
rnx network create tenant-b --cidr=10.101.0.0/24

# Run jobs in isolated networks
rnx run --network=tenant-a nginx
rnx run --network=tenant-b nginx

# Jobs cannot communicate across networks
rnx run --network=tenant-a ping 10.101.0.2  # Will fail
```

### Security Groups Simulation

```bash
# Database network (no internet)
rnx network create db-net --cidr=10.20.0.0/24

# Application network (with internet)
rnx network create app-net --cidr=10.21.0.0/24

# Run database in isolated network
rnx run \
  --network=db-net \
  --volume=pgdata \
  postgres:latest

# Run app with access to both networks (if supported)
# Note: Joblet typically supports one network per job
```

## Inter-Job Communication

### Service Discovery

```bash
# Start a service
rnx run \
  --network=app-net \
  redis-server

# Get service IP
SERVICE_IP=$(rnx run --network=app-net --json hostname -I | jq -r .output | awk '{print $1}')

# Connect from another job
rnx run \
  --network=app-net \
  python app.py
```

### Multi-Service Application

```bash
# Create application network
rnx network create microservices --cidr=10.30.0.0/24

# Start backend service
BACKEND_JOB=$(rnx run --json \
  --network=microservices \
  python -m http.server 8000 | jq -r .id)

# Get backend IP
sleep 2
BACKEND_IP=$(rnx run --network=microservices ip addr show | grep "inet 10.30" | awk '{print $2}' | cut -d/ -f1)

# Start frontend connecting to backend
rnx run \
  --network=microservices \
  node frontend.js
```

### Load Balancing Pattern

```bash
# Start multiple backend instances
for i in {1..3}; do
  rnx run \
    --network=app-net \
    python app.py --port=$((8000 + i)) &
done

# Simple load balancer
rnx run \
  --network=app-net \
  --upload=nginx.conf \
  nginx -c /work/nginx.conf
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
$ rnx run --workflow=microservices-workflow.yaml
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
rnx run \
  --network=secure-db \
  --volume=secure-data \
  --env=POSTGRES_PASSWORD_FILE=/volumes/secure-data/password \
  postgres

# 3. Access only through application
rnx run \
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
rnx run \
  --network=none \
  --volume=offline-data \
  python process_sensitive.py
```

## Performance and Bandwidth

### Bandwidth Limiting

```bash
# Limit network bandwidth via I/O limits
# Note: --max-iobps affects all I/O, not just network

# Run with bandwidth limit (10MB/s total I/O)
rnx run \
  --max-iobps=10485760 \
  --network=bridge \
  wget https://example.com/large-file.zip

# Test network performance
rnx run --network=bridge iperf3 -c iperf.server.com
```

### Network Performance Testing

```bash
# Test latency
rnx run --network=mynet ping -c 10 8.8.8.8

# Test bandwidth
rnx run --network=mynet \
  curl -o /dev/null -w "Download Speed: %{speed_download} bytes/sec\n" \
  https://speed.cloudflare.com/__down?bytes=10000000

# DNS performance
rnx run --network=mynet \
  dig @8.8.8.8 google.com
```

### Optimizing Network Performance

```bash
# Use host network for maximum performance
rnx run --network=host iperf3 -s

# Minimize network hops
# Place related services in same network
rnx network create fast-local --cidr=10.70.0.0/24

# Run communicating services together
rnx run --network=fast-local data-producer
rnx run --network=fast-local data-consumer
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
rnx network create redis-cluster --cidr=10.5.1.0/24
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
rnx run --network=dmz nginx
rnx run --network=app-tier python app.py
rnx run --network=data-tier postgres
```

### 4. Network Cleanup

```bash
# Regular cleanup script
#!/bin/bash
# List unused networks (no running jobs)
for network in $(rnx network list --json | jq -r '.[].name'); do
  if [ "$network" != "bridge" ]; then
    JOBS=$(rnx list --json | jq --arg net "$network" '.[] | select(.network == $net) | .id')
    if [ -z "$JOBS" ]; then
      echo "Network $network appears unused"
      # rnx network delete $network
    fi
  fi
done

# Remove test networks
rnx network list --json | \
  jq -r '.[] | select(.name | startswith("test-")) | .name' | \
  xargs -I {} rnx network delete {}
```

### 5. Monitoring

```bash
# Network usage monitoring
rnx run --network=myapp ifstat 1 10

# Connection tracking
rnx run --network=myapp netstat -an | grep ESTABLISHED

# DNS resolution testing
rnx run --network=myapp nslookup google.com
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
rnx run --network=custom-net ping 8.8.8.8

# If fails, check if network has NAT enabled
# Bridge network should have internet access
rnx run --network=bridge ping google.com
```

**3. Jobs Cannot Communicate**

```bash
# Ensure jobs are in same network
rnx run --network=app-net nc -l 8080
rnx run --network=app-net nc <job-ip> 8080  # Should connect

# Different networks cannot communicate
rnx run --network=net1 nc -l 8080
rnx run --network=net2 nc <job-ip> 8080  # Will fail
```

**4. DNS Resolution Issues**

```bash
# Test DNS
rnx run --network=mynet nslookup google.com

# Check resolv.conf
rnx run --network=mynet cat /etc/resolv.conf

# Use specific DNS
rnx run --network=mynet dig @1.1.1.1 cloudflare.com
```

**5. Network Performance Issues**

```bash
# Check network interface
rnx run --network=slow-net ip link show

# Test bandwidth
rnx run --network=slow-net \
  dd if=/dev/zero bs=1M count=100 | nc remote-host 9999

# Check for packet loss
rnx run --network=slow-net ping -c 100 target-host
```

### Debugging Commands

```bash
# Show network interfaces
rnx run --network=debug-net ip addr show

# Show routing table
rnx run --network=debug-net ip route show

# Show network statistics
rnx run --network=debug-net netstat -i

# Trace network path
rnx run --network=debug-net traceroute google.com

# Check connectivity
rnx run --network=debug-net nc -zv target-host 80
```

## Examples

### Microservices Architecture

```bash
# Create service networks
rnx network create frontend-net --cidr=10.80.1.0/24
rnx network create backend-net --cidr=10.80.2.0/24
rnx network create cache-net --cidr=10.80.3.0/24

# Deploy services
# Frontend
rnx run \
  --network=frontend-net \
  --upload-dir=./frontend \
  npm start

# Backend API
rnx run \
  --network=backend-net \
  --upload-dir=./backend \
  python app.py

# Cache layer
rnx run \
  --network=cache-net \
  redis-server
```

### Testing Environment

```bash
# Create isolated test network
rnx network create test-env --cidr=10.90.0.0/24

# Run test database
rnx run \
  --network=test-env \
  postgres

# Run integration tests
rnx run \
  --network=test-env \
  --upload-dir=./tests \
  pytest integration_tests/
```

### Development Workflow

```bash
# Create development network
rnx network create dev --cidr=10.100.0.0/24

# Start all services
docker-compose -f dev-services.yml up -d

# Run development job with hot reload
rnx run \
  --network=dev \
  --volume=code \
  --upload-dir=./src \
  nodemon app.js
```

## See Also

- [Job Execution Guide](./JOB_EXECUTION.md)
- [Volume Management](./VOLUME_MANAGEMENT.md)
- [Security Guide](./SECURITY.md)
- [Configuration Guide](./CONFIGURATION.md)