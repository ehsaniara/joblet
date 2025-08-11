# Python 3.11 Multi-Architecture Runtime

Modern Python 3.11 runtime with virtual environment support, optimized for multiple CPU architectures and Linux
distributions.

## 🌐 Multi-Architecture Support

### 📊 Platform Compatibility

| **Architecture**  | **Support Level** | **Build Method**       | **Performance** | **Package Support** |
|-------------------|-------------------|------------------------|-----------------|---------------------|
| **x86_64/amd64**  | ✅ Full            | Source + optimizations | Maximum         | Complete ecosystem  |
| **aarch64/arm64** | ✅ Full            | Native ARM64 build     | Native ARM64    | Full compatibility  |
| **armv7l/armhf**  | ✅ Compatible      | Compatibility build    | Good            | Basic packages      |

### 🌍 Distribution Support

- **Ubuntu/Debian**: Full APT integration with development packages
- **CentOS/RHEL/Amazon Linux**: YUM with Python development tools
- **Fedora**: DNF with modern Python packages
- **openSUSE/SLES**: Zypper with development libraries
- **Arch/Manjaro**: Pacman with comprehensive build tools
- **Alpine**: APK with musl-compatible builds

## 🚀 Quick Start

### Auto-Detecting Remote Deployment

```bash
# Automatically detects target architecture and deploys optimized runtime
./deploy_to_host.sh user@target-host

# Examples for different architectures:
./deploy_to_host.sh user@x86-server     # Intel/AMD with full optimizations
./deploy_to_host.sh user@arm64-server   # ARM64 native compilation
./deploy_to_host.sh user@pi-server      # ARM32 compatibility mode
./deploy_to_host.sh user@aws-instance   # Amazon Linux detection
```

### Local Multi-Architecture Setup

```bash
# Auto-detects local system and installs optimized Python 3.11
sudo ./setup_python_3_11_multiarch.sh

# View platform compatibility
./setup_python_3_11_multiarch.sh --help
```

### Zero-Contamination Deployment (Production)

```bash
# Step 1: Build on test system (⚠️ installs build dependencies)
sudo ./setup_python_3_11_multiarch.sh
# Creates: /tmp/runtime-deployments/python-3.11-runtime.tar.gz

# Step 2: Deploy to production (zero host modification)
scp /tmp/runtime-deployments/python-3.11-runtime.tar.gz admin@prod-host:/tmp/
ssh admin@prod-host 'sudo tar -xzf /tmp/python-3.11-runtime.tar.gz -C /opt/joblet/runtimes/python/'
```

## 📦 What's Included

### Architecture-Optimized Components

**All Architectures:**

- **Python 3.11.9** (compiled from source)
- **Virtual Environment Support** (`python -m venv`)
- **pip Package Manager** (latest version)
- **Common Packages**: requests, urllib3, certifi

**Python 3.11 Modern Features:**

- **Exception Groups** and `except*`
- **Task Groups** in asyncio
- **TOML Support** (tomllib)
- **Fine-grained Error Locations**
- **Faster CPython** (10-60% performance improvement)

**x86_64 Specific:**

- Full compilation optimizations (`--enable-optimizations`)
- Link-time optimization (LTO) where supported
- Profile-guided optimization (PGO)

**ARM64 Specific:**

- Native ARM64 compilation with optimizations
- ARM64-specific build flags
- Full feature parity with x86_64

**ARM32 Specific:**

- Compatibility mode compilation
- Some optimizations disabled for stability
- Basic package ecosystem support

## 🎯 Usage Examples

### Basic Python Usage

```bash
# List available runtimes
rnx runtime list

# View Python 3.11 runtime details  
rnx runtime info python:3.11

# Test Python installation
rnx run --runtime=python:3.11 python --version

# Test modern Python 3.11 features
rnx run --runtime=python:3.11 python -c "import sys; print(f'Python {sys.version} on {sys.platform}')"
```

### Package Management

```bash
# Install packages in isolated environment
rnx run --runtime=python:3.11 pip install requests beautifulsoup4

# List installed packages  
rnx run --runtime=python:3.11 pip list

# Create requirements file
rnx run --runtime=python:3.11 pip freeze > requirements.txt

# Install from requirements
rnx run --runtime=python:3.11 --upload=requirements.txt pip install -r requirements.txt
```

### Python 3.11 Modern Features

