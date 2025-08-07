# Node.js Examples

Node.js examples for Joblet environments that have Node.js installed.

## 📚 Overview

This directory contains Node.js examples that demonstrate running JavaScript applications in Joblet's isolated environment.

**⚠️ Note:** These examples require Node.js to be installed in the Joblet server environment. If Node.js is not available, you'll need to either install it or use a container image that includes Node.js.

| Example | Files | Description | Complexity | Resources |
|---------|-------|-------------|------------|-----------|
| [Node.js Demo](#nodejs-demo) | `simple_nodejs.js`, `run_simple_demo.sh` | Node.js with built-in modules | Beginner | 256MB RAM |

## 📋 Prerequisites

**Required:**
- Joblet server with Node.js installed
- RNX client configured and connected
- 256MB RAM available for jobs

**Node.js Availability:**
If Node.js is not installed in your Joblet environment, you have several options:
1. Install Node.js in the Joblet server environment
2. Use a container image that includes Node.js
3. Try the [Python Analytics](../python-analytics/) examples instead (Python is more commonly available)

## 🟢 Node.js Demo

Demonstrates Node.js capabilities using only built-in modules.

### Files Included

- **`simple_nodejs.js`**: Complete Node.js application using built-in modules
- **`run_simple_demo.sh`**: Demo execution script

### What It Demonstrates

- **System Analysis**: Using `os` module to gather system information
- **Data Processing**: JSON manipulation and statistical analysis
- **File Operations**: Reading and writing files with `fs` module
- **Process Information**: Accessing process details and environment
- **Volume Integration**: Saving results to persistent volumes

### Usage

```bash
./run_simple_demo.sh
```

If Node.js is available, this will:
1. Create a volume for storing results
2. Upload the Node.js script to the job environment
3. Execute the script with resource limits
4. Display commands to view the results

### Expected Behavior

**If Node.js is available:**
- The script will execute and generate system analysis
- Results will be saved to the nodejs-data volume
- You can view results with the provided commands

**If Node.js is not available:**
- The job will fail with "command node not found"
- Consider using Python examples instead, or install Node.js

## 💡 Key Concepts (When Working)

### Built-in Modules

- **File System (`fs`)**: File creation, reading, writing, and deletion
- **Operating System (`os`)**: System information and resource monitoring
- **Path (`path`)**: Cross-platform path manipulation
- **Process (`process`)**: Runtime information and environment access

### Joblet Integration

- **Volume Usage**: Persistent storage for results and data
- **Resource Limits**: Running within memory constraints
- **Isolation**: Understanding the isolated execution environment
- **File Upload**: Getting scripts into the job workspace

## 🔧 Troubleshooting

### "command node not found"

This error means Node.js is not installed in the Joblet server environment. Solutions:

1. **Install Node.js**: Add Node.js to the Joblet server
2. **Use Container**: Deploy with a Node.js-enabled container image
3. **Try Python**: Use the [Python Analytics](../python-analytics/) examples instead

### Permission Issues

If you encounter permission errors:
```bash
chmod +x run_simple_demo.sh
```

## 🚀 Alternative Examples

If Node.js is not available in your environment, try these working examples:

- **[Python Analytics](../python-analytics/)** - Data processing with Python standard library
- **[Basic Usage](../basic-usage/)** - Shell commands and basic operations
- **[Advanced](../advanced/)** - Job coordination patterns

## 📚 Additional Resources

- [Node.js Built-in Modules](https://nodejs.org/api/)
- [Basic Usage Examples](../basic-usage/) - Fundamental Joblet patterns
- [Python Analytics](../python-analytics/) - Alternative data processing examples
- [Joblet Documentation](../../docs/) - Core concepts and configuration