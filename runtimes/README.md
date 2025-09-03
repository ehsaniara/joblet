# Joblet Runtime Environments

This directory contains runtime environments that provide pre-installed languages, tools, and services for Joblet jobs.
Runtimes eliminate the need to install dependencies on every job run, significantly improving performance.

## 📋 Table of Contents

- [Available Runtimes](#available-runtimes)
- [Using Runtimes](#using-runtimes)
- [GitHub Repository Installation](#github-repository-installation)
- [Creating New Runtimes](#creating-new-runtimes)
- [Packaging Runtimes for GitHub](#packaging-runtimes-for-github)
- [Runtime Manifest System Architecture](#runtime-manifest-system-architecture)
- [Best Practices](#best-practices)
- [Contributing](#contributing)
- [Support](#support)

## Available Runtimes

| Runtime          | Description                                 | Platforms                                |
|------------------|---------------------------------------------|------------------------------------------|
| `openjdk-21`     | OpenJDK 21 with Java development tools      | Ubuntu, RHEL, Amazon Linux (AMD64/ARM64) |
| `python-3.11-ml` | Python 3.11 with machine learning libraries | Ubuntu, RHEL, Amazon Linux (AMD64/ARM64) |
| `graalvmjdk-21`  | GraalVM JDK 21 with native-image support    | Ubuntu, RHEL, Amazon Linux (AMD64/ARM64) |

## Using Runtimes

### Local Installation

Install a runtime from your local codebase:

```bash
rnx runtime install openjdk-21
```

### GitHub Repository Installation with Manifest System

The new manifest-based system provides rich metadata and platform validation:

```bash
# List available runtimes with detailed metadata
rnx runtime list --github-repo=owner/repo/tree/main/runtimes

# Install with automatic platform compatibility checking
rnx runtime install openjdk-21 --github-repo=owner/repo/tree/main/runtimes

# JSON output for programmatic use
rnx runtime list --github-repo=owner/repo/tree/main/runtimes --json
```

### Enhanced Runtime Information

Get detailed information about runtimes:

```bash
# List locally installed runtimes
rnx runtime list

# List runtimes from GitHub with platform and version info
rnx runtime list --github-repo=owner/repo/tree/main/runtimes

# Get detailed runtime information
rnx runtime info openjdk-21
```

### Remove a Runtime

```bash
rnx runtime remove openjdk-21
```

### Manifest-Based Features

The new system provides:

- **Pre-installation visibility**: See runtime details before downloading
- **Platform validation**: Automatic compatibility checking
- **Resource requirements**: RAM, disk, and GPU requirements displayed
- **Rich metadata**: Version info, supported platforms, provided executables

## GitHub Repository Installation

### How It Works

When using the `--github-repo` flag, RNX downloads **pre-packaged runtime archives** directly from GitHub. This is much
more efficient than cloning entire repositories.

**Important:** GitHub installations require pre-packaged runtime archives (`.tar.gz` or `.zip` files) to be present in
the repository. The system will NOT download entire repositories.

### Requirements for GitHub Repositories

To make your runtimes available via GitHub, you must:

1. **Package each runtime** as a `.tar.gz` or `.zip` file
2. **Commit and push** these archives to your GitHub repository
3. **Use the correct naming**: `<runtime-name>.tar.gz` or `<runtime-name>.zip`

Example structure:

```
runtimes/
├── openjdk-21/           # Source files
│   ├── setup.sh
│   └── setup-*.sh
├── openjdk-21.tar.gz    # Pre-packaged archive (REQUIRED for GitHub)
├── python-3.11-ml/       # Source files
│   ├── setup.sh
│   └── setup-*.sh
├── python-3.11-ml.tar.gz # Pre-packaged archive (REQUIRED for GitHub)
└── README.md
```

### Error Messages

If a runtime is not found, users will see a clear error message:

```
❌ ERROR: Runtime 'nodejs-20' not found in repository

The runtime 'nodejs-20' is not available as a pre-packaged archive in:
  Repository: owner/repo
  Branch: main
  Path: runtimes

Expected to find one of these files:
  • runtimes/nodejs-20.tar.gz
  • runtimes/nodejs-20.zip
```

## Creating New Runtimes

### Runtime Structure

Each runtime must have:

1. A main `setup.sh` script that detects the platform
2. Platform-specific setup scripts (e.g., `setup-ubuntu-amd64.sh`)
3. A self-contained installation that doesn't depend on host system packages

### Example Runtime Directory

```
openjdk-21/
├── setup.sh              # Main entry point
├── setup-ubuntu-amd64.sh # Ubuntu AMD64 specific
├── setup-ubuntu-arm64.sh # Ubuntu ARM64 specific  
├── setup-rhel-amd64.sh   # RHEL/CentOS AMD64 specific
├── setup-rhel-arm64.sh   # RHEL/CentOS ARM64 specific
├── setup-amzn-amd64.sh   # Amazon Linux AMD64 specific
└── setup-amzn-arm64.sh   # Amazon Linux ARM64 specific
```

### Main Setup Script Template

```bash
#!/bin/bash
# Main setup script for runtime

set -e

# Detect OS and architecture
OS_ID=$(grep "^ID=" /etc/os-release | cut -d= -f2 | tr -d '"')
ARCH=$(uname -m)

# Map architecture names
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
esac

# Determine setup script
SETUP_SCRIPT="setup-${OS_ID}-${ARCH}.sh"

# Execute platform-specific setup
if [ -f "$SETUP_SCRIPT" ]; then
    chmod +x "$SETUP_SCRIPT"
    exec "./$SETUP_SCRIPT"
else
    echo "ERROR: Unsupported platform: ${OS_ID}-${ARCH}"
    exit 1
fi
```

## Packaging Runtimes for GitHub

### Automated Packaging with Manifest Updates

The **recommended approach** is to use the single build script that handles everything:

```bash
cd /home/jay/joblet/runtimes
./build-runtimes
```

This comprehensive script:

- 🔍 **Auto-discovers runtimes** by scanning for directories with `setup.sh`
- 📦 **Packages all runtimes** as `.tar.gz` files
- 📝 **Auto-generates runtime-manifest.json** with current metadata
- ✅ **Validates JSON structure** for correctness
- 📊 **Updates archive sizes** and timestamps automatically
- 🎯 **Provides clear next steps** for git operations

### Alternative: Individual Runtime Packaging

For packaging individual runtimes only (without manifest updates):

```bash
./package-runtime.sh openjdk-21   # Individual runtime packaging
```

**Note**: After individual packaging, run `./build-runtimes` to update the manifest.

### Runtime Manifest System

The manifest system provides **rich metadata** before download:

- **Platform compatibility checking** - Validates support before installation
- **Resource requirements** - RAM, disk space, GPU requirements
- **Detailed runtime info** - Executables, libraries, environment variables
- **Usage examples** - Documentation and command examples

### Publishing to GitHub

After packaging (automatically includes manifest updates):

```bash
git add runtimes/*.tar.gz runtimes/runtime-manifest.json
git commit -m "Update pre-packaged runtimes and manifest"
git push
```

### Manifest-Based Installation

Users can now see available runtimes before installing:

```bash
# List available runtimes from GitHub repository
rnx runtime list --github-repo=owner/repo/tree/main/runtimes

# Install with automatic platform validation
rnx runtime install openjdk-21 --github-repo=owner/repo/tree/main/runtimes
```

### Important Notes

1. **Always use `./build-runtimes`** for complete workflow automation
2. **Manifest is auto-maintained** - no manual JSON editing required
3. **Platform compatibility** is automatically validated during installation
4. **Rich metadata** enables better user experience with detailed runtime info
5. **Archive sizes** are automatically detected and updated in manifest
6. **JSON validation** ensures manifest correctness before commits

## Runtime Manifest System Architecture

### Manifest Schema

The runtime manifest is a JSON file that contains metadata about all available runtimes in a repository. It enables RNX
to:

- **Validate platform compatibility** before downloading
- **Display rich metadata** to users
- **Check resource requirements**
- **Provide usage examples and documentation**

#### Top-Level Structure

```json
{
  "version": "1.0.0",
  "generated": "2025-08-31T15:10:00Z",
  "repository": "joblet/joblet",
  "base_url": "https://github.com/joblet/joblet/raw/main/runtimes",
  "runtimes": {
    /* Runtime definitions */
  }
}
```

**Top-Level Fields:**

| Field        | Type   | Description                                    |
|--------------|--------|------------------------------------------------|
| `version`    | string | Manifest format version (currently "1.0.0")    |
| `generated`  | string | ISO 8601 timestamp when manifest was generated |
| `repository` | string | GitHub repository in "owner/repo" format       |
| `base_url`   | string | Base URL for downloading runtime archives      |
| `runtimes`   | object | Map of runtime name to runtime definition      |

#### Runtime Definition

Each runtime in the `runtimes` object has this structure:

```json
"runtime-name": {
  "name": "runtime-name",
  "display_name": "Human Readable Name", 
  "version": "1.0.0",
  "description": "Brief description of the runtime",
  "category": "language-runtime",
  "language": "java|python|javascript|etc",
  "platforms": {
    /* Platform support */
  },
  "requirements": {/* Resource requirements */},
  "provides": {/* What the runtime provides */},
  "documentation": {/* Usage documentation */},
  "tags": ["tag1", "tag2"]
}
```

**Runtime Fields:**

| Field           | Type   | Required | Description                                             |
|-----------------|--------|----------|---------------------------------------------------------|
| `name`          | string | ✅        | Runtime identifier (must match object key)              |
| `display_name`  | string | ✅        | Human-readable name for UI display                      |
| `version`       | string | ✅        | Runtime version (semantic versioning recommended)       |
| `description`   | string | ✅        | Brief description of the runtime and its purpose        |
| `category`      | string | ✅        | Runtime category (e.g., "language-runtime", "database") |
| `language`      | string | ✅        | Primary programming language or "unknown"               |
| `platforms`     | object | ✅        | Platform compatibility definitions                      |
| `requirements`  | object | ✅        | Resource requirements                                   |
| `provides`      | object | ✅        | Executables, libraries, and environment provided        |
| `documentation` | object | ✅        | Usage examples and documentation                        |
| `tags`          | array  | ✅        | Searchable tags for categorization                      |

#### Platform Support

The `platforms` object defines which operating systems and architectures are supported:

```json
"platforms": {
  "ubuntu-amd64": {
    "supported": true,
    "archive_url": "runtime-name.tar.gz",
    "archive_size": 12345,
    "checksum": "sha256:abcd1234..."
  },
  "ubuntu-arm64": { /* ... */ },
  "rhel-amd64": { /* ... */ },
  "rhel-arm64": { /* ... */ },
  "amzn-amd64": { /* ... */ },
  "amzn-arm64": { /* ... */ }
}
```

**Platform Keys:**

- `ubuntu-amd64` - Ubuntu/Debian on x86_64
- `ubuntu-arm64` - Ubuntu/Debian on ARM64
- `rhel-amd64` - RHEL/CentOS/Rocky on x86_64
- `rhel-arm64` - RHEL/CentOS/Rocky on ARM64
- `amzn-amd64` - Amazon Linux on x86_64
- `amzn-arm64` - Amazon Linux on ARM64

**Platform Fields:**

| Field          | Type    | Required | Description                                      |
|----------------|---------|----------|--------------------------------------------------|
| `supported`    | boolean | ✅        | Whether this platform is supported               |
| `archive_url`  | string  | ✅        | Filename of the runtime archive                  |
| `archive_size` | number  | ✅        | Size of the archive in bytes                     |
| `checksum`     | string  | ✅        | SHA256 checksum (can be "sha256:auto-generated") |

#### Requirements

Resource requirements for the runtime:

```json
"requirements": {
  "min_ram_mb": 512,
  "min_disk_mb": 100,
  "gpu_required": false
}
```

| Field          | Type    | Description                              |
|----------------|---------|------------------------------------------|
| `min_ram_mb`   | number  | Minimum RAM required in megabytes        |
| `min_disk_mb`  | number  | Minimum disk space required in megabytes |
| `gpu_required` | boolean | Whether GPU access is required           |

#### Provides

What the runtime provides to jobs:

```json
"provides": {
  "executables": ["java", "javac", "jar"],
  "libraries": ["numpy", "pandas"],
  "environment_vars": {
    "JAVA_HOME": "/usr/lib/jvm/java-21-openjdk",
    "PATH": "/usr/lib/jvm/java-21-openjdk/bin:$PATH"
  }
}
```

| Field              | Type   | Description                                     |
|--------------------|--------|-------------------------------------------------|
| `executables`      | array  | List of executable commands provided            |
| `libraries`        | array  | List of libraries/packages available (optional) |
| `environment_vars` | object | Environment variables set by the runtime        |

#### Documentation

Usage information and examples:

```json
"documentation": {
  "usage": "Use with: rnx run --runtime=openjdk-21 java MyApp.java",
  "examples": [
    "rnx run --runtime=openjdk-21 java -version",
    "rnx run --runtime=openjdk-21 javac HelloWorld.java && java HelloWorld"
  ]
}
```

| Field      | Type   | Description              |
|------------|--------|--------------------------|
| `usage`    | string | Brief usage instructions |
| `examples` | array  | List of example commands |

#### Tags

Searchable tags for runtime discovery:

```json
"tags": ["java", "jdk", "openjdk", "development", "compilation"]
```

Tags should be:

- **Lowercase** for consistency
- **Descriptive** of the runtime's purpose
- **Include language names**, framework names, use cases
- **Avoid duplicating** information already in other fields

#### Complete Example Runtime

```json
"openjdk-21": {
  "name": "openjdk-21",
  "display_name": "OpenJDK 21",
  "version": "21.0.1",
  "description": "OpenJDK 21 with Java development tools and JVM runtime",
  "category": "language-runtime",
  "language": "java",
  "platforms": {
    "ubuntu-amd64": {
      "supported": true,
      "archive_url": "openjdk-21.tar.gz", 
      "archive_size": 5928,
      "checksum": "sha256:auto-generated"
    }
  },
  "requirements": {
    "min_ram_mb": 512,
    "min_disk_mb": 100,
    "gpu_required": false
  },
  "provides": {
    "executables": ["java", "javac", "jar", "jarsigner"],
    "environment_vars": {
      "JAVA_HOME": "/usr/lib/jvm/java-21-openjdk",
      "PATH": "/usr/lib/jvm/java-21-openjdk/bin:$PATH"
    }
  },
  "documentation": {
    "usage": "Use with: rnx run --runtime=openjdk-21 java MyApp.java",
    "examples": [
      "rnx run --runtime=openjdk-21 java -version",
      "rnx run --runtime=openjdk-21 javac HelloWorld.java && java HelloWorld"
    ]
  },
  "tags": ["java", "jdk", "openjdk", "development", "compilation"]
}
```

#### Manifest Generation and Validation

The manifest is automatically generated by `./build-runtimes`. Manual editing is not recommended as it will be
overwritten.

The manifest should always be valid JSON. You can validate it using:

```bash
python3 -m json.tool runtime-manifest.json > /dev/null
```

### Installation Process with Manifest System

1. **Manifest Download**: RNX downloads `runtime-manifest.json` from GitHub
2. **Runtime Validation**: Checks if requested runtime exists in manifest
3. **Platform Detection**: Detects local OS and architecture (ubuntu-amd64, rhel-arm64, etc.)
4. **Compatibility Check**: Validates runtime supports the detected platform
5. **Resource Verification**: Checks minimum RAM/disk requirements
6. **Archive Download**: Downloads the specific runtime archive from manifest URL
7. **Platform-Specific Setup**: Executes the appropriate setup script
8. **Isolated Installation**: Installs everything under `/opt/joblet/runtimes/<runtime-name>`
9. **Configuration Generation**: Creates `runtime.yml` with mount points and environment variables

### Isolation Principles

Runtimes follow these principles:

- **Self-contained**: All dependencies included, no reliance on host packages
- **Isolated**: Installed in dedicated directory structure
- **Portable**: Can be moved between compatible systems
- **Secure**: Read-only mounts except where write access is required

### Runtime Configuration (runtime.yml)

Each runtime generates a `runtime.yml` that specifies:

- Mount points for binaries, libraries, and configuration
- Environment variables (PATH, LD_LIBRARY_PATH, etc.)
- Runtime metadata (name, version, description)

Example:

```yaml
name: openjdk-21
version: "21.0.1"
description: "OpenJDK 21 Runtime"

mounts:
  - source: "isolated/usr/bin"
    target: "/usr/bin"
    readonly: true
  - source: "isolated/usr/lib/jvm"
    target: "/usr/lib/jvm"
    readonly: true

environment:
  JAVA_HOME: "/usr/lib/jvm/java-21-openjdk"
  PATH: "/usr/lib/jvm/java-21-openjdk/bin:/usr/bin:/bin"
```

## Best Practices

### Runtime Development

1. **Version Control**: Tag releases when updating runtimes
2. **Testing**: Test on all supported platforms before release
3. **Documentation**: Document any special requirements or limitations
4. **Security**: Never include secrets or sensitive data in runtimes
5. **Efficiency**: Package only necessary files to minimize download size
6. **Compatibility**: Ensure backward compatibility when updating

### Manifest System

1. **Always use `./build-runtimes`** for packaging to ensure manifest consistency
2. **Validate locally** before committing: `python3 -m json.tool runtime-manifest.json`
3. **Test both installation methods**: local and GitHub-based installations
4. **Update platform support** when adding new OS/architecture combinations
5. **Keep resource requirements accurate** for optimal user experience
6. **Use descriptive tags** to help users discover relevant runtimes

### Troubleshooting

#### Manifest Not Found Error

```
❌ ERROR: Could not download runtime manifest
```

**Solutions:**

1. Ensure `runtime-manifest.json` exists in your repository
2. Verify the repository URL format: `owner/repo/tree/branch/path`
3. Check if the file is committed and pushed to GitHub

#### Platform Not Supported Error

```
❌ ERROR: Runtime 'openjdk-21' does not support platform 'ubuntu-arm64'
```

**Solutions:**

1. Update the manifest to include the missing platform
2. Create platform-specific setup scripts if needed
3. Run `./build-runtimes.py` to regenerate with updated platform support

#### Archive Size Mismatch

If archive sizes are incorrect in the manifest:

```bash
# Rebuild everything (recommended)
./build-runtimes
```

#### JSON Validation Errors

```bash
# Validate JSON structure
python3 -m json.tool runtime-manifest.json

# Fix and regenerate if needed
./build-runtimes
```

## Contributing

To contribute a new runtime:

1. Create the runtime directory structure
2. Implement platform-specific setup scripts
3. Test on all target platforms
4. Package the runtime using `package-runtime.sh`
5. Submit a pull request with both source files and packaged archive

## Support

For issues or questions about runtimes:

- Check the [Joblet documentation](https://github.com/joblet/joblet)
- Open an issue in the repository
- Contact the maintainers

---

*Last updated: 2025*