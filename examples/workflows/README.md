# Joblet Workflow Examples

This directory contains realistic workflow templates that demonstrate Joblet's workflow capabilities using only existing
Joblet features.

## Available Templates

### 1. `ml-pipeline/`

Complete machine learning pipeline with real ML scripts.

- **Location**: `examples/workflows/ml-pipeline/`
- **Commands**: `python3` with actual ML processing scripts
- **Features**: Data prep → Feature selection → Training → Evaluation → Deployment
- **Scripts**: 5 Python ML scripts with JSON data flow
- **Test**: `cd examples/workflows/ml-pipeline && rnx run --template=ml-pipeline.yaml:data-validation`

### 2. `data-pipeline/`

ETL data processing workflow with file uploads and shared volumes.

- **Location**: `examples/workflows/data-pipeline/`
- **Commands**: `python3` scripts, `rm` for cleanup
- **Features**: File uploads, volume mounting, resource limits
- **Scripts**: 5 Python ETL scripts with JSON data flow
- **Test**: `cd examples/workflows/data-pipeline && rnx run --template=data-pipeline.yaml:extract-data`

### 3. `web-service/`

Generic deployment workflow with build system.

- **Location**: `examples/workflows/web-service/`
- **Commands**: `make` (with real Makefile), `tar`, `echo` (simulated deployment)
- **Features**: Build → Test → Package → Deploy → Verify pattern
- **Scripts**: Real Makefile for build/test operations
- **Test**: `cd examples/workflows/web-service && rnx run --template=web-service.yaml:compile-code`

### 4. `multi-workflow/`

Multiple named workflows in one file.

- **Location**: `examples/workflows/multi-workflow/`
- **Commands**: `python3` (with real ML scripts), `tar`, `rsync`
- **Features**: Named workflow selection, ML training + deployment workflows
- **Scripts**: Same 5 ML scripts as ml-pipeline
- **Test**: `cd examples/workflows/multi-workflow && rnx run --template=multi-workflow.yaml:data-prep`

### 5. `parallel-jobs/`

Parallel batch processing without dependencies.

- **Location**: `examples/workflows/parallel-jobs/`
- **Commands**: `python3` with batch processing scripts
- **Features**: Independent parallel jobs, different processing times
- **Scripts**: 3 batch processing scripts with simulated work
- **Test**: `cd examples/workflows/parallel-jobs && rnx run --template=parallel-jobs.yaml:batch1`

## Realistic Joblet Features Used

All examples use only confirmed Joblet capabilities:

✅ **Command Execution**

- Standard Linux commands (`echo`, `python3`, `make`, `tar`, `rm`, `scp`, `ssh`)
- Command arguments and flags
- Exit code handling

✅ **Resource Management**

- `max_cpu` - CPU percentage limits
- `max_memory` - Memory limits in MB
- `max_iobps` - IO bandwidth limits

✅ **Template Features**

- YAML job definitions
- Job selection with `file.yaml:job-name`
- File uploads with `uploads.files`
- Volume mounting with `volumes`
- Runtime environments with `runtime: "python:3.11"`
- Resource limits (CPU/memory)
- Argument passing

## Current Status

**Individual Job Execution**: ✅ **Fully Working**

```bash
cd examples/workflows/ml-pipeline
rnx run --template=ml-pipeline.yaml:data-validation
```

**Workflow Orchestration**: 🔄 **Pending Integration**

```bash
cd examples/workflows/ml-pipeline  
rnx run --template=ml-pipeline.yaml
# Returns: "workflow execution is not yet fully integrated"
```

## No Docker/Container References

These examples intentionally avoid Docker, Kubernetes, or container orchestration since:

- Joblet **IS** the container replacement
- All execution happens in Joblet's native Linux isolation
- Focus on realistic command-line tools and scripts

## Running Individual Jobs

You can run individual jobs from any workflow template:

```bash
# Run specific job from workflow
rnx run --template=examples/workflows/ml-pipeline.yaml:data-validation
```

## Workflow Template Format

### Basic Structure

```yaml
version: "3.0"

jobs:
  job-name:
    command: "command-to-run"
    args: [ "arg1", "arg2" ]
    requires:
      - dependency-job: "COMPLETED"
    resources:
      max_memory: 1024
      max_cpu: 50
```

### Dependency Expressions

```yaml
requires:
  # Simple dependency
  - job-a: "COMPLETED"

  # Complex expression
  - expression: "(job-a=COMPLETED AND job-b=COMPLETED) OR job-c=FAILED"

  # IN operator
  - expression: "job-x IN (COMPLETED,FAILED,CANCELED)"
```

### Supported Job States

- `COMPLETED`: Job finished successfully
- `FAILED`: Job failed with error
- `CANCELED`: Job was canceled
- `STOPPED`: Job was stopped by user
- `RUNNING`: Job is currently executing
- `PENDING`: Job is waiting to start
- `SCHEDULED`: Job is scheduled for future execution

### Expression Operators

- `AND` or `&&`: Both conditions must be true
- `OR` or `||`: Either condition must be true
- `NOT` or `!`: Negation
- `=` or `==`: Equality
- `!=` or `<>`: Inequality
- `IN`: Value is in list
- `NOT_IN`: Value is not in list
- `()`: Grouping for precedence