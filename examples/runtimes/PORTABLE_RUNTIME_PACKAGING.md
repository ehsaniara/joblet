# 📦 Portable Runtime Packaging Guide

## Overview

Instead of running contaminating setup scripts on production hosts, you can:

1. **Build runtimes in test/development environment** (contamination acceptable)
2. **Package the compiled runtime** into portable archives
3. **Deploy clean packages to production** (zero contamination)

This approach provides **complete host isolation** while maintaining all runtime benefits.

## 🏗️ Build Environment Setup

### Test Host Requirements

- VM, container, or dedicated build machine
- Same OS/architecture as production hosts
- OK to install build dependencies (will be contained)

```bash
# Example: Ubuntu 22.04 build environment
docker run -it --rm -v $(pwd)/runtime-packages:/packages ubuntu:22.04 bash

# Or use a dedicated VM/host where contamination is acceptable
```

## 📦 Runtime Packaging Workflow

### 1. Build & Package Runtimes (Decoupled Approach)

```bash
# On test/build host - contamination OK here

# Method 1: Use runtime_manager.sh for all runtimes
sudo ./runtime_manager.sh build-all    # Builds and packages everything

# Method 2: Individual runtime management
sudo ./python-3.11-ml/setup_python_3_11_ml.sh setup   # Build
sudo ./python-3.11-ml/setup_python_3_11_ml.sh package # Package

sudo ./java-17/setup_java_17.sh setup
sudo ./java-17/setup_java_17.sh package

sudo ./java-21/setup_java_21.sh setup  
sudo ./java-21/setup_java_21.sh package

sudo ./node-18/setup_node_18.sh setup
sudo ./node-18/setup_node_18.sh package
```

### 2. Package Location

```bash
# All packages are created in /tmp/runtime-packages/
ls -la /tmp/runtime-packages/

# Example output:
# python-3.11-ml-runtime.tar.gz + .manifest + .sha256
# java-17-runtime.tar.gz + .manifest + .sha256  
# java-21-runtime.tar.gz + .manifest + .sha256
# node-18-runtime.tar.gz + .manifest + .sha256
# COMBINED_MANIFEST.txt
# COMBINED_CHECKSUMS.txt
```

### 3. Distribute Packages

```bash
# Copy to production hosts
scp /tmp/runtime-packages/* admin@production-host:/tmp/

# Or upload to artifact repository
# aws s3 cp /tmp/runtime-packages/ s3://my-runtime-packages/ --recursive
# Or store in corporate artifact store
```

## 🚀 Production Installation (Zero Contamination)

### Installation Script for Production

```bash
#!/bin/bash
# runtime_manager.sh install-all - Clean installation on production

set -euo pipefail

PACKAGES_DIR="/tmp"  # Where packages were copied
RUNTIME_BASE="/opt/joblet/runtimes"

if [[ $EUID -ne 0 ]]; then
    echo "❌ Must run as root: sudo $0"
    exit 1
fi

echo "📦 Installing Joblet Runtime Packages (Zero Host Contamination)"
echo "=============================================================="

# Verify checksums
cd "$PACKAGES_DIR"
if [[ -f checksums.txt ]]; then
    echo "🔐 Verifying package integrity..."
    sha256sum -c checksums.txt || {
        echo "❌ Checksum verification failed!"
        exit 1
    }
    echo "✅ All packages verified"
fi

# Create base directories
mkdir -p "$RUNTIME_BASE"/{python,java,node}

# Install Python runtime
if [[ -f python-3.11-ml-runtime.tar.gz ]]; then
    echo "🐍 Installing Python 3.11 ML runtime..."
    tar -xzf python-3.11-ml-runtime.tar.gz -C "$RUNTIME_BASE/python/"
    chown -R joblet:joblet "$RUNTIME_BASE/python/" 2>/dev/null || true
    echo "✅ Python runtime installed"
fi

# Install Java 17 runtime
if [[ -f java-17-runtime.tar.gz ]]; then
    echo "☕ Installing Java 17 runtime..."
    tar -xzf java-17-runtime.tar.gz -C "$RUNTIME_BASE/java/"
    chown -R joblet:joblet "$RUNTIME_BASE/java/java-17" 2>/dev/null || true  
    echo "✅ Java 17 runtime installed"
fi

# Install Java 21 runtime
if [[ -f java-21-runtime.tar.gz ]]; then
    echo "☕ Installing Java 21 runtime..."
    tar -xzf java-21-runtime.tar.gz -C "$RUNTIME_BASE/java/"
    chown -R joblet:joblet "$RUNTIME_BASE/java/java-21" 2>/dev/null || true
    echo "✅ Java 21 runtime installed" 
fi

echo ""
echo "🎉 Runtime installation complete!"
echo "✅ Zero host contamination - only extracted pre-built runtimes"
echo ""
echo "Verify installation:"
echo "  rnx runtime list"
echo "  rnx runtime test python-3.11-ml" 
echo "  rnx runtime test java:17"
echo "  rnx runtime test node:18"
```

