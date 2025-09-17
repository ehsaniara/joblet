# Workflow Validation Test Cases

This directory contains negative test cases designed to verify that the workflow validation system correctly catches
various types of errors before workflow submission.

## Test Files

### `test-missing-volume.yaml`

- **Purpose**: Tests volume validation
- **Expected Result**: Should fail with "missing volumes: [nonexistent-volume]"
- **Usage**: `rnx job run --workflow=test-missing-volume.yaml`

### `test-missing-runtime.yaml`

- **Purpose**: Tests runtime validation
- **Expected Result**: Should fail with "missing runtimes: [nonexistent:runtime]"
- **Usage**: `rnx job run --workflow=test-missing-runtime.yaml`

### `test-circular-dependency.yaml`

- **Purpose**: Tests circular dependency detection
- **Expected Result**: Should fail with "circular dependency detected in workflow"
- **Creates**: A cycle: job-a → job-b → job-c → job-a
- **Usage**: `rnx job run --workflow=test-circular-dependency.yaml`

### `test-invalid-dependency.yaml`

- **Purpose**: Tests job dependency validation
- **Expected Result**: Should fail with "unable to create valid job ordering"
- **Issue**: References a job that doesn't exist
- **Usage**: `rnx job run --workflow=test-invalid-dependency.yaml`

### `test-valid-workflow.yaml`

- **Purpose**: Positive test case to ensure validation doesn't have false positives
- **Expected Result**: Should pass validation and create workflow successfully
- **Usage**: `rnx job run --workflow=test-valid-workflow.yaml`

### `test-missing-network.yaml`

- **Purpose**: Tests network validation
- **Expected Result**: Should fail with "missing networks: [non-existent-network]"
- **Usage**: `rnx job run --workflow=test-missing-network.yaml`

### `test-valid-builtin-networks.yaml`

- **Purpose**: Tests built-in network validation (positive test)
- **Expected Result**: Should pass validation with bridge, isolated, and none networks
- **Usage**: `rnx job run --workflow=test-valid-builtin-networks.yaml`

### `test-valid-custom-network.yaml`

- **Purpose**: Tests custom network validation (positive test)
- **Expected Result**: Should pass validation if custom-validation-test network exists
- **Prerequisites**: Run `rnx network create custom-validation-test --cidr=10.3.0.0/24` first
- **Usage**: `rnx job run --workflow=test-valid-custom-network.yaml`

### `test-network-isolation.yaml`

- **Purpose**: Tests network isolation functionality between different networks
- **Expected Result**: Should demonstrate network isolation (jobs in different networks cannot communicate)
- **Usage**: `rnx job run --workflow=test-network-isolation.yaml`

## Validation System

The workflow validation system performs the following checks before submission:

1. **Circular Dependency Detection**: Uses DFS algorithm to detect cycles in job dependencies
2. **Volume Validation**: Verifies all referenced volumes exist on the server
3. **Network Validation**: Confirms all specified networks exist (built-in: none, isolated, bridge + custom networks)
4. **Runtime Validation**: Confirms all specified runtimes are available (with name normalization)
5. **Job Dependency Validation**: Ensures all job dependencies reference existing jobs

All validation errors are caught early and provide clear error messages to help users fix their workflow definitions.