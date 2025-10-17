# GitHub Runtime Repository Guide

Complete guide for installing runtimes from GitHub repositories and creating your own runtime repositories.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [How GitHub Installation Works](#how-github-installation-works)
- [Repository Structure](#repository-structure)
- [Runtime Manifest Schema](#runtime-manifest-schema)
- [Creating Your Runtime Repository](#creating-your-runtime-repository)
- [Testing Your Repository](#testing-your-repository)
- [Advanced Configuration](#advanced-configuration)
- [Troubleshooting](#troubleshooting)

## Overview

Joblet supports installing runtimes directly from GitHub repositories, enabling:

- **Centralized runtime distribution** - Host runtimes in version-controlled repositories
- **Custom runtime ecosystems** - Create organization-specific runtime collections
- **Easy versioning** - Use Git branches/tags for runtime versions
- **Secure installation** - Runtimes installed in isolated chroot environments
- **No local build required** - Download pre-packaged runtimes directly

## Quick Start

### Installing from GitHub

```bash
# List available runtimes from a GitHub repository
rnx runtime list --github-repo=ehsaniara/joblet/tree/main/runtimes

# Install a specific runtime
rnx runtime install openjdk-21 --github-repo=ehsaniara/joblet/tree/main/runtimes

# Install from different branch
rnx runtime install python-3.11-ml --github-repo=owner/repo/tree/develop/runtimes
```

### Supported GitHub URL Formats

```bash
# Full path format
owner/repo/tree/branch/path

# Short format (uses default branch)
owner/repo

# Path without branch (uses main/master)
owner/repo/path
```

**Examples:**
```bash
# Full specification
ehsaniara/joblet/tree/main/runtimes

# Short format (looks for runtimes/ in default branch)
ehsaniara/joblet

# Custom path
myorg/custom-runtimes/tree/production/runtime-definitions
```

## How GitHub Installation Works

Understanding the installation flow helps with debugging and creating custom runtimes.

### Installation Flow Diagram

```
┌─────────────────┐
│ User Command    │
│ rnx runtime     │
│ install         │
└────────┬────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 1. Parse GitHub URL                 │
│    - Extract owner/repo/branch/path │
│    - Validate format                │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 2. Fetch Manifest                   │
│    - Download runtime-manifest.json │
│    - Validate runtime exists        │
│    - Check platform compatibility   │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 3. Download ZIP Archive             │
│    - URL: github.com/{repo}/archive │
│           /refs/heads/{branch}.zip  │
│    - Stream to temporary directory  │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 4. Extract Archive                  │
│    - Unzip to temp directory        │
│    - Locate runtime directory       │
│    - Find setup script              │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 5. Create Chroot Environment        │
│    - Temporary isolated filesystem  │
│    - Mount necessary directories:   │
│      • /proc, /sys, /dev            │
│      • /etc/resolv.conf (DNS)       │
│    - Copy runtime files             │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 6. Execute Setup Script             │
│    - Run setup.sh in chroot         │
│    - Platform detection:            │
│      • Ubuntu: setup-ubuntu-*.sh    │
│      • RHEL: setup-rhel-*.sh        │
│      • Amazon: setup-amzn-*.sh      │
│    - Install dependencies           │
│    - Configure environment          │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 7. Copy Runtime to Host             │
│    - Move from chroot to:           │
│      /opt/joblet/runtimes/{name}    │
│    - Verify runtime.yml exists      │
│    - Set permissions                │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 8. Cleanup & Verification           │
│    - Remove chroot environment      │
│    - Unmount filesystems            │
│    - Verify installation success    │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────┐
│ Runtime Ready   │
└─────────────────┘
```

### Security Isolation

**Chroot Environment:**
- Each installation runs in a temporary chroot jail
- Isolated from host system
- Restricted filesystem access
- Automatic cleanup after installation

**Mount Strategy:**
```
/tmp/joblet-runtime-build-{uuid}/
├── proc/       (mounted read-only)
├── sys/        (mounted read-only)
├── dev/        (mounted with necessary devices)
├── etc/
│   └── resolv.conf (bind-mounted for DNS)
└── runtime/    (writable - runtime files)
```

**Why Chroot?**
1. **Security** - Prevents malicious scripts from accessing host
2. **Isolation** - No contamination of host system packages
3. **Reproducibility** - Consistent environment across installations
4. **Cleanup** - Easy to remove if installation fails

## Repository Structure

### Basic Structure

```
my-runtime-repo/
├── README.md
├── runtime-manifest.json          # Required: Lists all available runtimes
└── runtimes/                      # Recommended: Contains runtime directories
    ├── python-3.11-ml/
    │   ├── setup.sh               # Required: Main setup entry point
    │   ├── setup-ubuntu-amd64.sh  # Platform-specific setup
    │   ├── setup-ubuntu-arm64.sh
    │   ├── setup-rhel-amd64.sh
    │   ├── setup-amzn-amd64.sh
    │   └── runtime.yml            # Optional: Pre-created metadata
    │
    ├── openjdk-21/
    │   ├── setup.sh
    │   ├── setup-ubuntu-amd64.sh
    │   └── ...
    │
    └── custom-runtime/
        └── ...
```

### Manifest File Location

The `runtime-manifest.json` must be at the root of the specified path:

```bash
# If using: --github-repo=owner/repo/tree/main/runtimes
# Manifest must be at: runtimes/runtime-manifest.json

# If using: --github-repo=owner/repo
# Manifest must be at: runtime-manifest.json (repo root)
```

### Setup Script Requirements

**`setup.sh` (Required)**

Main entry point that delegates to platform-specific scripts:

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Detect platform
if [ -f /etc/os-release ]; then
    . /etc/os-release
    PLATFORM_ID="${ID}"
else
    echo "Cannot detect platform"
    exit 1
fi

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) PLATFORM_ARCH="amd64" ;;
    aarch64) PLATFORM_ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Execute platform-specific script
PLATFORM_SCRIPT="setup-${PLATFORM_ID}-${PLATFORM_ARCH}.sh"
if [ -f "$SCRIPT_DIR/$PLATFORM_SCRIPT" ]; then
    echo "Running platform-specific setup: $PLATFORM_SCRIPT"
    bash "$SCRIPT_DIR/$PLATFORM_SCRIPT"
else
    echo "No platform-specific script found: $PLATFORM_SCRIPT"
    exit 1
fi
```

**Platform-Specific Scripts (Required)**

Example: `setup-ubuntu-amd64.sh`

```bash
#!/bin/bash
set -e

echo "Installing Python 3.11 with ML libraries for Ubuntu AMD64..."

# Install system dependencies
apt-get update
apt-get install -y python3.11 python3.11-venv python3-pip

# Create virtual environment
python3.11 -m venv /opt/runtime/python-3.11-ml

# Activate and install packages
source /opt/runtime/python-3.11-ml/bin/activate
pip install --upgrade pip
pip install numpy==2.2.6 pandas==2.3.2 scikit-learn matplotlib scipy

# Create runtime.yml
cat > /opt/runtime/python-3.11-ml/runtime.yml <<EOF
name: python-3.11-ml
version: 3.11.0
language: python
environment_paths:
  - /opt/runtime/python-3.11-ml/bin
environment_vars:
  VIRTUAL_ENV: /opt/runtime/python-3.11-ml
  PATH: /opt/runtime/python-3.11-ml/bin:\$PATH
EOF

echo "Python 3.11 ML runtime installation complete"
```

## Runtime Manifest Schema

The `runtime-manifest.json` file describes all available runtimes in your repository.

### Complete Schema

```json
{
  "version": "1.0.0",                    // Manifest schema version (required)
  "generated": "2025-09-14T01:30:00Z",   // ISO 8601 timestamp (optional)
  "repository": "owner/repo",            // GitHub repository (optional)
  "base_url": "https://...",             // Base URL for archives (optional)
  "runtimes": {                          // Runtime definitions (required)
    "runtime-name": {
      // Runtime specification object
    }
  }
}
```

### Runtime Specification Object

```json
{
  "name": "python-3.11-ml",              // Runtime identifier (required)
  "display_name": "Python 3.11 ML",      // Human-readable name (required)
  "version": "3.11.0",                   // Runtime version (required)
  "description": "Python 3.11 with ML libraries", // Description (required)
  "category": "language-runtime",        // Category (required)
  "language": "python",                  // Primary language (required)

  // Archive information (optional - for pre-packaged runtimes)
  "archive_url": "python-3.11-ml.tar.gz",
  "archive_size": 7700,                  // Size in KB
  "checksum": "sha256:abc123...",        // SHA256 checksum

  // Platform support (required)
  "platforms": [
    "ubuntu-amd64",
    "ubuntu-arm64",
    "rhel-amd64",
    "amzn-amd64"
  ],

  // System requirements (required)
  "requirements": {
    "min_ram_mb": 512,                   // Minimum RAM
    "min_disk_mb": 100,                  // Minimum disk space
    "gpu_required": false                // GPU requirement
  },

  // Runtime capabilities (optional but recommended)
  "provides": {
    "executables": [                     // Available executables
      "python",
      "pip",
      "python3"
    ],
    "environment_vars": {                // Environment variables
      "VIRTUAL_ENV": "/opt/runtime/python-3.11-ml",
      "PATH": "/opt/runtime/python-3.11-ml/bin:$PATH"
    },
    "packages": [                        // Installed packages
      "numpy==2.2.6",
      "pandas==2.3.2"
    ]
  },

  // Documentation (optional but recommended)
  "documentation": {
    "usage": "Use with: rnx job run --runtime=python-3.11-ml python script.py",
    "examples": [
      "rnx job run --runtime=python-3.11-ml python -c 'import numpy; print(numpy.__version__)'",
      "rnx job run --runtime=python-3.11-ml python analyze.py"
    ],
    "url": "https://docs.example.com/python-ml"
  },

  // Metadata (optional)
  "tags": ["python", "machine-learning", "data-science"],
  "maintainer": "team@company.com",
  "license": "MIT",
  "homepage": "https://github.com/owner/repo"
}
```

### Minimal Valid Manifest

```json
{
  "version": "1.0.0",
  "runtimes": {
    "my-runtime": {
      "name": "my-runtime",
      "display_name": "My Custom Runtime",
      "version": "1.0.0",
      "description": "A custom runtime",
      "category": "language-runtime",
      "language": "custom",
      "platforms": [
        "ubuntu-amd64"
      ],
      "requirements": {
        "min_ram_mb": 256,
        "min_disk_mb": 50,
        "gpu_required": false
      }
    }
  }
}
```

### Platform Identifiers

Supported platform identifiers follow the format: `{os}-{arch}`

**Operating Systems:**
- `ubuntu` - Ubuntu/Debian
- `rhel` - RHEL/CentOS/Rocky Linux
- `amzn` - Amazon Linux

**Architectures:**
- `amd64` - x86_64 (Intel/AMD 64-bit)
- `arm64` - ARM 64-bit (aarch64)

**Examples:**
- `ubuntu-amd64` - Ubuntu on x86_64
- `ubuntu-arm64` - Ubuntu on ARM64
- `rhel-amd64` - RHEL on x86_64
- `amzn-arm64` - Amazon Linux on ARM64

### Categories

Standard categories for organizing runtimes:

- `language-runtime` - Programming language environments (Python, Java, Node.js)
- `ml-framework` - Machine learning frameworks (TensorFlow, PyTorch)
- `database` - Database tools and clients
- `build-tools` - Build and compilation tools
- `custom` - Custom/specialized runtimes

## Creating Your Runtime Repository

### Step 1: Initialize Repository

```bash
# Create new repository
mkdir my-runtimes && cd my-runtimes
git init

# Create basic structure
mkdir -p runtimes/my-first-runtime
```

### Step 2: Create Setup Scripts

**Create `runtimes/my-first-runtime/setup.sh`:**

```bash
#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Detect platform
. /etc/os-release
PLATFORM_ID="${ID}"
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) PLATFORM_ARCH="amd64" ;;
    aarch64) PLATFORM_ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Execute platform-specific script
PLATFORM_SCRIPT="setup-${PLATFORM_ID}-${PLATFORM_ARCH}.sh"
bash "$SCRIPT_DIR/$PLATFORM_SCRIPT"
```

**Create `runtimes/my-first-runtime/setup-ubuntu-amd64.sh`:**

```bash
#!/bin/bash
set -e

echo "Installing my-first-runtime..."

# Install dependencies
apt-get update
apt-get install -y build-essential curl

# Install runtime components
mkdir -p /opt/runtime/my-first-runtime
# ... your installation commands ...

# Create runtime.yml
cat > /opt/runtime/my-first-runtime/runtime.yml <<EOF
name: my-first-runtime
version: 1.0.0
language: custom
environment_paths:
  - /opt/runtime/my-first-runtime/bin
EOF

echo "Installation complete"
```

### Step 3: Create Manifest

**Create `runtime-manifest.json`:**

```json
{
  "version": "1.0.0",
  "repository": "myorg/my-runtimes",
  "runtimes": {
    "my-first-runtime": {
      "name": "my-first-runtime",
      "display_name": "My First Runtime",
      "version": "1.0.0",
      "description": "My custom runtime environment",
      "category": "custom",
      "language": "custom",
      "platforms": [
        "ubuntu-amd64",
        "ubuntu-arm64"
      ],
      "requirements": {
        "min_ram_mb": 256,
        "min_disk_mb": 100,
        "gpu_required": false
      },
      "provides": {
        "executables": ["my-tool"],
        "environment_vars": {
          "MY_RUNTIME_HOME": "/opt/runtime/my-first-runtime"
        }
      },
      "documentation": {
        "usage": "rnx job run --runtime=my-first-runtime my-tool",
        "examples": [
          "rnx job run --runtime=my-first-runtime my-tool --version"
        ]
      }
    }
  }
}
```

### Step 4: Test Locally

Before pushing to GitHub, test your runtime locally:

```bash
# Make scripts executable
chmod +x runtimes/my-first-runtime/setup.sh
chmod +x runtimes/my-first-runtime/setup-*.sh

# Commit changes
git add .
git commit -m "Add my-first-runtime"
```

### Step 5: Push to GitHub

```bash
# Add remote
git remote add origin https://github.com/myorg/my-runtimes.git

# Push to main branch
git push -u origin main
```

### Step 6: Install from GitHub

```bash
# List runtimes from your repository
rnx runtime list --github-repo=myorg/my-runtimes

# Install your runtime
rnx runtime install my-first-runtime --github-repo=myorg/my-runtimes
```

## Testing Your Repository

### Local Testing Strategy

**1. Test Setup Scripts Locally**

```bash
# Test in Docker container (safe isolation)
docker run -it --rm -v $(pwd):/workspace ubuntu:22.04

# Inside container
cd /workspace/runtimes/my-first-runtime
bash setup.sh
```

**2. Validate Manifest**

```bash
# Check JSON syntax
jq . runtime-manifest.json

# Validate required fields
jq '.runtimes | to_entries | .[] | {name: .key, has_platforms: (.value.platforms != null), has_requirements: (.value.requirements != null)}' runtime-manifest.json
```

**3. Test Platform Detection**

```bash
# Test on different platforms
./runtimes/my-runtime/setup.sh

# Verify correct platform script is executed
```

### GitHub Testing

**1. Test Installation from GitHub**

```bash
# After pushing to GitHub, test installation
rnx runtime install my-runtime --github-repo=myorg/my-runtimes/tree/main

# Verify installation
rnx runtime list | grep my-runtime
rnx runtime info my-runtime
```

**2. Test Runtime Execution**

```bash
# Test basic execution
rnx job run --runtime=my-runtime echo "Hello from runtime"

# Test environment variables
rnx job run --runtime=my-runtime env | grep MY_RUNTIME
```

**3. Test Different Branches**

```bash
# Create development branch
git checkout -b develop
# ... make changes ...
git push origin develop

# Test from develop branch
rnx runtime install my-runtime --github-repo=myorg/my-runtimes/tree/develop
```

## Advanced Configuration

### Multi-Platform Support

**Strategy 1: Separate Scripts (Recommended)**

```
my-runtime/
├── setup.sh                    # Dispatcher
├── setup-ubuntu-amd64.sh       # Ubuntu x86_64
├── setup-ubuntu-arm64.sh       # Ubuntu ARM64
├── setup-rhel-amd64.sh         # RHEL x86_64
└── setup-amzn-amd64.sh         # Amazon Linux x86_64
```

**Strategy 2: Unified Script with Conditionals**

```bash
#!/bin/bash
set -e

. /etc/os-release
ARCH=$(uname -m)

if [ "$ID" = "ubuntu" ] && [ "$ARCH" = "x86_64" ]; then
    # Ubuntu AMD64 installation
    apt-get install -y package-amd64
elif [ "$ID" = "ubuntu" ] && [ "$ARCH" = "aarch64" ]; then
    # Ubuntu ARM64 installation
    apt-get install -y package-arm64
else
    echo "Unsupported platform: $ID-$ARCH"
    exit 1
fi
```

### Version Management

**Use Git Tags:**

```bash
# Tag runtime version
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin v1.0.0

# Install specific version
rnx runtime install my-runtime --github-repo=myorg/my-runtimes/tree/v1.0.0
```

**Semantic Versioning in Manifest:**

```json
{
  "runtimes": {
    "my-runtime": {
      "version": "1.0.0",
      "tags": ["v1", "v1.0", "v1.0.0", "latest"]
    }
  }
}
```

### Pre-packaged Archives

For faster installation, pre-build runtime archives:

```bash
# Build runtime locally
cd runtimes/my-runtime
bash setup.sh

# Package runtime
tar -czf my-runtime-ubuntu-amd64.tar.gz -C /opt/runtime my-runtime

# Upload to GitHub Releases
# Add archive_url to manifest
```

**Manifest with Archive:**

```json
{
  "runtimes": {
    "my-runtime": {
      "archive_url": "https://github.com/myorg/my-runtimes/releases/download/v1.0.0/my-runtime-ubuntu-amd64.tar.gz",
      "archive_size": 15360,
      "checksum": "sha256:abc123..."
    }
  }
}
```

## Troubleshooting

### Common Issues

**1. Runtime Not Found in Manifest**

```
Error: Runtime 'my-runtime' not found in manifest
```

**Solution:**
- Verify runtime name matches exactly in manifest
- Check manifest is at correct path in repository
- Ensure manifest JSON is valid

```bash
# Verify manifest
curl -L https://raw.githubusercontent.com/owner/repo/main/runtime-manifest.json | jq '.runtimes | keys'
```

**2. Platform Not Supported**

```
Error: Platform ubuntu-amd64 not supported for runtime 'my-runtime'
```

**Solution:**
- Add platform to `platforms` array in manifest
- Create platform-specific setup script

```bash
# Check your platform
. /etc/os-release
echo "Platform: ${ID}-$(uname -m)"

# Add to manifest
"platforms": ["ubuntu-amd64", "ubuntu-arm64"]
```

**3. Setup Script Not Found**

```
Error: No platform-specific script found: setup-ubuntu-amd64.sh
```

**Solution:**
- Verify setup script exists and has correct name
- Make script executable
- Check file is committed to repository

```bash
# Fix permissions locally
chmod +x runtimes/my-runtime/setup-*.sh

# Verify in repository
git ls-files runtimes/my-runtime/
```

**4. GitHub Download Failed**

```
Error: Failed to download repository archive
```

**Solution:**
- Check repository exists and is public
- Verify branch name is correct
- Check network connectivity

```bash
# Test GitHub access
curl -I https://github.com/owner/repo/archive/refs/heads/main.zip

# Check branch exists
git ls-remote https://github.com/owner/repo.git
```

**5. Chroot Setup Failed**

```
Error: Failed to create chroot environment
```

**Solution:**
- Verify sufficient disk space
- Check /tmp has execute permissions
- Ensure running with appropriate privileges

```bash
# Check disk space
df -h /tmp

# Check mount options
mount | grep /tmp
```

### Debug Mode

Enable verbose logging:

```bash
# Set debug environment variable
export JOBLET_DEBUG=1

# Install with debug output
rnx runtime install my-runtime --github-repo=owner/repo 2>&1 | tee install.log
```

### Manifest Validation Tool

Create a validation script:

```bash
#!/bin/bash
# validate-manifest.sh

MANIFEST="runtime-manifest.json"

echo "Validating $MANIFEST..."

# Check JSON syntax
if ! jq empty "$MANIFEST" 2>/dev/null; then
    echo "❌ Invalid JSON syntax"
    exit 1
fi

# Check required fields
REQUIRED_FIELDS=("version" "runtimes")
for field in "${REQUIRED_FIELDS[@]}"; do
    if ! jq -e ".$field" "$MANIFEST" >/dev/null; then
        echo "❌ Missing required field: $field"
        exit 1
    fi
done

# Validate each runtime
jq -r '.runtimes | keys[]' "$MANIFEST" | while read runtime; do
    echo "Checking runtime: $runtime"

    # Check required runtime fields
    RUNTIME_FIELDS=("name" "display_name" "version" "platforms" "requirements")
    for field in "${RUNTIME_FIELDS[@]}"; do
        if ! jq -e ".runtimes[\"$runtime\"].$field" "$MANIFEST" >/dev/null; then
            echo "  ❌ Missing field: $field"
        else
            echo "  ✅ $field"
        fi
    done
done

echo "✅ Manifest validation complete"
```

### Getting Help

**Check Logs:**

```bash
# View server logs
sudo journalctl -u joblet -f

# Check runtime installation logs
ls -la /opt/joblet/logs/runtime-install-*.log
```

**Community Support:**

- GitHub Issues: https://github.com/ehsaniara/joblet/issues
- Documentation: https://github.com/ehsaniara/joblet/tree/main/docs

## Best Practices

### Repository Organization

**✅ Good Structure:**
```
my-runtimes/
├── README.md
├── runtime-manifest.json
├── .github/
│   └── workflows/
│       └── validate-runtimes.yml
└── runtimes/
    ├── python-3.11/
    ├── nodejs-20/
    └── java-21/
```

**❌ Poor Structure:**
```
my-runtimes/
├── python.sh          # Not organized
├── node.sh
├── manifest.json      # Wrong name
└── scripts/           # Unclear purpose
```

### Manifest Maintenance

1. **Use semantic versioning** - `major.minor.patch`
2. **Update checksums** - When changing archives
3. **Document breaking changes** - In version descriptions
4. **Test before releasing** - Always validate changes
5. **Keep dependencies current** - Update package versions

### Setup Script Best Practices

```bash
#!/bin/bash
# ✅ Good practices

set -e                     # Exit on error
set -u                     # Exit on undefined variable
set -o pipefail            # Catch pipe failures

# Log everything
exec 1> >(tee -a /var/log/runtime-install.log)
exec 2>&1

# Validate prerequisites
if ! command -v apt-get &> /dev/null; then
    echo "Error: apt-get not found"
    exit 1
fi

# Use specific versions
pip install numpy==2.2.6   # ✅ Specific version
# pip install numpy         # ❌ Latest (unpredictable)

# Clean up on error
cleanup() {
    echo "Cleaning up..."
    rm -rf /tmp/build-*
}
trap cleanup EXIT
```

### Security Considerations

1. **Validate downloads** - Use checksums
2. **Minimize privileges** - Don't require root when possible
3. **Clean up secrets** - Remove temporary credentials
4. **Audit scripts** - Review all commands
5. **Use HTTPS** - Always use secure connections

### Performance Optimization

```bash
# ✅ Optimize package installation
apt-get install -y --no-install-recommends python3.11

# ✅ Clean up caches
apt-get clean
rm -rf /var/lib/apt/lists/*

# ✅ Parallel downloads
pip install package1 package2 &
pip install package3 package4 &
wait
```

## Examples

### Example 1: Python Data Science Runtime

Complete example: [runtimes/python-3.11-ml/](../runtimes/python-3.11-ml/)

### Example 2: Node.js Runtime

```bash
# setup-ubuntu-amd64.sh
#!/bin/bash
set -e

curl -fsSL https://deb.nodesource.com/setup_20.x | bash -
apt-get install -y nodejs

mkdir -p /opt/runtime/nodejs-20
npm config set prefix /opt/runtime/nodejs-20

# Install global packages
npm install -g typescript @types/node

cat > /opt/runtime/nodejs-20/runtime.yml <<EOF
name: nodejs-20
version: 20.10.0
language: javascript
environment_paths:
  - /opt/runtime/nodejs-20/bin
EOF
```

### Example 3: Java Runtime

```bash
# setup-ubuntu-amd64.sh
#!/bin/bash
set -e

apt-get update
apt-get install -y openjdk-21-jdk maven gradle

cat > /opt/runtime/openjdk-21/runtime.yml <<EOF
name: openjdk-21
version: 21.0.1
language: java
environment_vars:
  JAVA_HOME: /usr/lib/jvm/java-21-openjdk-amd64
  PATH: /usr/lib/jvm/java-21-openjdk-amd64/bin:\$PATH
EOF
```

## Related Documentation

- [Runtime System Overview](RUNTIME_SYSTEM.md) - Core runtime concepts
- [RNX CLI Reference](RNX_CLI_REFERENCE.md) - Complete command reference
- [Runtime Implementation](RUNTIME_IMPLEMENTATION.md) - Technical implementation details
- [Security Guide](SECURITY.md) - Security best practices

---

**Need Help?** Create an issue on GitHub or check the troubleshooting section above.
