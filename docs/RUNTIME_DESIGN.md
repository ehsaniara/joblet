# Runtime System Design and Examples

This comprehensive document covers both the technical design of the Joblet runtime system and practical examples for
using it effectively.

## Table of Contents

- [Part 1: System Design](#part-1-system-design)
    - [Overview](#overview)
    - [Key Design Principles](#key-design-principles)
    - [Architecture](#architecture)
    - [Implementation Details](#implementation-details)
    - [Execution Internals](#execution-internals)
    - [Security Considerations](#security-considerations)
    - [Performance Optimization](#performance-optimization)
- [Part 2: Practical Examples](#part-2-practical-examples)
    - [Basic Runtime Usage](#basic-runtime-usage)
    - [Advanced Runtime Scenarios](#advanced-runtime-scenarios)
    - [Development Workflows](#development-workflows)
    - [Performance Optimization Examples](#performance-optimization-examples)
    - [Best Practices](#best-practices-examples)

---

# Part 1: System Design

## Overview

The Joblet runtime system provides isolated, version-specific execution environments for different programming languages
and frameworks. This system allows jobs to specify their runtime requirements and execute within completely isolated
environments without contaminating the host system.

## Key Design Principles

### 1. Complete Host Isolation

- Runtime environments exist only in `/opt/joblet/runtimes/`
- Host system remains completely clean and uncontaminated
- Build dependencies are automatically removed after runtime installation
- No runtime packages installed on host system

### 2. Version-Specific Support

- Support multiple versions of the same language (e.g., `python-3.11-ml`, `python-3.12`)
- Each runtime is completely independent
- Version-specific directory structure prevents conflicts

### 3. Mount-Based Runtime Loading

- Runtimes are loaded via bind mounts into job containers
- Read-only mounts ensure runtime integrity
- Selective mounting of specific binaries and libraries

## Architecture

### Directory Structure

```
/opt/joblet/runtimes/
├── python/
│   ├── python-3.11-ml/          # Python 3.11 + ML packages
│   │   ├── runtime.yml          # Runtime configuration
│   │   ├── python-install/      # Isolated Python installation
│   │   ├── ml-venv/            # Virtual environment with ML packages
│   │   ├── bin/                # Symlinks for mounting
│   │   └── lib/                # Library symlinks
│   └── python-3.12/            # Python 3.12 modern features
├── java/
│   ├── java-17/                # OpenJDK 17 LTS
│   └── java-21/                # OpenJDK 21 with modern features
├── node/
│   └── node-18/                # Node.js 18 LTS
└── [future runtimes]
```

### Runtime Configuration

Each runtime includes a `runtime.yml` file specifying:

- Runtime metadata (name, version, description)
- Mount points for job containers
- Environment variables
- Package manager configuration
- Resource requirements

Example:

```yaml
name: "python-3.11-ml"
type: "system"
version: "3.11"
description: "Python 3.11 with ML packages"

mounts:
  - source: "bin"
    target: "/usr/local/bin"
    readonly: true
    selective: [ "python", "python3", "pip" ]
  - source: "ml-venv/lib/python3.11/site-packages"
    target: "/usr/local/lib/python3.11/site-packages"
    readonly: true

environment:
  PYTHON_HOME: "/usr/local"
  PYTHONPATH: "/usr/local/lib/python3.11/site-packages"
  PATH_PREPEND: "/usr/local/bin"

requirements:
  min_memory: "512MB"
  recommended_memory: "2GB"
```

## Runtime Types

### System Runtimes

Runtimes that provide complete language environments with interpreters/compilers and standard libraries.

**Examples:**

- `python-3.11-ml` - Python 3.11 with NumPy, Pandas, Scikit-learn
- `python-3.12` - Python 3.12 with modern features
- `java:17` - OpenJDK 17 LTS with Maven
- `java:21` - OpenJDK 21 with Virtual Threads

## Implementation Details

### Runtime Resolution

1. Job specifies runtime via `--runtime=python-3.11-ml`
2. Runtime manager resolves to `/opt/joblet/runtimes/python/python-3.11-ml/`
3. Configuration loaded from `runtime.yml`
4. Mount points prepared for job container

### Job Execution Flow

1. **Pre-execution**: Runtime mounts prepared
2. **Container Setup**: Runtime binaries/libraries mounted into job container
3. **Environment Setup**: Runtime environment variables applied
4. **Execution**: Job runs with access to runtime tools
5. **Cleanup**: Runtime mounts cleaned up

## Execution Internals

This section documents the internal implementation details of how runtimes are resolved, mounted, and executed.

### Execution Flow Overview

When a user runs `rnx job run --runtime=python-3.11 "python script.py"`, the following flow occurs:

```
┌─────────────────────┐
│ 1. CLI Parsing      │  internal/rnx/jobs/run.go
│    Parse --runtime  │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 2. gRPC Request     │  RunJobRequest protobuf
│    Send to server   │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 3. Job Building     │  internal/joblet/core/joblet.go
│    Create domain.Job│
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 4. Execution Coord  │  internal/joblet/core/execution/coordinator.go
│    Set JOB_RUNTIME  │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 5. Process Fork     │  Namespace isolation
│    PID/NET/MNT/IPC  │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 6. Filesystem Setup │  internal/joblet/core/filesystem/isolator.go
│    Mount runtime    │
│    Perform chroot   │
└─────────┬───────────┘
          │
          ▼
┌─────────────────────┐
│ 7. Job Execution    │  internal/modes/jobexec/jobexec.go
│    Execute command  │
└─────────────────────┘
```

### Key Components

| Component | File | Key Functions |
|-----------|------|---------------|
| CLI Run Command | `internal/rnx/jobs/run.go` | `runRun()` (lines 137-376) |
| Job Handler | `internal/joblet/core/joblet.go` | `StartJob()` (lines 99-160) |
| Execution Coordinator | `internal/joblet/core/execution/coordinator.go` | `StartJob()` (lines 52-184) |
| Environment Builder | `internal/joblet/core/execution/environment_service.go` | `BuildEnvironment()` (lines 44-107) |
| Filesystem Isolator | `internal/joblet/core/filesystem/isolator.go` | `Setup()` (lines 336-436) |
| Runtime Mounting | `internal/joblet/core/filesystem/isolator.go` | `mountRuntime()` (lines 1204-1227) |
| Mount Manager | `internal/joblet/core/filesystem/isolator.go` | `mountRuntimeWithManager()` (lines 1230-1402) |
| Runtime Resolver | `internal/joblet/runtime/resolver.go` | `FindRuntimeDirectory()` (lines 214-307) |
| Job Execution | `internal/modes/jobexec/jobexec.go` | `ExecuteJob()` (lines 83-107) |

### Environment Variable Flow

The runtime specification flows through the system via environment variables:

```
CLI Input: --runtime=python-3.11
    │
    ▼
gRPC RunJobRequest.Runtime = "python-3.11"
    │
    ▼
domain.Job.Runtime = "python-3.11"
    │
    ▼
ExecutionCoordinator.BuildEnvironment()
    │ Creates: JOB_RUNTIME=python-3.11
    │ Also adds runtime-specific env vars from runtime.yml
    ▼
JobFilesystem.loadRuntimeFromEnvironment()
    │ Reads: f.Runtime = os.Getenv("JOB_RUNTIME")
    ▼
JobFilesystem.mountRuntime()
    │ Resolves path, loads config, bind mounts
    ▼
Job Process
    │ Executes with runtime binaries in PATH
    └─ Environment includes PYTHONPATH, PATH_PREPEND, etc.
```

### Runtime Resolution Algorithm

The resolver (`internal/joblet/runtime/resolver.go`) finds the runtime directory:

```go
// Input: spec = "python-3.11"
//
// Step 1: Scan /opt/joblet/runtimes/
//   - Check for versioned structure: <name>/<version>/runtime.yml
//   - Check for flat structure: <name>/runtime.yml
//
// Step 2: Load runtime.yml from each candidate
//   - Parse YAML to get runtime name, version, language
//
// Step 3: Match against spec
//   - Exact name match: "python-3.11" == config.Name
//   - Semantic matching for version specs
//
// Output: /opt/joblet/runtimes/python-3.11/1.3.1/
```

**Supported Specification Formats:**
- `python-3.11` - Direct name match
- `python-3.11@1.3.1` - Name with version (npm-style)
- `python-3.11@latest` - Latest version

### Two-Phase Mount Process

Runtime mounting in `mountRuntimeWithManager()` uses a two-phase approach to avoid read-only parent directory issues:

**Phase 1: Create Mount Points** (before any mounts)
```go
for _, mount := range config.Mounts {
    // 1. Check if source exists in runtime directory
    // 2. Create target directory in chroot
    // 3. Handle file vs directory sources
}
```

**Phase 2: Bind Mount** (after all directories created)
```go
for _, mount := range config.Mounts {
    // 1. Bind mount: mount(source, target, "", MS_BIND, "")
    // 2. If readonly: remount with MS_BIND | MS_REMOUNT | MS_RDONLY
}
```

**Example Mount from Python 3.11:**
```
Source: /opt/joblet/runtimes/python-3.11/1.3.1/bin
Target: /usr/local/bin (in chroot)
Flags:  MS_BIND (read-write for runtime)
Result: Chrooted /usr/local/bin contains python, pip, etc.
```

### Filesystem Isolation Sequence

The `JobFilesystem.Setup()` function (lines 336-436) performs isolation in this order:

1. **Validate job context** - Check JOB_ID, verify PID 1
2. **Create essential directories** - /etc, /tmp, /proc, /dev, /work, /volumes
3. **Mount allowed read-only host directories** - /bin, /usr/bin, /lib, etc.
4. **Load runtime from environment** - Read JOB_RUNTIME env var
5. **Mount runtime** ← Critical step, allows runtime to override defaults
6. **Mount volumes** - User-specified persistent volumes
7. **Setup /tmp** - Isolated writable temp space
8. **Perform chroot** - Change root to isolated filesystem
9. **Mount essential filesystems** - /proc, /dev after chroot

### Runtime Environment Loading

The environment service (`internal/joblet/core/execution/environment_service.go`) loads runtime-specific environment variables:

```go
// getRuntimeEnvironment(runtimeSpec string) []string
//
// 1. Load /opt/joblet/runtimes/<spec>/runtime.yml
// 2. Parse YAML environment section
// 3. Handle PATH_PREPEND special case
// 4. Return array of "KEY=VALUE" strings
```

**Example runtime.yml environment:**
```yaml
environment:
  PATH_PREPEND: "/usr/local/bin"
  PYTHONPATH: "/usr/local/lib/python3.11/site-packages"
  PYTHON_VERSION: "3.11"
```

**Result in job process:**
```
PATH=/usr/local/bin:$ORIGINAL_PATH
PYTHONPATH=/usr/local/lib/python3.11/site-packages
PYTHON_VERSION=3.11
```

### Critical Design Points

1. **Mount Order Matters**: Runtime mounting happens AFTER allowed directories but BEFORE chroot, allowing runtime files to override host defaults

2. **Environment Variable Bridge**: `JOB_RUNTIME` env var bridges CLI input through server to filesystem isolation layer

3. **Two-Phase Mounting**: Prevents "read-only filesystem" errors when parent directories are already mounted read-only

4. **Chroot Boundary**: All mounting completes before `chroot()` call; after chroot, mounts are in place

5. **PID 1 Validation**: Filesystem setup validates it's running as PID 1 in new namespace for security

### Data Flow Summary

| Stage | Component | Data | Key Variable |
|-------|-----------|------|--------------|
| CLI | run.go | "python-3.11" | Command line arg |
| gRPC | RunJobRequest | runtime field | Protobuf field |
| Server | domain.Job | Job.Runtime | Struct field |
| Execution | Coordinator | Build environment | JOB_RUNTIME env |
| Isolation | JobFilesystem | loadRuntime() | Reads env var |
| Mount | Resolver | FindRuntimeDirectory() | Returns path |
| Config | runtime.yml | mounts & environment | YAML config |
| Process | Job | Runtime in PATH | Modified PATH |

### Network Integration

- Runtimes work with Joblet's network isolation
- Web runtimes can use `--network=web` for external access
- Package managers can use cached volumes

### Volume Integration

- Package manager caches (`pip-cache`, `maven-cache`, `npm-cache`)
- User package volumes for persistent installations
- Runtime-specific volume isolation

## Installation Process

### Automated Setup

The runtime installation system provides:

- Command-line options for selective installation
- Pre-installation checks (disk space, network)
- Automatic dependency removal
- Configuration integration with Joblet

### Installation Options

```bash
# Install from default registry
rnx runtime install python-3.11-ml

# Install specific version
rnx runtime install python-3.11-ml@1.0.2

# Install from custom registry
rnx runtime install custom-runtime --registry=myorg/runtimes
```

### Post-Installation

- Joblet configuration automatically updated
- Runtime support enabled
- Service restart recommended

## Security Considerations

### Isolation Boundaries

- Each runtime completely isolated from host
- Read-only runtime mounts prevent job contamination
- Runtime-specific volume isolation
- No runtime persistence between jobs

### Build Security

- Build dependencies removed after installation
- No build tools remain on host system
- Source code cleanup after compilation
- Minimal runtime footprint

## Performance Optimization

### Mount Optimization

- Selective mounting reduces container overhead
- Read-only mounts improve security and performance
- Shared library optimization where possible

### Resource Management

- Runtime-specific memory recommendations
- CPU affinity support for multi-core runtimes
- Disk space monitoring and cleanup

## Future Extensions

### Planned Runtimes

- `python:3.13` - Latest Python features
- `go:1.22` - Latest Go version
- `rust:stable` - Rust stable toolchain
- `dotnet:8` - .NET 8 runtime

### Advanced Features

- Custom runtime definitions
- Runtime inheritance and composition
- Multi-architecture support (ARM64)
- GPU-enabled ML runtimes

## Troubleshooting

### Common Issues

1. **Runtime Not Found**: Check runtime installation and naming
2. **Permission Errors**: Verify runtime directory permissions
3. **Mount Failures**: Check available disk space and filesystem support
4. **Package Issues**: Verify network connectivity and cache volumes

### Debugging

- Use `rnx runtime list` to verify available runtimes
- Check runtime logs in `/var/log/joblet/`
- Verify mount points with `mount | grep joblet`

## Migration from Legacy Systems

### From Host-Installed Runtimes

1. Identify currently installed language versions
2. Install equivalent isolated runtimes
3. Update job configurations to specify runtimes
4. Remove host-installed packages (optional)

### Compatibility Matrix

- Jobs without `--runtime` use host system (legacy mode)
- Jobs with `--runtime` use isolated environments
- Gradual migration supported

---

# Part 2: Practical Examples

## Basic Runtime Usage

### Python Examples

#### Data Analysis with ML Runtime

```bash
# Upload data file and run analysis
rnx job run --runtime=python-3.11-ml \
        --upload=data.csv \
        --volume=analysis-results \
        python -c "
import pandas as pd
import numpy as np
data = pd.read_csv('data.csv')
print(f'Dataset shape: {data.shape}')
print(data.describe())
data.to_json('/volumes/analysis-results/summary.json')
"
```

#### Modern Python Features

```bash
# Using Python 3.12 modern syntax
rnx job run --runtime=python-3.12 \
        --upload=modern_app.py \
        python modern_app.py
```

#### Web API Development

```bash
# Flask web application with external access
rnx job run --runtime=python-3.11-ml \
        --upload=api.py \
        --network=web \
        --max-memory=512 \
        python api.py
```

### Java Examples

#### Enterprise Application (Java 17)

```bash
# Compile and run Java application
rnx job run --runtime=java:17 \
        --upload=Application.java \
        --volume=maven-cache \
        bash -c "javac Application.java && java Application"
```

#### Modern Java with Virtual Threads (Java 21)

```bash
# High-concurrency application using Virtual Threads
rnx job run --runtime=java:21 \
        --upload=VirtualThreadApp.java \
        --max-memory=1024 \
        bash -c "javac VirtualThreadApp.java && java VirtualThreadApp"
```

#### Maven Project Build

```bash
# Build entire Maven project
rnx job run --runtime=java:17 \
        --upload-dir=spring-project \
        --volume=maven-cache \
        --max-memory=2048 \
        mvn clean package
```

#### Spring Boot Web Application

```bash
# Run Spring Boot application with external access
rnx job run --runtime=java:17 \
        --upload=application.jar \
        --upload=application.properties \
        --network=web \
        --volume=app-data \
        java -jar application.jar
```

## Advanced Runtime Scenarios

### Multi-Stage Processing Pipeline

#### Stage 1: Data Preparation (Python)

```bash
# Prepare and clean data
rnx job run --runtime=python-3.11-ml \
        --upload=raw_data.csv \
        --volume=pipeline-data \
        python -c "
import pandas as pd
data = pd.read_csv('raw_data.csv')
cleaned = data.dropna().reset_index(drop=True)
cleaned.to_csv('/volumes/pipeline-data/cleaned.csv', index=False)
print(f'Cleaned dataset: {len(cleaned)} rows')
"
```

#### Stage 2: Analysis (Python ML)

```bash
# Run machine learning analysis
rnx job run --runtime=python-3.11-ml \
        --volume=pipeline-data \
        --max-memory=2048 \
        python -c "
import pandas as pd
from sklearn.ensemble import RandomForestClassifier
import joblib

data = pd.read_csv('/volumes/pipeline-data/cleaned.csv')
# ... ML processing ...
joblib.dump(model, '/volumes/pipeline-data/model.pkl')
"
```

#### Stage 3: Report Generation (Java)

```bash
# Generate PDF reports
rnx job run --runtime=java:17 \
        --volume=pipeline-data \
        --upload=ReportGenerator.java \
        bash -c "
javac ReportGenerator.java
java ReportGenerator /volumes/pipeline-data/model.pkl
"
```

## Development Workflows

### Python Package Development

```bash
# Test package in isolated environment
rnx job run --runtime=python-3.12 \
        --upload-dir=my-package \
        --volume=dev-pip-cache \
        bash -c "
cd my-package
pip install -e .
python -m pytest tests/
python setup.py sdist
"
```

### Java Library Testing

```bash
# Multi-version compatibility testing
# Test on Java 17
rnx job run --runtime=java:17 \
        --upload-dir=java-library \
        --volume=test-results \
        bash -c "
cd java-library
mvn test
cp target/surefire-reports/* /volumes/test-results/java17-
"

# Test on Java 21
rnx job run --runtime=java:21 \
        --upload-dir=java-library \
        --volume=test-results \
        bash -c "
cd java-library
mvn test
cp target/surefire-reports/* /volumes/test-results/java21-
"
```

### Web Development Examples

#### Full-Stack Development Server

```bash
# Frontend build (Java-based)
rnx job run --runtime=java:17 \
        --upload-dir=frontend \
        --volume=frontend-dist \
        bash -c "
cd frontend
mvn clean package
cp -r target/* /volumes/frontend-dist/
"

# Backend API (Python)
rnx job run --runtime=python-3.11-ml \
        --upload-dir=backend \
        --volume=frontend-dist \
        --network=web \
        python -c "
from flask import Flask, send_from_directory
app = Flask(__name__)

@app.route('/')
def frontend():
    return send_from_directory('/volumes/frontend-dist', 'index.html')

@app.route('/api/data')
def api():
    return {'message': 'Hello from Python API'}

app.run(host='0.0.0.0', port=8080)
"
```

### Database Integration Examples

#### Python Database Analysis

```bash
# Connect to database and analyze
rnx job run --runtime=python-3.11-ml \
        --network=database \
        --volume=db-cache \
        python -c "
import pandas as pd
import psycopg2
from sqlalchemy import create_engine

engine = create_engine('postgresql://user:pass@db:5432/analytics')
data = pd.read_sql('SELECT * FROM sales', engine)
summary = data.groupby('region').sum()
summary.to_csv('/volumes/db-cache/region_summary.csv')
"
```

### Batch Processing Examples

#### Large File Processing

```bash
# Process large CSV files in chunks
rnx job run --runtime=python-3.11-ml \
        --upload=large_dataset.csv \
        --volume=processed-chunks \
        --max-memory=4096 \
        --max-cpu=4 \
        python -c "
import pandas as pd
import numpy as np

chunk_size = 10000
chunk_num = 0

for chunk in pd.read_csv('large_dataset.csv', chunksize=chunk_size):
    processed = chunk.apply(lambda x: x.str.upper() if x.dtype == 'object' else x)
    processed.to_csv(f'/volumes/processed-chunks/chunk_{chunk_num}.csv', index=False)
    chunk_num += 1
    print(f'Processed chunk {chunk_num}')
"
```

### Cross-Runtime Communication

#### Message Queue Processing

```bash
# Producer (Python)
rnx job run --runtime=python-3.11-ml \
        --network=message-queue \
        python -c "
import json
import time
import requests

for i in range(100):
    message = {'id': i, 'data': f'message {i}'}
    requests.post('http://queue:8080/messages', json=message)
    time.sleep(0.1)
"

# Consumer (Java)
rnx job run --runtime=java:17 \
        --network=message-queue \
        --upload=Consumer.java \
        java Consumer
```

## Performance Optimization Examples

### Memory-Optimized Processing

```bash
# Large dataset processing with memory constraints
rnx job run --runtime=python-3.11-ml \
        --upload=big_data.csv \
        --max-memory=1024 \
        --volume=temp-storage \
        python -c "
import pandas as pd
import gc

# Process in chunks to manage memory
def process_large_file(filename):
    chunk_iter = pd.read_csv(filename, chunksize=1000)
    results = []

    for chunk in chunk_iter:
        result = chunk.groupby('category').sum()
        results.append(result)

        # Force garbage collection
        gc.collect()

    final_result = pd.concat(results).groupby(level=0).sum()
    final_result.to_csv('/volumes/temp-storage/summary.csv')

process_large_file('big_data.csv')
"
```

### CPU-Intensive Processing

```bash
# Multi-core processing
rnx job run --runtime=python-3.11-ml \
        --upload=compute_task.py \
        --max-cpu=8 \
        --max-memory=4096 \
        python -c "
from multiprocessing import Pool
import numpy as np

def cpu_intensive_task(data_chunk):
    # Simulate heavy computation
    return np.fft.fft(data_chunk).sum()

if __name__ == '__main__':
    # Generate test data
    data = np.random.random(1000000)
    chunks = np.array_split(data, 8)

    with Pool(processes=8) as pool:
        results = pool.map(cpu_intensive_task, chunks)

    print(f'Final result: {sum(results)}')
"
```

## Testing and Validation Examples

### Runtime Compatibility Testing

```bash
# Test script across multiple Python versions
for runtime in python-3.11-ml python-3.12; do
    echo "Testing on $runtime"
    rnx job run --runtime=$runtime \
            --upload=compatibility_test.py \
            python compatibility_test.py
done
```

### Integration Testing

```bash
# Test complete application stack
# Database setup
rnx job run --runtime=python-3.11-ml \
        --network=test-net \
        --volume=test-db \
        python setup_test_db.py

# API testing
rnx job run --runtime=python-3.11-ml \
        --network=test-net \
        --upload=test_api.py \
        python test_api.py

# Frontend testing (Java-based)
rnx job run --runtime=java:17 \
        --network=test-net \
        --upload-dir=frontend-tests \
        bash -c "cd frontend-tests && mvn test"
```

## Migration Examples

### Legacy Python to Runtime

```bash
# Before (host-dependent)
python3 my_script.py

# After (runtime-isolated)
rnx job run --runtime=python-3.11-ml \
        --upload=my_script.py \
        python my_script.py
```

### Complex Application Migration

```bash
# Legacy complex deployment
# sudo apt install python3.11 python3-pip
# pip3 install pandas numpy
# python3 app.py

# New runtime-based deployment
rnx job run --runtime=python-3.11-ml \
        --upload=app.py \
        --upload=requirements.txt \
        --volume=app-data \
        --network=web \
        python app.py
```

## Best Practices Examples

### Resource Management

```bash
# Appropriate resource allocation
rnx job run --runtime=python-3.11-ml \
        --max-memory=2048 \      # 2GB for ML workloads
        --max-cpu=4 \            # 4 cores for parallel processing
        --max-runtime=3600 \     # 1 hour timeout
        --upload=ml_training.py \
        python ml_training.py
```

### Volume Usage

```bash
# Persistent data and cache management
rnx job run --runtime=java:17 \
        --volume=maven-cache \     # Persistent Maven cache
        --volume=project-data \    # Persistent project data
        --upload-dir=java-app \
        bash -c "
cd java-app
mvn install  # Uses cached dependencies
mvn exec:java
"
```

### Network Isolation

```bash
# Secure network access patterns
rnx job run --runtime=python-3.11-ml \
        --network=database \     # Access only to database
        --upload=data_processor.py \
        python data_processor.py

# Web service with controlled access
rnx job run --runtime=java:17 \
        --network=web \          # External web access
        --upload=ApiServer.java \
        java ApiServer
```

---

## See Also

- [RUNTIME_SYSTEM.md](./RUNTIME_SYSTEM.md) - User guide for runtime system
- [RUNTIME_REGISTRY_GUIDE.md](./RUNTIME_REGISTRY_GUIDE.md) - Registry usage and custom registries
- [RUNTIME_ADVANCED.md](./RUNTIME_ADVANCED.md) - Advanced implementation details
- [RNX_CLI_REFERENCE.md](./RNX_CLI_REFERENCE.md) - Complete CLI reference
