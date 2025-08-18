# Environment Variables

Joblet provides comprehensive support for environment variables in both CLI jobs and workflow-based jobs. Environment variables allow you to pass configuration, secrets, and runtime parameters to your jobs in a secure and flexible manner.

## Table of Contents

- [Overview](#overview)
- [CLI Usage](#cli-usage)
- [Workflow Usage](#workflow-usage)
- [Security Features](#security-features)
- [Validation](#validation)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Advanced Use Cases](#advanced-use-cases)

## Overview

Joblet supports two types of environment variables:

1. **Regular Environment Variables** (`--env`, `environment`): Visible in logs and debugging output
2. **Secret Environment Variables** (`--secret-env`, `secret_environment`): Hidden from logs for security

### Key Features

- ✅ **Dual environment variable types** (regular vs secret)
- ✅ **Multiple input methods** (command line flags, workflow YAML)
- ✅ **Workflow inheritance** (global variables inherited by jobs with override support)
- ✅ **Variable templating** (`${VAR_NAME}` syntax for referencing other variables)
- ✅ **Status display masking** (secret variables shown as `***` in status output)
- ✅ **Comprehensive validation** (name format, value size, conflict detection)
- ✅ **Security-first design** (secret variables hidden from logs)
- ✅ **Reserved variable warnings** (system variables like PATH, HOME)

## CLI Usage

### Basic Syntax

```bash
# Regular environment variables (visible in logs)
rnx run --env="KEY=value" command

# Secret environment variables (hidden from logs)
rnx run --secret-env="SECRET_KEY=secret_value" command

# Multiple variables
rnx run --env="VAR1=value1" --env="VAR2=value2" command

# Mixed usage
rnx run --env="PUBLIC_VAR=public" --secret-env="SECRET_KEY=secret" command
```

### Short Flags

```bash
# Short flags available
rnx run -e "VAR=value" -s "SECRET=secret" command
```


### CLI Examples

```bash
# Simple web application
rnx run --env="PORT=8080" --secret-env="DB_PASSWORD=secret123" node server.js

# Data processing job
rnx run --env="INPUT_FILE=data.csv" --env="OUTPUT_DIR=/results" python process.py

# Machine learning job
rnx run --env="MODEL_TYPE=xgboost" --secret-env="MODEL_API_KEY=secret" python train.py

# Development vs production
rnx run --env="NODE_ENV=development" --env="DEBUG=true" npm start
rnx run --env="NODE_ENV=production" --env="DEBUG=false" npm start
```

## Workflow Usage

### YAML Syntax

Environment variables in workflows are defined at the job level:

```yaml
version: "3.0"

jobs:
  my-job:
    command: "python"
    args: ["app.py"]
    
    # Regular environment variables (visible in logs)
    environment:
      APP_NAME: "my-application"
      NODE_ENV: "production"
      PORT: "8080"
      DEBUG_MODE: "false"
    
    # Secret environment variables (hidden from logs)
    secret_environment:
      DATABASE_PASSWORD: "super_secret_password"
      API_KEY: "dummy_api_key_example"
      JWT_SECRET: "your_jwt_signing_secret"
    
    resources:
      max_memory: 512
```

### Variable Expansion

Environment variables support shell expansion in workflow commands:

```yaml
jobs:
  process-data:
    command: "bash"
    args: ["-c", "echo \"Processing $INPUT_FILE to $OUTPUT_DIR\""]
    environment:
      INPUT_FILE: "data.csv"
      OUTPUT_DIR: "/results"
```

### Conditional Variables

Use shell parameter expansion for conditional logic:

```yaml
jobs:
  conditional-job:
    command: "bash"
    args: ["-c", "echo \"Debug mode: ${DEBUG_MODE:+enabled}\"; echo \"Secret configured: ${SECRET_KEY:+yes}\""]
    environment:
      DEBUG_MODE: "true"
    secret_environment:
      SECRET_KEY: "secret_value"
```

## Workflow Environment Variable Inheritance

Joblet supports global environment variables that are inherited by all jobs in a workflow, with job-specific variables taking precedence.

### Global Workflow Variables

Define global environment variables at the workflow level:

```yaml
version: "3.0"

# Global environment variables inherited by all jobs
environment:
  PIPELINE_ID: "prod-001"
  SHARED_CONFIG: "global-config-value"
  BASE_URL: "https://api.production.com"

# Global secret environment variables inherited by all jobs  
secret_environment:
  PIPELINE_SECRET: "global-pipeline-secret"
  SHARED_API_KEY: "shared-secret-key"

jobs:
  setup:
    command: "python"
    args: ["setup.py"]
    # Inherits: PIPELINE_ID, SHARED_CONFIG, BASE_URL, PIPELINE_SECRET, SHARED_API_KEY
    environment:
      STAGE: "setup"           # Job-specific variable
    
  process:
    command: "python"
    args: ["process.py"]
    # Inherits all global variables
    environment:
      STAGE: "processing"      # Job-specific variable
      SHARED_CONFIG: "override-config"  # Overrides global value
    secret_environment:
      PROCESS_SECRET: "process-specific-secret"
```

### Inheritance Rules

1. **Job variables override global variables** - If the same variable is defined globally and at the job level, the job value takes precedence
2. **Both regular and secret variables are inherited** - Global environment and secret_environment are both inherited
3. **Independent inheritance** - Regular and secret environments are inherited separately

### Example: Data Pipeline with Inheritance

```yaml
version: "3.0"

# Global configuration for entire pipeline
environment:
  PIPELINE_ID: "data-pipeline-v2"
  LOG_LEVEL: "INFO"
  BATCH_SIZE: "1000"
  OUTPUT_FORMAT: "parquet"

secret_environment:
  GLOBAL_API_KEY: "global-secret-key"
  MONITORING_TOKEN: "monitoring-secret"

jobs:
  extract:
    command: "python"
    args: ["extract.py"]
    # Inherits all global variables
    environment:
      STAGE: "extract"
      DATA_SOURCE: "production_db"
    secret_environment:
      DB_PASSWORD: "extract-db-secret"
      
  transform:
    command: "python"
    args: ["transform.py"]
    # Inherits global variables, overrides some
    environment:
      STAGE: "transform"
      BATCH_SIZE: "500"        # Override global batch size
      VALIDATION_LEVEL: "strict"
    
  load:
    command: "python"
    args: ["load.py"]
    environment:
      STAGE: "load"
      TARGET_TABLE: "processed_data"
    secret_environment:
      WAREHOUSE_PASSWORD: "warehouse-secret"
    requires:
      - transform: "COMPLETED"
```

## Environment Variable Templating

Environment variables support `${VAR_NAME}` templating syntax for referencing other environment variables within the same job.

### Templating Syntax

Use `${VARIABLE_NAME}` to reference other environment variables:

```yaml
jobs:
  templating-example:
    command: "bash"
    args: ["-c", "echo Processing $INPUT_FILE to $OUTPUT_FILE"]
    environment:
      BASE_PATH: "/opt/data"
      PROJECT: "analytics"
      VERSION: "1.0.0"
      
      # Variables using templating
      WORK_DIR: "${BASE_PATH}/work"
      INPUT_FILE: "${BASE_PATH}/${PROJECT}/input.csv"
      OUTPUT_DIR: "${BASE_PATH}/${PROJECT}/v${VERSION}/output"
      CONFIG_FILE: "${BASE_PATH}/${PROJECT}/config.yml"
      
    secret_environment:
      API_KEY: "secret-api-key"
      # Secret variables can also use templating
      SECRET_CONFIG: "${BASE_PATH}/secrets/${API_KEY}.conf"
```

### Advanced Templating Examples

```yaml
jobs:
  advanced-templating:
    command: "python"
    args: ["process.py"]
    environment:
      # Base configuration
      ENVIRONMENT: "production"
      SERVICE_NAME: "data-processor"
      VERSION: "2.1.0"
      
      # Templated paths and URLs
      LOG_FILE: "/var/log/${SERVICE_NAME}/${ENVIRONMENT}.log"
      CONFIG_PATH: "/etc/${SERVICE_NAME}/${ENVIRONMENT}/config.yml"
      METRICS_ENDPOINT: "https://metrics.${ENVIRONMENT}.company.com/${SERVICE_NAME}"
      
      # Version-specific paths
      BINARY_PATH: "/opt/${SERVICE_NAME}/v${VERSION}/bin"
      LIB_PATH: "/opt/${SERVICE_NAME}/v${VERSION}/lib"
      
    secret_environment:
      DB_HOST: "db.${ENVIRONMENT}.internal"
      SECRET_KEY_FILE: "/secrets/${SERVICE_NAME}/${ENVIRONMENT}/key.pem"
```

### Cross-Reference Between Variable Types

Regular and secret environment variables can reference each other:

```yaml
jobs:
  cross-reference:
    command: "app"
    environment:
      APP_NAME: "payment-service"
      ENVIRONMENT: "production"
      # Can reference secret variables
      CONFIG_FILE: "/config/${APP_NAME}/${DB_NAME}.yml"
      
    secret_environment:
      DB_NAME: "payments_prod"
      # Can reference regular variables  
      DB_CONNECTION: "postgresql://${APP_NAME}:${DB_PASSWORD}@${DB_HOST}:5432/${DB_NAME}"
      DB_PASSWORD: "secret-password"
      DB_HOST: "db.${ENVIRONMENT}.internal"
```

### Templating Rules

1. **Variable resolution order** - Variables are resolved in the order they appear in the environment maps
2. **Undefined variables remain unchanged** - `${UNDEFINED_VAR}` stays as literal text if the variable doesn't exist
3. **Cross-type references supported** - Regular variables can reference secret variables and vice versa
4. **No recursive resolution** - Variable references are resolved once, not recursively

## Status Display Masking

When viewing job status, secret environment variables are automatically masked for security:

### Status Display Examples

```bash
# Regular job status shows environment variables
rnx status job-123

Job Details:
ID: job-123
Status: COMPLETED
Command: python app.py

Environment Variables:
  NODE_ENV=production
  PORT=8080
  DEBUG_MODE=false
  
  API_KEY=*** (secret)
  DB_PASSWORD=*** (secret)
```

### Status Display Features

- **Regular variables**: Displayed with full key=value pairs
- **Secret variables**: Keys shown, values masked as `***` with `(secret)` label
- **Empty environments**: Section omitted if no variables are present
- **Consistent masking**: Same masking applied in web UI, CLI, and API responses

## Security Features

### Secret Environment Variables

Secret environment variables are designed for sensitive data:

```bash
# CLI: Secret variables hidden from logs
rnx run --secret-env="DB_PASSWORD=secret123" --secret-env="API_KEY=dummy_api_key_abc" python app.py
```

```yaml
# Workflow: Secret variables hidden from logs
jobs:
  secure-job:
    command: "python"
    args: ["secure_app.py"]
    secret_environment:
      DATABASE_URL: "postgresql://user:password@db:5432/app"
      STRIPE_SECRET_KEY: "dummy_stripe_key_example"
      JWT_SIGNING_KEY: "your_super_secret_jwt_key"
```

### Logging Behavior

```bash
# Regular variables: Keys and values visible in debug logs
[DEBUG] adding regular environment variables | variables=["NODE_ENV", "PORT", "DEBUG_MODE"]

# Secret variables: Only count shown in logs
[DEBUG] adding secret environment variables | count=3
```

### Security Validation

Joblet performs security checks on environment variable values:

- **Command injection detection**: Warns about `$()`, backticks
- **Dangerous commands**: Warns about `rm -rf`, `del /f`
- **Path traversal**: Warns about `../` patterns
- **System file access**: Warns about `/etc/shadow`, `passwd`

## Validation

### Variable Name Validation

Environment variable names must follow POSIX standards:

✅ **Valid names:**
```bash
--env="VALID_VAR=value"
--env="_UNDERSCORE_START=value"
--env="VAR_123=value"
--env="myApp_Config=value"
```

❌ **Invalid names:**
```bash
--env="123INVALID=value"        # Cannot start with number
--env="INVALID-VAR=value"       # Cannot contain hyphens
--env="INVALID VAR=value"       # Cannot contain spaces
--env="INVALID.VAR=value"       # Cannot contain dots
```

### Value Validation

- **Size limit**: Maximum 32KB (32,768 bytes) per variable value
- **Security patterns**: Warns about potentially dangerous content
- **Reserved variables**: Warns about system variables (PATH, HOME, etc.)

### Conflict Detection

Variables cannot be defined in both regular and secret environments:

❌ **Invalid (will fail validation):**
```yaml
jobs:
  conflict-job:
    environment:
      CONFLICTED_VAR: "regular_value"
    secret_environment:
      CONFLICTED_VAR: "secret_value"  # ❌ Conflict detected
```

### Reserved Variable Warnings

Joblet warns when you override system variables:

```bash
rnx run --env="PATH=/custom/path" command
# Warning: 'PATH' is a reserved environment variable name
```

**Reserved variables include:**
- System: `PATH`, `HOME`, `USER`, `SHELL`, `TERM`, `PWD`, `HOSTNAME`
- Docker: `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`
- Joblet: `JOBLET_JOB_ID`, `JOBLET_WORKFLOW_ID`, `JOBLET_RUNTIME`

## Best Practices

### 1. Use Secret Variables for Sensitive Data

```yaml
# ✅ Good: Secrets hidden from logs
jobs:
  secure-app:
    environment:
      APP_NAME: "payment-service"      # Public info
      NODE_ENV: "production"           # Public info
    secret_environment:
      DATABASE_PASSWORD: "secret123"   # Sensitive data
      STRIPE_SECRET_KEY: "dummy_key_..."  # Sensitive data
```

### 2. Use Organized Variable Grouping

```bash
# ✅ Good: Organized configuration
rnx run --env="APP_NAME=myapp" --env="NODE_ENV=prod" --secret-env="API_KEY=secret" python app.py
```

### 3. Use Descriptive Variable Names

```yaml
# ✅ Good: Clear, descriptive names
environment:
  DATABASE_MAX_CONNECTIONS: "20"
  CACHE_TTL_SECONDS: "3600"
  LOG_LEVEL: "INFO"

# ❌ Avoid: Unclear abbreviations
environment:
  DB_MAX_CONN: "20"
  CACHE_TTL: "3600"
  LOG_LVL: "INFO"
```

### 4. Group Related Variables

```yaml
# ✅ Good: Logical grouping
environment:
  # Database configuration
  DB_HOST: "localhost"
  DB_PORT: "5432"
  DB_NAME: "myapp"
  
  # API configuration
  API_BASE_URL: "https://api.example.com"
  API_TIMEOUT: "30"
  API_RETRIES: "3"

secret_environment:
  # Database secrets
  DB_PASSWORD: "secret123"
  
  # API secrets
  API_KEY: "dummy_key_..."
```

### 5. Use Default Values and Conditional Logic

```yaml
jobs:
  flexible-job:
    command: "bash"
    args: ["-c", "echo \"Debug: ${DEBUG_MODE:-false}\"; echo \"Port: ${PORT:-8080}\""]
    environment:
      NODE_ENV: "production"
      # DEBUG_MODE and PORT can be overridden by user
```

## Examples

### Example 1: Web Application Deployment

```yaml
version: "3.0"

jobs:
  web-app:
    command: "node"
    args: ["server.js"]
    environment:
      NODE_ENV: "production"
      PORT: "8080"
      APP_NAME: "my-web-app"
      LOG_LEVEL: "info"
      REDIS_HOST: "redis.internal"
      REDIS_PORT: "6379"
    secret_environment:
      DATABASE_URL: "postgresql://user:pass@db:5432/app"
      SESSION_SECRET: "super_secret_session_key"
      REDIS_PASSWORD: "redis_secret_password"
    resources:
      max_memory: 512
      max_cpu: 50
```

### Example 2: Data Pipeline

```yaml
version: "3.0"

jobs:
  extract-data:
    command: "python3"
    args: ["extract.py"]
    environment:
      DATA_SOURCE: "api"
      OUTPUT_FORMAT: "parquet"
      BATCH_SIZE: "1000"
      LOG_LEVEL: "INFO"
    secret_environment:
      API_KEY: "your_api_key_here"
      DATABASE_PASSWORD: "extraction_db_password"
    volumes: ["data-pipeline"]
    resources:
      max_memory: 1024

  transform-data:
    command: "python3"
    args: ["transform.py"]
    environment:
      INPUT_FORMAT: "parquet"
      OUTPUT_FORMAT: "json"
      VALIDATION_LEVEL: "strict"
    volumes: ["data-pipeline"]
    requires:
      - extract-data: "COMPLETED"
    resources:
      max_memory: 2048

  load-data:
    command: "python3"
    args: ["load.py"]
    environment:
      TARGET_DATABASE: "warehouse"
      BATCH_SIZE: "500"
      RETRY_COUNT: "3"
    secret_environment:
      WAREHOUSE_PASSWORD: "warehouse_secret"
      WAREHOUSE_CONNECTION_STRING: "postgresql://..."
    volumes: ["data-pipeline"]
    requires:
      - transform-data: "COMPLETED"
    resources:
      max_memory: 512
```

### Example 3: Machine Learning Pipeline

```yaml
version: "3.0"

jobs:
  train-model:
    command: "python3"
    args: ["train.py"]
    environment:
      MODEL_TYPE: "xgboost"
      TRAINING_EPOCHS: "100"
      BATCH_SIZE: "32"
      LEARNING_RATE: "0.001"
      VALIDATION_SPLIT: "0.2"
      OUTPUT_MODEL_PATH: "/volumes/models/trained_model.pkl"
    secret_environment:
      WANDB_API_KEY: "your_wandb_api_key"
      MLFLOW_TRACKING_TOKEN: "your_mlflow_token"
    volumes: ["ml-data", "models"]
    runtime: "python:3.11-ml"
    resources:
      max_memory: 4096
      max_cpu: 80

  evaluate-model:
    command: "python3"
    args: ["evaluate.py"]
    environment:
      MODEL_PATH: "/volumes/models/trained_model.pkl"
      TEST_DATA_PATH: "/volumes/ml-data/test.csv"
      METRICS_OUTPUT: "/volumes/models/metrics.json"
      EVALUATION_MODE: "comprehensive"
    volumes: ["ml-data", "models"]
    runtime: "python:3.11-ml"
    requires:
      - train-model: "COMPLETED"
    resources:
      max_memory: 2048

  deploy-model:
    command: "python3"
    args: ["deploy.py"]
    environment:
      MODEL_PATH: "/volumes/models/trained_model.pkl"
      DEPLOYMENT_TARGET: "production"
      API_VERSION: "v1"
      HEALTH_CHECK_ENDPOINT: "/health"
    secret_environment:
      DEPLOYMENT_API_KEY: "production_deployment_key"
      MODEL_REGISTRY_TOKEN: "model_registry_secret"
    volumes: ["models"]
    requires:
      - evaluate-model: "COMPLETED"
    resources:
      max_memory: 1024
```

### Example 4: Microservices Configuration

```yaml
version: "3.0"

jobs:
  user-service:
    command: "java"
    args: ["-jar", "user-service.jar"]
    environment:
      SPRING_PROFILES_ACTIVE: "production"
      SERVER_PORT: "8081"
      SERVICE_NAME: "user-service"
      LOG_LEVEL: "INFO"
      CACHE_ENABLED: "true"
      CACHE_TTL: "3600"
    secret_environment:
      DATABASE_PASSWORD: "user_db_password"
      JWT_SECRET: "user_service_jwt_secret"
      REDIS_PASSWORD: "redis_password"
    network: "microservices"
    resources:
      max_memory: 1024

  payment-service:
    command: "java"
    args: ["-jar", "payment-service.jar"]
    environment:
      SPRING_PROFILES_ACTIVE: "production"
      SERVER_PORT: "8082"
      SERVICE_NAME: "payment-service"
      USER_SERVICE_URL: "http://user-service:8081"
      LOG_LEVEL: "INFO"
    secret_environment:
      DATABASE_PASSWORD: "payment_db_password"
      STRIPE_SECRET_KEY: "dummy_stripe_secret_key"
      JWT_SECRET: "payment_service_jwt_secret"
    network: "microservices"
    requires:
      - user-service: "RUNNING"
    resources:
      max_memory: 1024

  api-gateway:
    command: "node"
    args: ["gateway.js"]
    environment:
      NODE_ENV: "production"
      PORT: "8080"
      USER_SERVICE_URL: "http://user-service:8081"
      PAYMENT_SERVICE_URL: "http://payment-service:8082"
      RATE_LIMIT_REQUESTS: "1000"
      RATE_LIMIT_WINDOW: "60000"
    secret_environment:
      API_SECRET: "gateway_api_secret"
      AUTH_TOKEN: "gateway_auth_token"
    network: "microservices"
    requires:
      - user-service: "RUNNING"
      - payment-service: "RUNNING"
    resources:
      max_memory: 512
```

## Advanced Use Cases

### Environment Variable Interpolation

Use shell parameter expansion for advanced variable manipulation:

```yaml
jobs:
  advanced-variables:
    command: "bash"
    args: ["-c", |
      echo "App: ${APP_NAME:-default-app}"
      echo "Debug enabled: ${DEBUG_MODE:+yes}"
      echo "Port: ${PORT:-8080}"
      echo "Config file: ${CONFIG_FILE:-/app/config.yml}"
      echo "Log level: $LOG_LEVEL"
      echo "Environment: $NODE_ENV"
    ]
    environment:
      APP_NAME: "my-application"
      DEBUG_MODE: "true"
      LOG_LEVEL: "info"
      NODE_ENV: "PRODUCTION"
```

### Conditional Job Execution

Use environment variables to control job behavior:

```yaml
jobs:
  conditional-job:
    command: "bash"
    args: ["-c", |
      if [ "$SKIP_PROCESSING" = "true" ]; then
        echo "Skipping processing as requested"
        exit 0
      fi
      
      if [ "$ENVIRONMENT" = "production" ]; then
        echo "Running in production mode"
        python app.py --production
      else
        echo "Running in development mode"
        python app.py --debug
      fi
    ]
    environment:
      ENVIRONMENT: "production"
      SKIP_PROCESSING: "false"
```

### Multi-Environment Configurations

Create different configurations for different environments:

**development.yaml:**
```yaml
version: "3.0"
jobs:
  app:
    command: "python"
    args: ["app.py"]
    environment:
      NODE_ENV: "development"
      DEBUG: "true"
      LOG_LEVEL: "DEBUG"
      DATABASE_URL: "sqlite:///dev.db"
    resources:
      max_memory: 256
```

**production.yaml:**
```yaml
version: "3.0"
jobs:
  app:
    command: "python"
    args: ["app.py"]
    environment:
      NODE_ENV: "production"
      DEBUG: "false"
      LOG_LEVEL: "INFO"
    secret_environment:
      DATABASE_URL: "postgresql://user:pass@prod-db:5432/app"
    resources:
      max_memory: 1024
```

### Dynamic Configuration Loading

```yaml
jobs:
  config-loader:
    command: "bash"
    args: ["-c", |
      # Load configuration based on environment
      CONFIG_FILE="/configs/${ENVIRONMENT:-development}.json"
      if [ -f "$CONFIG_FILE" ]; then
        echo "Loading config from $CONFIG_FILE"
        export $(cat "$CONFIG_FILE" | jq -r 'to_entries[] | "\(.key)=\(.value)"')
      fi
      
      # Run the actual application
      python app.py
    ]
    environment:
      ENVIRONMENT: "production"
    volumes: ["configs"]
```

---

## Troubleshooting

### Common Issues

1. **Variables not expanding**: Ensure you're using double quotes in YAML
2. **Validation errors**: Check variable names follow POSIX standards
3. **Conflicts detected**: Don't define same variable in both environment types
4. **Reserved variable warnings**: Consider if overriding system variables is necessary

### Error Messages

```bash
# Invalid variable name
Error: invalid environment variable name 'INVALID-VAR': must start with letter or underscore

# Conflict detection  
Error: environment variable conflicts detected - variables defined in both environments: [CONFLICTED_VAR]

# Value too long
Error: environment variable value too long (40000 bytes, max 32768)
```

### Getting Help

For more assistance:
- Check the [Joblet Documentation](README.md)
- Review [Workflow Examples](../examples/workflows/)
- See [API Reference](API.md)

---

*This documentation covers Joblet's comprehensive environment variable support. For additional features and updates, refer to the main Joblet documentation.*