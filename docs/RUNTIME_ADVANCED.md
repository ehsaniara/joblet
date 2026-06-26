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
    - Runtime management commands (`rnx runtime build/list/info`)
    - Runtime build using declarative YAML specifications

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

## Declarative Runtime Builder

Runtime building uses a 14-phase declarative pipeline:

### Builder Components

1. **Runtime Builder** (`pkg/builder/builder.go`)
    - 14-phase build orchestration
    - Platform detection and package management
    - Progress streaming via gRPC

2. **YAML Parser** (`pkg/builder/parser.go`)
    - Schema validation for runtime.yaml
    - Support for pip, npm, and custom hooks
    - Environment variable configuration

3. **Language Profiles** (`pkg/builder/profiles.go`)
    - Pre-defined packages for Python, Java, Node.js, Go, Rust
    - System-level dependencies per language
    - Version-specific package selection

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

### Directory Structure

The runtime builder automatically creates this structure:

```
/opt/joblet/runtimes/openjdk-21/1.0.0/
├── runtime.yml               # Runtime configuration with isolated paths
└── isolated/                 # Self-contained runtime files
    ├── usr/
    │   ├── lib/jvm/          # Copied Java installation
    │   └── bin/              # Copied Java binaries
    ├── lib/                  # System libraries
    └── etc/ssl/certs/        # Copied certificates
```

### How the Builder Creates Isolated Runtimes

The `rnx runtime build` command uses OverlayFS to install packages in isolation and then copies the results:

```bash
# Build creates the isolated structure automatically
rnx runtime build ./examples/java-21/runtime.yaml

# The builder performs these steps internally:
# 1. Creates OverlayFS with host as read-only lower layer
# 2. Installs packages in the overlay (changes captured in upper layer)
# 3. Copies binaries/libraries from upper layer to isolated/ directory
# 4. Generates runtime.yml with isolated paths
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

```mermaid
flowchart LR
    N1["Dev Host"] -->|Auto Package| N2["Runtime Built"]
    N2 --> N3["Runtime.tar.gz"]
    N3 -->|Direct Extract| N4["Multiple Production Hosts"]
```

## Standard Deployment Method

All runtimes use direct extraction for deployment:

```bash
# Pre-built packages available:
# - python-3.11-ml-runtime.tar.gz (226MB)
# - java-17-runtime-complete.tar.gz (193MB)
# - java-21-runtime-complete.tar.gz (208MB)

# Extract directly to runtimes directory
sudo tar -xzf python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/
sudo tar -xzf java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/

# Set proper permissions
sudo chown -R joblet:joblet /opt/joblet/runtimes/

# Verify deployment
rnx runtime list
```

## Step-by-Step Deployment

### Step 1: Create Runtime Specification

```yaml
# runtime.yaml
schema_version: "1.0"
name: python-3.11-ml
version: 1.0.0
description: Python 3.11 with ML packages

base:
  language: python
  version: "3.11"

pip:
  - numpy
  - pandas
  - scikit-learn
```

### Step 2: Build Runtime on Target Host

```bash
# Build runtime directly on production host using YAML builder
rnx runtime build ./runtime.yaml

# Or build with verbose output
rnx runtime build -v ./runtime.yaml
```

The builder automatically:

1. Validates the YAML specification
2. Creates isolated build environment using OverlayFS
3. Installs all dependencies and packages
4. Copies binaries/libraries to `/opt/joblet/runtimes/{name}/{version}/`
5. Generates runtime.yml configuration

### Step 3: Multi-Host Deployment

```bash
# Deploy to multiple hosts by building on each
for host in prod-01 prod-02 prod-03; do
    scp runtime.yaml admin@$host:/tmp/
    ssh admin@$host "rnx runtime build /tmp/runtime.yaml"
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

RUNTIME_YAML="./runtimes/python-3.11-ml/runtime.yaml"
ENVIRONMENTS=("staging" "prod-eu" "prod-us" "prod-asia")

# Deploy to all environments by building from YAML specification
for env in "${ENVIRONMENTS[@]}"; do
    echo "🚀 Deploying runtime to $env..."

    # Get environment host list
    hosts=$(kubectl get nodes -l environment=$env --no-headers | awk '{print $1}')

    for host in $hosts; do
        # Copy runtime specification to host
        scp "$RUNTIME_YAML" admin@$host:/tmp/runtime.yaml

        # Build runtime on target host
        ssh admin@$host "rnx runtime build /tmp/runtime.yaml" || {
            echo "❌ Failed to build runtime on $host"
            exit 1
        }

        # Verify deployment
        ssh admin@$host "rnx runtime test python-3.11-ml" || {
            echo "❌ Runtime test failed on $host"
            exit 1
        }
    done

    echo "✅ $env deployment complete"
