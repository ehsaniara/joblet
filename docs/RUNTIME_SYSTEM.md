# Runtime System Guide

The Joblet Runtime System provides **pre-built, isolated runtime environments** that eliminate package installation delays and provide instant access to language runtimes and their ecosystems.

## 🚀 Why Runtime System?

### The Traditional Problem
```bash
# Traditional approach: 5-45 minutes every time
rnx run 'apt-get update && apt-get install python3-pip && pip install pandas numpy scikit-learn matplotlib && python analysis.py'
```

### The Runtime Solution
```bash
# Runtime approach: 2-3 seconds total
rnx run --runtime=python-3.11-ml python analysis.py
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

### Python 3.11 + ML Stack (`python-3.11-ml`)

**Complete isolated Python environment with machine learning packages**

- **Python**: 3.11.9 (compiled from source for optimal performance)
- **Pre-installed Packages**:
  - NumPy 1.24.x (pinned to 1.x for compatibility)
  - Pandas 2.0.x (data analysis)
  - Scikit-learn 1.3.x (complete ML toolkit)
  - Matplotlib 3.7.x (visualization)
  - Seaborn 0.12.x (statistical plotting)
  - SciPy 1.11.x (scientific computing)
  - Requests 2.31.0 (HTTP library)
  - OpenPyXL 3.1.2 (Excel file support)
- **Package Size**: ~226MB compressed
- **Setup Time**: ~2-3 seconds vs 5-45 minutes traditional
- **Use Cases**: Data analysis, machine learning, AI development, research

```bash
# Usage examples
rnx run --runtime=python-3.11-ml python -c "import pandas; print('Ready!')"
rnx run --runtime=python-3.11-ml --upload=analysis.py python analysis.py
rnx runtime info python-3.11-ml  # See all packages and details
```

### Java 17 LTS (`java:17`)

**Enterprise-ready OpenJDK 17 with Maven**

- **Java**: OpenJDK 17.0.11 (Long Term Support)
- **Build Tools**: Apache Maven 3.9.6
- **Features**: Enterprise stability, production-ready
- **Tools**: javac, jar, javap, jshell (interactive shell)
- **Package Size**: ~193MB compressed
- **Use Cases**: Enterprise applications, Spring Boot, microservices

### Java 21 LTS (`java:21`)

**Modern Java with cutting-edge features**

- **Java**: OpenJDK 21.0.4 (Long Term Support)
- **Build Tools**: Apache Maven 3.9.6
- **Modern Features**:
  - Virtual Threads (Project Loom)
  - Pattern Matching for switch
  - String Templates (Preview)
  - Record Patterns
  - Foreign Function & Memory API
- **Package Size**: ~208MB compressed
- **Use Cases**: Modern Java development, high-concurrency applications

```bash
# Usage examples
rnx run --runtime=java-17 java -version
rnx run --runtime=java-17 --upload=HelloWorld.java javac HelloWorld.java && java HelloWorld
rnx run --runtime=java-17 --upload=pom.xml --upload=src/ mvn compile exec:java
```

### Node.js 18 LTS (`nodejs-18`)

**Complete isolated Node.js environment with development tools**

- **Node.js**: 18.20.4 LTS (compiled from source for optimal performance)
- **Pre-installed Packages**:
  - Express 4.19.2 (web framework)
  - TypeScript 5.5.4 (TypeScript compiler and types)
  - @types/node 18.19.42 (Node.js TypeScript types)
  - Nodemon 3.1.4 (development auto-restart)
  - ESLint 8.57.0 (code linting)
  - Prettier 3.3.3 (code formatting)
- **Setup Time**: ~2-3 seconds vs 60-300 seconds traditional
- **Use Cases**: Web APIs, microservices, real-time applications, TypeScript development

```bash
# Usage examples
rnx run --runtime=nodejs-18 node --version
rnx run --runtime=nodejs-18 --upload=server.js node server.js
rnx run --runtime=nodejs-18 --upload=app.ts --upload=tsconfig.json tsc app.ts && node app.js
rnx runtime info nodejs-18  # See all packages and details
```

### Java 21 Modern (`java-21`)

**Modern Java with cutting-edge features**

- **Java**: OpenJDK 21.0.4 (Latest LTS)
- **Modern Features**:
  - Virtual Threads (Project Loom) for massive concurrency
  - Pattern Matching for switch expressions
  - String Templates (Preview)
  - Record Patterns
  - Foreign Function & Memory API (Preview)
  - Vector API (Incubator)
- **Build Tools**: Apache Maven 3.9.6
- **Tools**: javac, jar, javap, jshell, jcmd, jstat
- **Use Cases**: Modern applications, high-performance computing, research

```bash
# Usage examples
rnx run --runtime=java-21 --upload=VirtualThreadsApp.java javac VirtualThreadsApp.java && java VirtualThreadsApp
rnx run --runtime=java-21 jshell  # Interactive modern Java shell
```

## 🚀 Getting Started

### 1. Install Runtime Environments

Runtimes are installed on the Joblet server (not the client):

```bash
# On the Joblet server (requires root)
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh
sudo /opt/joblet/examples/runtimes/java-17/setup_java_17.sh
sudo /opt/joblet/examples/runtimes/java-21/setup_java_21.sh
sudo /opt/joblet/examples/runtimes/nodejs-18/setup_nodejs_18.sh
```

### 2. List Available Runtimes

```bash
# From any RNX client
rnx runtime list
```

**Output:**
```
RUNTIME         VERSION  TYPE    SIZE     DESCRIPTION
-------         -------  ----    ----     -----------
python:3.11-ml  3.11     system  724.8MB  Completely isolated Python 3.11 with ML packages
java:17         17.0.12  system  445.2MB  OpenJDK 17 LTS with Maven - completely isolated runtime
java:21         21.0.4   system  467.1MB  OpenJDK 21 with modern features - completely isolated runtime
nodejs:18       18.20.4  system  451.3MB  Node.js 18 LTS with development tools - completely isolated runtime

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
  rnx run --runtime=python-3.11-ml <command>
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
  rnx run --runtime=python-3.11-ml python --version
