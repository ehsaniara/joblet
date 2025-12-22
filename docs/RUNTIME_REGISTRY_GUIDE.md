# Runtime Builder Guide

Complete guide for building custom runtimes for Joblet using the declarative YAML-based builder.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Runtime YAML Specification](#runtime-yaml-specification)
- [Build Process](#build-process)
- [Supported Languages](#supported-languages)
- [Examples](#examples)
- [Hooks](#hooks)
- [Troubleshooting](#troubleshooting)

## Overview

Joblet uses a declarative runtime builder that enables:

- **Declarative configuration** - Define runtimes with simple YAML files
- **Remote building** - Build executes on the joblet server, not locally
- **14-phase build process** - Comprehensive build pipeline with progress streaming
- **Language profiles** - Pre-defined packages for Python, Java, Node.js, Go, Rust
- **Custom hooks** - Pre and post-install scripts for customization
- **Multi-platform support** - Ubuntu, RHEL, Amazon Linux on amd64/arm64

## Quick Start

### 1. Create a runtime.yaml

```yaml
schema_version: "1.0"
name: python-3.11-ml
version: 1.0.0
description: Python 3.11 with machine learning packages

base:
  language: python
  version: "3.11"

pip:
  - numpy
  - pandas
  - scikit-learn

environment:
  PYTHONUNBUFFERED: "1"
```

### 2. Validate the specification

```bash
rnx runtime validate ./runtime.yaml
```

### 3. Build the runtime

```bash
# Build on remote server
rnx runtime build ./runtime.yaml

# Or with verbose output
rnx runtime build -v ./runtime.yaml

# Validate before building (comprehensive server-side validation)
rnx runtime validate ./runtime.yaml
```

### 4. Use the runtime

```bash
rnx job run --runtime=python-3.11-ml python -c "import numpy; print(numpy.__version__)"
```

## Runtime YAML Specification

### Complete Schema

```yaml
# Required: Schema version (must be "1.0")
schema_version: "1.0"

# Required: Runtime name (lowercase, hyphens, dots, max 64 chars)
name: my-runtime

# Required: Semantic version (X.Y.Z)
version: 1.0.0

# Required: Description (max 256 chars)
description: My custom runtime environment

# Required: Base language configuration
base:
  language: python  # python | java | node | go | rust
  version: "3.11"   # Language-specific version

# Optional: Python packages to install via pip
pip:
  - numpy
  - pandas>=2.0.0
  - scikit-learn==1.3.2

# Optional: Pip options (e.g., custom index)
pip_options: "--index-url https://pypi.example.com/simple"

# Optional: Node.js packages to install via npm
npm:
  - express
  - lodash

# Optional: Additional library patterns to copy to isolated runtime
# Use this for system libraries installed via pre_install hooks
libraries:
  - libopenblas*
  - libgfortran*

# Optional: Environment variables
environment:
  PYTHONUNBUFFERED: "1"
  MY_VAR: "value"

# Optional: Runtime requirements
requirements:
  gpu: false
  cuda_version: "12.0"
  min_memory: "256MB"

# Optional: Supported platforms (default: all)
platforms:
  - ubuntu-amd64
  - ubuntu-arm64
  - rhel-amd64

# Optional: Build hooks
hooks:
  timeout: "30m"  # Hook timeout (default: 20m)
  pre_install: |
    echo "Running before package installation"
  post_install: |
    echo "Running after package installation"
```

### Field Reference

| Field | Required | Description |
|-------|----------|-------------|
| `schema_version` | Yes | Must be "1.0" |
| `name` | Yes | Runtime identifier (lowercase, hyphens, dots) |
| `version` | Yes | Semantic version X.Y.Z |
| `description` | Yes | Human-readable description (max 256 chars) |
| `base.language` | Yes | Base language: python, java, node, go, rust |
| `base.version` | Yes | Language version (e.g., "3.11", "21") |
| `pip` | No | List of Python packages |
| `pip_options` | No | Additional pip install options |
| `npm` | No | List of Node.js packages |
| `libraries` | No | Additional library patterns to copy (e.g., `libopenblas*`) |
| `environment` | No | Environment variables |
| `requirements` | No | GPU and memory requirements |
| `platforms` | No | Supported platforms |
| `hooks` | No | Pre/post-install scripts |

## Build Process

The runtime builder executes a 14-phase pipeline:

| Phase | Name | Description |
|-------|------|-------------|
| 1 | Parse & Validate | Parse YAML and validate schema |
| 2 | Detect Platform | Identify OS, architecture, package manager |
| 3 | Check Disk Space | Ensure sufficient disk space (1GB minimum) |
| 4 | Validate Packages | Verify packages are available |
| 5 | Prepare Directories | Create runtime directory structure |
| 6 | Pre-install Hook | Run pre-install script (if defined) |
| 7 | Install Base | Install base language packages |
| 8 | Install Packages | Install pip/npm packages |
| 9 | Post-install Hook | Run post-install script (if defined) |
| 10 | Copy Binaries | Copy language binaries to isolated dir |
| 11 | Copy Libraries | Copy required shared libraries |
| 12 | Copy Configuration | Copy SSL certs, resolv.conf, etc. |
| 13 | Generate Config | Generate runtime.yml for joblet |
| 14 | Validate Build | Verify build integrity |

### Build Output

Runtimes are installed to: `/opt/joblet/runtimes/{name}/{version}/`

Directory structure:
```
/opt/joblet/runtimes/python-3.11-ml/1.0.0/
├── runtime.yml           # Generated runtime configuration
└── isolated/             # Isolated filesystem
    ├── usr/
    │   ├── bin/          # Binaries (python3, pip, etc.)
    │   └── lib/          # Libraries
    ├── lib/              # System libraries
    ├── etc/              # Configuration files
    └── tmp/              # Temporary directory
```

### Generated `runtime.yml`

The build generates a `runtime.yml` with comprehensive metadata:

```yaml
name: python-3.11-ml
language: python
language_version: "3.11"
version: 1.0.0
description: Python 3.11 with machine learning packages
mounts: [...]
environment:
  PYTHONUNBUFFERED: "1"
  PATH: /usr/local/bin:/usr/bin:/bin
packages:
  - numpy==1.26.2
  - pandas==2.1.3
libraries:
  - libopenblas*
  - libgfortran*
requirements:
  architectures: [amd64]
build_info:
  built_at: "2025-12-22T17:43:00Z"
  platform: ubuntu-amd64
```

View runtime details:
```bash
rnx runtime info python-3.11-ml
```

## Supported Languages

### Python

```yaml
base:
  language: python
  version: "3.11"  # 3.9, 3.10, 3.11, 3.12

pip:
  - numpy
  - pandas
```

Installed packages: python3.X, python3.X-dev, python3.X-venv, python3-pip, libssl-dev, zlib1g-dev, libffi-dev

### Java

```yaml
base:
  language: java
  version: "21"  # 11, 17, 21

# No additional packages needed - JVM is included
```

Installed packages: openjdk-X-jdk, ca-certificates

### Node.js

```yaml
base:
  language: node
  version: "20"  # 18, 20

npm:
  - express
  - lodash
```

Installed packages: nodejs, npm

### Go

```yaml
base:
  language: go
  version: "1.21"
```

Installed packages: golang, git

### Rust

```yaml
base:
  language: rust
  version: "1.75"
```

Installed packages: rustc, cargo

## Examples

### Python ML Runtime

```yaml
schema_version: "1.0"
name: python-3.11-ml
version: 1.0.0
description: Python 3.11 with machine learning packages

base:
  language: python
  version: "3.11"

pip:
  - numpy
  - pandas
  - scikit-learn
  - matplotlib

# Copy system libraries needed by ML packages
libraries:
  - libopenblas*
  - libgfortran*
  - libgomp*

environment:
  PYTHONUNBUFFERED: "1"

hooks:
  pre_install: |
    apt-get install -y libopenblas-dev || yum install -y openblas-devel
```

### Java 21 Runtime

```yaml
schema_version: "1.0"
name: openjdk-21
version: 1.0.0
description: OpenJDK 21 runtime

base:
  language: java
  version: "21"

environment:
  JAVA_OPTS: "-Xmx512m"
```

### Python Analytics with Custom Index

```yaml
schema_version: "1.0"
name: python-analytics
version: 1.0.0
description: Python for data analytics

base:
  language: python
  version: "3.11"

pip:
  - pandas
  - matplotlib
  - seaborn

pip_options: "--index-url https://pypi.company.com/simple"

environment:
  PYTHONUNBUFFERED: "1"
```

### Runtime with Hooks

```yaml
schema_version: "1.0"
name: python-custom
version: 1.0.0
description: Python with custom setup

base:
  language: python
  version: "3.11"

pip:
  - numpy

hooks:
  timeout: "30m"
  pre_install: |
    echo "Configuring system before installation"
    # Custom pre-install commands
  post_install: |
    echo "Running post-installation tasks"
    python3.11 -c "import numpy; print('NumPy version:', numpy.__version__)"
```

## Hooks

### Pre-install Hook

Runs after base package installation but before pip/npm packages.

Use cases:
- Configure system settings
- Install additional system dependencies
- Set up authentication for private package repositories

### Post-install Hook

Runs after all packages are installed.

Use cases:
- Verify installation
- Compile assets
- Run tests
- Clean up temporary files

### Hook Environment

Hooks execute with:
- Root privileges (sudo)
- Working directory: runtime's isolated directory
- Access to installed packages
- Timeout enforcement (default: 20 minutes)

## Troubleshooting

### Build Fails at Package Validation

```
Error: package not found: python3.11-dev
```

**Solution:** The package name might differ on your platform. Check available packages:
```bash
apt-cache search python3.11
```

### Pip Installation Fails

```
Error: pip install failed: Could not find a version that satisfies the requirement
```

**Solutions:**
- Check package name spelling
- Verify package exists on PyPI
- Check network connectivity on the server
- Use `pip_options` for custom index

### Permission Denied

```
Error: failed to create directory: permission denied
```

**Solution:** Ensure joblet service has write access to `/opt/joblet/runtimes/`

### Disk Space Error

```
Error: insufficient disk space: 500 MB available, need at least 1024 MB
```

**Solution:** Free up disk space on the server or expand the volume.

### Hook Timeout

```
Error: pre-install hook timed out after 20m
```

**Solution:** Increase timeout in hooks configuration:
```yaml
hooks:
  timeout: "60m"
  pre_install: |
    # Long-running installation
```

## Best Practices

1. **Version pinning** - Pin package versions for reproducibility:
   ```yaml
   pip:
     - numpy==1.26.2
     - pandas>=2.0.0,<3.0.0
   ```

2. **Minimal dependencies** - Only include packages you need

3. **Test locally first** - Validate YAML before building:
   ```bash
   rnx runtime validate ./runtime.yaml
   ```

4. **Use validate** - Comprehensive server-side validation before building:
   ```bash
   rnx runtime validate ./runtime.yaml
   ```

5. **Semantic versioning** - Use proper version numbers for your runtimes

## Related Documentation

- [Runtime System Overview](RUNTIME_SYSTEM.md)
- [RNX CLI Reference](RNX_CLI_REFERENCE.md)
- [Job Execution Guide](QUICKSTART.md)

---

**Questions?** Check [Troubleshooting](#troubleshooting) or create an issue on GitHub.
