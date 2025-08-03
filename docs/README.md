# Joblet Documentation

Welcome to the complete Joblet documentation! This guide covers everything you need to know about installing, configuring, and using Joblet - a Linux-native job execution platform with advanced isolation and resource management.

## 📚 Documentation Overview

### Getting Started
- [**Quick Start Guide**](./QUICKSTART.md) - Get up and running in 5 minutes
- [**Installation Guide**](./INSTALLATION.md) - Detailed installation instructions for all platforms
- [**Configuration**](./CONFIGURATION.md) - Complete configuration reference

### User Guides
- [**RNX CLI Reference**](./RNX_CLI_REFERENCE.md) - Complete command reference with examples
- [**Job Execution Guide**](./JOB_EXECUTION.md) - Running jobs with resource limits and isolation
- [**Volume Management**](./VOLUME_MANAGEMENT.md) - Persistent and temporary storage for jobs
- [**Network Management**](./NETWORK_MANAGEMENT.md) - Network isolation and custom networks
- [**Monitoring & Metrics**](./MONITORING.md) - System and job monitoring

### Advanced Topics
- [**Security Guide**](./SECURITY.md) - mTLS, authentication, and best practices
- [**Multi-Node Setup**](./MULTI_NODE.md) - Managing multiple Joblet servers
- [**CI/CD Integration**](./CI_CD_INTEGRATION.md) - Using Joblet in CI/CD pipelines
- [**Troubleshooting**](./TROUBLESHOOTING.md) - Common issues and solutions

### Reference
- [**API Reference**](./API.md) - gRPC API documentation
- [**Architecture**](./DESIGN.md) - System design and architecture
- [**Examples**](./EXAMPLES.md) - Real-world usage examples

## 🚀 What is Joblet?

Joblet is a powerful job execution platform that provides:

- **🔒 Security**: Process isolation with Linux namespaces and cgroups
- **📊 Resource Management**: CPU, memory, and I/O limits
- **🌐 Network Isolation**: Custom networks with traffic isolation
- **💾 Volume Management**: Persistent and temporary storage
- **📡 Real-time Monitoring**: Live log streaming and metrics
- **🔐 mTLS Authentication**: Certificate-based security
- **🖥️ Cross-platform CLI**: RNX client works on Linux, macOS, and Windows

## 🎯 Use Cases

- **CI/CD Pipelines**: Secure job execution with resource limits
- **Batch Processing**: Run scheduled jobs with isolation
- **Development Environments**: Isolated environments for testing
- **Microservices Testing**: Network-isolated service testing
- **Resource-Limited Execution**: Control CPU, memory, and I/O usage

## 📋 Prerequisites

- **Server**: Linux with kernel 3.10+ (cgroups v2 support)
- **Client**: Any OS (Linux, macOS, Windows)
- **Go**: 1.21+ for building from source

## 🏃 Quick Example

```bash
# Run a simple command
rnx run echo "Hello, Joblet!"

# Run with resource limits
rnx run --max-cpu=50 --max-memory=512 --max-iobps=10485760 python script.py

# Use volumes for persistent storage
rnx volume create mydata --size=1GB --type=filesystem
rnx run --volume=mydata python process_data.py

# Create isolated network
rnx network create mynet --cidr=10.10.0.0/24
rnx run --network=mynet ./my-service
```

## 📖 Start Here

New to Joblet? Start with our [Quick Start Guide](./QUICKSTART.md) to get up and running in minutes!

For detailed installation instructions, see the [Installation Guide](./INSTALLATION.md).

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](../CONTRIBUTING.md) for details.

## 📄 License

Joblet is licensed under the [MIT License](../LICENSE).