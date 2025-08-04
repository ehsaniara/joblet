# Joblet Demo Setup Guide

This guide helps you run the comprehensive Joblet examples that demonstrate Python analytics, Node.js applications, and agentic AI foundations.

## Quick Start

### 1. Prerequisites

- **Joblet Server**: Running joblet daemon
- **RNX Client**: Configured and connected to server
- **Python 3.8+**: For Python examples  
- **Node.js 16+**: For Node.js examples
- **Dependencies**: Will be installed automatically during demos

### 2. Verify Setup

```bash
# Check RNX connection
rnx list

# Check system status
rnx monitor status
```

### 3. Run All Demos

```bash
# Run comprehensive demo suite
cd examples/
./run_all_demos.sh
```

### 4. Run Individual Demos

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

## Demo Contents

### 🐍 Python Analytics (`python-analytics/`)

**Files Created:**
- `analyze_sales.py` - Sales data analysis with pandas
- `train_clustering.py` - ML customer segmentation  
- `feature_engineering.py` - Distributed data processing
- `sales_data.csv` - Sample sales dataset
- `customers.csv` - Sample customer data
- `requirements.txt` - Python dependencies

**What It Demonstrates:**
- Data analysis with pandas and matplotlib
- Machine learning with scikit-learn
- Distributed processing across multiple jobs
- Persistent storage of results and models
- Resource management (CPU, memory limits)

### 🟨 Node.js (`nodejs/`)

**Files Created:**
- `package.json` - Node.js project configuration
- `app.js` - Express.js microservice
- `api.test.js` - Comprehensive API test suite
- `process_data.js` - CSV data processing
- `event-processor.js` - Real-time event handling
- `build-pipeline.sh` - Complete CI/CD pipeline

**What It Demonstrates:**
- API testing and validation
- Microservice deployment patterns
- Data processing with streams
- Build automation and CI/CD
- Event-driven architecture
- Error handling and logging

### 🤖 Agentic AI (`agentic-ai/`)

**Files Created:**
- `llm_inference.py` - LLM service with caching
- `multi_agent_system.py` - Coordinated multi-agent workflow
- `rag_system.py` - RAG with vector database
- `distributed_training.py` - Distributed ML training
- `requirements.txt` - AI/ML dependencies

**What It Demonstrates:**
- LLM inference with intelligent caching
- Multi-agent coordination and task distribution
- Semantic search and retrieval-augmented generation
- Distributed machine learning training
- End-to-end AI pipeline orchestration
- Performance monitoring and metrics collection

## Expected Outputs

### Volume Structure
After running demos, you'll have:

```
/volumes/
├── analytics-data/          # Python analytics results
│   ├── results/            # Analysis outputs
│   ├── processed/          # Feature engineering results
│   └── raw/               # Input data chunks
├── ml-models/              # Machine learning artifacts
│   ├── clustering_model_*.pkl
│   ├── model_metadata_*.json
│   └── cluster_summary_*.csv
├── nodejs-projects/        # Node.js outputs
│   ├── reports/           # Test and build reports
│   ├── builds/            # Build artifacts
│   ├── events/            # Event processing logs
│   └── logs/              # Application logs
├── ai-cache/              # AI inference cache
├── ai-outputs/            # AI processing results
├── ai-metrics/            # AI performance metrics
└── ai-models/             # AI model artifacts
```

### Sample Results

**Python Analytics:**
- Monthly sales trend visualization
- Customer segmentation model (K-means)  
- Feature engineering pipeline results
- Performance metrics and processing reports

**Node.js Applications:**
- API test results with coverage metrics
- Build pipeline artifacts and reports
- Event processing logs and statistics
- Data transformation outputs

**Agentic AI:**
- LLM inference results with caching stats
- Multi-agent coordination workflow outputs
- RAG system responses with source attribution
- Distributed training metrics and model artifacts

## Monitoring & Inspection

### View Job Status
```bash
# List all jobs
rnx list

# Monitor system in real-time
rnx monitor

# Check specific job logs
rnx log <job-id>
```

### Inspect Results
```bash
# View volume contents
rnx volume list

# Browse analytics results
rnx run --volume=analytics-data ls -la /volumes/analytics-data/

# Check AI outputs
rnx run --volume=ai-outputs find /volumes/ai-outputs -name "*.json"

# Read specific results
rnx run --volume=ai-outputs cat /volumes/ai-outputs/inference_results_*.json
```

### Performance Analysis
```bash
# System metrics
rnx monitor status --json

# Resource utilization
rnx list --json | jq '.[] | {id, status, max_memory, max_cpu}'

# Volume usage
rnx volume list --json | jq '.[] | {name, size_used, size_total}'
```

## Troubleshooting

### Common Issues

**Connection Problems:**
```bash
# Check RNX configuration
rnx nodes

# Test connection
rnx run echo "test"
```

**Volume Issues:**
```bash
# Check volume status
rnx volume list

# Recreate if needed
rnx volume remove analytics-data
rnx volume create analytics-data --size=2GB --type=filesystem
```

**Job Failures:**
```bash
# Check job logs
rnx log <job-id>

# Check system resources
rnx monitor status
```

### Cleanup

```bash
# Remove demo volumes (optional)
rnx volume remove analytics-data ml-models nodejs-projects
rnx volume remove ai-cache ai-outputs ai-metrics ai-models

# Stop running jobs
rnx list | grep RUNNING | awk '{print $1}' | xargs -I {} rnx stop {}
```

## Next Steps

1. **Customize Examples**: Modify scripts for your specific use cases
2. **Scale Up**: Increase resource limits and data sizes
3. **Production Deployment**: Implement monitoring, alerting, and CI/CD
4. **Integration**: Connect with your existing data and AI infrastructure

## Support

- Review individual README files in each demo directory
- Check [Joblet Documentation](../docs/) for detailed configuration
- Examine job logs with `rnx log <job-id>` for troubleshooting