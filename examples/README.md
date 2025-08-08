# Joblet Examples

Comprehensive examples demonstrating Joblet's capabilities, from instant runtime environments to advanced job
coordination patterns.

## ⚡ Runtime-Powered Examples (Instant Startup)

### 🐍 [Python 3.11 ML](./python-3.11-ml/) ⚡ **Instant Execution**

- **Advanced Analytics**: Complete ML stack with visualization
- **Libraries**: NumPy, Pandas, Matplotlib, SciPy, Seaborn
- **Examples**: Data analysis, statistical visualization
- **Runtime**: python-3.11-ml (pre-built environment)
- **Status**: ✅ With runtime / ⚠️ Without runtime

### ☕ [Java 17](./java-17/) ⚡ **2-3 seconds vs 30-120 seconds**

- **Enterprise Java**: OpenJDK 17 LTS with Maven
- **Development**: Complete Java development environment
- **Performance**: ~15-40x faster than JDK installation
- **Runtime**: java-17 (pre-compiled)
- **Status**: ✅ With runtime / ⚠️ Without runtime

### ☕ [Java 21](./java-21/) ⚡ **Modern Java Features**

- **Cutting-edge Java**: Virtual Threads, Pattern Matching, Records
- **Modern Features**: Latest Java 21 language enhancements
- **Development**: Modern Java development patterns
- **Runtime**: java-21 (latest features)
- **Status**: ✅ With runtime / ⚠️ Without runtime

## 📋 Traditional Examples (System Dependencies)

### 💻 [Basic Usage](./basic-usage/) ✅ **Always Works**

- **Simple Commands**: Basic command execution patterns
- **File Operations**: Upload files and workspace management
- **Resource Management**: CPU, memory, and I/O limits
- **Volume Storage**: Persistent data storage between jobs
- **Job Monitoring**: Status tracking and log viewing
- **Dependencies**: Shell commands only
- **Status**: ✅ Always works

### 🐍 [Python Analytics](./python-analytics/) ✅ **Always Works**

- **Sales Analysis**: Statistical analysis using Python standard library
- **Customer Segmentation**: K-means clustering implemented from scratch
- **Time Series Processing**: Data generation and moving averages
- **Dependencies**: Python 3 standard library only
- **Status**: ✅ Works out of the box with Python 3

### 🔗 [Advanced Examples](./advanced/) ✅ **Works with Python 3**

- **Job Coordination**: Sequential jobs with data dependencies
- **Volume Sharing**: Data passing between isolated jobs
- **Error Handling**: Robust job pipeline patterns
- **Dependencies**: Python 3 standard library
- **Status**: ✅ Works with Python 3

### 🤖 [Agentic AI](./agentic-ai/) ⚠️ **Requires Dependencies**

- **AI/ML Workflows**: LLM inference, RAG systems, distributed training
- **Multi-Agent Systems**: Complex AI coordination patterns
- **Dependencies**: External Python packages
- **Status**: ⚠️ May require additional setup

## 🚀 Quick Start

### 🏃‍♂️ Instant Start with Runtimes (Recommended)

```bash
# Option 1: Deploy pre-built packages (fastest - no host contamination)
# Copy packages from examples/packages/ to target host
scp examples/packages/python-3.11-ml-runtime.tar.gz admin@host:/tmp/
scp examples/packages/java-17-runtime-complete.tar.gz admin@host:/tmp/

# Deploy on target host
ssh admin@host
sudo tar -xzf /tmp/python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/
sudo tar -xzf /tmp/java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/
sudo chown -R joblet:joblet /opt/joblet/runtimes/

# Option 2: Build from setup scripts (builds on host)
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh
sudo /opt/joblet/examples/runtimes/java-17/setup_java_17.sh
sudo /opt/joblet/examples/runtimes/java-21/setup_java_21.sh

# 2. Verify runtimes are available
rnx runtime list

# 3. Try instant ML demo (2-3 seconds total)
cd python-3.11-ml/
rnx run --runtime=python-3.11-ml python example_data_analysis.py

# 4. Try Java development (instant compilation)
cd ../java-17/
rnx run --runtime=java:17 --upload=HelloJoblet.java bash -c "javac HelloJoblet.java && java HelloJoblet"
```

