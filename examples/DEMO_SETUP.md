# Joblet Demo Setup Guide

This guide helps you run the Joblet examples that demonstrate core functionality, including pre-built runtime
environments for instant execution.

## 🚀 Quick Start

### 1. Prerequisites

**Essential:**

- **Joblet Server**: Running joblet daemon
- **RNX Client**: Configured and connected to server

**Pre-built Runtimes (Recommended for best experience):**

- **Python 3.11 + ML Runtime**: `sudo /opt/joblet/runtimes/python-3.11-ml/setup_python_3_11_ml.sh`
- **Java 17 LTS Runtime**: `sudo /opt/joblet/runtimes/java-17/setup_java_17.sh`
- **Java 21 Runtime**: `sudo /opt/joblet/runtimes/java-21/setup_java_21.sh`

**Fallback (System Dependencies):**

- **Python 3**: For analytics examples (uses standard library only)
- **Java**: For Java examples (if available in job environment)

### 2. Verify Setup

```bash
# Check RNX connection
rnx job list

# Check server connectivity
rnx job run echo "Hello Joblet"

# Check available runtimes (if installed)
rnx runtime list

# Test runtime functionality
rnx runtime test python-3.11-ml    # If Python runtime installed
```

### 3. Run Working Examples

#### 🚀 Runtime-Powered Examples (Instant Startup)

**Python ML Examples:**

```bash
# Machine Learning Demo (2-3 seconds vs 5-45 minutes traditional)
cd python-ml/
./run_demo.sh

# Data Analysis Example
cd python-3.11-ml/
rnx job run --runtime=python-3.11-ml --upload=example_data_analysis.py python example_data_analysis.py
```

**Java Development:**

```bash
# Java 17 LTS Example
cd java-17/
rnx job run --runtime=java:17 --upload=HelloJoblet.java javac HelloJoblet.java && java HelloJoblet

# Java 21 with Modern Features
cd java-21/
rnx job run --runtime=java:21 --upload=VirtualThreadExample.java javac VirtualThreadExample.java && java VirtualThreadExample
```

#### 📋 Traditional Examples (System Dependencies)

**Basic Usage (Always Works):**

```bash
cd basic-usage/
./run_demos.sh
```

**Python Analytics (Standard Library):**

```bash
cd python-analytics/
./run_demo.sh
```

**Advanced Job Coordination:**

```bash
cd advanced/
./job_coordination.sh
```

### 4. Run All Working Demos

```bash
# Run all working examples
./run_all_demos.sh
```

## 📚 Demo Contents

### 🚀 Runtime-Powered Examples (Instant Startup)

### Python ML (`python-ml/`)

- **Runtime**: python-3.11-ml (2-3 seconds startup)
- **Features**: Machine learning with NumPy, Pandas, Scikit-learn
- **Examples**: Data classification, statistical analysis
- **Performance**: ~100-300x faster than package installation
- **Status**: ✅ With runtime / ⚠️ Without runtime

### Python 3.11 ML (`python-3.11-ml/`)

- **Runtime**: python-3.11-ml (instant execution)
- **Features**: Advanced data analysis, visualization
- **Libraries**: NumPy, Pandas, Matplotlib, SciPy
- **Storage**: Results saved to persistent volumes
- **Status**: ✅ With runtime / ⚠️ Without runtime

### Java Examples (`java-17/`, `java-21/`)

- **Runtime**: java-17 or java-21 (instant compilation)
- **Features**: Enterprise Java, modern language features
- **Tools**: Maven, JDK tools, Virtual Threads (Java 21)
- **Examples**: Hello World, modern Java features
- **Performance**: ~15-40x faster than JDK installation
- **Status**: ✅ With runtime / ⚠️ Without runtime

### 📋 Traditional Examples (System Dependencies)

### Basic Usage (`basic-usage/`)

- **Dependencies**: Shell commands only
- **Features**: File operations, resource limits, volume storage, job monitoring
- **Storage**: Temporary files and volumes
- **Status**: ✅ Always works

### Python Analytics (`python-analytics/`)

- **Dependencies**: Python 3 standard library only
- **Features**: Sales analysis, customer segmentation, time series processing
- **Storage**: Results saved to persistent volumes
- **Status**: ✅ Always works with Python 3

### Advanced Examples (`advanced/`)

- **Dependencies**: Python 3 standard library
- **Features**: Job coordination, data passing between jobs
- **Storage**: Persistent volumes for coordination
- **Status**: ✅ Works with Python 3

### Agentic AI (`agentic-ai/`)

- **Dependencies**: Python 3 + pip packages
- **Features**: LLM inference, RAG systems, distributed training
- **Examples**: Multi-agent systems, AI workflows
- **Status**: ⚠️ Requires package installation

## 🔧 Troubleshooting

### Common Issues

#### "command not found" errors

**With Runtimes (Recommended):**

- **Python**: Install runtime: `sudo /opt/joblet/runtimes/python-3.11-ml/setup_python_3_11_ml.sh`
- **Java**: Install runtime: `sudo /opt/joblet/runtimes/java-17/setup_java_17.sh`
- **Verify**: Check with `rnx runtime list`

**With System Dependencies:**

- **Python scripts**: Ensure Python 3 is installed in job environment
- **Java scripts**: JDK may not be available in job environment
- **Fallback**: Use basic-usage or python-analytics examples

#### Job failures

- **Check logs**: `rnx job log <job-id>`
- **Resource limits**: Increase memory limits if needed
- **Dependencies**: Use examples with minimal dependencies

#### Volume errors

- **Storage space**: Ensure adequate disk space on server
- **Permissions**: Check Joblet server permissions
- **Cleanup**: Remove unused volumes with `rnx volume remove`

### Debug Commands

```bash
# List all jobs
rnx job list

# View job output
rnx job log <job-id>

# Check volumes
rnx volume list

# Monitor system
rnx monitor
```

## 📊 Expected Results

### After Running Python Analytics

```bash
# View sales analysis
rnx job run --volume=analytics-data cat /volumes/analytics-data/results/sales_analysis.json

# View processed time series
rnx job run --volume=analytics-data ls /volumes/analytics-data/processed/
```

### After Running Job Coordination

```bash
# View coordination results
rnx job run --volume=shared-data cat /volumes/shared-data/results.json
```

## 💡 Best Practices

### For Reliable Demos

1. **Start Simple**: Begin with basic-usage examples
2. **Check Prerequisites**: Verify required runtimes are available
3. **Monitor Resources**: Watch memory and CPU usage
4. **Clean Up**: Remove unused volumes and jobs periodically

### For Production Use

1. **Resource Planning**: Set appropriate CPU and memory limits
2. **Error Handling**: Implement proper error checking
3. **Monitoring**: Use logging and status tracking
4. **Security**: Follow security best practices for production workloads

## 📚 Next Steps

1. **Explore Examples**: Run each demo to understand different patterns
2. **Modify Data**: Replace sample data with your own datasets
3. **Scale Up**: Increase resource limits for larger workloads
4. **Custom Workflows**: Build your own job coordination patterns

## Getting Help

- **Check Logs**: Always start with `rnx job log <job-id>` for failures
- **Resource Issues**: Monitor with `rnx monitor`
- **Connectivity**: Test with simple `rnx job run echo "test"` commands
- **Documentation**: See individual example README files for specific guidance