# Workflow Test Examples

This directory contains test workflow YAML files for validating specific features and functionality of the Joblet
workflow system.

## Test Workflows

### 🚀 Basic Functionality Tests

#### `demo-workflow.yaml`

**Purpose**: Basic 3-step sequential workflow demonstration

- **Jobs**: step1 → step2 → step3
- **Features**: Simple dependency chain, basic commands
- **Usage**: `rnx job run --workflow=demo-workflow.yaml`

#### `test-simple-workflow.yaml`

**Purpose**: Job names feature validation with 2-job workflow

- **Jobs**: job-one → job-two
- **Features**: Job names display, dependency tracking
- **Usage**: `rnx job run --workflow=test-simple-workflow.yaml`
- **Validation**: Tests job ID vs job name display in CLI

### 🔍 Job Names Feature Tests

#### `test-workflow-names.yaml`

**Purpose**: Comprehensive job names testing with 4-job dependency chain

- **Jobs**: setup-data → process-data → validate-results → generate-report
- **Features**:
    - Human-readable job names from YAML keys
    - Complex dependency relationships
    - Environment variables (regular and secret)
    - Resource limits per job
- **Usage**: `rnx job run --workflow=test-workflow-names.yaml`
- **Validation**:
    - Job names appear in workflow status display
    - Dependencies show job name relationships
    - CLI displays both job IDs and job names correctly

## Testing Job Names Feature

The job names feature allows workflow jobs to have names derived from YAML job keys:

### Expected CLI Output Format

```bash
# rnx job status --workflow 1
Jobs in Workflow:
-----------------------------------------------------------------------------------------
JOB ID          JOB NAME             STATUS       EXIT CODE  DEPENDENCIES        
-----------------------------------------------------------------------------------------
42              setup-data           COMPLETED    0          -                   
43              process-data         RUNNING      -          setup-data          
0               validate-results     PENDING      -          process-data        
0               generate-report      PENDING      -          validate-results    
```

### Key Distinctions

- **JOB ID**: Actual unique identifier for started jobs (e.g., "42", "43"), "0" for non-started jobs
- **JOB NAME**: Name from YAML (e.g., "setup-data", "process-data")
- **DEPENDENCIES**: Lists job names for clarity (not job IDs)

## Running Tests

### Individual Test Execution

```bash
# Test basic job names with 2 jobs
rnx job run --workflow=examples/workflows/tests/test-simple-workflow.yaml
rnx job status --workflow <workflow-id>

# Test comprehensive job names with 4 jobs
rnx job run --workflow=examples/workflows/tests/test-workflow-names.yaml
rnx job status --workflow <workflow-id>

# Basic workflow demo
rnx job run --workflow=examples/workflows/tests/demo-workflow.yaml
rnx job status --workflow <workflow-id>
```

### Batch Testing

```bash
# Run all test workflows
cd examples/workflows/tests/
for file in *.yaml; do
    echo "Testing workflow: $file"
    rnx job run --workflow="$file"
    sleep 10  # Allow time for completion
done
```

## Validation Checklist

When testing job names functionality:

### ✅ CLI Display Validation

- [ ] JOB ID column shows actual job IDs for started jobs, "0" for non-started jobs
- [ ] JOB NAME column shows YAML job names
- [ ] DEPENDENCIES column shows job name relationships
- [ ] Status colors work correctly
- [ ] JSON output includes both `id` and `name` fields

### ✅ Workflow Execution Validation

- [ ] Jobs execute in correct dependency order
- [ ] Job names are preserved throughout execution
- [ ] Status updates work with actual job IDs
- [ ] Workflow completion status is accurate

### ✅ Error Handling Validation

- [ ] Failed job startup shows job names correctly
- [ ] Cancelled workflows display proper job status
- [ ] Job ID mapping failures are handled gracefully

## Integration with Main Examples

These test workflows complement the main workflow examples in:

- `/examples/workflows/data-pipeline/` - Production data pipeline example
- `/examples/workflows/ml-pipeline/` - Machine learning workflow example
- `/examples/workflows/verification/` - Validation and error condition tests

## Contributing Test Workflows

When adding new test workflows:

1. **Use descriptive job names** that clearly indicate their purpose
2. **Include resource limits** to test job isolation
3. **Add environment variables** to test variable handling
4. **Document the test purpose** in this README
5. **Follow YAML version "3.0"** format for consistency