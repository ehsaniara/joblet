# Joblet Runtime Packages

Pre-built runtime packages for instant deployment across your Joblet infrastructure.

## 📦 Available Packages

| Runtime            | Package                           | Size  | Description                                              |
|--------------------|-----------------------------------|-------|----------------------------------------------------------|
| **Java 17 LTS**    | `java-17-runtime-complete.tar.gz` | 193MB | OpenJDK 17 with Maven, enterprise-ready                  |
| **Java 21 LTS**    | `java-21-runtime-complete.tar.gz` | 208MB | OpenJDK 21 with Virtual Threads, Pattern Matching        |
| **Python 3.11 ML** | `python-3.11-ml-runtime.tar.gz`   | 226MB | Python 3.11 with NumPy, Pandas, Scikit-learn, Matplotlib |

## 🚀 Quick Deployment

### Deploy a Runtime Package

```bash
# 1. Copy package to target host
scp java-17-runtime-complete.tar.gz admin@target-host:/tmp/

# 2. SSH to target host and extract
ssh admin@target-host
sudo mkdir -p /opt/joblet/runtimes/java
sudo tar -xzf /tmp/java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/

# 3. Set permissions
sudo chown -R joblet:joblet /opt/joblet/runtimes/java/java-17

# 4. Verify deployment
exit  # Return to local machine
rnx runtime test java:17
```

## 📋 Package Contents

### Java 17 Runtime (`java-17-runtime-complete.tar.gz`)

```
java-17/
├── jdk/                  # OpenJDK 17.0.11 binaries
├── maven/                # Apache Maven 3.9.6
├── bin/                  # Symlinks for job execution
├── lib/                  # Java libraries
└── runtime.yml           # Runtime configuration
```

**Features:**

- OpenJDK 17.0.11 LTS
- Apache Maven 3.9.6
- Complete JDK tools (javac, jar, jshell, etc.)
- Enterprise-ready configuration
- Zero host contamination

### Java 21 Runtime (`java-21-runtime-complete.tar.gz`)

```
java-21/
├── jdk/                  # OpenJDK 21.0.4 binaries
├── maven/                # Apache Maven 3.9.6
├── bin/                  # Symlinks for job execution
├── lib/                  # Java libraries
└── runtime.yml           # Runtime configuration
```

**Features:**

- OpenJDK 21.0.4 LTS
- Virtual Threads (Project Loom)
- Pattern Matching for switch
- String Templates (Preview)
- Record Patterns
- Apache Maven 3.9.6

### Python 3.11 ML Runtime (`python-3.11-ml-runtime.tar.gz`)

```
python-3.11-ml/
├── python-install/       # Python 3.11.9 compiled binaries
├── ml-venv/             # Virtual environment with ML packages
├── bin/                 # Python wrapper scripts
├── lib/                 # Python libraries
└── runtime.yml          # Runtime configuration
```

**Pre-installed Packages:**

- NumPy 1.24.x (pinned to 1.x for stability)
- Pandas 2.0.x
- Scikit-learn 1.3.x
- Matplotlib 3.7.x
- Seaborn 0.12.x
- SciPy 1.11.x
- Requests 2.31.0
- OpenPyXL 3.1.2

## 🎯 Deployment Methods

### Method 1: Direct Package Deployment (Fastest)

```bash
# Deploy Java 17
scp java-17-runtime-complete.tar.gz admin@host:/tmp/
ssh admin@host
sudo tar -xzf /tmp/java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/
sudo chown -R joblet:joblet /opt/joblet/runtimes/java/java-17

# Deploy Python ML
scp python-3.11-ml-runtime.tar.gz admin@host:/tmp/
ssh admin@host
sudo tar -xzf /tmp/python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/
sudo chown -R joblet:joblet /opt/joblet/runtimes/python/python-3.11-ml
```

### Method 2: Using Setup Scripts (Builds from Source)

```bash
# On the target host (as root)
sudo /opt/joblet/examples/runtimes/java-17/setup_java_17.sh
sudo /opt/joblet/examples/runtimes/java-21/setup_java_21.sh
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh
```

### Method 3: Automated Deployment Script

```bash
#!/bin/bash
# deploy_runtime.sh - Deploy runtime to multiple hosts

RUNTIME_PACKAGE=$1
TARGET_HOSTS="host1 host2 host3"

for host in $TARGET_HOSTS; do
    echo "Deploying to $host..."
    scp $RUNTIME_PACKAGE admin@$host:/tmp/
    
    # Determine runtime type and path
    if [[ $RUNTIME_PACKAGE == *"java"* ]]; then
        RUNTIME_PATH="/opt/joblet/runtimes/java"
    elif [[ $RUNTIME_PACKAGE == *"python"* ]]; then
        RUNTIME_PATH="/opt/joblet/runtimes/python"
    fi
    
    ssh admin@$host "
        sudo mkdir -p $RUNTIME_PATH
        sudo tar -xzf /tmp/$(basename $RUNTIME_PACKAGE) -C $RUNTIME_PATH/
        sudo chown -R joblet:joblet $RUNTIME_PATH/
    "
done
```

## 🔍 Verification

### Verify Runtime Installation

```bash
# List all available runtimes
rnx runtime list

# Test specific runtime
rnx runtime test java:17
rnx runtime test java:21
rnx runtime test python-3.11-ml

# Run test commands
rnx run --runtime=java:17 java -version
rnx run --runtime=java:21 java -version
rnx run --runtime=python-3.11-ml python --version
```

### Test Runtime Functionality

