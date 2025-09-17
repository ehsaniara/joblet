# Environment Variables Examples

This directory contains comprehensive examples demonstrating Joblet's environment variable support in workflows.

## Files Overview

### Basic Examples

- **`basic-env.yaml`** - Simple environment variable usage with regular and secret variables
- **`corrected-test.yaml`** - Corrected workflow showing proper variable expansion in bash
- **`simple-test.yaml`** - Minimal test for environment variable functionality

### Real-World Use Cases

- **`data-pipeline-env.yaml`** - Data processing pipeline with environment configuration
- **`microservices-env.yaml`** - Microservices deployment with service-specific variables
- **`cicd-env.yaml`** - CI/CD pipeline with build and deployment variables
- **`ml-pipeline-env.yaml`** - Machine learning workflow with model configuration
- **`secrets-demo.yaml`** - Demonstration of secret vs regular environment variables

### Validation Examples

- **`validation-test.yaml`** - Contains conflicts to demonstrate validation (will fail)
- **`invalid-env-test.yaml`** - Contains invalid variable names (will fail)
- **`valid-validation-test.yaml`** - Valid environment variables that pass validation

## Key Features Demonstrated

### 1. Regular vs Secret Environment Variables

```yaml
jobs:
  example-job:
    environment:
      # Regular variables (visible in logs)
      NODE_ENV: "production"
      PORT: "8080"
    secret_environment:
      # Secret variables (hidden from logs)
      DATABASE_PASSWORD: "secret123"
      API_KEY: "dummy_api_key_..."
```

### 2. Variable Expansion in Commands

```yaml
jobs:
  expansion-demo:
    command: "bash"
    args: ["-c", "echo \"Environment: $NODE_ENV, Port: $PORT\""]
    environment:
      NODE_ENV: "production"
      PORT: "8080"
```

### 3. Conditional Variable Usage

```yaml
jobs:
  conditional-demo:
    command: "bash"
    args: ["-c", "echo \"Secret configured: ${SECRET_KEY:+yes}\""]
    secret_environment:
      SECRET_KEY: "secret-value"
```

## Running Examples

### Test Basic Functionality

```bash
# Run a working example
./bin/rnx job run --workflow=examples/workflows/environment-examples/basic-env.yaml

# Test validation (should fail with conflict error)
./bin/rnx job run --workflow=examples/workflows/environment-examples/validation-test.yaml

# Test with valid variables
./bin/rnx job run --workflow=examples/workflows/environment-examples/valid-validation-test.yaml
```

### Check Logs to See Environment Variables

```bash
# Check workflow status
./bin/rnx job status 1 --workflow

# View job logs to see environment variable output
./bin/rnx job log <job-id>
```

## Environment Variable Validation

Joblet validates environment variables at submission time:

### ✅ Valid Variable Names

- `VALID_VAR`
- `_UNDERSCORE_START`
- `VAR_123`
- `myApp_Config`

### ❌ Invalid Variable Names

- `123INVALID` (starts with number)
- `INVALID-VAR` (contains hyphen)
- `INVALID VAR` (contains space)
- `INVALID.VAR` (contains dot)

### Security Features

- **Conflict detection**: Same variable cannot be in both regular and secret environments
- **Reserved variable warnings**: Warns about system variables (PATH, HOME, etc.)
- **Value size limits**: Maximum 32KB per variable value
- **Security pattern detection**: Warns about potentially dangerous values

## Best Practices Demonstrated

1. **Use secret variables for sensitive data** - API keys, passwords, tokens
2. **Use regular variables for configuration** - ports, URLs, feature flags
3. **Use descriptive variable names** - Clear naming conventions
4. **Group related variables** - Logical organization in workflows
5. **Validate before deployment** - Test workflows with validation examples

## Integration with Other Features

### With Volumes

Environment variables can reference volume paths:

```yaml
environment:
  CONFIG_PATH: "/volumes/config/app.yml"
  DATA_DIR: "/volumes/data"
```

### With Networks

Configure service discovery and networking:

```yaml
environment:
  SERVICE_HOST: "api-service"
  SERVICE_PORT: "8080"
network: "microservices"
```

### With Dependencies

Pass data between jobs using environment variables and volumes:

```yaml
jobs:
  producer:
    environment:
      OUTPUT_FILE: "/volumes/shared/data.json"
  
  consumer:
    environment:
      INPUT_FILE: "/volumes/shared/data.json"
    requires:
      - producer: "COMPLETED"
```

For complete documentation, see [docs/ENVIRONMENT_VARIABLES.md](../../../docs/ENVIRONMENT_VARIABLES.md).