```bash
# Exception Groups (Python 3.11+)
cat > exception_groups_demo.py << 'EOF'
# Exception Groups Demo
def raise_multiple():
    errors = []
    try:
        raise ValueError("First error")
    except ValueError as e:
        errors.append(e)
    
    try:
        raise TypeError("Second error") 
    except TypeError as e:
        errors.append(e)
    
    if errors:
        raise ExceptionGroup("Multiple errors occurred", errors)

try:
    raise_multiple()
except* ValueError as eg:
    print(f"Caught ValueError group: {eg.exceptions}")
except* TypeError as eg:
    print(f"Caught TypeError group: {eg.exceptions}")
EOF

rnx run --runtime=python:3.11 --upload=exception_groups_demo.py python exception_groups_demo.py
```

### Async Task Groups

```bash
# Task Groups (Python 3.11+)
cat > task_groups_demo.py << 'EOF'
import asyncio

async def fetch_data(name, delay):
    await asyncio.sleep(delay)
    return f"Data from {name}"

async def main():
    async with asyncio.TaskGroup() as tg:
        task1 = tg.create_task(fetch_data("API-1", 1))
        task2 = tg.create_task(fetch_data("API-2", 2))
        task3 = tg.create_task(fetch_data("API-3", 1.5))
    
    print("All tasks completed:")
    print(task1.result())
    print(task2.result()) 
    print(task3.result())

asyncio.run(main())
EOF

rnx run --runtime=python:3.11 --upload=task_groups_demo.py python task_groups_demo.py
```

### Template-Based Usage

```bash
# Use YAML templates for common tasks (if available)
cd /opt/joblet/examples/python-analytics
rnx run --template=jobs.yaml:sales-analysis

# Web scraping with requests
rnx run --template=jobs.yaml:web-scraper
```

## ⚡ Performance Benefits

### Architecture-Specific Performance

| **Architecture** | **Traditional Setup**        | **Runtime Startup** | **Speedup**          | **Optimization Level** |
|------------------|------------------------------|---------------------|----------------------|------------------------|
| **x86_64**       | 10-45 min (build + packages) | 2-3 seconds         | **200-540x faster**  | Full optimizations     |
| **ARM64**        | 15-60 min (native build)     | 3-5 seconds         | **180-720x faster**  | Native ARM64           |
| **ARM32**        | 20-90 min (slow build)       | 5-10 seconds        | **240-1080x faster** | Compatibility mode     |

### Python 3.11 Performance Improvements

**CPython Optimizations:**

- **10-60% faster** than Python 3.10
- **Adaptive bytecode** interpreter
- **Faster function calls**
- **Optimized error handling**

**Architecture Benefits:**

- **x86_64**: Full optimization with PGO and LTO
- **ARM64**: Native ARM64 performance gains
- **ARM32**: Stable performance on resource-constrained systems

## 🔧 Architecture-Specific Troubleshooting

### x86_64/amd64 Issues

```bash
# Should work out-of-box with full optimizations
# Verify Python compilation
rnx run --runtime=python:3.11 python -c "import sys; print(sys.version_info)"

# Check optimization flags
rnx run --runtime=python:3.11 python -c "import sysconfig; print(sysconfig.get_config_var('CONFIG_ARGS'))"
```

### ARM64/aarch64 Issues

```bash
# Verify ARM64 native binary
file /opt/joblet/runtimes/python/python-3.11/python-install/bin/python3
# Should show: ELF 64-bit LSB executable, ARM aarch64

# Test ARM64-specific performance
rnx run --runtime=python:3.11 python -c "
import time
start = time.time()
sum(range(1000000))
print(f'ARM64 performance test: {time.time() - start:.4f}s')
"
```

### ARM32/armhf Issues

```bash
# Check if optimizations were disabled for compatibility
rnx run --runtime=python:3.11 python -c "import sysconfig; print('Optimizations:', '--enable-optimizations' in sysconfig.get_config_var('CONFIG_ARGS'))"

# ARM32 may have package installation issues
rnx run --runtime=python:3.11 pip install --no-binary :all: some-package
```

### Distribution-Specific Issues

**Amazon Linux:**

```bash
# Ensure Python development packages
sudo yum install python3-devel gcc openssl-devel libffi-devel
```

**Alpine Linux:**

```bash
# May need specific build packages for musl
sudo apk add python3-dev build-base openssl-dev libffi-dev
```

**Arch Linux:**

```bash
# Usually excellent support
sudo pacman -S python base-devel openssl libffi
```

### Package Installation Issues

```bash
# For packages requiring compilation
rnx run --runtime=python:3.11 pip install --no-binary :all: numpy

# For packages with ARM issues
rnx run --runtime=python:3.11 pip install --only-binary=all some-package

# Build from source if binaries unavailable
rnx run --runtime=python:3.11 pip install --no-deps package-name
```

## 📊 Runtime Manifest

The runtime creates a detailed manifest at `/opt/joblet/runtimes/python/python-3.11/runtime.yml`:

