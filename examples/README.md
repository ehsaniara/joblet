# Joblet Examples

This directory contains comprehensive examples demonstrating how to use Joblet for various use cases, from simple command execution to complex agentic AI workflows.

## 📁 Directory Structure

### 🐍 [Python Analytics](./python-analytics/)
- **Sales Analysis**: Real sales data processing with pandas and matplotlib
- **Customer Segmentation**: K-means clustering with scikit-learn and model persistence
- **Distributed Feature Engineering**: Parallel data processing across multiple jobs
- **Complete Demo Suite**: Automated execution with `run_demos.sh`

### 🟨 [Node.js](./nodejs/)
- **API Testing**: Comprehensive test suite with Jest and result collection
- **Microservice Demo**: Express.js service with health checks and middleware
- **Data Processing**: CSV stream processing with transformations and reporting
- **Build Pipeline**: Complete CI/CD workflow with caching and artifact generation
- **Event Processing**: Real-time event handling with metrics and workflow triggers
- **Complete Demo Suite**: Automated execution with `run_demos.sh`

### 🤖 [Agentic AI Foundations](./agentic-ai/)
- **LLM Inference Service**: Language model inference with intelligent caching and metrics
- **Multi-Agent System**: Coordinated workflows with researcher, analyst, and writer agents
- **RAG System**: Retrieval-Augmented Generation with vector database and semantic search
- **Distributed Training**: Multi-worker ML training simulation with synchronization
- **Complete Demo Suite**: Automated execution with `run_demos.sh`

### 🎆 [Master Demo Suite](./)
- **`run_all_demos.sh`**: Execute all examples across Python, Node.js, and AI
- **`DEMO_SETUP.md`**: Complete setup and troubleshooting guide
- **Comprehensive Results**: Inspect outputs, metrics, and artifacts across all demos

### 📚 [Basic Usage](./basic-usage/)
- **Getting Started**: First commands, simple examples
- **Resource Management**: CPU, memory, I/O limits
- **File Operations**: Uploads, downloads, workspace usage
- **Networking**: Custom networks, isolation patterns

### 🚀 [Advanced Usage](./advanced/)
- **Distributed Computing**: Multi-node job coordination
- **Performance Optimization**: Resource tuning, monitoring
- **Security Patterns**: Network isolation, secure execution
- **Production Deployment**: Scaling, monitoring, maintenance

## 🎯 Quick Examples

### Run All Demos
```bash
# Execute comprehensive demo suite
cd examples/
./run_all_demos.sh
```

### Individual Demo Suites
```bash
# Python Analytics & ML
cd python-analytics/
./run_demos.sh

# Node.js Applications
cd nodejs/
./run_demos.sh

# Agentic AI Foundations
cd agentic-ai/
./run_demos.sh
```

### Sample Individual Commands
```bash
# Sales analysis with data visualization
rnx run --upload=sales_data.csv --upload=analyze_sales.py \
       --volume=analytics-data --max-memory=512 \
       python3 analyze_sales.py

# Multi-agent AI workflow
rnx run --upload=multi_agent_system.py \
       --volume=ai-outputs --max-memory=4096 \
       python3 multi_agent_system.py

# Node.js API testing suite
rnx run --upload-dir=. --volume=nodejs-projects \
       --max-memory=256 npm test
```

## 🛠️ Prerequisites

- **Joblet Server**: Running joblet daemon with proper configuration
- **RNX Client**: Configured with server connection
- **Dependencies**: Language runtimes, tools as needed by examples

## 📖 Getting Started

1. **Setup Joblet**: Follow the [Quick Start Guide](../docs/QUICKSTART.md)
2. **Choose Use Case**: Browse examples by technology or use case
3. **Run Examples**: Each directory has detailed README with step-by-step instructions
4. **Adapt & Extend**: Modify examples for your specific needs

## 📁 What You'll Get

After running the demos, you'll have comprehensive examples of:

### Python Analytics Results
- Sales trend analysis with matplotlib visualizations
- Trained K-means clustering model with customer segments
- Distributed feature engineering across multiple parallel jobs
- Performance metrics and processing statistics

### Node.js Application Results
- API test results with comprehensive coverage reports
- Running microservice with health checks and logging
- Processed CSV data with stream transformations
- Complete build pipeline with artifacts and reports
- Event processing logs with real-time metrics

### Agentic AI Results
- LLM inference responses with caching statistics
- Multi-agent workflow coordination results
- RAG system responses with source attribution
- Distributed training metrics and model artifacts
- End-to-end AI pipeline results

## 🔗 Related Documentation

- [RNX CLI Reference](../docs/RNX_CLI_REFERENCE.md) - Complete command reference
- [DEMO_SETUP.md](./DEMO_SETUP.md) - Detailed setup and troubleshooting guide
- [Configuration Guide](../docs/CONFIGURATION.md) - Server and client setup
- [Volume Management](../docs/VOLUME_MANAGEMENT.md) - Persistent storage patterns

## 📊 Monitoring Your Demo Results

```bash
# Monitor all jobs in real-time
rnx monitor

# Check job status and resource usage  
rnx list

# View volume usage across all demos
rnx volume list

# Inspect specific results
rnx run --volume=analytics-data cat /volumes/analytics-data/results/monthly_sales.csv
rnx run --volume=ai-outputs cat /volumes/ai-outputs/inference_results_*.json
rnx run --volume=nodejs-projects cat /volumes/nodejs-projects/reports/test-report.txt
```

## 🤝 Contributing Examples

Have a great use case? We'd love to include it! See each subdirectory for contribution guidelines and example templates.

## 🎆 Ready for Production

These examples demonstrate Joblet's readiness for:
- **Agentic AI Foundations**: LLM inference, multi-agent coordination, RAG systems
- **Data Science Workflows**: ML training, distributed processing, analytics pipelines  
- **Application Development**: API testing, build pipelines, microservice deployment
- **Production Patterns**: Resource management, persistent storage, comprehensive monitoring

## 📝 License

All examples are provided under the same license as Joblet. See [LICENSE](../LICENSE) for details.