```

## 📦 Runtime Deployment

The runtime system supports **zero-contamination deployment** for production environments. Build runtimes once on development hosts, then deploy clean packages anywhere without installing build tools.

### Quick Deployment Workflow

```bash
# Step 1: Build runtime (on development/build host)
sudo ./examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh
# ✅ Creates: /tmp/runtime-deployments/python-3.11-ml-runtime.zip

# Step 2: Copy to workstation
scp build-host:/tmp/runtime-deployments/python-3.11-ml-runtime.zip .

# Step 3: Deploy to production (zero contamination)
sudo unzip python-3.11-ml-runtime.zip -d /opt/joblet/runtimes/
# ✅ Deploys to: /opt/joblet/runtimes/python/python-3.11-ml
```

### Benefits of Runtime Deployment

- **Zero Contamination**: Production hosts need no compilers, package managers, or build tools
- **Consistent Environments**: Same runtime package works identically across all hosts
- **Fast Deployment**: 2-3 seconds vs 5-45 minutes for package installation
- **Build Once, Deploy Many**: Single build creates package for unlimited deployments

### Multi-Host Deployment

```bash
# Deploy to multiple production hosts
for host in prod-01 prod-02 prod-03; do
    scp python-3.11-ml-runtime.zip admin@$host:/tmp/
    ssh admin@$host "sudo unzip /tmp/python-3.11-ml-runtime.zip -d /opt/joblet/runtimes/"
done
```

### Package Contents

Each runtime package is a self-contained zip file containing:
- Complete runtime environment (binaries, libraries)
- All pre-installed packages and dependencies
- Runtime metadata for auto-detection
- Proper directory structure for deployment

> **📚 Detailed Guide**: See [RUNTIME_DEPLOYMENT.md](RUNTIME_DEPLOYMENT.md) for comprehensive deployment documentation, CI/CD integration, and advanced scenarios.

## 🎯 Runtime Management

### Runtime CLI Commands

```bash
# List all available runtimes
rnx runtime list

# Get detailed information about a runtime
rnx runtime info <runtime-name>

# Test runtime functionality
rnx runtime test <runtime-name>
```

### Using Runtimes in Jobs

```bash
# Basic usage
rnx run --runtime=<runtime-name> <command>

# With file uploads
rnx run --runtime=python-3.11-ml --upload=script.py python script.py

# With resource limits
rnx run --runtime=java-17 --max-memory=2048 --max-cpu=50 java BigApplication

# With networks and volumes
rnx run --runtime=python-3.11-ml --volume=datasets --network=isolated python analysis.py

# Scheduled execution
rnx run --runtime=java-21 --schedule="1hour" java MaintenanceJob
```

### Runtime Naming

Runtime names support multiple formats:

```bash
# Hyphen-separated format (recommended)
--runtime=python-3.11-ml
--runtime=java-17
--runtime=java-21
--runtime=nodejs-18

