# Joblet Workflow Examples

This directory contains realistic workflow templates that demonstrate Joblet's workflow capabilities using only existing
Joblet features.

## Available Templates

### 1. `ml-pipeline/`

Machine learning pipeline with real ML scripts.

- **Location**: `examples/workflows/ml-pipeline/`
- **Commands**: `python3` with actual ML processing scripts
- **Features**: Data prep → Feature selection → Training → Evaluation → Deployment
- **Scripts**: 5 Python ML scripts with JSON data flow
- **Test**: `cd examples/workflows/ml-pipeline && rnx workflow run ml-pipeline.yaml

### 2. `data-pipeline/`

ETL data processing workflow with file uploads and shared volumes.

- **Location**: `examples/workflows/data-pipeline/`
- **Commands**: `python3` scripts, `rm` for cleanup
- **Features**: File uploads, volume mounting, resource limits
- **Scripts**: 5 Python ETL scripts with JSON data flow
- **Test**: `cd examples/workflows/data-pipeline && rnx workflow run data-pipeline.yaml

### 3. `multi-workflow/`

Multiple named workflows in one file.

- **Location**: `examples/workflows/multi-workflow/`
- **Commands**: `python3` (with real ML scripts), `tar`, `rsync`
- **Features**: Named workflow selection, ML training + deployment workflows
- **Scripts**: Same 5 ML scripts as ml-pipeline
- **Test**: `cd examples/workflows/multi-workflow && rnx workflow run multi-workflow.yaml

### 4. `parallel-jobs/`

Parallel batch processing without dependencies.

- **Location**: `examples/workflows/parallel-jobs/`
- **Commands**: `python3` with batch processing scripts
- **Features**: Independent parallel jobs, different processing times
- **Scripts**: 3 batch processing scripts with simulated work
- **Test**: `cd examples/workflows/parallel-jobs && rnx workflow run parallel-jobs.yaml

### 5. `basic-usage/`

Simple workflow examples demonstrating basic functionality.

- **Location**: `examples/workflows/basic-usage/`
- **Commands**: `echo`, `python3`, `nginx`, `bash`
- **Features**: Hello world, analytics, webserver, backup jobs
- **Scripts**: Python analytics script with sample data
- **Test**: `cd examples/workflows/basic-usage && rnx workflow run basic-jobs.yaml

### 6. `tests/` - Job Names Feature Testing 🧪

Test workflows specifically for validating job names functionality and workflow features.

- **Location**: `examples/workflows/tests/`
- **Commands**: `bash`, `echo`, `sleep` for simple testing
- **Features**: Job names display, dependency visualization, CLI testing
- **Files**:
    - `test-simple-workflow.yaml` - 2-job workflow for basic job names testing
    - `test-workflow-names.yaml` - 4-job workflow for comprehensive job names testing
    - `demo-workflow.yaml` - Basic 3-step sequential workflow
- **Test**: `cd examples/workflows/tests && rnx workflow run test-workflow-names.yaml`
- **Purpose**: Validate that workflow jobs display proper job IDs vs job names in CLI

### 7. `environment-examples/`

Environment variable configuration examples using inline code.

- **Location**: `examples/workflows/environment-examples/`
- **Commands**: `bash`, `python3` with inline code (`-c` flag)
- **Features**: Environment variables, secret variables, configuration management
- **Scripts**: All examples use inline code (no external script files)
- **Test**: `cd examples/workflows/environment-examples && rnx workflow run basic-env.yaml`
- **Purpose**: Demonstrate environment variable features without requiring external files

### 8. `verification/`

Negative test cases for workflow validation.

- **Location**: `examples/workflows/verification/`
- **Purpose**: Test error scenarios (missing volumes, circular dependencies, etc.)
- **Features**: Validation testing, error handling demonstrations
- **Test**: `cd examples/workflows/verification && rnx workflow run test-missing-volume.yaml` (should fail)

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
- Runtime environments with `runtime: "python-3.11"`
- Resource limits (CPU/memory)
- Argument passing

## Current Status

**Individual Job Execution**: ✅ **Fully Working**

```bash
cd examples/workflows/ml-pipeline
rnx workflow run ml-pipeline.yaml
```

**Workflow Orchestration**: ✅ **Fully Working** (NEW - Consolidated Commands)

```bash
cd examples/workflows/ml-pipeline  
rnx workflow run ml-pipeline.yaml     # ✅ Unified workflow execution
rnx job status <workflow-id>                # ✅ Unified status checking
```

**Current Commands**:

```bash
rnx workflow run ml-pipeline.yaml     # Run a workflow from YAML file
rnx job status <id>                         # Check workflow or job status
rnx workflow list                     # List all workflows
```

## Native Job Execution

These examples use Joblet's native Linux process isolation:

- All execution happens in Joblet's isolated environments
- Direct command-line tools and scripts
- Focus on realistic system administration and development workflows

## Running Individual Jobs

You can run individual jobs from any workflow template:

```bash
# Run specific job from workflow
rnx workflow run examples/workflows/ml-pipeline.yaml
```

## Workflow File Format

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