## 🔧 Automated Packaging Scripts

### Create Build Script

```bash
#!/bin/bash
# runtime_manager.sh - Decoupled runtime management

set -euo pipefail

BUILD_DIR="/opt/joblet/runtimes"
PACKAGE_DIR="/tmp/runtime-packages"
SCRIPT_DIR="/opt/joblet/examples/runtimes"

echo "🏗️ Building Joblet Runtime Packages"
echo "=================================="
echo "Build host: $(hostname)"
echo "Architecture: $(uname -m)"
echo "⚠️ This will contaminate the build host (acceptable)"
echo ""

# Create package directory
mkdir -p "$PACKAGE_DIR"

# Build each runtime (contamination acceptable here)
echo "🐍 Building Python 3.11 ML runtime..."
"$SCRIPT_DIR/python-3.11-ml/setup_python_3_11_ml.sh"

echo "☕ Building Java 17 runtime..."
"$SCRIPT_DIR/java-17/setup_java_17.sh"

echo "☕ Building Java 21 runtime..."  
"$SCRIPT_DIR/java-21/setup_java_21.sh"

echo "📦 Packaging runtimes..."

# Package each runtime
cd "$BUILD_DIR"
tar -czf "$PACKAGE_DIR/python-3.11-ml-runtime.tar.gz" python/python-3.11-ml/
tar -czf "$PACKAGE_DIR/java-17-runtime.tar.gz" java/java-17/
tar -czf "$PACKAGE_DIR/java-21-runtime.tar.gz" java/java-21/  

# Create manifest and checksums
cd "$PACKAGE_DIR"
cat > MANIFEST.txt << EOF
Joblet Runtime Packages
======================
Built: $(date)
Host: $(hostname)  
Arch: $(uname -m)
OS: $(lsb_release -d 2>/dev/null | cut -d: -f2 | xargs || uname -o)

Packages:
$(ls -lh *.tar.gz)

Total Size: $(du -sh . | cut -f1)
EOF

sha256sum *.tar.gz > checksums.txt

echo "✅ Runtime packages created in $PACKAGE_DIR"
echo ""
ls -lh "$PACKAGE_DIR"
```

## 🏭 CI/CD Integration

### GitHub Actions Example

```yaml
name: Build Runtime Packages

on:
  push:
    paths:
      - 'examples/runtimes/**'
  workflow_dispatch:

jobs:
  build-packages:
    runs-on: ubuntu-22.04
    steps:
      - uses: actions/checkout@v4

      - name: Build Runtime Packages
        run: |
          chmod +x examples/runtimes/runtime_manager.sh
          sudo examples/runtimes/runtime_manager.sh build-all

      - name: Upload Packages
        uses: actions/upload-artifact@v4
        with:
          name: joblet-runtime-packages
          path: /tmp/runtime-packages/

      - name: Release Packages
        if: startsWith(github.ref, 'refs/tags/')
        uses: softprops/action-gh-release@v1
        with:
          files: /tmp/runtime-packages/*
```

## 📋 Deployment Strategies

### 1. Manual Deployment

```bash
# On production host
wget https://releases.yourcompany.com/joblet-runtimes-v1.0.tar.gz
tar -xzf joblet-runtimes-v1.0.tar.gz
sudo ./runtime_manager.sh install-all
```

### 2. Configuration Management

```bash
# Ansible playbook example
- name: Deploy Joblet Runtimes
  unarchive:
    src: "{{ runtime_packages_url }}"
    dest: /tmp
    remote_src: yes
    
- name: Install Runtimes  
  script: /tmp/runtime_manager.sh
  become: yes
```

### 3. Container Base Images

```dockerfile
# Create base image with runtimes
FROM ubuntu:22.04
COPY runtime-packages/ /tmp/
RUN /tmp/runtime_manager.sh install-all && rm -rf /tmp/runtime-packages/
```

## ✅ Benefits of This Approach

### For Production Hosts

- **Zero contamination**: No build dependencies installed
- **Fast deployment**: Just extract pre-built artifacts
- **Consistent environments**: Same binaries across all hosts
- **Security**: No internet access needed for runtime building
- **Auditable**: Clear package manifests and checksums

### For Operations

- **Controlled builds**: Single build environment to manage
- **Version control**: Package and deploy specific runtime versions
- **Testing**: Test runtime packages before production deployment
- **Rollback**: Easy to revert to previous runtime packages

### For Development

- **Clean development**: No contamination of developer machines
- **Reproducible**: Same runtimes in dev/staging/prod
- **Fast setup**: New environments get runtimes in seconds

## 🎯 Recommendation

**Use this approach for all production deployments:**

1. ✅ **Python**: Package to avoid 200MB of build tools on production
2. ✅ **Java**: Package to avoid wget/curl installation on production
3. ✅ **All runtimes**: Standardized deployment process

This transforms runtime deployment from "contaminating setup scripts" to "clean package installation" - the industry
standard approach for production software deployment.