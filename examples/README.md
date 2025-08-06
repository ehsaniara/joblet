# Joblet Examples

Comprehensive examples demonstrating Joblet's capabilities, from simple command execution to advanced job coordination
patterns.

## 📁 Directory Structure

### 🐍 [Python Analytics](./python-analytics/) ✅ **Always Works**

- **Sales Analysis**: Statistical analysis using Python standard library
- **Customer Segmentation**: K-means clustering implemented from scratch
- **Time Series Processing**: Data generation and moving averages
- **Dependencies**: Python 3 standard library only
- **Status**: ✅ Works out of the box with Python 3

### 💻 [Basic Usage](./basic-usage/) ✅ **Always Works**

- **Simple Commands**: Basic command execution patterns
- **File Operations**: Upload files and workspace management
- **Resource Management**: CPU, memory, and I/O limits
- **Volume Storage**: Persistent data storage between jobs
- **Job Monitoring**: Status tracking and log viewing
- **Dependencies**: Shell commands only
- **Status**: ✅ Always works

### 🔗 [Advanced Examples](./advanced/) ✅ **Works with Python 3**

- **Job Coordination**: Sequential jobs with data dependencies
- **Volume Sharing**: Data passing between isolated jobs
- **Error Handling**: Robust job pipeline patterns
- **Dependencies**: Python 3 standard library
- **Status**: ✅ Works with Python 3

### 🟨 [Node.js](./nodejs/) ⚠️ **Requires Node.js**

- **System Analysis**: Platform and process information
- **Data Processing**: JSON manipulation and statistics
- **File Operations**: Built-in module demonstrations
- **Dependencies**: Node.js runtime in job environment
- **Status**: ⚠️ Requires Node.js installation

### 🤖 [Agentic AI](./agentic-ai/) ⚠️ **Requires Dependencies**

- **AI/ML Workflows**: Various AI-related examples
- **Dependencies**: External Python packages
- **Status**: ⚠️ May require additional setup

## 🚀 Quick Start

### Recommended Path (Always Works)

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

### Run All Working Examples

```bash
# Execute all working examples
./run_all_demos.sh
```

This will attempt to run all examples and report which ones succeed.

## 🎯 Key Examples by Use Case

### Data Analysis & Processing

- **[Python Analytics](./python-analytics/)** - Complete data analysis pipeline
- **[Advanced Job Coordination](./advanced/)** - Multi-step data workflows

### Learning Joblet Fundamentals

- **[Basic Usage](./basic-usage/)** - Core concepts and patterns
- **[Simple Commands](./basic-usage/01_simple_commands.sh)** - Start here

### Production Patterns

- **[Volume Storage](./basic-usage/04_volume_storage.sh)** - Persistent data
- **[Resource Limits](./basic-usage/03_resource_limits.sh)** - Resource management
- **[Job Coordination](./advanced/job_coordination.sh)** - Complex workflows

## 💡 Example Status Guide

### ✅ Always Works

These examples use only commonly available tools:

- **Python Analytics**: Uses Python 3 standard library
- **Basic Usage**: Uses shell commands only
- **Advanced Coordination**: Uses Python 3 standard library

### ⚠️ May Require Setup

These examples need specific runtime environments:

- **Node.js**: Requires Node.js in job environment
- **Agentic AI**: May require external packages

## 🔧 Prerequisites

### Minimal Setup (for ✅ examples)

- Joblet server running
- RNX client configured
- Python 3 available in job environment (for analytics)

### Full Setup (for all examples)

- Node.js runtime (for Node.js examples)
- External package installation capability (for AI examples)

## 📚 Learning Path

### 1. Start Here (5 minutes)

```bash
cd basic-usage/
./01_simple_commands.sh
```

### 2. Try Data Processing (10 minutes)

```bash
cd python-analytics/
./run_demo.sh
```

### 3. Learn Job Coordination (15 minutes)

```bash
cd advanced/
./job_coordination.sh
```

### 4. Explore All Basics (30 minutes)

```bash
cd basic-usage/
./run_demos.sh
```

## 🎉 Expected Results

### After Python Analytics

- Sales analysis results in JSON format
- Customer segmentation data
- Time series processing output

### After Job Coordination

- Multi-job workflow demonstration
- Data passing between jobs
- Dependency management patterns

### After Basic Usage

- Understanding of core Joblet concepts
- File upload and volume usage
- Resource management experience

## 🔍 Inspecting Results

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

### Common Issues

#### "command not found"

- **Python scripts**: Ensure Python 3 is available in job environment
- **Node.js scripts**: Node.js may not be installed in job environment
- **Solution**: Stick to ✅ examples or install missing runtime

#### Job failures

```bash
# Check job logs
rnx log <job-id>

# View job status
rnx list
```

#### Resource issues

```bash
# Monitor system resources
rnx monitor

# Adjust memory limits in scripts (--max-memory=512)
```

## 📖 Documentation

Each example directory contains detailed README files:

- **[Python Analytics README](./python-analytics/README.md)**
- **[Basic Usage README](./basic-usage/README.md)**
- **[Advanced README](./advanced/README.md)**
- **[Node.js README](./nodejs/README.md)**

## 🚀 Next Steps

1. **Master the Basics**: Complete basic-usage examples
2. **Explore Analytics**: Try python-analytics examples
3. **Learn Coordination**: Understand advanced job patterns
4. **Apply to Your Use Case**: Adapt examples for your workflows

Start with the ✅ examples for the best experience!