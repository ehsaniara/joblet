# Joblet Demo Setup Guide

This guide helps you run the Joblet examples that demonstrate core functionality with minimal dependencies.

## 🚀 Quick Start

### 1. Prerequisites

**Essential:**

- **Joblet Server**: Running joblet daemon
- **RNX Client**: Configured and connected to server
- **Python 3**: For analytics examples (uses standard library only)

**Optional:**

- **Node.js**: For Node.js examples (if available in job environment)
- **Additional packages**: Only needed for specific examples

### 2. Verify Setup

```bash
# Check RNX connection
rnx list

# Check server connectivity
rnx run echo "Hello Joblet"
```

### 3. Run Working Examples

#### Recommended: Start with Python Analytics (Always Works)

```bash
cd python-analytics/
./run_demo.sh
```

#### Basic Usage (Always Works)

```bash
cd basic-usage/
./run_demos.sh
```

#### Advanced Job Coordination (Works with Python 3)

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

### Python Analytics (`python-analytics/`)

- **Dependencies**: Python 3 standard library only
- **Features**: Sales analysis, customer segmentation, time series processing
- **Storage**: Results saved to persistent volumes
- **Status**: ✅ Always works with Python 3

### Basic Usage (`basic-usage/`)

- **Dependencies**: Shell commands only
- **Features**: File operations, resource limits, volume storage, job monitoring
- **Storage**: Temporary files and volumes
- **Status**: ✅ Always works

### Advanced Examples (`advanced/`)

- **Dependencies**: Python 3 standard library
- **Features**: Job coordination, data passing between jobs
- **Storage**: Persistent volumes for coordination
- **Status**: ✅ Works with Python 3

### Node.js Examples (`nodejs/`)

- **Dependencies**: Node.js runtime in job environment
- **Features**: System analysis, data processing with built-in modules
- **Storage**: Results to volumes if Node.js available
- **Status**: ⚠️ Requires Node.js in job environment

## 🔧 Troubleshooting

### Common Issues

#### "command not found" errors

- **Python scripts**: Ensure Python 3 is installed in job environment
- **Node.js scripts**: Node.js may not be available in job environment
- **Solution**: Use Python examples instead, or install missing runtime

#### Job failures

- **Check logs**: `rnx log <job-id>`
- **Resource limits**: Increase memory limits if needed
- **Dependencies**: Use examples with minimal dependencies

#### Volume errors

- **Storage space**: Ensure adequate disk space on server
- **Permissions**: Check Joblet server permissions
- **Cleanup**: Remove unused volumes with `rnx volume remove`

### Debug Commands

```bash
# List all jobs
rnx list

# View job output
rnx log <job-id>

# Check volumes
rnx volume list

# Monitor system
rnx monitor
```

## 📊 Expected Results

### After Running Python Analytics

```bash
# View sales analysis
rnx run --volume=analytics-data cat /volumes/analytics-data/results/sales_analysis.json

# View processed time series
rnx run --volume=analytics-data ls /volumes/analytics-data/processed/
```

### After Running Job Coordination

```bash
# View coordination results
rnx run --volume=shared-data cat /volumes/shared-data/results.json
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

- **Check Logs**: Always start with `rnx log <job-id>` for failures
- **Resource Issues**: Monitor with `rnx monitor`
- **Connectivity**: Test with simple `rnx run echo "test"` commands
- **Documentation**: See individual example README files for specific guidance