done
```

## Blue-Green Runtime Deployments

```bash
#!/bin/bash
# blue_green_runtime.sh - Zero-downtime runtime updates using versioned runtimes

deploy_runtime_blue_green() {
    local host=$1
    local runtime_yaml=$2
    local new_version=$3

    echo "🔄 Starting blue-green deployment on $host"

    # Step 1: Copy and build new runtime version
    scp "$runtime_yaml" admin@$host:/tmp/runtime.yaml
    ssh admin@$host "rnx runtime build /tmp/runtime.yaml" || {
        echo "❌ Build failed on $host"
        return 1
    }

    # Step 2: Health check new runtime (using versioned name from YAML)
    ssh admin@$host "rnx job run --runtime=python-3.11-ml@${new_version} python health_check.py" || {
        echo "❌ Health check failed on $host"
        return 1
    }

    # Step 3: Verify production traffic
    ssh admin@$host "rnx job run --runtime=python-3.11-ml python -c 'print(\"✅ Production ready\")'"

    echo "✅ Blue-green deployment complete on $host"
}

# Deploy to production cluster
PROD_HOSTS=("prod-web-01" "prod-web-02" "prod-worker-01" "prod-worker-02")
for host in "${PROD_HOSTS[@]}"; do
    deploy_runtime_blue_green "$host" "./runtime.yaml" "2.0.0"
done
```

## CI/CD Integration - GitHub Actions

```yaml
name: Runtime Build & Deploy Pipeline
on:
  push:
    paths:
      - 'runtimes/**/runtime.yaml'

jobs:
  validate-runtimes:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        runtime: [python-3.11-ml, openjdk-17, openjdk-21]

    steps:
      - uses: actions/checkout@v4

      - name: Validate Runtime YAML
        run: |
          rnx runtime validate ./runtimes/${{ matrix.runtime }}/runtime.yaml

  deploy-staging:
    needs: validate-runtimes
    runs-on: ubuntu-latest
    environment: staging

    steps:
      - uses: actions/checkout@v4

      - name: Deploy to Staging
        run: |
          for runtime in python-3.11-ml openjdk-17 openjdk-21; do
            for host in ${{ secrets.STAGING_HOSTS }}; do
              # Copy runtime.yaml to host and build
              scp "./runtimes/${runtime}/runtime.yaml" admin@${host}:/tmp/runtime.yaml
              ssh admin@${host} "rnx runtime build /tmp/runtime.yaml"
              ssh admin@${host} "rnx runtime test ${runtime}"
            done
          done

  deploy-production:
    needs: deploy-staging
    runs-on: ubuntu-latest
    environment: production

    steps:
      - uses: actions/checkout@v4

      - name: Production Deployment
        run: |
          echo "🚀 Starting production deployment..."
          for runtime in python-3.11-ml openjdk-17 openjdk-21; do
            for host in ${{ secrets.PROD_HOSTS }}; do
              scp "./runtimes/${runtime}/runtime.yaml" admin@${host}:/tmp/runtime.yaml
              ssh admin@${host} "rnx runtime build /tmp/runtime.yaml"
              ssh admin@${host} "rnx runtime test ${runtime}"
            done
          done
```

## Batch Runtime Updates

```bash
#!/bin/bash
# Update all runtimes across fleet using YAML builder

RUNTIMES=("python-3.11-ml" "openjdk-17" "openjdk-21")
HOSTS=("prod-01" "prod-02" "prod-03")

for runtime in "${RUNTIMES[@]}"; do
    echo "Deploying $runtime to all hosts..."
    for host in "${HOSTS[@]}"; do
        echo "  -> $host"
        scp "./runtimes/${runtime}/runtime.yaml" admin@$host:/tmp/runtime.yaml
        ssh admin@$host "rnx runtime build /tmp/runtime.yaml"
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
| `runtime not found`                      | Verify runtime was built with `rnx runtime build`              |
| `validation errors`                      | Check runtime.yaml syntax with `rnx runtime validate`          |
| `build failed`                           | Check disk space and network connectivity on target host       |

---

## See Also

- [RUNTIME_SYSTEM.md](./RUNTIME_SYSTEM.md) - User guide for runtime system
- [RUNTIME_DESIGN.md](./RUNTIME_DESIGN.md) - Design document and examples
- [RUNTIME_REGISTRY_GUIDE.md](./RUNTIME_REGISTRY_GUIDE.md) - Declarative runtime builder guide
- [SECURITY.md](./SECURITY.md) - Security considerations
- [RNX_CLI_REFERENCE.md](./RNX_CLI_REFERENCE.md) - Complete CLI reference