# Colon-separated format (legacy)
--runtime=python:3.11+ml
--runtime=java:17
--runtime=java:21
--runtime=nodejs:18
```

## ⚡ Performance Comparison

### Startup Time Benchmarks

| **Scenario** | **Traditional** | **Runtime** | **Speedup** |
|-------------|----------------|-------------|-------------|
| Python + NumPy/Pandas | 5-15 minutes | 2-3 seconds | **100-300x** |
| Python + Full ML Stack | 15-45 minutes | 2-3 seconds | **300-1000x** |
| Java Development | 30-120 seconds | 2-3 seconds | **15-40x** |
| Node.js + Dependencies | 60-300 seconds | 2-3 seconds | **20-100x** |

### Real-World Examples

#### Data Science Workflow

**Traditional Approach:**
```bash
# 15-30 minutes every time
rnx run 'apt-get update && apt-get install -y python3-pip && pip install pandas numpy scikit-learn matplotlib seaborn && python analysis.py'
```

**Runtime Approach:**
```bash
# 2-3 seconds total
rnx run --runtime=python-3.11-ml python analysis.py
```

#### Java Development

**Traditional Approach:**
```bash
# 2-5 minutes every time  
rnx run 'apt-get update && apt-get install -y openjdk-17-jdk maven && javac HelloWorld.java && java HelloWorld'
```

**Runtime Approach:**
```bash
# 2-3 seconds total
rnx run --runtime=java-17 bash -c "javac HelloWorld.java && java HelloWorld"
```

#### Node.js Web Development

**Traditional Approach:**
```bash
# 5-10 minutes every time
rnx run 'curl -fsSL https://deb.nodesource.com/setup_18.x | bash - && apt-get install -y nodejs npm && npm install express typescript && node server.js'
```

**Runtime Approach:**
```bash
# 2-3 seconds total
rnx run --runtime=nodejs-18 node server.js
```

## 🏗️ Architecture

### Runtime Structure

Each runtime is completely isolated with:

```
/opt/joblet/runtimes/
├── python/
│   └── python-3.11-ml/
│       ├── runtime.yml          # Runtime configuration
│       ├── bin/                 # Executable wrapper scripts
│       ├── python-install/      # Compiled Python from source
│       │   ├── bin/             # Python binaries
│       │   └── lib/             # Shared libraries
│       └── ml-venv/             # Virtual environment with ML packages
│           ├── bin/             # Venv Python binaries
│           └── lib/python3.11/site-packages/  # ML packages
├── java/
│   ├── java-17/
│   │   ├── runtime.yml          # Runtime configuration
│   │   ├── bin/                 # Java executables
│   │   ├── jdk/                 # OpenJDK installation
│   │   └── maven/               # Maven installation
│   └── java-21/
│       └── ...
└── nodejs/
    └── nodejs-18/
        ├── runtime.yml          # Runtime configuration
        ├── bin/                 # Node.js wrapper scripts
        ├── nodejs-install/      # Compiled Node.js from source
        │   ├── bin/             # Node.js binaries
        │   └── lib/             # Node.js shared libraries
        └── lib/node_modules/    # Global npm packages
```

### Runtime Configuration (`runtime.yml`)

```yaml
name: "python-3.11-ml"
type: "system"
version: "3.11"
description: "Completely isolated Python 3.11 with ML packages"

mounts:
  - source: "bin"
    target: "/usr/local/bin"
    readonly: true
    selective: ["python", "python3", "python3.11", "pip", "pip3"]
  - source: "ml-venv/lib/python3.11/site-packages"
    target: "/usr/local/lib/python3.11/site-packages"
    readonly: true
  - source: "python-install/lib"
    target: "/usr/local/lib"
    readonly: true

environment:
  PYTHON_HOME: "/usr/local"
  PYTHONPATH: "/usr/local/lib/python3.11/site-packages"
  PATH_PREPEND: "/usr/local/bin"
  LD_LIBRARY_PATH: "/usr/local/lib:/usr/local/lib64"

package_manager:
  type: "pip"
  cache_volume: "pip-cache"

requirements:
  min_memory: "512MB"
  recommended_memory: "2GB"
  architectures: ["x86_64", "amd64"]

packages:
  - "numpy>=1.24.3,<2.0"
  - "pandas>=2.0.3,<2.1"
  - "scikit-learn>=1.3.0,<1.4"
  - "matplotlib>=3.7.0,<3.8"
  - "seaborn>=0.12.0,<0.13"
  - "scipy>=1.11.0,<1.12"
```

### Isolation Mechanism

1. **Filesystem Isolation**: Runtime directories mounted read-only into job containers
2. **Environment Variables**: Automatic setup of PATH, library paths, and runtime homes
3. **Library Loading**: Proper LD_LIBRARY_PATH configuration for shared libraries
4. **Process Isolation**: Same process/network/cgroup isolation as regular jobs
5. **Security**: No write access to runtime files, complete separation from host

## 🛠️ Custom Runtimes

### Creating Custom Runtimes

You can create your own runtime environments:

1. **Choose Runtime Directory Structure**:
```bash
/opt/joblet/runtimes/<language>/<runtime-name>/
├── runtime.yml          # Configuration
├── bin/                 # Executables
├── lib/                 # Libraries
└── <custom-dirs>/       # Language-specific directories
```

2. **Create Runtime Configuration (`runtime.yml`)**:
```yaml
name: "my-custom-runtime"
type: "system"
version: "1.0"
description: "Custom runtime description"

