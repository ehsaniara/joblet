# Runtime Registry Guide

Complete guide for using and creating custom runtime registries for Joblet.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [How Runtime Registry Works](#how-runtime-registry-works)
- [Using Default Registry](#using-default-registry)
- [Using Custom Registry](#using-custom-registry)
- [Creating Your Own Registry](#creating-your-own-registry)
- [Registry Format](#registry-format)
- [Building Runtimes](#building-runtimes)
- [Testing Your Registry](#testing-your-registry)
- [Troubleshooting](#troubleshooting)

## Overview

Joblet uses an external runtime registry system that enables:

- **Centralized distribution** - Pre-packaged runtimes from trusted sources
- **Version management** - Multiple versions per runtime with @version notation
- **Custom registries** - Organizations can host their own runtime collections
- **Security** - SHA256 checksum verification for all downloads
- **Performance** - Direct tar.gz downloads (no git cloning needed)

## Quick Start

### Installing from Default Registry

```bash
# Install latest version
rnx runtime install python-3.11-ml

# Install specific version
rnx runtime install python-3.11-ml@1.0.2

# Install with explicit @latest
rnx runtime install python-3.11-ml@latest

# List available runtimes
rnx runtime list
```

### Installing from Custom Registry

```bash
# Use your organization's registry
rnx runtime install custom-runtime --registry=myorg/runtimes

# Install specific version from custom registry
rnx runtime install custom-runtime@2.0.0 --registry=myorg/runtimes
```

## How Runtime Registry Works

### Installation Flow

```
┌──────────────────┐
│ User Command     │
│ rnx runtime      │
│ install          │
└────────┬─────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 1. Parse Runtime Spec               │
│    - Extract name and version       │
│    - Default to @latest if no @     │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 2. Fetch Registry                   │
│    - URL: {registry}/registry.json  │
│    - Default: ehsaniara/joblet-     │
│               runtimes              │
│    - Or custom via --registry   │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 3. Resolve Version                  │
│    - @latest → highest semver       │
│    - @1.0.2 → exact match           │
│    - No @ → @latest                 │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 4. Download Runtime Package         │
│    - Download .tar.gz from release  │
│    - Verify SHA256 checksum         │
│    - Stream to temp directory       │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────┐
│ 5. Extract and Install              │
│    - Extract to versioned path:     │
│      /opt/joblet/runtimes/          │
│        {name}-{version}/            │
│    - Verify runtime.yml exists      │
└────────┬────────────────────────────┘
         │
         ▼
┌─────────────────┐
│ Runtime Ready   │
│ (Versioned)     │
└─────────────────┘
```

### Key Features

**Version Resolution:**

- `python-3.11-ml` → `python-3.11-ml@latest` → `python-3.11-ml@1.0.3`
- `python-3.11-ml@1.0.2` → exact version
- Multiple versions can coexist

**Versioned Paths:**

```
/opt/joblet/runtimes/
├── python-3.11-ml-1.0.2/
├── python-3.11-ml-1.0.3/
└── openjdk-21-1.0.2/
```

**Fallback Behavior:**

1. Try registry installation
2. If fails → Check local `runtimes/` directory
3. If not found → Error

## Using Default Registry

The default registry is: `https://github.com/ehsaniara/joblet-runtimes`

### Available Runtimes

```bash
# List all runtimes
rnx runtime list

# Get runtime info
rnx runtime info python-3.11-ml
```

### Installation Examples

```bash
# Basic installation (latest)
rnx runtime install python-3.11-ml

# Specific version
rnx runtime install python-3.11-ml@1.0.2

# Force reinstall
rnx runtime install python-3.11-ml@1.0.2 --force

# Different runtimes
rnx runtime install openjdk-21
rnx runtime install python-3.11-pytorch-cuda
rnx runtime install graalvmjdk-21
```

### Checking Installed Versions

```bash
# List installed runtimes
rnx runtime list

# Shows:
# python-3.11-ml-1.0.2
# python-3.11-ml-1.0.3
# openjdk-21-1.0.2
```

## Using Custom Registry

Organizations can host their own runtime registries.

### Specify Custom Registry

```bash
# Use custom registry
rnx runtime install my-runtime --registry=myorg/custom-runtimes

# With version
rnx runtime install my-runtime@2.1.0 --registry=myorg/runtimes

# Multiple commands
rnx runtime install runtime-a --registry=acme/runtimes
rnx runtime install runtime-b@1.5.0 --registry=acme/runtimes
```

### Use Cases for Custom Registries

**1. Private/Proprietary Runtimes**

```bash
# Company-specific Python with internal packages
rnx runtime install acme-python@2.0.0 --registry=acme-corp/runtimes
```

**2. Air-Gapped Environments**

```bash
# Self-hosted GitLab/Gitea
rnx runtime install secure-java # Note: Only GitHub registries are supported (format: owner/repo)
```

**3. Pre-release/Development**

```bash
# Beta runtimes
rnx runtime install python-beta@3.0.0-rc1 --registry=myorg/beta-runtimes
```

## Creating Your Own Registry

### Step 1: Initialize Repository

```bash
# Fork or create new repository
git clone https://github.com/myorg/my-runtimes.git
cd my-runtimes

# Create structure
mkdir -p runtimes/my-first-runtime
```

### Step 2: Create Runtime

**Create `runtimes/my-first-runtime/manifest.yaml`:**

```yaml
name: my-first-runtime
version: 1.0.0
description: My custom runtime environment
platforms:
  - ubuntu-amd64
  - ubuntu-arm64
  - rhel-amd64
```

**Create `runtimes/my-first-runtime/setup.sh`:**

```bash
#!/bin/bash
set -e

# Install dependencies
apt-get update
apt-get install -y build-essential curl

# Install your runtime
mkdir -p /opt/joblet/runtimes/my-first-runtime
# ... installation commands ...

# Create runtime.yml
cat > /opt/joblet/runtimes/my-first-runtime/runtime.yml <<EOF
name: my-first-runtime
version: 1.0.0
language: custom
EOF

echo "Installation complete"
```

### Step 3: Copy GitHub Workflow

Copy `.github/workflows/release.yml` from `ehsaniara/joblet-runtimes`:

```yaml
name: Build and Release Runtimes
on:
  push:
    tags:
      - 'v*.*.*'

permissions:
  contents: write

jobs:
  build-and-release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Build all runtimes
        run: ./build/build-all.sh

      - name: Generate registry.json
        run: |
          # Auto-generates multi-version registry.json
          # (See full workflow in joblet-runtimes repository)

      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: releases/*.tar.gz
```

### Step 4: Create Release

```bash
# Commit your runtime
git add .
git commit -m "Add my-first-runtime v1.0.0"
git push origin main

# Create release tag
git tag v1.0.0
git push origin v1.0.0
```

**GitHub Actions will:**

1. Build all runtimes
2. Create .tar.gz packages
3. Calculate SHA256 checksums
4. Generate `registry.json`
5. Create GitHub release

### Step 5: Use Your Registry

```bash
# Install from your registry
rnx runtime install my-first-runtime --registry=myorg/my-runtimes

# Verify installation
rnx runtime list | grep my-first-runtime
```

## Registry Format

### Registry.json Structure

Multi-version nested format:

```json
{
  "version": "1",
  "updated_at": "2025-10-19T12:00:00Z",
  "runtimes": {
    "python-custom": {
      "1.0.0": {
        "version": "1.0.0",
        "description": "Custom Python runtime",
        "download_url": "https://github.com/myorg/runtimes/releases/download/v1.0.0/python-custom-1.0.0.tar.gz",
        "checksum": "sha256:abc123...",
        "size": 145678900,
        "platforms": ["ubuntu-amd64", "ubuntu-arm64"]
      },
      "1.0.1": {
        "version": "1.0.1",
        "description": "Custom Python runtime",
        "download_url": "https://github.com/myorg/runtimes/releases/download/v1.0.1/python-custom-1.0.1.tar.gz",
        "checksum": "sha256:def456...",
        "size": 146000000,
        "platforms": ["ubuntu-amd64", "ubuntu-arm64"]
      }
    },
    "java-enterprise": {
      "2.0.0": {
        "version": "2.0.0",
        ...
      }
    }
  }
}
```

### Required Fields

- `version` - Registry schema version ("1")
- `updated_at` - ISO 8601 timestamp
- `runtimes` - Object containing all runtimes
    - `{runtime-name}` - Runtime identifier
        - `{version}` - Semantic version number
            - `version` - Version string
            - `description` - Human-readable description
            - `download_url` - Direct download URL for .tar.gz
            - `checksum` - SHA256 checksum with `sha256:` prefix
            - `size` - File size in bytes
            - `platforms` - Array of supported platforms

### Platform Identifiers

Format: `{os}-{arch}`

**Operating Systems:**

- `ubuntu` - Ubuntu/Debian
- `rhel` - RHEL/CentOS/Rocky
- `amzn` - Amazon Linux

**Architectures:**

- `amd64` - x86_64
- `arm64` - ARM64/aarch64

**Examples:**

- `ubuntu-amd64`
- `ubuntu-arm64`
- `rhel-amd64`

## Building Runtimes

### Build Script Structure

```bash
#!/bin/bash
# build/build-all.sh

set -e

mkdir -p releases

for runtime_dir in runtimes/*/; do
    runtime_name=$(basename "$runtime_dir")
    version=$(grep '^version:' "$runtime_dir/manifest.yaml" | awk '{print $2}')

    echo "Building $runtime_name-$version..."

    # Run setup script in isolated environment
    # Package to releases/
    tar -czf "releases/${runtime_name}-${version}.tar.gz" -C /opt/joblet/runtimes "$runtime_name"
done
```

### Manifest Schema (manifest.yaml)

```yaml
name: python-custom
version: 1.0.0
description: Custom Python 3.11 with internal packages
platforms:
  - ubuntu-amd64
  - ubuntu-arm64
  - rhel-amd64
```

## Testing Your Registry

### Local Testing

```bash
# 1. Build runtime locally
cd runtimes/my-runtime
sudo bash setup.sh

# 2. Verify installation
ls -la /opt/joblet/runtimes/my-runtime/
cat /opt/joblet/runtimes/my-runtime/runtime.yml

# 3. Package runtime
cd /opt/joblet/runtimes
tar -czf ~/my-runtime-1.0.0.tar.gz my-runtime/

# 4. Calculate checksum
sha256sum ~/my-runtime-1.0.0.tar.gz
```

### Registry Testing

```bash
# 1. Push changes and create tag
git tag v1.0.0
git push origin v1.0.0

# 2. Wait for GitHub Actions to complete
# 3. Verify registry.json was generated
curl https://raw.githubusercontent.com/myorg/my-runtimes/main/registry.json | jq

# 4. Test installation
rnx runtime install my-runtime --registry=myorg/my-runtimes

# 5. Verify runtime works
rnx job run --runtime=my-runtime echo "Hello"
```

## Troubleshooting

### Registry Not Found

```
Error: failed to fetch registry
```

**Solutions:**

- Verify registry URL is correct
- Check `registry.json` exists in repository root
- Ensure repository is public or access token is configured
- Test URL manually: `curl {registry}/registry.json`

### Runtime Not in Registry

```
Error: runtime 'my-runtime' not found in registry
```

**Solutions:**

- Check runtime name matches exactly
- Verify `registry.json` includes the runtime
- Inspect registry: `curl {url}/registry.json | jq '.runtimes | keys'`

### Version Not Found

```
Error: version '1.0.5' not found for runtime 'my-runtime'
```

**Solutions:**

- Check available versions: `curl {url}/registry.json | jq '.runtimes["my-runtime"] | keys'`
- Verify tag was pushed and workflow ran
- Check GitHub releases for .tar.gz file

### Checksum Verification Failed

```
Error: checksum mismatch
```

**Solutions:**

- Re-download the runtime
- Verify file wasn't corrupted
- Check if .tar.gz file was modified after registry.json generation
- Regenerate release with correct checksum

### Download Failed (404)

```
Error: HTTP 404: 404 Not Found
```

**Solutions:**

- Verify download URL in registry.json
- Check GitHub release exists
- Ensure .tar.gz file was uploaded to release
- Verify repository owner/name matches

## Best Practices

### Version Management

**✅ Good:**

- Use semantic versioning: `1.0.0`, `1.0.1`, `2.0.0`
- Tag format: `v1.0.0`, `v2.0.0`
- One tag per release
- Immutable releases (don't modify after publishing)

**❌ Avoid:**

- Moving tags (`git tag -f`)
- Deleting releases
- Modifying release assets after publication

### Registry Maintenance

**✅ Good:**

```json
{
  "runtimes": {
    "python-custom": {
      "1.0.0": {...},  // Keep old versions
      "1.0.1": {...},
      "1.0.2": {...}
    }
  }
}
```

**❌ Avoid:**

```json
{
  "runtimes": {
    "python-custom": {
      "1.0.2": {...}  // Only latest (users can't pin versions)
    }
  }
}
```

### Security

1. **Verify checksums** - Always include SHA256
2. **HTTPS only** - Never use HTTP URLs
3. **Minimal permissions** - Don't require root unnecessarily
4. **Audit dependencies** - Review all installed packages
5. **Scan for vulnerabilities** - Use security scanning tools

### Performance

- Keep runtime sizes reasonable (< 1GB recommended)
- Clean up build artifacts
- Use compressed tar.gz format
- Test download speeds

## Related Documentation

- [Runtime System Overview](RUNTIME_SYSTEM.md)
- [RNX CLI Reference](RNX_CLI_REFERENCE.md)
- [Creating Custom Runtimes](RUNTIME_ADVANCED.md)
- [Deployment Guide](DEPLOYMENT.md)

---

**Questions?** Check [Troubleshooting](#troubleshooting) or create an issue on GitHub.
