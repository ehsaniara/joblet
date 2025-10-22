# GitHub Runtime Repository Guide

> **⚠️ DEPRECATED**
> This guide is deprecated as of October 2025. The `--github-repo` flag has been removed.
>
> **Please use the new Runtime Registry system instead:**
> - See [Runtime Registry Guide](RUNTIME_REGISTRY_GUIDE.md) for the current documentation
> - Use `--registry` flag for custom registries instead of `--github-repo`
>
> The new registry system provides:
> - Multi-version support with `@version` notation
> - Faster installation (direct .tar.gz downloads)
> - SHA256 checksum verification
> - Better version management
> - Simpler, more consistent interface

---

## Migration Guide

### Old Approach (Deprecated)

```bash
# Old: Direct GitHub installation
rnx runtime install python-3.11-ml --github-repo=owner/repo/tree/main/runtimes
```

### New Approach

```bash
# New: Registry-based installation
rnx runtime install python-3.11-ml

# With custom registry
rnx runtime install python-3.11-ml --registry=myorg/custom-runtimes

# With version specification
rnx runtime install python-3.11-ml@1.0.2
```

## Creating Custom Registries

Instead of hosting runtime scripts in a GitHub repository, you now create a **runtime registry** that packages and
distributes pre-built runtimes.

**Benefits:**

- **Faster** - No git clone, direct tar.gz download
- **Versioned** - Multiple versions can coexist (`python-3.11-ml-1.0.2`, `python-3.11-ml-1.0.3`)
- **Secure** - SHA256 checksum verification
- **Simpler** - One flag (`--registry`) instead of complex GitHub paths

**See [Runtime Registry Guide](RUNTIME_REGISTRY_GUIDE.md) for complete documentation on:**

- How to create your own runtime registry
- Registry format (registry.json)
- Building and packaging runtimes
- Testing and deployment
- Troubleshooting

## Quick Migration Steps

### For Users

1. **Remove `--github-repo` flag from your commands**
   ```bash
   # Before
   rnx runtime install my-runtime --github-repo=owner/repo/tree/main/runtimes

   # After
   rnx runtime install my-runtime --registry=owner/repo
   ```

2. **Update to versioned installations**
   ```bash
   # Specify versions for reproducibility
   rnx runtime install my-runtime@1.0.2 --registry=owner/repo
   ```

### For Registry Maintainers

1. **Fork/copy [ehsaniara/joblet-runtimes](https://github.com/ehsaniara/joblet-runtimes) repository**

2. **Add your runtimes** to `runtimes/` directory with `manifest.yaml` and setup scripts

3. **Copy GitHub Actions workflow** from joblet-runtimes (`.github/workflows/release.yml`)
    - Auto-generates `registry.json`
    - Builds .tar.gz packages
    - Creates GitHub releases
    - Calculates checksums

4. **Tag releases** to trigger builds
   ```bash
   git tag v1.0.0
   git push origin v1.0.0
   ```

5. **Direct users to your registry**
   ```bash
   rnx runtime install your-runtime --registry=yourorg/your-registry
   ```

## Why the Change?

| Old (GitHub Direct)      | New (Registry)          |
|--------------------------|-------------------------|
| Clone entire repository  | Download single .tar.gz |
| No version management    | Full semver support     |
| No checksum verification | SHA256 verification     |
| Complex GitHub paths     | Simple registry URL     |
| Slow (git clone + build) | Fast (direct download)  |
| Build on installation    | Pre-built packages      |

## Support

For questions about the new registry system, see:

- [Runtime Registry Guide](RUNTIME_REGISTRY_GUIDE.md) - Complete documentation
- [RNX CLI Reference](RNX_CLI_REFERENCE.md) - Command reference
- [GitHub Issues](https://github.com/ehsaniara/joblet/issues) - Report problems

---

**This document is kept for historical reference only. Please use
the [Runtime Registry Guide](RUNTIME_REGISTRY_GUIDE.md) for current documentation.**