mounts:
  - source: "bin"
    target: "/usr/local/bin"
    readonly: true
  - source: "lib"
    target: "/usr/local/lib"
    readonly: true

environment:
  PATH_PREPEND: "/usr/local/bin"
  LD_LIBRARY_PATH: "/usr/local/lib"

requirements:
  min_memory: "256MB"
  recommended_memory: "1GB"
  architectures: ["x86_64", "amd64"]
```

3. **Install Language/Packages**:
   - Compile from source or install packages
   - Ensure everything is self-contained
   - Test thoroughly in isolation

4. **Create Setup Script** (see existing examples):
   - `/opt/joblet/examples/runtimes/<name>/setup_<name>.sh`
   - Automated installation and configuration
   - Host system cleanup

### Custom Runtime Example

Here's a basic Node.js runtime structure:

```bash
# 1. Create directory structure
sudo mkdir -p /opt/joblet/runtimes/nodejs/nodejs-18-full
cd /opt/joblet/runtimes/nodejs/nodejs-18-full

# 2. Install Node.js from source or binaries
# ... installation steps ...

# 3. Create runtime.yml
sudo tee runtime.yml << EOF
name: "nodejs-18-full"
type: "managed"
version: "18.17.0"
description: "Node.js 18 with common packages"

mounts:
  - source: "bin"
    target: "/usr/local/bin"
    readonly: true
    selective: ["node", "npm", "npx"]
  - source: "lib"
    target: "/usr/local/lib"
    readonly: true

environment:
  NODE_HOME: "/usr/local"
  PATH_PREPEND: "/usr/local/bin"

package_manager:
  type: "npm"
  cache_volume: "npm-cache"
EOF
```

## ✅ Best Practices

### Performance Optimization

1. **Memory Allocation**: Allocate sufficient memory for runtime environments
   ```bash
   # Python ML jobs typically need 1-4GB
   rnx run --runtime=python-3.11-ml --max-memory=2048 python analysis.py
   ```

2. **CPU Allocation**: Use appropriate CPU limits
   ```bash
   # CPU-intensive ML workloads
   rnx run --runtime=python-3.11-ml --max-cpu=75 --cpu-cores="0-3" python training.py
   ```

3. **Storage**: Use volumes for large datasets
   ```bash
   rnx volume create datasets --size=10GB
   rnx run --runtime=python-3.11-ml --volume=datasets python process_data.py
   ```

### Security Considerations

1. **Read-Only Mounts**: All runtime files are mounted read-only
2. **Isolation**: Same security model as regular jobs
3. **No Privilege Escalation**: Runtime files owned by joblet user
4. **Library Verification**: All packages verified during setup

### Development Workflow

1. **Development Phase**: Use runtimes for fast iteration
   ```bash
   rnx run --runtime=python-3.11-ml --upload=experiment.py python experiment.py
   ```

2. **Testing Phase**: Test with resource limits
   ```bash
   rnx run --runtime=python-3.11-ml --max-memory=512 --upload=test.py python test.py
   ```

3. **Production Phase**: Use volumes and networks
   ```bash
   rnx run --runtime=python-3.11-ml --volume=data --network=prod python production.py
   ```

## 🔧 Troubleshooting

### Common Issues

#### 1. Runtime Not Found

**Error**: `runtime not found: python-3.11-ml`

**Solution**: Install the runtime on the server
```bash
# On Joblet server
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh
```

#### 2. Library Loading Errors

**Error**: `error while loading shared libraries: libpython3.11.so.1.0: cannot open shared object file`

**Solution**: Runtime installation includes library fixes. Reinstall:
```bash
# Remove old installation
sudo rm -rf /opt/joblet/runtimes/python/python-3.11-ml

# Reinstall with updated script
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh
```

#### 3. Memory Issues

**Error**: Job killed due to memory limit

**Solution**: Increase memory allocation
```bash
# ML jobs typically need 1-4GB
rnx run --runtime=python-3.11-ml --max-memory=2048 python analysis.py
```

#### 4. Package Compatibility

**Error**: `numpy.dtype size changed, may indicate binary incompatibility`

**Solution**: Runtime uses compatible package versions. Use runtime packages:
```bash
# Don't install additional packages - use what's pre-installed
rnx run --runtime=python-3.11-ml python -c "import numpy; print(numpy.__version__)"
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
rnx run --runtime=python-3.11-ml python -c "import sys; print(sys.executable)"

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

The Runtime System transforms Joblet from a job execution platform into a **complete development and production environment** with instant access to any language ecosystem! 🚀