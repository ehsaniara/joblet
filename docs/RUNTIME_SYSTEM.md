# Runtime System Guide

The Joblet Runtime System provides **pre-built, isolated runtime environments** that eliminate installation
delays and provide instant access to programming languages, databases, web servers, and other services.

## 🚀 Why Runtime System?

### The Traditional Problem

```bash
# Traditional approach: 5-45 minutes every time
rnx job run 'apt-get update && apt-get install python3-pip && pip install pandas numpy scikit-learn matplotlib && python analysis.py'
```

### The Runtime Solution

```bash
# Runtime approach: 2-3 seconds total
rnx job run --runtime=python-3.11-ml python analysis.py
```

## 📋 Table of Contents

1. [Available Runtimes](#available-runtimes)
2. [Getting Started](#getting-started)
3. [Runtime Deployment](#runtime-deployment)
4. [Runtime Management](#runtime-management)
5. [Performance Comparison](#performance-comparison)
6. [Architecture](#architecture)
7. [Custom Runtimes](#custom-runtimes)
8. [Best Practices](#best-practices)
9. [Troubleshooting](#troubleshooting)

## 🏃‍♂️ Available Runtimes

Joblet provides curated runtime environments for Python, Java, and machine learning applications:

### Python 3.11 + ML Stack (`python-3.11-ml`)

**Complete isolated Python environment with machine learning packages**

- **Python**: 3.11.9 (system package installation)
- **Pre-installed Packages**: System-based ML packages for reliability
    - NumPy (scientific computing core)
    - Pandas (data analysis and manipulation)
    - Scikit-learn (machine learning toolkit)
    - Matplotlib (plotting and visualization)
    - Seaborn (statistical data visualization)
    - SciPy (scientific computing)
    - Requests (HTTP library)
    - OpenPyXL (Excel file support)
- **Package Size**: ~475MB with 142 packages
- **Installation**: System package manager for maximum reliability
- **Setup Time**: Fast startup with pre-built environment
- **Use Cases**: AI development, machine learning research, data science workflows

```bash
# Usage examples
rnx job run --runtime=python-3.11-ml python3 -c "import pandas; print('Ready!')"
rnx job run --runtime=python-3.11-ml --upload=analysis.py python3 analysis.py
rnx runtime info python-3.11-ml  # See all packages and details
```

### Python 3.11 Standard (`python-3.11`)

**Lightweight Python runtime for general development**

- **Python**: 3.11.9 (lightweight installation)
- **Pre-installed Packages**: Essential packages for development
    - pip, requests, urllib3
- **Package Size**: ~200MB
- **Use Cases**: General scripting, automation, lightweight microservices

```bash
# Usage examples
rnx job run --runtime=python-3.11 python --version
rnx job run --runtime=python-3.11 pip install requests
rnx job run --runtime=python-3.11 python script.py
```

### OpenJDK 21 (`openjdk-21`)

**Modern Java with Long Term Support**

- **Java**: OpenJDK 21.0.8 (Long Term Support)
- **Build Tools**: javac, jar, javap, jshell (built-in)
- **Modern Features**:
    - Virtual Threads (Project Loom)
    - Pattern Matching for switch
    - String Templates (Preview)
    - Record Patterns
    - Foreign Function & Memory API
- **Package Size**: ~371MB with 1,872 files
- **System Libraries**: Includes all necessary shared libraries
- **Use Cases**: Enterprise applications, microservices, compute-intensive jobs

```bash
# Usage examples
rnx job run --runtime=openjdk-21 java -version
rnx job run --runtime=openjdk-21 --upload=HelloWorld.java javac HelloWorld.java
rnx job run --runtime=openjdk-21 java HelloWorld
```

### GraalVM JDK 21 (`graalvmjdk-21`)

**High-performance Java with native compilation and polyglot support**

- **GraalVM**: Community Edition JDK 21 with native-image
- **Advanced Features**:
    - Native image compilation (AOT)
    - Polyglot programming (JavaScript, Python, Ruby on JVM)
    - Superior performance for microservices
    - Faster startup times
- **Build Tools**: java, javac, native-image, gu, js, node, npm
- **Use Cases**: High-performance microservices, native binaries, polyglot applications

```bash
# Usage examples
rnx job run --runtime=graalvmjdk-21 java -version
rnx job run --runtime=graalvmjdk-21 native-image --version
rnx job run --runtime=graalvmjdk-21 js --version

## 🚀 Getting Started

### 1. Build Runtime Environments

Runtimes are built using the RNX CLI with a declarative YAML specification. The build executes remotely on the Joblet server:

```bash
# Create a runtime.yaml specification
cat > runtime.yaml << 'EOF'
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

# Optional: Additional libraries to copy (for system dependencies)
libraries:
  - libopenblas*
  - libgfortran*

environment:
  PYTHONUNBUFFERED: "1"

hooks:
  pre_install: |
    apt-get install -y libopenblas-dev || yum install -y openblas-devel
EOF

# Validate the specification (comprehensive server-side validation)
rnx runtime validate ./runtime.yaml

# Build the runtime on the remote server
rnx runtime build ./runtime.yaml

# Or with verbose output
rnx runtime build -v ./runtime.yaml

# Force rebuild (overwrite existing runtime)
rnx runtime build --force ./runtime.yaml
```

**Build Options:**

- `--force` or `-f`: Overwrite existing runtime if it already exists
- `-v` or `--verbose`: Enable verbose output showing all build phases and logs

**Architecture Benefits:**

- **Declarative YAML**: Define runtimes with simple, version-controlled YAML files
- **Remote Building**: Build executes on the Joblet server, not locally
- **14-Phase Pipeline**: Comprehensive build process with progress streaming
- **Zero Host Contamination**: Build tools and packages installed only in isolated environment
- **Language Profiles**: Pre-defined packages for Python, Java, Node.js, Go, Rust

### 2. List Available Runtimes

```bash
# From any RNX client
rnx runtime list
```

**Output:**

```
RUNTIME         VERSION  TYPE    SIZE     DESCRIPTION
-------         -------  ----    ----     -----------
python-3.11-ml  3.11     system  475MB    Python 3.11 with ML packages for AI development
python-3.11     3.11     system  200MB    Lightweight Python for general development
openjdk-21      21.0     system  371MB    OpenJDK 21 LTS for enterprise applications
graalvmjdk-21   21.0     system  490MB    GraalVM JDK 21 with native-image and polyglot

Total runtimes: 4
```

### 3. Get Runtime Information

```bash
# Detailed runtime information
rnx runtime info python-3.11-ml
```

**Output:**

```
Runtime: python-3.11-ml
Type: system
Version: 3.11
Description: Completely isolated Python 3.11 with ML packages

Requirements:
  Minimum Memory: 512MB
  Recommended Memory: 2GB
  Architectures: x86_64, amd64

Pre-installed Packages:
  - numpy>=1.24.3,<2.0
  - pandas>=2.0.3,<2.1
  - scikit-learn>=1.3.0,<1.4
  - matplotlib>=3.7.0,<3.8
  - seaborn>=0.12.0,<0.13
  - scipy>=1.11.0,<1.12

Usage:
  rnx job run --runtime=python-3.11-ml <command>
```

### 4. Test Runtime Functionality

```bash
# Test runtime is working correctly
rnx runtime test python-3.11-ml
```

**Output:**

```
Testing runtime: python-3.11-ml
✓ Runtime test passed
Output: Runtime resolution successful

To test the runtime in a job:
  rnx job run --runtime=python-3.11-ml python --version
```

## 📦 Runtime Deployment

The runtime system supports **remote building** for production environments. Build runtimes directly on the Joblet
server using declarative YAML specifications.

### Quick Deployment Workflow

```bash
# Step 1: Create runtime.yaml specification locally
cat > runtime.yaml << 'EOF'
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

libraries:
  - libopenblas*
  - libgfortran*

hooks:
  pre_install: |
    apt-get install -y libopenblas-dev || yum install -y openblas-devel
EOF

# Step 2: Build runtime on remote Joblet server
rnx runtime build ./runtime.yaml
# ✅ Builds to: /opt/joblet/runtimes/python-3.11-ml/1.0.0/

# Step 3: Verify installation
rnx runtime list
rnx runtime info python-3.11-ml
```

### Benefits of Declarative Runtime Building

- **Version Controlled**: Runtime specifications are simple YAML files that can be versioned in git
- **Reproducible Builds**: Same specification produces identical runtimes across environments
- **Remote Execution**: Build happens on the Joblet server, not locally
- **14-Phase Pipeline**: Comprehensive build process with progress streaming
- **Language Profiles**: Pre-defined packages for Python, Java, Node.js, Go, Rust

### Multi-Host Deployment

```bash
# Deploy to multiple Joblet servers
for host in joblet-01 joblet-02 joblet-03; do
    # Configure rnx to point to each server and build
    RNX_SERVER=$host rnx runtime build ./runtime.yaml
done
```

### Runtime Directory Structure

Each runtime is installed as a self-contained environment:

```
/opt/joblet/runtimes/python-3.11-ml/1.0.0/
├── runtime.yml              # Generated runtime configuration
└── isolated/                # Complete isolated filesystem
    ├── usr/bin/             # Language binaries
    ├── usr/lib/             # Libraries and packages
    ├── etc/                 # Configuration files
    └── lib/                 # System libraries
```

> **📚 Detailed Guide**: See [RUNTIME_REGISTRY_GUIDE.md](RUNTIME_REGISTRY_GUIDE.md) for complete YAML specification,
> supported languages, hooks, and troubleshooting.

## 🎯 Runtime Management

### Runtime CLI Commands

```bash
# List all available runtimes
rnx runtime list

# Get detailed information about a runtime
rnx runtime info <runtime-name>

# Test runtime functionality
rnx runtime test <runtime-name>

# Build a runtime from YAML specification
rnx runtime build <path-to-runtime.yaml>

# Build with verbose output
rnx runtime build -v <path-to-runtime.yaml>

# Force rebuild (overwrite existing runtime)
rnx runtime build --force <path-to-runtime.yaml>

# Validate runtime YAML specification (comprehensive server-side validation)
rnx runtime validate <path-to-runtime.yaml>

# Remove an installed runtime
rnx runtime remove <runtime-name>

# Remove specific version
rnx runtime remove <runtime-name>@<version>
```

### Runtime Building and Management

```bash
# Build Python ML runtime from specification
rnx runtime build ./examples/python-3.11-ml/runtime.yaml

# Build Java runtime
rnx runtime build ./examples/java-21/runtime.yaml

# Check installed runtime details
rnx runtime info python-3.11-ml

# Remove runtime when no longer needed
rnx runtime remove python-3.11-ml

# List all installed runtimes
rnx runtime list
```

### Using Runtimes in Jobs

```bash
# Basic usage
rnx job run --runtime=<runtime-name> <command>

# With file uploads
rnx job run --runtime=python-3.11-ml --upload=script.py python script.py

# With resource limits
rnx job run --runtime=java:17 --max-memory=2048 --max-cpu=50 java BigApplication

# With networks and volumes
rnx job run --runtime=python-3.11-ml --volume=datasets --network=isolated python analysis.py

# Scheduled execution
rnx job run --runtime=java:21 --schedule="1hour" java MaintenanceJob
```

### Runtime Naming

Runtime names support multiple formats:

```bash
# Hyphen-separated format (recommended)
--runtime=python-3.11-ml
--runtime=java:17
--runtime=java:21

# Colon-separated format (legacy)
--runtime=python-3.11-ml
--runtime=java:17
--runtime=java:21
```

## ⚡ Performance Comparison

### Startup Time Benchmarks

| **Scenario**           | **Traditional** | **Runtime** | **Speedup**   |
|------------------------|-----------------|-------------|---------------|
| Python + NumPy/Pandas  | 5-15 minutes    | 2-3 seconds | **100-300x**  |
| Python + Full ML Stack | 15-45 minutes   | 2-3 seconds | **300-1000x** |
| Java Development       | 30-120 seconds  | 2-3 seconds | **15-40x**    |
| Node.js + Dependencies | 60-300 seconds  | 2-3 seconds | **20-100x**   |

### Real-World Examples

#### Data Science Workflow

**Traditional Approach:**

```bash
# 15-30 minutes every time
rnx job run 'apt-get update && apt-get install -y python3-pip && pip install pandas numpy scikit-learn matplotlib seaborn && python analysis.py'
```

**Runtime Approach:**

```bash
# 2-3 seconds total
rnx job run --runtime=python-3.11-ml python analysis.py
```

#### Java Development

**Traditional Approach:**

```bash
# 2-5 minutes every time  
rnx job run 'apt-get update && apt-get install -y openjdk-17-jdk maven && javac HelloWorld.java && java HelloWorld'
```

**Runtime Approach:**

```bash
# 2-3 seconds total
rnx job run --runtime=java:17 bash -c "javac HelloWorld.java && java HelloWorld"
```

#### Node.js Web Development

**Traditional Approach:**

```bash
# 5-10 minutes every time
rnx job run 'apt-get update && apt-get install -y default-jdk maven && mvn compile exec:java'
```

**Runtime Approach:**

```bash
# 2-3 seconds total  
rnx job run --runtime=java:17 java Application
```

## 🏗️ Architecture

### Runtime Structure

Each runtime is **completely self-contained**, including both runtime-specific files AND all necessary system binaries
in its `isolated/` directory. This eliminates dependency on host system files during job execution.

```
/opt/joblet/runtimes/
├── python-3.11-ml/              # Flat structure (no nested language dirs)
│   ├── runtime.yml              # Runtime configuration
│   └── isolated/                # Complete filesystem for job
│       ├── bin/                 # System binaries (bash, sh, ls, etc.)
│       ├── lib/                 # System libraries
│       ├── lib64/              # 64-bit libraries
│       ├── usr/
│       │   ├── bin/            # User binaries (python3, pip, etc.)
│       │   ├── lib/            # User libraries
│       │   ├── lib/python3/    # Python packages
│       │   └── local/          # Python runtime installation
│       ├── etc/                # Configuration files (ssl, ca-certificates)
│       └── lib/x86_64-linux-gnu/  # Architecture-specific libraries (AMD64)
│           or
│       └── lib/aarch64-linux-gnu/ # Architecture-specific libraries (ARM64)
├── openjdk-21/
│   ├── runtime.yml
│   └── isolated/
│       ├── bin/                 # System binaries
│       ├── lib/                 # System libraries
│       ├── usr/
│       │   ├── lib/jvm/        # Complete JVM installation
│       │   └── share/java/     # Java libraries
│       └── etc/                # Java configuration
└── nodejs-20/                   # Future runtime example
    ├── runtime.yml
    └── isolated/
        └── ...                  # Complete Node.js environment
```

### Runtime Configuration (`runtime.yml`)

The generated `runtime.yml` contains all metadata about the built runtime:

```yaml
name: python-3.11-ml
language: python
language_version: "3.11"
version: 1.0.0
description: Python 3.11 with machine learning and data science packages

# All mounts from isolated/ - no host dependencies
mounts:
  - source: isolated/usr/local/lib
    target: /usr/local/lib
    readonly: true
  - source: isolated/usr/bin
    target: /usr/bin
    readonly: true
  - source: isolated/bin
    target: /bin
    readonly: true
  - source: isolated/lib
    target: /lib
    readonly: true
  - source: isolated/lib64
    target: /lib64
    readonly: true
  - source: isolated/usr/lib
    target: /usr/lib
    readonly: true
  - source: isolated/etc/ssl
    target: /etc/ssl
    readonly: true

# Environment variables configured for the runtime
environment:
  MPLBACKEND: Agg
  PATH: /usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin
  PYTHONPATH: /usr/local/lib/python3/site-packages
  PYTHONUNBUFFERED: "1"

# Pre-installed packages (pip/npm)
packages:
  - numpy==1.26.2
  - pandas==2.1.3
  - scikit-learn==1.3.2
  - matplotlib==3.8.2
  - seaborn==0.13.0

# Additional library patterns copied from runtime.yaml
libraries:
  - libopenblas*
  - libgfortran*
  - libquadmath*
  - libgomp*

# System requirements
requirements:
  architectures:
    - amd64
  gpu: false

# Build metadata
build_info:
  built_at: "2025-12-22T17:43:00Z"
  built_with: joblet-builder
  platform: ubuntu-amd64

# Original runtime.yaml (preserved for reference)
original_yaml: |
  schema_version: "1.0"
  name: python-3.11-ml
  ...
```

#### Runtime Configuration Fields

| Field | Description |
|-------|-------------|
| `name` | Runtime name (e.g., "python-3.11-ml") |
| `language` | Base language (python, java, node, go, rust) |
| `language_version` | Language version (e.g., "3.11") |
| `version` | Runtime version (semantic versioning) |
| `description` | Human-readable description |
| `mounts` | Filesystem mount points from isolated/ directory |
| `environment` | Environment variables set for jobs |
| `packages` | Pre-installed pip/npm packages |
| `libraries` | Custom library patterns copied |
| `requirements` | System requirements (architectures, GPU) |
| `build_info` | Build timestamp, builder version, platform |
| `original_yaml` | Original runtime.yaml content (for reference) |

### Isolation Mechanism

1. **Filesystem Isolation**: Runtime directories mounted read-only into job containers
2. **Environment Variables**: Automatic setup of PATH, library paths, and runtime homes
3. **Library Loading**: Proper LD_LIBRARY_PATH configuration for shared libraries
4. **Process Isolation**: Same process/network/cgroup isolation as regular jobs
5. **Security**: No write access to runtime files, complete separation from host

### Platform & Architecture Support

Each runtime includes platform-specific setup scripts that handle:

1. **Multi-Platform Support**:
    - Ubuntu/Debian (APT-based)
    - Amazon Linux (YUM-based)
    - RHEL/CentOS (DNF/YUM-based)

2. **Architecture Awareness**:
    - **AMD64/x86_64**: Uses `/lib/x86_64-linux-gnu/` paths
    - **ARM64/aarch64**: Uses `/lib/aarch64-linux-gnu/` paths
    - Proper dynamic linker configuration per architecture
    - Architecture-specific `LD_LIBRARY_PATH` settings

3. **Self-Contained Design**:
    - Each runtime includes ALL necessary system binaries
    - No dependency on host system files
    - Complete isolation from host filesystem
    - Portable across different Linux distributions

## 🛠️ Custom Runtimes

### Creating Custom Runtimes

You can create your own runtime environments using platform-specific setup scripts:

1. **Choose Runtime Directory Structure**:

```bash
/opt/joblet/runtimes/<runtime-name>/  # Flat structure
├── runtime.yml                       # Configuration
└── isolated/                         # Complete filesystem
    ├── bin/                          # System binaries
    ├── lib/                          # System libraries
    ├── usr/                          # User space
    └── etc/                          # Configuration
```

2. **Create Platform-Specific Setup Scripts**:

```bash
runtimes/<runtime-name>/
├── setup-ubuntu-amd64.sh    # Ubuntu AMD64 setup
├── setup-ubuntu-arm64.sh    # Ubuntu ARM64 setup
├── setup-amzn-amd64.sh      # Amazon Linux AMD64
├── setup-amzn-arm64.sh      # Amazon Linux ARM64
├── setup-rhel-amd64.sh      # RHEL/CentOS AMD64
└── setup-rhel-arm64.sh      # RHEL/CentOS ARM64
```

3. **Create Runtime Configuration (`runtime.yml`)**:

```yaml
name: "my-custom-runtime"
version: "1.0"
description: "Custom runtime - self-contained"

mounts:
  # All paths relative to isolated/
  - source: "isolated/bin"
    target: "/bin"
    readonly: true
  - source: "isolated/usr/bin"
    target: "/usr/bin"
    readonly: true
  - source: "isolated/lib"
    target: "/lib"
    readonly: true
  - source: "isolated/usr/lib"
    target: "/usr/lib"
    readonly: true

environment:
  PATH: "/usr/bin:/bin"
  LD_LIBRARY_PATH: "/usr/lib:/lib"

requirements:
  min_memory: "256MB"
  recommended_memory: "1GB"
  architectures: [ "x86_64", "amd64" ]
```

3. **Install Language/Packages**:
    - Compile from source or install packages
    - Ensure everything is self-contained
    - Test thoroughly in isolation

4. **Create Setup Script** (see existing examples):
    - `/opt/joblet/runtimes/<name>/setup_<name>.sh`
    - Automated installation and configuration
    - Host system cleanup

### Custom Runtime Example

The recommended way to create custom runtimes is using the declarative YAML builder:

```yaml
# runtime.yaml
schema_version: "1.0"
name: python-3.12-custom
version: 1.0.0
description: Python 3.12 with custom packages

base:
  language: python
  version: "3.12"

pip:
  - requests
  - pyyaml

hooks:
  post_install: |
    echo "Custom setup complete"
```

```bash
# Build the runtime on the server
rnx runtime build ./runtime.yaml

# Verify installation
rnx runtime list
rnx runtime info python-3.12-custom
```

The runtime will be installed to `/opt/joblet/runtimes/python-3.12-custom/1.0.0/` with the complete isolated filesystem.

> **Note**: See [RUNTIME_REGISTRY_GUIDE.md](RUNTIME_REGISTRY_GUIDE.md) for the complete YAML specification and advanced customization options.

## ✅ Best Practices

### Performance Optimization

1. **Memory Allocation**: Allocate sufficient memory for runtime environments
   ```bash
   # Python ML jobs typically need 1-4GB
   rnx job run --runtime=python-3.11-ml --max-memory=2048 python analysis.py
   ```

2. **CPU Allocation**: Use appropriate CPU limits
   ```bash
   # CPU-intensive ML workloads
   rnx job run --runtime=python-3.11-ml --max-cpu=75 --cpu-cores="0-3" python training.py
   ```

3. **Storage**: Use volumes for large datasets
   ```bash
   rnx volume create datasets --size=10GB
   rnx job run --runtime=python-3.11-ml --volume=datasets python process_data.py
   ```

### Security Considerations

1. **Read-Only Mounts**: All runtime files are mounted read-only
2. **Isolation**: Same security model as regular jobs
3. **No Privilege Escalation**: Runtime files owned by joblet user
4. **Library Verification**: All packages verified during setup

### Development Workflow

1. **Development Phase**: Use runtimes for fast iteration
   ```bash
   rnx job run --runtime=python-3.11-ml --upload=experiment.py python experiment.py
   ```

2. **Testing Phase**: Test with resource limits
   ```bash
   rnx job run --runtime=python-3.11-ml --max-memory=512 --upload=test.py python test.py
   ```

3. **Production Phase**: Use volumes and networks
   ```bash
   rnx job run --runtime=python-3.11-ml --volume=data --network=prod python production.py
   ```

## 🔧 Troubleshooting

### Common Issues

#### 1. Runtime Not Found

**Error**: `runtime not found: python-3.11-ml`

**Solution**: Build the runtime from a YAML specification

```bash
# Build the runtime
rnx runtime build ./examples/python-3.11-ml/runtime.yaml

# Verify it was installed
rnx runtime list
```

#### 2. Library Loading Errors

**Error**: `error while loading shared libraries: libpython3.11.so.1.0: cannot open shared object file`

**Solution**: Remove and rebuild the runtime:

```bash
# Remove old installation
rnx runtime remove python-3.11-ml

# Rebuild from specification
rnx runtime build ./examples/python-3.11-ml/runtime.yaml
```

#### 3. Memory Issues

**Error**: Job killed due to memory limit

**Solution**: Increase memory allocation

```bash
# ML jobs typically need 1-4GB
rnx job run --runtime=python-3.11-ml --max-memory=2048 python analysis.py
```

#### 4. Package Compatibility

**Error**: `numpy.dtype size changed, may indicate binary incompatibility`

**Solution**: Runtime uses compatible package versions. Use runtime packages:

```bash
# Don't install additional packages - use what's pre-installed
rnx job run --runtime=python-3.11-ml python -c "import numpy; print(numpy.__version__)"
```

### Diagnostic Commands

```bash
# Check runtime availability
rnx runtime list

# Test specific runtime
rnx runtime test python-3.11-ml

# Check runtime details
rnx runtime info python-3.11-ml

# Verify job execution
rnx job run --runtime=python-3.11-ml python -c "import sys; print(sys.executable)"

# Check server logs (on server)
sudo journalctl -u joblet.service -f
```

### Getting Help

1. **Check Examples**: `/opt/joblet/examples/python-ml/` contains working examples
2. **Runtime Info**: `rnx runtime info <runtime>` shows package versions
3. **Test Command**: `rnx runtime test <runtime>` validates setup
4. **Server Logs**: Check journalctl for detailed error messages
5. **GitHub Issues**: Report runtime-specific issues with full error details

## 🔗 Related Documentation

- **[Job Execution Guide](JOB_EXECUTION.md)** - Using runtimes with jobs
- **[RNX CLI Reference](RNX_CLI_REFERENCE.md)** - Complete command reference
- **[Configuration Guide](CONFIGURATION.md)** - Server configuration
- **[Troubleshooting](TROUBLESHOOTING.md)** - General troubleshooting

---

**Next Steps:**

- Install runtimes on your Joblet server
- Experiment with the Python ML runtime for data analysis
- Try Java runtimes for development workflows
- Create custom runtimes for your specific needs

The Runtime System transforms Joblet from a job execution platform into a **complete development and production
environment** with instant access to any language ecosystem! 🚀