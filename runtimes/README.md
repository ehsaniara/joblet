# Joblet Runtime Environments

Pre-built runtime environments for fast job execution without dependency installation.

## Available Runtimes

| Runtime          | Description                              | Size  | Use Case                                  |
|------------------|------------------------------------------|-------|-------------------------------------------|
| `python-3.11`    | Basic Python with essential packages     | 9.3KB | AI agents, lightweight scripts            |
| `python-3.11-ml` | Python with ML libraries (NumPy, Pandas) | 7.7KB | Data science, machine learning            |
| `openjdk-21`     | OpenJDK 21 with development tools        | 8.6KB | Java applications, development            |
| `graalvmjdk-21`  | GraalVM JDK 21 with native-image         | 4.8KB | High-performance Java, native compilation |

## Quick Start

### Install a Runtime

```bash
# Install lightweight Python runtime
rnx runtime install python-3.11

# Install Python with ML libraries
rnx runtime install python-3.11-ml

# Install Java runtime
rnx runtime install openjdk-21

# Install GraalVM runtime
rnx runtime install graalvmjdk-21
```

### Use a Runtime

```bash
# Basic Python for AI agents
rnx job run --runtime=python-3.11 python -c "import requests; print('Ready!')"

# Python with ML libraries
rnx job run --runtime=python-3.11-ml python -c "import numpy; print(numpy.__version__)"

# Java applications
rnx job run --runtime=openjdk-21 java -version

# Native compilation
rnx job run --runtime=graalvmjdk-21 native-image --version
```

## Runtime Features

### Python Runtimes

**`python-3.11` (Basic)**

- ✅ Fast startup (~1 second)
- ✅ Essential packages: `requests`, `urllib3`, `certifi`
- ✅ Perfect for AI agents and utility scripts

**`python-3.11-ml` (Machine Learning)**

- ✅ Full ML stack: NumPy 2.2.6, Pandas 2.3.2, Scikit-learn
- ✅ Data visualization: Matplotlib
- ✅ Scientific computing: SciPy

### Java Runtimes

**`openjdk-21` (Standard)**

- ✅ OpenJDK 21.0.1 with complete JDK tools
- ✅ Development tools: `javac`, `jar`, `jarsigner`
- ✅ Enterprise-ready Java runtime

**`graalvmjdk-21` (High Performance)**

- ✅ GraalVM Community Edition JDK 21
- ✅ Native image compilation with `native-image`
- ✅ Optimized for performance and fast startup

## Install from GitHub

Install runtimes directly from GitHub repositories:

```bash
# List available runtimes from repository
rnx runtime list --github-repo=owner/repo/tree/main/runtimes

# Install from GitHub
rnx runtime install python-3.11 --github-repo=owner/repo/tree/main/runtimes
```

## Platform Support

All runtimes support:

- Ubuntu/Debian (AMD64, ARM64)
- RHEL/CentOS/Rocky (AMD64, ARM64)
- Amazon Linux (AMD64, ARM64)

## Runtime Management

```bash
# List installed runtimes
rnx runtime list

# Get runtime information
rnx runtime info python-3.11

# Remove a runtime
rnx runtime remove python-3.11
```

## Architecture

Each runtime provides:

- **Complete isolation** - No host system dependencies
- **Read-only mounts** - Secure runtime environment
- **Platform detection** - Automatic OS/architecture support
- **Resource validation** - RAM/disk requirement checking

## Creating New Runtimes

1. Create runtime directory with platform-specific setup scripts:
   ```
   my-runtime/
   ├── setup.sh                 # Main entry point
   ├── setup-ubuntu-amd64.sh    # Ubuntu AMD64
   ├── setup-ubuntu-arm64.sh    # Ubuntu ARM64
   └── setup-*.sh               # Other platforms
   ```

2. Package the runtime:
   ```bash
   ./package-runtime.sh my-runtime
   ```

3. Update manifest and publish:
   ```bash
   ./build-runtimes
   git add *.tar.gz runtime-manifest.json
   git commit -m "Add my-runtime"
   git push
   ```

## Troubleshooting

**Runtime not found:**

- Ensure runtime is installed: `rnx runtime list`
- Install if missing: `rnx runtime install <name>`

**Platform not supported:**

- Check supported platforms in manifest
- Runtime may not support your OS/architecture

**Installation fails:**

- Verify minimum system requirements
- Check disk space and permissions
- Review error logs for specific issues

---

For detailed technical information, see the [runtime manifest schema](runtime-manifest.json) and individual runtime
documentation.