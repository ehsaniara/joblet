# Runtime Package Downloads

The pre-built runtime packages are large binary files (600MB+ total) and are not stored in this repository.

## Download Locations

### Option 1: GitHub Releases (Recommended)

Download the latest runtime packages from GitHub Releases:

```bash
# Download all runtime packages
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/java-17-runtime-complete.tar.gz -o java-17-runtime-complete.tar.gz
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/java-21-runtime-complete.tar.gz -o java-21-runtime-complete.tar.gz  
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/python-3.11-ml-runtime.tar.gz -o python-3.11-ml-runtime.tar.gz
```

### Option 2: Build from Source

Build runtime packages locally using the setup scripts:

```bash
# Build Java 17 runtime
cd examples/runtimes/java-17
./setup_java_17.sh

# Build Java 21 runtime  
cd examples/runtimes/java-21
./setup_java_21.sh

# Build Python 3.11 ML runtime
cd examples/runtimes/python-3.11-ml
./setup_python_3_11_ml.sh
```

## Package Information

| Runtime        | File                              | Size   | Description                  |
|----------------|-----------------------------------|--------|------------------------------|
| Java 17        | `java-17-runtime-complete.tar.gz` | ~193MB | OpenJDK 17 + Maven + Gradle  |
| Java 21        | `java-21-runtime-complete.tar.gz` | ~208MB | OpenJDK 21 + Modern Features |
| Python 3.11 ML | `python-3.11-ml-runtime.tar.gz`   | ~226MB | Python 3.11 + ML Libraries   |

## Installation

After downloading, extract packages to your Joblet runtime directory:

```bash
mkdir -p ~/.joblet/runtimes
tar -xzf java-17-runtime-complete.tar.gz -C ~/.joblet/runtimes/
tar -xzf java-21-runtime-complete.tar.gz -C ~/.joblet/runtimes/
tar -xzf python-3.11-ml-runtime.tar.gz -C ~/.joblet/runtimes/
```

## Verification

Verify runtime installation:

```bash
joblet runtime list
# Should show: java:17, java:21, python:3.11:ml
```