### 📚 Traditional Path (Always Works)

```bash
# 1. Start with Python Analytics (most comprehensive working example)
cd python-analytics/
./run_demo.sh

# 2. Try Basic Usage (fundamental concepts)
cd ../basic-usage/
./run_demos.sh

# 3. Explore Advanced Patterns (job coordination)
cd ../advanced/
./job_coordination.sh
```

### 🎯 Run All Examples

```bash
# Execute all working examples
./run_all_demos.sh
```

## ⚡ Performance Comparison

| **Example Type**     | **Traditional**              | **Runtime** | **Speedup**   |
|----------------------|------------------------------|-------------|---------------|
| **Python ML**        | 5-45 minutes (pip install)   | 2-3 seconds | **~100-300x** |
| **Java Development** | 30-120 seconds (JDK install) | 2-3 seconds | **~15-40x**   |
| **Python Analytics** | 30-60 seconds                | 2-5 seconds | **~10-20x**   |

## 🎯 Key Examples by Use Case

### 🤖 AI/ML Development (Runtime-Powered)

- **[Python 3.11 ML](./python-3.11-ml/)** - Advanced data analysis and visualization
- **[Agentic AI](./agentic-ai/)** - Complex AI workflows and multi-agent systems

### ☕ Enterprise Development (Runtime-Powered)

- **[Java 17 LTS](./java-17/)** - Enterprise Java with instant compilation
- **[Java 21 Modern](./java-21/)** - Cutting-edge Java features

### 📊 Data Analysis & Processing

- **[Python Analytics](./python-analytics/)** - Complete data analysis pipeline (no dependencies)
- **[Advanced Job Coordination](./advanced/)** - Multi-step data workflows

### 🎓 Learning Joblet Fundamentals

- **[Basic Usage](./basic-usage/)** - Core concepts and patterns
- **[Simple Commands](./basic-usage/01_simple_commands.sh)** - Start here

## 🔧 Prerequisites

### 🏎️ Runtime Approach (Recommended)

**Option 1: Deploy Pre-built Packages (Fastest, Zero Contamination):**

```bash
# Available packages in examples/packages/:
# - python-3.11-ml-runtime.tar.gz (226MB)
# - java-17-runtime-complete.tar.gz (193MB)  
# - java-21-runtime-complete.tar.gz (208MB)

# Deploy Python ML runtime
scp examples/packages/python-3.11-ml-runtime.tar.gz admin@host:/tmp/
ssh admin@host "sudo tar -xzf /tmp/python-3.11-ml-runtime.tar.gz -C /opt/joblet/runtimes/python/"

# Deploy Java runtimes
scp examples/packages/java-*-runtime-complete.tar.gz admin@host:/tmp/
ssh admin@host "sudo tar -xzf /tmp/java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/"
ssh admin@host "sudo tar -xzf /tmp/java-21-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/"
```

**Option 2: Build from Setup Scripts (on Joblet server):**

```bash
# Python 3.11 + ML Stack (builds from source)
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh

# Java 17 LTS + Maven
sudo /opt/joblet/examples/runtimes/java-17/setup_java_17.sh

# Java 21 + Modern Features  
sudo /opt/joblet/examples/runtimes/java-21/setup_java_21.sh
```

**Benefits:**

- ⚡ 10-1000x faster startup times
- 🔒 Complete isolation from host system
- 📦 Pre-installed packages and tools
- 🛡️ Security and consistency

### 📚 Traditional Approach (System Dependencies)

**Minimal Setup (for ✅ examples):**

- Joblet server running
- RNX client configured
- Python 3 available in job environment (for analytics)

**Full Setup (for all examples):**

- Java JDK (for Java examples)
- External package installation capability (for AI examples)

## 💡 Example Status Guide

### ⚡ Runtime-Powered (Instant Startup)

**With Runtimes Installed:**

- **Python ML**: ✅ Instant execution with complete ML stack
- **Java**: ✅ Instant compilation and execution

**Without Runtimes:**

- **Python ML**: ⚠️ Requires pip install (5-45 minutes)
- **Java**: ⚠️ Requires JDK installation (30-120 seconds)

### ✅ Always Works (System Dependencies)

These examples use commonly available tools:

