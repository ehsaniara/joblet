# Runtime Advanced Guide

This document consolidates advanced runtime implementation details, security considerations, enterprise deployment
patterns, and sophisticated scenarios for production environments.

## Table of Contents

- [Part 1: Implementation Architecture](#part-1-implementation-architecture)
- [Part 2: Security & Isolation](#part-2-security--isolation)
- [Part 3: Deployment Strategies](#part-3-deployment-strategies)
- [Part 4: Enterprise Patterns](#part-4-enterprise-patterns)

---

# Part 1: Implementation Architecture

## Implementation Overview

The Joblet runtime system provides a sophisticated isolation and mounting architecture for secure job execution.

### Core Components

1. **Runtime Resolver** (`internal/joblet/runtime/resolver.go`)
    - Parses runtime specifications (e.g., `openjdk-21`, `python-3.11-ml`)
    - Locates runtime directories and configurations
    - Validates system compatibility

2. **Filesystem Isolator** (`internal/joblet/core/filesystem/isolator.go`)
    - Handles mounting of runtime `isolated/` directories into job filesystems
    - Manages environment variable injection from runtime.yml
    - Provides complete filesystem isolation using self-contained runtime structures

3. **Runtime Types** (`internal/joblet/runtime/types.go`)
    - Defines data structures for runtime configurations
    - Simplified structure supporting self-contained runtimes

4. **CLI Integration** (`internal/rnx/resources/runtime.go`)
    - Runtime management commands (`rnx runtime install/list/info`)
    - Runtime installation using platform-specific setup scripts

### Directory Structure

Runtime configurations are stored in `/opt/joblet/runtimes/`:

```
/opt/joblet/runtimes/
├── openjdk-21/
│   ├── isolated/          # Self-contained runtime files
│   │   ├── usr/bin/       # System binaries
│   │   ├── usr/lib/       # System libraries
│   │   ├── usr/lib/jvm/   # Java installation
│   │   ├── etc/           # Configuration files
│   │   └── ...            # Complete runtime environment
│   └── runtime.yml        # Runtime configuration
└── python-3.11-ml/
    ├── isolated/          # Self-contained Python+ML environment
    └── runtime.yml
```

### Runtime Configuration Format

```yaml
name: openjdk-21
version: "21.0.8"
description: "OpenJDK 21 - self-contained (1872 files)"

# All mounts from isolated/ - no host dependencies
mounts:
  # Essential system binaries
  - source: "isolated/usr/bin"
    target: "/usr/bin"
    readonly: true
  # Essential libraries
  - source: "isolated/lib"
    target: "/lib"
    readonly: true
  # Java-specific mounts
  - source: "isolated/usr/lib/jvm"
    target: "/usr/lib/jvm"
    readonly: true

environment:
  JAVA_HOME: "/usr/lib/jvm"
  PATH: "/usr/lib/jvm/bin:/usr/bin:/bin"
  JAVA_VERSION: "21.0.8"
  LD_LIBRARY_PATH: "/usr/lib/x86_64-linux-gnu:/lib/x86_64-linux-gnu:/usr/lib/jvm/lib"
```

## Template-Based Installation System

Runtime installation uses a modular, template-based architecture:

### Strategy Pattern Components

1. **Runtime Installer Interface** (`internal/joblet/runtime/installers/interfaces.go`)
    - Unified interface for all runtime installation types
    - Standardized InstallSpec and InstallResult structures
    - Support for GitHub, local, and script-based sources

2. **Installer Manager** (`internal/joblet/runtime/installers/manager.go`)
    - Central coordinator that delegates to appropriate installers
    - Source type detection and routing
    - Error handling and validation

3. **Template Engine** (`internal/joblet/runtime/installers/base.go`)
    - Go template rendering with embedded template files
    - Parameterized shell script generation
    - Runtime-specific variable substitution

## Basic Usage Examples

```bash
# List available runtimes
rnx runtime list

# Get runtime information
rnx runtime info openjdk-21

# Run job with runtime
rnx job run --runtime=openjdk-21 java -version

# Upload and run script with runtime
rnx job run --runtime=openjdk-21 --upload=App.java bash -c "javac App.java && java App"

# ML/Data Science with Python
rnx job run --runtime=python-3.11-ml \
        --volume=datasets \
        --upload=analysis.py \
        python analysis.py
```

## Integration Points

- **Job Domain Object**: Extended `Job` struct with `Runtime` field
- **Execution Engine**: Enhanced with runtime manager integration
- **gRPC Protocol**: Added `runtime` field to `RunJobReq` message
- **CLI Commands**: Extended `rnx job run` with `--runtime` flag

---

# Part 2: Security & Isolation

## Runtime Isolation Cleanup System

The cleanup system transforms builder chroot runtime installations into secure, isolated runtime environments for
production jobs.

### Security Problem Statement

When runtimes are built in the builder chroot environment, they initially have access to the full host OS filesystem.
This creates a security risk if not properly isolated.

**Before Cleanup (INSECURE):**

```yaml
# Points to actual host OS paths
mounts:
  - source: "usr/lib/jvm/java-21-openjdk-amd64"  # HOST PATH!
    target: "/usr/lib/jvm/java-21-openjdk-amd64"
    readonly: true
```

### Cleanup Process

The cleanup system transforms runtime installations into isolated, self-contained packages:

```
1. Runtime Built in Builder Chroot
2. Cleanup Phase Initiated
3. Parse runtime.yml
4. Create isolated/ Directory
5. Copy Runtime Files from Host Paths
6. Update runtime.yml with Isolated Paths
7. Backup Original Configuration
8. Secure Runtime Ready for Production
```

### Directory Structure Transformation

**Before Cleanup:**

```
/opt/joblet/runtimes/java/openjdk-21/
├── runtime.yml                # Points to host OS paths
└── setup.sh
```

**After Cleanup (SECURE):**

```
/opt/joblet/runtimes/java/openjdk-21/
├── isolated/                 # Self-contained runtime files
│   ├── usr/
│   │   ├── lib/jvm/          # Copied Java installation
│   │   └── bin/              # Copied Java binaries
│   └── etc/ssl/certs/        # Copied certificates
├── runtime.yml               # Updated with isolated paths
├── runtime.yml.original      # Backup of original
└── setup.sh
```

### File Copying Strategy

```bash
# Java Runtime Cleanup Example
mkdir -p "/opt/joblet/runtimes/java/openjdk-21/isolated/usr/lib/jvm"
cp -r "/usr/lib/jvm/java-21-openjdk-amd64" \
      "/opt/joblet/runtimes/java/openjdk-21/isolated/usr/lib/jvm/"

mkdir -p "/opt/joblet/runtimes/java/openjdk-21/isolated/usr/bin"
cp "/usr/bin/java" "/opt/joblet/runtimes/java/openjdk-21/isolated/usr/bin/"
cp "/usr/bin/javac" "/opt/joblet/runtimes/java/openjdk-21/isolated/usr/bin/"
```

### Configuration Update

**After cleanup, runtime.yml is rewritten:**

```yaml
# SECURE - Points to isolated copy
mounts:
  - source: "isolated/usr/lib/jvm/java-21-openjdk-amd64"  # Isolated path
    target: "/usr/lib/jvm/java-21-openjdk-amd64"
    readonly: true
```

### Security Analysis

**Attack Vector Mitigation:**

- **Before**: Production job can access host `/usr/bin/java` and explore host filesystem
- **After**: Production job only accesses isolated copy within `/opt/joblet/runtimes/`

**Defense in Depth:**

1. **Isolation Boundary**: Runtime files copied to `/opt/joblet/runtimes/{runtime}/isolated/`
2. **Read-Only Mounts**: All runtime files mounted as read-only
3. **Path Validation**: All runtime mounts must be within the runtime directory
4. **Configuration Backup**: Original config preserved for audit trails

---

# Part 3: Deployment Strategies

## Build-Once, Deploy-Many Architecture

```
Dev Host → Runtime Built → Runtime.tar.gz → Multiple Production Hosts
          (Auto Package)                   (Direct Extract)
```

## Standard Deployment Method

All runtimes use direct extraction for deployment:

```bash
# Pre-built packages available:
# - python-3.11-ml-runtime.tar.gz (226MB)
# - java-17-runtime-complete.tar.gz (193MB)
# - java-21-runtime-complete.tar.gz (208MB)

# Extract directly to runtimes directory
sudo tar -xzf python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/
sudo tar -xzf java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/

# Set proper permissions
sudo chown -R joblet:joblet /opt/joblet/runtimes/

# Verify deployment
rnx runtime list
```

## Step-by-Step Deployment

### Step 1: Build Runtime (Development Host)

```bash
# Build on development host where contamination is acceptable
sudo ./runtimes/python-3.11-ml/setup_python_3_11_ml.sh
sudo ./runtimes/java-17/setup_java_17.sh
sudo ./runtimes/java-21/setup_java_21.sh
```

Each script automatically:

1. Builds the complete runtime environment
2. Installs all dependencies and packages
3. Creates deployment package at `/tmp/runtime-deployments/[runtime]-runtime.zip`
4. Includes metadata for auto-detection

### Step 2: Copy Runtime Package

```bash
# Copy from build host
scp build-host:/tmp/runtime-deployments/python-3.11-ml-runtime.zip .
scp build-host:/tmp/runtime-deployments/java-17-runtime.zip .
```

### Step 3: Deploy to Production Hosts

```bash
# Single host deployment
sudo unzip python-3.11-ml-runtime.zip -d /opt/joblet/runtimes/

# Multi-host deployment
for host in prod-01 prod-02 prod-03; do
    scp python-3.11-ml-runtime.zip admin@$host:/tmp/
    ssh admin@$host "sudo unzip /tmp/python-3.11-ml-runtime.zip -d /opt/joblet/runtimes/"
done
```

### Step 4: Verify Deployment

```bash
# List available runtimes
rnx runtime list

# Test runtime functionality
rnx runtime test python-3.11-ml
rnx runtime info python-3.11-ml

# Run jobs using deployed runtime
rnx job run --runtime=python-3.11-ml python script.py
```

## Zero-Contamination Guarantee

**Production hosts require NO:**

- Compilers (gcc, g++, javac)
- Package managers (apt, yum, npm, pip)
- Build tools (make, cmake, maven)
- Development headers (python3-dev, etc.)

**Only requirement:**

- Joblet daemon running
- RNX client with runtime package

---

# Part 4: Enterprise Patterns

## Multi-Environment Runtime Promotion

```bash
#!/bin/bash
# runtime_promotion.sh - Promote runtimes through environments

RUNTIME="python-3.11-ml"
ENVIRONMENTS=("staging" "prod-eu" "prod-us" "prod-asia")
ARTIFACT_REGISTRY="https://artifacts.company.com/joblet-runtimes"

# Download certified runtime from artifact registry
curl -H "Authorization: Bearer $REGISTRY_TOKEN" \
     "$ARTIFACT_REGISTRY/$RUNTIME-runtime.zip" \
     -o "$RUNTIME-runtime.zip"

# Deploy to all environments
for env in "${ENVIRONMENTS[@]}"; do
    echo "🚀 Deploying $RUNTIME to $env..."

    # Get environment host list
    hosts=$(kubectl get nodes -l environment=$env --no-headers | awk '{print $1}')

    for host in $hosts; do
        scp "$RUNTIME-runtime.zip" admin@$host:/tmp/
        ssh admin@$host "sudo unzip /tmp/$RUNTIME-runtime.zip -d /opt/joblet/runtimes/"

        # Verify deployment
        ssh admin@$host "rnx runtime test $RUNTIME" || {
            echo "❌ Failed to deploy $RUNTIME on $host"
            exit 1
        }
    done

    echo "✅ $env deployment complete"
done
```

## Blue-Green Runtime Deployments

```bash
#!/bin/bash
# blue_green_runtime.sh - Zero-downtime runtime updates

deploy_runtime_blue_green() {
    local host=$1
    local runtime_file=$2

    echo "🔄 Starting blue-green deployment on $host"

    # Step 1: Deploy new runtime with versioned name
    ssh admin@$host "sudo unzip /tmp/$runtime_file -d /opt/joblet/runtimes/"

    # Step 2: Health check new runtime
    ssh admin@$host "rnx job run --runtime=python-3.11-ml-v2.0 python health_check.py" || {
        echo "❌ Health check failed on $host"
        return 1
    }

    # Step 3: Update symlink for seamless cutover
    ssh admin@$host "sudo ln -sfn /opt/joblet/runtimes/python/python-3.11-ml-v2.0 /opt/joblet/runtimes/python/python-3.11-ml"

    # Step 4: Verify production traffic
    ssh admin@$host "rnx job run --runtime=python-3.11-ml python -c 'print(\"✅ Production ready\")'"

    echo "✅ Blue-green deployment complete on $host"
}

# Deploy to production cluster
PROD_HOSTS=("prod-web-01" "prod-web-02" "prod-worker-01" "prod-worker-02")
for host in "${PROD_HOSTS[@]}"; do
    deploy_runtime_blue_green "$host" "$NEW_RUNTIME_FILE"
done
```

## CI/CD Integration - GitHub Actions

```yaml
name: Runtime Build & Deploy Pipeline
on:
  push:
    paths:
      - 'runtimes/**'

jobs:
  build-runtime:
    runs-on: [self-hosted, linux, build-cluster]
    strategy:
      matrix:
        runtime: [python-3.11-ml, node-18, java-17]

    steps:
      - uses: actions/checkout@v4

      - name: Build Runtime
        run: |
          sudo ./runtimes/${{ matrix.runtime }}/setup_${{ matrix.runtime }}.sh

      - name: Security Scan
        run: |
          trivy fs /opt/joblet/runtimes/ --format table --exit-code 1

      - name: Package Verification
        run: |
          zip -T /tmp/runtime-deployments/${{ matrix.runtime }}-runtime.zip

      - name: Upload to Registry
        run: |
          curl -X PUT \
               -H "Authorization: Bearer ${{ secrets.REGISTRY_TOKEN }}" \
               -F "file=@/tmp/runtime-deployments/${{ matrix.runtime }}-runtime.zip" \
               "${{ secrets.ARTIFACT_REGISTRY }}/${{ matrix.runtime }}-runtime.zip"

  deploy-staging:
    needs: build-runtime
    runs-on: ubuntu-latest
    environment: staging

    steps:
      - name: Deploy to Staging
        run: |
          for runtime in python-3.11-ml node-18 java-17; do
            for host in ${{ secrets.STAGING_HOSTS }}; do
              curl -H "Authorization: Bearer ${{ secrets.REGISTRY_TOKEN }}" \
                   "${{ secrets.ARTIFACT_REGISTRY }}/${runtime}-runtime.zip" \
                   -o "${runtime}-runtime.zip"

              scp "${runtime}-runtime.zip" admin@${host}:/tmp/
              ssh admin@${host} "sudo unzip /tmp/${runtime}-runtime.zip -d /opt/joblet/runtimes/"
              ssh admin@${host} "rnx runtime test ${runtime}"
            done
          done

  deploy-production:
    needs: deploy-staging
    runs-on: ubuntu-latest
    environment: production

    steps:
      - name: Production Deployment
        run: |
          echo "🚀 Starting production deployment..."
          ./scripts/production_runtime_deploy.sh
```

## Batch Runtime Updates

```bash
#!/bin/bash
# Update all runtimes across fleet

RUNTIMES=("python-3.11-ml" "node-18" "java-17")
HOSTS=("prod-01" "prod-02" "prod-03")

for runtime in "${RUNTIMES[@]}"; do
    echo "Deploying $runtime to all hosts..."
    for host in "${HOSTS[@]}"; do
        echo "  -> $host"
        scp "${runtime}-runtime.zip" admin@$host:/tmp/
        ssh admin@$host "sudo unzip /tmp/${runtime}-runtime.zip -d /opt/joblet/runtimes/"
    done
done
```

## Troubleshooting

### Deployment Verification

```bash
# Check runtime installation
rnx runtime list | grep python-3.11-ml

# Test runtime functionality
rnx runtime test python-3.11-ml

# Run simple test job
rnx job run --runtime=python-3.11-ml python -c "import numpy; print('✅ Runtime working')"
```

### Common Issues

| Issue                                    | Solution                                                       |
|------------------------------------------|----------------------------------------------------------------|
| `runtime error: invalid zip file`        | Ensure zip was created properly, not corrupted during transfer |
| `could not detect runtime name`          | Verify zip contains proper directory structure with metadata   |
| `grpc: received message larger than max` | Use local copy approach for runtimes >128MB                    |

---

## See Also

- [RUNTIME_SYSTEM.md](./RUNTIME_SYSTEM.md) - User guide for runtime system
- [RUNTIME_DESIGN.md](./RUNTIME_DESIGN.md) - Design document and examples
- [RUNTIME_REGISTRY_GUIDE.md](./RUNTIME_REGISTRY_GUIDE.md) - Registry usage
- [SECURITY.md](./SECURITY.md) - Security considerations
- [RNX_CLI_REFERENCE.md](./RNX_CLI_REFERENCE.md) - Complete CLI reference