```bash
# Java 17 - Compile and run
rnx run --runtime=java:17 bash -c "echo 'public class Test { public static void main(String[] args) { System.out.println(\"Java 17 works!\"); } }' > Test.java && javac Test.java && java Test"

# Java 21 - Test Virtual Threads
rnx run --runtime=java:21 jshell -s - << 'EOF'
Thread.startVirtualThread(() -> System.out.println("Virtual Thread works!")).join();
EOF

# Python ML - Test packages
rnx run --runtime=python-3.11-ml python -c "import numpy as np; import pandas as pd; print(f'NumPy {np.__version__}, Pandas {pd.__version__}')"
```

## 📊 Performance Comparison

| Operation       | Traditional Setup | Pre-built Package | Speedup      |
|-----------------|-------------------|-------------------|--------------|
| Java 17 Setup   | 30-120 seconds    | 2-5 seconds       | **15-40x**   |
| Java 21 Setup   | 30-120 seconds    | 2-5 seconds       | **15-40x**   |
| Python ML Setup | 5-45 minutes      | 2-5 seconds       | **100-500x** |

## 🔧 Building New Packages

### Generate Package from Existing Runtime

```bash
# Package existing Java 17 runtime
cd /opt/joblet/runtimes/java
tar -czf java-17-runtime-complete.tar.gz java-17/

# Package existing Python ML runtime
cd /opt/joblet/runtimes/python
tar -czf python-3.11-ml-runtime.tar.gz python-3.11-ml/
```

### Build Fresh Package

```bash
# Build and package Java 17
sudo /opt/joblet/examples/runtimes/java-17/setup_java_17.sh package

# Build and package Python ML
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh package

# Packages will be created in /tmp/runtime-packages/
```

## 🛡️ Security Considerations

### Package Integrity

All packages include SHA256 checksums for verification:

```bash
# Generate checksum
sha256sum java-17-runtime-complete.tar.gz > java-17-runtime-complete.sha256

# Verify before deployment
sha256sum -c java-17-runtime-complete.sha256
```

### Isolation Guarantees

- **Zero Host Contamination**: Runtimes are completely isolated
- **Read-only Mounts**: Runtime files are mounted read-only into jobs
- **No System Modifications**: No changes to host system libraries
- **User Isolation**: Runs under joblet user, not root

## 📈 Scaling Considerations

### Storage Requirements

| Runtime   | Extracted Size | Recommended Space |
|-----------|----------------|-------------------|
| Java 17   | ~450MB         | 1GB               |
| Java 21   | ~480MB         | 1GB               |
| Python ML | ~550MB         | 1.5GB             |

### Network Transfer

For large-scale deployments, consider:

- Using a central package repository
- Implementing CDN or local mirrors
- Compressing packages with higher ratios (xz vs gz)
- Using rsync for incremental updates

## 🔄 Updates and Maintenance

### Updating Packages

```bash
# 1. Update runtime on build host
sudo /opt/joblet/examples/runtimes/java-17/setup_java_17.sh

# 2. Create new package
cd /opt/joblet/runtimes/java
tar -czf java-17-runtime-complete-v2.tar.gz java-17/

# 3. Deploy to all hosts
./deploy_runtime.sh java-17-runtime-complete-v2.tar.gz
```

### Version Management

Consider implementing version tags:

```bash
# Naming convention
java-17-runtime-complete-2024.08.09.tar.gz
python-3.11-ml-runtime-2024.08.09.tar.gz

# Or semantic versioning
java-17-runtime-v1.0.0.tar.gz
python-3.11-ml-runtime-v1.2.3.tar.gz
```

## 💡 Best Practices

1. **Always Verify Packages**: Check SHA256 sums before deployment
2. **Test Before Production**: Deploy to staging environment first
3. **Document Changes**: Keep changelog for package updates
4. **Automate Deployment**: Use configuration management tools
5. **Monitor Disk Space**: Ensure adequate space for runtimes
6. **Regular Updates**: Keep packages updated with security patches

## 📚 Related Documentation

- [Runtime Setup Scripts](../runtimes/README.md)
- [Portable Runtime Packaging](../runtimes/PORTABLE_RUNTIME_PACKAGING.md)
- [Runtime Examples](../README.md)
- [Java 17 Example](../java-17/README.md)
- [Java 21 Example](../java-21/README.md)
- [Python ML Example](../python-3.11-ml/README.md)

## 🆘 Troubleshooting

### Common Issues

**"Permission denied" during extraction:**

```bash
# Ensure proper permissions
sudo tar -xzf package.tar.gz -C /opt/joblet/runtimes/
sudo chown -R joblet:joblet /opt/joblet/runtimes/
```

**"Runtime not found" after deployment:**

```bash
# Verify extraction path
ls -la /opt/joblet/runtimes/java/
ls -la /opt/joblet/runtimes/python/

# Check runtime.yml exists
cat /opt/joblet/runtimes/java/java-17/runtime.yml
```

**"Package corrupted" errors:**

```bash
# Re-download and verify checksum
sha256sum package.tar.gz
# Compare with original checksum
```

## 🎯 Summary

These pre-built runtime packages enable:

- ⚡ **Instant Deployment**: 2-5 seconds vs minutes/hours
- 🔒 **Complete Isolation**: Zero host contamination
- 📦 **Portability**: Deploy anywhere with tar
- 🚀 **Production Ready**: Tested and optimized
- 💾 **Efficient Storage**: Compressed packages ~200MB each

Deploy once, run instantly, scale effortlessly! 🚀