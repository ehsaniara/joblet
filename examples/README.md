# Joblet Examples

This directory contains example code and runtime definitions for testing Joblet.

## Directory Structure

```
examples/
├── python/             # Basic Python runtime and examples
│   └── runtime.yaml    # Python 3.11 runtime definition
├── python-3.11-ml/     # Python ML runtime and examples
│   └── runtime.yaml    # Python 3.11 + ML packages
├── python-analytics/   # Python analytics runtime
│   └── runtime.yaml    # Python 3.11 + analytics packages
├── java-21/            # OpenJDK 21 runtime
│   └── runtime.yaml    # OpenJDK 21 runtime definition
├── java-17/            # OpenJDK 17 runtime
│   └── runtime.yaml    # OpenJDK 17 runtime definition
├── java/               # Java source files
├── basic-usage/        # Basic usage shell scripts
└── log-streaming/      # Log streaming examples
```

## Quick Start

### 1. Build a Runtime

Before running jobs, build the required runtime:

```bash
# Build Python 3.11 ML runtime
sudo rnx runtime build examples/python-3.11-ml/runtime.yaml

# Build OpenJDK 21 runtime
sudo rnx runtime build examples/java-21/runtime.yaml

# Preview a build without executing (dry-run)
rnx runtime build --dry-run examples/python-3.11-ml/runtime.yaml
```

### 2. Verify Runtime is Available

```bash
# List installed runtimes
rnx runtime list

# Get runtime details
rnx runtime info python-3.11-ml
```

### 3. Run Jobs

```bash
# Run a Python ML job
rnx job run --runtime=python-3.11-ml python3 -c "import numpy; print(numpy.__version__)"

# Run a Java job
rnx job run --runtime=openjdk-21 java --version

# Run with file upload
rnx job run --runtime=python-3.11-ml --upload=script.py python3 script.py
```

## Runtime Definitions

Each runtime directory contains a `runtime.yaml` file that defines how to build the runtime.

### Python 3.11 (Basic)

```yaml
schema_version: "1.0"
name: python-3.11
version: 1.0.0
description: Lightweight Python 3.11 runtime

base:
  language: python
  version: "3.11"
```

### Python 3.11 ML

```yaml
schema_version: "1.0"
name: python-3.11-ml
version: 1.0.0
description: Python 3.11 with ML packages

base:
  language: python
  version: "3.11"

pip:
  - numpy==1.26.2
  - pandas==2.1.3
  - scikit-learn==1.3.2
```

### OpenJDK 21

```yaml
schema_version: "1.0"
name: openjdk-21
version: 1.0.0
description: OpenJDK 21 LTS runtime

base:
  language: java
  version: "21"
```

## Runtime Selection Guide

### Choose `python-3.11` for:

- Lightweight scripts
- Fast startup required
- Basic HTTP operations (with urllib)
- File processing and JSON handling

### Choose `python-3.11-ml` for:

- Machine learning workloads
- Data science with NumPy/Pandas
- Statistical analysis
- Scientific computing

### Choose `openjdk-21` for:

- Standard Java applications
- Enterprise development
- Spring Boot applications

## Java Examples

```bash
# Build the runtime first
sudo rnx runtime build examples/java-21/runtime.yaml

# Compile and run Java program
rnx job run --runtime=openjdk-21 --upload=examples/java/JavaRuntimeTest.java javac JavaRuntimeTest.java
rnx job run --runtime=openjdk-21 java JavaRuntimeTest
```

## Python Examples

```bash
# Build the runtime first
sudo rnx runtime build examples/python-3.11-ml/runtime.yaml

# Run with ML runtime
rnx job run --runtime=python-3.11-ml --upload=examples/python/comprehensive-python-test.py python3 comprehensive-python-test.py
```

## Basic Usage Examples

The `basic-usage/` directory contains shell scripts demonstrating various joblet features:

```bash
# Run the demo scripts
cd examples/basic-usage
./01_simple_commands.sh
./02_file_operations.sh
./03_resource_limits.sh
./04_volume_storage.sh
./05_job_monitoring.sh
```

## Creating Custom Runtimes

See `docs/design/RUNTIME_YAML_QUICKREF.md` for the complete runtime.yaml specification.

Basic template:

```yaml
schema_version: "1.0"
name: my-runtime
version: 1.0.0
description: My custom runtime

base:
  language: python  # python | java | node | go | rust
  version: "3.11"

# Optional: language-specific packages
pip:
  - requests==2.31.0

# Optional: environment variables
environment:
  MY_VAR: "value"

# Optional: pre/post install hooks
hooks:
  pre_install: |
    apt-get install -y some-package
```
