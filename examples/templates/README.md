# Joblet YAML Templates

This directory contains example YAML template files for running jobs with Joblet using the `--template` flag.

## Usage

Run a job using a template:

```bash
# Run a specific job from a template file
rnx run --template=basic-jobs.yaml:analytics

# If the template has only one job, you can omit the job name
rnx run --template=single-job.yaml

# Override template settings with command-line flags
rnx run --template=basic-jobs.yaml:webserver --max-memory=256 --network=custom

# Pass additional arguments to the command
rnx run --template=ml-pipeline.yaml:train -- --epochs=200 --batch-size=64
```

## Template Structure

Each YAML template file follows this structure:

```yaml
version: "1.0"

# Optional: Default settings for all jobs
defaults:
  resources:
    max_memory: 256
    max_cpu: 50
  network: bridge
  runtime: "python:3.11"

# Job definitions
jobs:
  job-name:
    name: "Human-readable name"
    description: "Job description"
    command: "command-to-run"
    args:
      - "arg1"
      - "arg2"
    resources:
      max_memory: 512      # MB
      max_cpu: 75          # Percentage
      max_iobps: 1048576   # Bytes per second
      cpu_cores: "0-3"     # CPU core binding
    uploads:
      files:
        - "file1.py"
        - "file2.json"
      directories:
        - "scripts/"
    volumes:
      - "data-volume"
      - "logs-volume"
    network: "custom-network"
    runtime: "python:3.11-ml"
    schedule: "1hour"      # Optional scheduling
```

## Available Templates

### 1. basic-jobs.yaml

Simple example jobs demonstrating common patterns:

- `hello` - Simple echo command
- `analytics` - Python data analysis with file uploads
- `webserver` - Long-running nginx server
- `backup` - Scheduled backup job

### 2. ml-pipeline.yaml

Machine learning pipeline jobs:

- `preprocess` - Data preprocessing
- `train` - Model training with GPU support
- `evaluate` - Model evaluation
- `inference` - Model serving
- `pipeline` - Complete ML pipeline

### 3. java-services.yaml

Java microservices configuration:

- `api-gateway` - Spring Cloud Gateway
- `user-service` - User management service
- `order-service` - Order processing
- `payment-service` - Payment processing with high resources
- `db-migration` - Database migrations with Flyway
- `batch-processor` - Scheduled batch processing

### 4. data-pipeline.yaml

Data processing and ETL jobs:

- `etl` - Complete ETL pipeline
- `validate` - Data validation
- `compress` - Data compression for archival
- `db-backup` - Database backup
- `report` - Report generation
- `sync` - Data synchronization
- `cleanup` - Cleanup old files and logs

## Creating Your Own Templates

1. Create a new YAML file following the structure above
2. Define your jobs with appropriate resource limits
3. Use the template with: `rnx run --template=your-template.yaml:job-name`

## Best Practices

1. **Use defaults**: Define common settings in the `defaults` section
2. **Set resource limits**: Always specify appropriate memory and CPU limits
3. **Use volumes**: For persistent data that needs to survive job completion
4. **Network isolation**: Use custom networks for related services
5. **Schedule wisely**: Use scheduling for recurring tasks
6. **Version control**: Keep your templates in version control

## Advanced Features

### Job Inheritance

Jobs can extend other jobs using the `extends` field:

```yaml
jobs:
  base-python:
    runtime: "python:3.11"
    resources:
      max_memory: 512

  analytics:
    extends: base-python
    command: python3
    args: [ "analyze.py" ]
```

### Environment Variables

Pass environment variables (if supported):

```yaml
jobs:
  webapp:
    command: node
    args: [ "app.js" ]
    environment:
      NODE_ENV: "production"
      PORT: "3000"
```

### Multi-line Commands

Use YAML's multi-line string syntax for complex commands:

```yaml
jobs:
  complex:
    command: bash
    args:
      - "-c"
      - |
        echo "Starting complex job..."
        step1.sh
        step2.sh
        echo "Job completed!"
```

## Troubleshooting

- **Job not found**: Ensure you specify the correct job name after the colon
- **File not found**: Check that upload paths are relative to current directory
- **Resource limits**: Ensure limits don't exceed available system resources
- **Network issues**: Verify network names match existing Joblet networks

## Examples

```bash
# Run Python analytics job
rnx run --template=basic-jobs.yaml:analytics

# Run ML training with custom epochs
rnx run --template=ml-pipeline.yaml:train -- --epochs=200

# Run Java service with override
rnx run --template=java-services.yaml:api-gateway --max-memory=1024

# Run scheduled ETL pipeline
rnx run --template=data-pipeline.yaml:etl

# Run cleanup job immediately (override schedule)
rnx run --template=data-pipeline.yaml:cleanup --schedule=""
```