- **Python Analytics**: Uses Python 3 standard library
- **Basic Usage**: Uses shell commands only
- **Advanced Coordination**: Uses Python 3 standard library

## 📚 Learning Path

### 🚀 Runtime Path (Recommended - Instant Results)

```bash
# 1. Install runtimes first (5-10 minutes one-time setup)
# See Prerequisites section above

# 2. Try ML demo (2-3 seconds)
cd python-3.11-ml/ && rnx run --runtime=python-3.11-ml python example_data_analysis.py

# 3. Try Java development (2-3 seconds)
cd ../java-17/ && rnx run --runtime=java-17 --upload=HelloJoblet.java javac HelloJoblet.java && java HelloJoblet
```

### 📋 Traditional Path (No Setup Required)

```bash
# 1. Start Here (5 minutes)
cd basic-usage/ && ./01_simple_commands.sh

# 2. Try Data Processing (10 minutes)
cd ../python-analytics/ && ./run_demo.sh

# 3. Learn Job Coordination (15 minutes)
cd ../advanced/ && ./job_coordination.sh

# 4. Explore All Basics (30 minutes)
cd ../basic-usage/ && ./run_demos.sh
```

## 🎉 Expected Results

### Runtime-Powered Examples

**After Python ML Demo:**

- Machine learning classification results
- Statistical analysis with NumPy/Pandas
- Execution time: 2-3 seconds total

**After Java Development:**

- Compiled Java application output
- Modern Java features demonstration
- Execution time: 2-3 seconds total

### Traditional Examples

**After Python Analytics:**

- Sales analysis results in JSON format
- Customer segmentation data
- Time series processing output

**After Job Coordination:**

- Multi-job workflow demonstration
- Data passing between jobs
- Dependency management patterns

## 🔍 Inspecting Results

### Runtime Examples

```bash
# View Python ML results
rnx run --runtime=python-3.11-ml --volume=ml-data cat /volumes/ml-data/results.json

# Check runtime availability
rnx runtime list
rnx runtime info python-3.11-ml
```

### Traditional Examples

```bash
# View analytics results
rnx run --volume=analytics-data cat /volumes/analytics-data/results/sales_analysis.json

# View coordination results  
rnx run --volume=shared-data cat /volumes/shared-data/results.json

# List all volumes
rnx volume list

# Check job history
rnx list
```

## 🛠️ Troubleshooting

### Runtime Issues

#### "runtime not found"

```bash
# Install the missing runtime on Joblet server
sudo /opt/joblet/examples/runtimes/python-3.11-ml/setup_python_3_11_ml.sh

# Verify installation
rnx runtime list
rnx runtime test python-3.11-ml
```

### Traditional Issues

#### "command not found"

- **Python scripts**: Ensure Python 3 is available in job environment
- **Java scripts**: JDK may not be available in job environment
- **Solution**: Install runtimes or stick to ✅ examples

#### Job failures

```bash
# Check job logs
rnx log <job-id>

# View job status
rnx list

# Test connectivity
rnx run echo "test"
```

#### Resource issues

```bash
# Monitor system resources
rnx monitor

# Adjust memory limits in scripts (--max-memory=1024)
# For runtime examples, 512MB-2GB recommended
```

## 📖 Documentation

Each example directory contains detailed README files:

- **[Java 17 README](./java-17/README.md)** - Enterprise Java development
- **[Python Analytics README](./python-analytics/README.md)** - Data analysis
- **[Basic Usage README](./basic-usage/README.md)** - Fundamental concepts
- **[Advanced README](./advanced/README.md)** - Job coordination patterns

## 🚀 Next Steps

### 🏎️ Runtime Approach (Recommended)

1. **Install Runtimes**: Set up pre-built environments (5-10 minutes one-time)
2. **Try ML Demo**: Experience 100-300x speedup
3. **Build APIs**: Create HTTP servers instantly
4. **Scale Up**: Use runtimes for production workloads

### 📚 Traditional Approach

1. **Master the Basics**: Complete basic-usage examples
2. **Explore Analytics**: Try python-analytics examples
3. **Learn Coordination**: Understand advanced job patterns
4. **Apply to Your Use Case**: Adapt examples for your workflows

**The runtime-powered examples transform development from minutes of waiting to seconds of execution!** 🚀