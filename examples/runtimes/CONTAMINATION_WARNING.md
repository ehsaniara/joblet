# ⚠️ Runtime Script Host Contamination Warning

## Current Status

### ⚠️ Scripts That Contaminate Host System

- **`python-3.11-ml/setup_python_3_11_ml.sh`**: Installs build dependencies on host
- **`java-17/setup_java_17.sh`**: Installs wget/curl on host
- **`java-21/setup_java_21.sh`**: Installs wget/curl on host

## Contamination Details

### Python Script Issues

```bash
# Lines 49-53: Installs packages globally on host
apt-get install -y wget build-essential libssl-dev zlib1g-dev \
                   libbz2-dev libreadline-dev libsqlite3-dev \
                   libncursesw5-dev xz-utils tk-dev libxml2-dev \
                   libxmlsec1-dev libffi-dev liblzma-dev
```

**Host Impact:**

- Adds ~200MB of build tools to host system
- Modifies system package database
- Installs development headers and libraries globally

### Java Scripts Issues

```bash
# Lines 46-47: Installs packages globally on host
apt-get update -qq
apt-get install -y wget curl
```

**Host Impact:**

- Adds wget/curl if not present
- Updates package database
- Minor contamination but still affects host

## Solutions

### 🔄 Recommended Solutions for Other Runtimes

#### Python Script Fix

```bash
# Instead of: apt-get install build-dependencies
# Use: Pre-compiled Python binary distribution
wget "https://github.com/indygreg/python-build-standalone/releases/download/..."
# OR use existing system Python with isolated venv (if acceptable)
```

#### Java Scripts Fix

```bash
# Instead of: apt-get install wget curl
# Check if tools exist, fail gracefully if missing
# Use built-in alternatives where possible
```

## Clean Installation Verification

### Test for Host Contamination

```bash
# Before running script
dpkg -l | grep -E "(python3-dev|build-essential)" > before.txt

# After running script  
dpkg -l | grep -E "(python3-dev|build-essential)" > after.txt

# Compare - should be identical for clean scripts
diff before.txt after.txt
```

## ✅ RECOMMENDED SOLUTION: Portable Runtime Packages

### Best Practice Approach

Instead of running contaminating scripts on production hosts:

1. **Build Environment**: Set up runtimes in test/dev environment (contamination OK)
2. **Package Runtimes**: Create portable packages with zero contamination
3. **Deploy Clean**: Install packages on production (complete isolation)

```bash
# Build environment (contamination acceptable)
sudo ./runtime_manager.sh build-all

# Production deployment (ZERO contamination)  
sudo ./runtime_manager.sh install-all
```

**See: [PORTABLE_RUNTIME_PACKAGING.md](./PORTABLE_RUNTIME_PACKAGING.md) for complete guide**

## Recommendations

### For Production Environments (Recommended)

1. **Use portable packaging approach** - zero contamination
2. **Build once, deploy everywhere** - consistent environments
3. **Version control packages** - reproducible deployments
4. **Fast deployment** - just extract pre-built artifacts

### For Development/Testing

1. **Use Node.js script directly** - it's completely clean
2. **Be aware** that Python/Java scripts modify host system
3. **Use packaging approach** for consistency with production

### For Script Improvements

1. **Download pre-built binaries** instead of compiling
2. **Use static binaries** where possible
3. **Check for existing tools** before installing
4. **Provide containerized versions** for ultra-clean environments

## Container Alternative

For completely clean installations, run in containers:

```bash
# Example: Install in container, copy runtime out
docker run --rm -v /opt/joblet/runtimes:/runtimes ubuntu:22.04 /bin/bash -c "
  cd /tmp && 
  curl -o script.sh https://raw.../setup_node_18.sh &&
  chmod +x script.sh &&
  ./script.sh
"
```

## Impact Assessment

### Python Runtime ⚠️

- **Host Impact**: ~200MB build tools
- **Isolation**: Application code isolated, build tools on host
- **Recommended**: With awareness of host changes

### Java Runtimes ⚠️

- **Host Impact**: Minimal (wget/curl)
- **Isolation**: Java runtime isolated, tools on host
- **Recommended**: With awareness of host changes

