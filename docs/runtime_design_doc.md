# Runtime System Design Document

## Overview

The Joblet runtime system provides isolated, version-specific execution environments for different programming languages and frameworks. This system allows jobs to specify their runtime requirements and execute within completely isolated environments without contaminating the host system.

## Key Design Principles

### 1. Complete Host Isolation
- Runtime environments exist only in `/opt/joblet/runtimes/`
- Host system remains completely clean and uncontaminated
- Build dependencies are automatically removed after runtime installation
- No runtime packages installed on host system

### 2. Version-Specific Support
- Support multiple versions of the same language (e.g., `python:3.11+ml`, `python:3.12`)
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
    selective: ["python", "python3", "pip"]
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
- `python:3.11+ml` - Python 3.11 with NumPy, Pandas, Scikit-learn
- `python:3.12` - Python 3.12 with modern features
- `java:17` - OpenJDK 17 LTS with Maven
- `java:21` - OpenJDK 21 with Virtual Threads
- `node:18` - Node.js 18 LTS with npm

## Implementation Details

### Runtime Resolution
1. Job specifies runtime via `--runtime=python:3.11+ml`
2. Runtime manager resolves to `/opt/joblet/runtimes/python/python-3.11-ml/`
3. Configuration loaded from `runtime.yml`
4. Mount points prepared for job container

### Job Execution Flow
1. **Pre-execution**: Runtime mounts prepared
2. **Container Setup**: Runtime binaries/libraries mounted into job container
3. **Environment Setup**: Runtime environment variables applied
4. **Execution**: Job runs with access to runtime tools
5. **Cleanup**: Runtime mounts cleaned up

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
The `deploy_runtimes.sh` script provides:
- Command-line options for selective installation
- Pre-installation checks (disk space, network)
- Automatic dependency removal
- Configuration integration with Joblet

### Installation Options
```bash
sudo ./deploy_runtimes.sh --all                    # All runtimes
sudo ./deploy_runtimes.sh --python-3.11-ml        # Specific runtime
sudo ./deploy_runtimes.sh --python-all --node-18  # Multiple selections
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