# Advanced Joblet Examples

Advanced patterns for orchestrating multiple jobs with dependencies and data flow.

## 📚 Overview

This example demonstrates sophisticated job coordination patterns using Joblet's isolated execution environment.

| Example                               | File                  | Description                           | Complexity | Resources |
|---------------------------------------|-----------------------|---------------------------------------|------------|-----------|
| [Job Coordination](#job-coordination) | `job_coordination.sh` | Multi-job workflows with dependencies | Advanced   | 256MB RAM |

## 🚀 Quick Start

```bash
# Run the job coordination example
./job_coordination.sh
```

This example works with Python 3 standard library only - no external dependencies required!

## 🔗 Job Coordination

Advanced patterns for orchestrating multiple jobs with dependencies and data flow.

### File Included

- **`job_coordination.sh`**: Complete working example of job coordination

### What It Demonstrates

- **Sequential Processing**: Jobs that depend on previous job outputs
- **Volume Sharing**: Using persistent volumes to pass data between jobs
- **Job Synchronization**: Proper timing and dependency management
- **Data Exchange**: Structured data passing using JSON format
- **Error Handling**: Basic error checking and validation

### How It Works

The example runs two coordinated jobs:

1. **Job 1 - Data Generation**:
    - Generates 100 random data records
    - Saves data as JSON to shared volume
    - Uses Python standard library only

2. **Job 2 - Data Analysis**:
    - Waits for Job 1 to complete
    - Reads data from shared volume
    - Performs statistical analysis (mean, median, standard deviation)
    - Saves results back to shared volume

### Usage

```bash
./job_coordination.sh
```

The script will:

1. Create a shared volume for data exchange
2. Submit the data generation job
3. Wait for completion
4. Submit the analysis job that depends on the first job's output
5. Display commands to view the results

### Expected Output

```
🔗 Simple Job Coordination Demo
===============================

📦 Creating shared volume...
Volume created successfully: shared-data

📝 Job 1: Generate data
Job started: ID: X

⏳ Waiting for Job 1 to complete...

📊 Job 2: Process data (depends on Job 1)
Job started: ID: Y

⏳ Waiting for Job 2 to complete...

✅ Job Coordination Complete!

📊 View results:
  rnx run --volume=shared-data cat /volumes/shared-data/data.json
  rnx run --volume=shared-data cat /volumes/shared-data/results.json
```

### Viewing Results

After the demo completes, you can inspect the coordinated job results:

```bash
# View the generated data
rnx run --volume=shared-data cat /volumes/shared-data/data.json

# View the analysis results
rnx run --volume=shared-data cat /volumes/shared-data/results.json
```

The analysis results will show statistical summary:

- Record count
- Sum of all values
- Mean, median, and standard deviation

## 💡 Key Concepts Demonstrated

### Job Dependencies

- **Sequential Execution**: Job 2 only starts after Job 1 completes
- **Data Validation**: Job 2 checks that Job 1's output exists before proceeding
- **Error Propagation**: Failed dependencies prevent downstream jobs from running

### Volume-Based Communication

- **Shared Storage**: Jobs communicate through persistent volumes
- **Structured Data**: JSON format ensures reliable data exchange
- **File Organization**: Clear naming conventions for data files

### Resource Management

- **Memory Limits**: Each job runs with appropriate memory constraints
- **Isolation**: Jobs run in separate isolated environments
- **Volume Lifecycle**: Shared volumes persist across job executions

## 🚀 Extending the Example

### Add More Jobs

```bash
# Add a third job that further processes the results
rnx run --volume=shared-data --max-memory=256 \
    python3 -c "
import json

# Read analysis results
with open('/volumes/shared-data/results.json', 'r') as f:
    results = json.load(f)

# Create summary report
report = {
    'summary': f'Processed {results[\"count\"]} records',
    'average_value': results['mean'],
    'data_spread': results['stdev']
}

with open('/volumes/shared-data/report.json', 'w') as f:
    json.dump(report, f, indent=2)

print('Summary report generated')
"
```

### Handle Larger Datasets

- Increase volume sizes for bigger datasets
- Adjust memory limits based on data size
- Consider data partitioning for very large datasets

### Add Error Handling

- Check job exit codes before proceeding
- Implement retry logic for failed jobs
- Add validation for data format and content

## 📚 Additional Resources

- [Basic Usage Examples](../basic-usage/) - Fundamental Joblet patterns
- [Python Analytics](../python-analytics/) - Data processing examples
- [Joblet Documentation](../../docs/) - Core concepts and configuration