```yaml
name: "python:3.11"
version: "3.11.9"
description: "Python 3.11 with virtual environment support"
type: "python"
system:
  architecture: "amd64"  # Detected architecture
  os: "Linux"
  distribution: "ubuntu"  # Detected distribution
paths:
  python_home: "/opt/joblet/runtimes/python/python-3.11/python-install"
  venv_home: "/opt/joblet/runtimes/python/python-3.11/base-venv"
binaries:
  python: "/opt/joblet/runtimes/python/python-3.11/base-venv/bin/python"
  pip: "/opt/joblet/runtimes/python/python-3.11/base-venv/bin/pip"
features:
  - "Exception Groups (Python 3.11+)"
  - "Task Groups (asyncio)"
  - "TOML support (tomllib)"
  - "Virtual Environment Support"
packages:
  - "requests"
  - "urllib3" 
  - "certifi"
```

## 🌟 Python 3.11 Feature Deep Dive

### Exception Groups Example

```python
# exception_groups_advanced.py
class ValidationError(Exception):
    pass

class NetworkError(Exception):
    pass

def validate_and_fetch():
    errors = []
    
    # Simulate validation errors
    try:
        if not True:  # Some validation
            raise ValidationError("Invalid input")
    except ValidationError as e:
        errors.append(e)
    
    try:
        # Simulate network error
        raise NetworkError("Connection failed")
    except NetworkError as e:
        errors.append(e)
    
    if errors:
        raise ExceptionGroup("Operation failed", errors)

# Handle multiple exception types
try:
    validate_and_fetch()
except* ValidationError as eg:
    print("Validation errors:")
    for exc in eg.exceptions:
        print(f"  - {exc}")
except* NetworkError as eg:
    print("Network errors:")
    for exc in eg.exceptions:
        print(f"  - {exc}")
```

### TOML Support Example

```python
# toml_example.py
import tomllib

# Read TOML configuration
config_toml = """
[database]
host = "localhost"
port = 5432

[api]
timeout = 30
retries = 3

[features]
enable_cache = true
debug_mode = false
"""

# Parse TOML
config = tomllib.loads(config_toml)
print("Database config:", config['database'])
print("API timeout:", config['api']['timeout'])
print("Cache enabled:", config['features']['enable_cache'])
```

## 🏗️ Manual Installation Steps

If you need to understand what the scripts do:

### 1. System Detection and Dependencies

```bash
# Detect architecture and distribution
uname -m        # Get CPU architecture
cat /etc/os-release  # Get Linux distribution

# Install build dependencies (varies by distribution)
# Ubuntu/Debian:
sudo apt-get install python3-dev build-essential libssl-dev libffi-dev

# CentOS/RHEL:
sudo yum install python3-devel gcc openssl-devel libffi-devel
```

### 2. Download and Compile Python

```bash
# Download Python 3.11 source
wget "https://www.python.org/ftp/python/3.11.9/Python-3.11.9.tgz"
tar -xzf Python-3.11.9.tgz
cd Python-3.11.9

# Configure with architecture-specific flags
./configure --prefix=/opt/joblet/runtimes/python/python-3.11/python-install \
           --enable-optimizations \
           --with-ensurepip=install \
           --enable-shared

# Compile (architecture-optimized)
make -j$(nproc)
make install
```

### 3. Create Virtual Environment

```bash
# Create base virtual environment
/opt/joblet/runtimes/python/python-3.11/python-install/bin/python3 -m venv \
  /opt/joblet/runtimes/python/python-3.11/base-venv

# Install common packages
source /opt/joblet/runtimes/python/python-3.11/base-venv/bin/activate
pip install --upgrade pip requests urllib3 certifi
```

## 📚 Related Documentation

- **[Multi-Arch Main README](../README.md)**: Complete multi-architecture system overview
- **[System Detection](../common/detect_system.sh)**: Architecture detection library
- **[Python ML Runtime](../python-3.11-ml/README.md)**: ML-enhanced version
- **[Example Usage](/opt/joblet/examples/python-analytics/)**: Analytics examples

## 🎉 Summary

The Python 3.11 multi-architecture runtime provides:

- **🌟 Modern Python**: Exception Groups, Task Groups, TOML support
- **🌐 Universal Linux Support**: Works on x86_64, ARM64, and ARM32
- **🚀 Instant Startup**: 180-1080x faster than traditional Python setup
- **🔒 Complete Isolation**: Zero host system contamination in production
- **📦 Package Ready**: Virtual environment with common packages
- **🎯 Auto-Detection**: Automatically optimizes for target architecture

**Perfect for modern Python development with the latest features across any Linux architecture!** 🐍⚡