# Environment Variables

Joblet provides comprehensive support for environment variables in both CLI jobs and workflow-based jobs. Environment
variables allow you to pass configuration, secrets, and runtime parameters to your jobs in a secure and flexible manner.

## Table of Contents

- [Overview](#overview)
- [CLI Usage](#cli-usage)
- [Workflow Usage](#workflow-usage)
- [Secret Detection](#secret-detection)
- [Security Features](#security-features)
- [Validation](#validation)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Advanced Use Cases](#advanced-use-cases)

## Overview

**v5.0.0 Update**: Joblet now uses a unified `environment` field with automatic secret detection by naming convention.

### Key Features

- **Automatic secret detection** (based on variable naming conventions)
- **Multiple input methods** (command line flags, workflow YAML)
- **Variable templating** (`${VAR_NAME}` syntax for referencing other variables)
- **Status display masking** (secret variables shown as `***` in status output)
- **Variable validation** (name format, value size, conflict detection)
- **Security design** (secret variables hidden from logs)
- **Reserved variable warnings** (system variables like PATH, HOME)

### Breaking Changes in v5.0.0

❌ **REMOVED**: Separate `secret_environment` field
✅ **NEW**: Automatic secret detection by naming convention
✅ **NEW**: Single `environment` field for all variables

## CLI Usage

### Basic Syntax

```bash
# Regular environment variables (visible in logs)
rnx job run --env="KEY=value" command

# Secret environment variables (auto-detected by naming)
rnx job run --env="SECRET_KEY=secret_value" command
rnx job run --env="DATABASE_PASSWORD=pass123" command
rnx job run --env="API_TOKEN=token123" command

# Multiple variables
rnx job run --env="VAR1=value1" --env="VAR2=value2" command

# Mixed usage (secrets auto-detected)
rnx job run --env="NODE_ENV=production" --env="SECRET_API_KEY=secret" command
```

### Examples

#### Example 1: Web Server with Database

```bash
rnx job run \
  --env="NODE_ENV=production" \
  --env="PORT=3000" \
  --env="DATABASE_PASSWORD=secret123" \
  --env="JWT_SECRET=jwt_secret_key" \
  node server.js
```

Variables `DATABASE_PASSWORD` and `JWT_SECRET` are automatically detected as secrets and masked in logs.

#### Example 2: Data Processing Job

```bash
rnx job run \
  --env="INPUT_PATH=/data/input" \
  --env="OUTPUT_PATH=/data/output" \
  --env="BATCH_SIZE=1000" \
  --env="SECRET_API_KEY=api_key_here" \
  python process_data.py
```

## Workflow Usage

### Basic Example

```yaml
jobs:
  my-job:
    command: "python3"
    args: ["app.py"]
    environment:
      # Regular variables (visible)
      NODE_ENV: "production"
      DEBUG_MODE: "false"
      BATCH_SIZE: "100"

      # Secrets (auto-detected, masked in logs)
      DATABASE_PASSWORD: "super_secret_password"
      API_KEY: "dummy_api_key_example"
      JWT_SECRET: "your_jwt_signing_secret"
      SECRET_ENCRYPTION_KEY: "encryption_key_here"
```

### Variable Templating

```yaml
jobs:
  build-job:
    command: "bash"
    args: ["-c", "echo \"Debug mode: ${DEBUG_MODE:+enabled}\"; echo \"Secret configured: ${SECRET_KEY:+yes}\""]
    environment:
      DEBUG_MODE: "true"
      SECRET_KEY: "secret_value"  # Auto-detected as secret
```

## Secret Detection

**New in v5.0.0**: Secrets are automatically detected based on naming conventions.

### Auto-Detected Secret Patterns

Variables matching these patterns are automatically treated as secrets:

| Pattern      | Example                               | Description       |
|--------------|---------------------------------------|-------------------|
| `SECRET_*`   | `SECRET_DATABASE_PASSWORD`            | Prefix: SECRET_   |
| `*_TOKEN`    | `GITHUB_TOKEN`, `AUTH_TOKEN`          | Suffix: _TOKEN    |
| `*_KEY`      | `API_KEY`, `ENCRYPTION_KEY`           | Suffix: _KEY      |
| `*_PASSWORD` | `DATABASE_PASSWORD`, `ADMIN_PASSWORD` | Suffix: _PASSWORD |
| `*_SECRET`   | `OAUTH_SECRET`, `JWT_SECRET`          | Suffix: _SECRET   |

### Examples

```yaml
environment:
  # Regular variables (visible in logs)
  NODE_ENV: "production"
  PORT: "3000"
  LOG_LEVEL: "info"

  # Auto-detected secrets (masked in logs)
  SECRET_DATABASE_URL: "postgresql://..."      # SECRET_ prefix
  API_KEY: "abc123"                             # _KEY suffix
  DATABASE_PASSWORD: "pass123"                  # _PASSWORD suffix
  GITHUB_TOKEN: "ghp_123"                       # _TOKEN suffix
  JWT_SECRET: "secret123"                       # _SECRET suffix
```

## Security Features

### Secret Masking

Secrets are automatically masked in:

- Job status output (`***`)
- Log output (redacted)
- gRPC responses (masked)
- CLI display (hidden)

### Example Output

```bash
$ rnx job status abc123

Environment: 5 variables set
  NODE_ENV: "production"
  PORT: "3000"
  API_KEY: ***                    # Masked
  DATABASE_PASSWORD: ***          # Masked
  JWT_SECRET: ***                 # Masked
```

### Security Best Practices

✅ **DO**:

- Use naming conventions for secrets (`SECRET_*`, `*_KEY`, `*_PASSWORD`, `*_TOKEN`, `*_SECRET`)
- Keep secret values in external secret management systems
- Rotate secrets regularly
- Use different secrets per environment

❌ **DON'T**:

- Commit secrets to version control
- Use predictable secret values
- Share secrets in plain text
- Reuse secrets across environments

## Validation

Joblet validates environment variables for:

### Name Validation

- Format: `^[A-Z][A-Z0-9_]*$`
- Must start with uppercase letter
- Can contain uppercase letters, numbers, and underscores
- Maximum length: 256 characters

### Value Validation

- Maximum size: 32KB per variable
- No null bytes allowed
- UTF-8 encoding required

### Conflict Detection

```yaml
jobs:
  conflict-job:
    environment:
      CONFLICTED_VAR: "value1"   # ❌ Duplicate key not allowed
      CONFLICTED_VAR: "value2"   # Error: duplicate key
```

### Reserved Variables

Warning issued for system variables:

- `PATH`, `HOME`, `USER`, `SHELL`
- `PWD`, `OLDPWD`, `LANG`
- `LC_*` variables

## Best Practices

### 1. Naming Conventions

```yaml
# ✅ GOOD
environment:
  NODE_ENV: "production"           # Clear purpose
  DATABASE_PASSWORD: "secret"      # Auto-detected secret
  SECRET_API_KEY: "key123"         # Explicit secret
  MAX_RETRY_COUNT: "3"             # Descriptive

# ❌ BAD
environment:
  env: "prod"                      # Too generic
  secret: "key123"                 # Unclear
  x: "3"                           # Not descriptive
```

### 2. Organize by Category

```yaml
environment:
  # Application config
  NODE_ENV: "production"
  APP_NAME: "payment-service"

  # Feature flags
  ENABLE_CACHING: "true"
  ENABLE_METRICS: "true"

  # Secrets (auto-detected)
  DATABASE_PASSWORD: "secret123"
  STRIPE_SECRET_KEY: "sk_live_..."
  JWT_SIGNING_KEY: "jwt_secret"
```

### 3. Use Templating

```yaml
environment:
  BASE_PATH: "/opt/app"
  PROJECT: "payment-service"
  VERSION: "1.2.3"

  # Derived variables
  BIN_PATH: "${BASE_PATH}/${PROJECT}/v${VERSION}/bin"
  CONFIG_FILE: "${BASE_PATH}/${PROJECT}/config.yml"

  # Secrets can also use templating
  SECRET_KEY_FILE: "/secrets/${PROJECT}/${VERSION}/key.pem"
```

## Examples

### Example 1: Machine Learning Training

```yaml
jobs:
  ml-training:
    command: "python"
    args: ["train.py"]
    environment:
      # Training config
      MODEL_NAME: "gpt-2"
      EPOCHS: "10"
      BATCH_SIZE: "32"
      LEARNING_RATE: "0.001"

      # Paths
      DATA_DIR: "/volumes/datasets"
      OUTPUT_DIR: "/volumes/models"

      # Secrets (auto-detected)
      WANDB_API_KEY: "your_wandb_api_key_here"
      HF_TOKEN: "huggingface_token_here"
      AWS_ACCESS_KEY: "aws_key_for_s3_data"
      AWS_SECRET_KEY: "aws_secret_for_s3_data"
    resources:
      max_memory: 16384
      gpu_count: 1
    volumes: ["ml-data", "models"]
```

### Example 2: Data Pipeline

```yaml
jobs:
  extract-data:
    command: "python"
    args: ["extract.py"]
    environment:
      # Pipeline config
      SOURCE_TYPE: "postgresql"
      OUTPUT_FORMAT: "parquet"
      BATCH_SIZE: "1000"
      LOG_LEVEL: "INFO"

      # Secrets (auto-detected)
      API_KEY: "your_api_key_here"
      DATABASE_PASSWORD: "extraction_db_password"
      DATABASE_URL: "postgresql://user:${DATABASE_PASSWORD}@host/db"
    volumes: ["data-pipeline"]

  transform-data:
    command: "python"
    args: ["transform.py"]
    environment:
      # Transform config
      INPUT_FORMAT: "parquet"
      OUTPUT_FORMAT: "parquet"
      VALIDATION_MODE: "strict"
    volumes: ["data-pipeline"]
    requires:
      - extract-data: "COMPLETED"

  load-data:
    command: "python"
    args: ["load.py"]
    environment:
      # Load config
      TARGET_DATABASE: "warehouse"
      BATCH_SIZE: "500"
      RETRY_COUNT: "3"

      # Secrets (auto-detected)
      WAREHOUSE_PASSWORD: "warehouse_secret"
      WAREHOUSE_CONNECTION_STRING: "postgresql://..."
    volumes: ["data-pipeline"]
    requires:
      - transform-data: "COMPLETED"
```

### Example 3: Microservices Deployment

```yaml
jobs:
  api-gateway:
    command: "node"
    args: ["gateway.js"]
    environment:
      # Service config
      PORT: "8080"
      NODE_ENV: "production"
      LOG_LEVEL: "info"
      RATE_LIMIT: "100"

      # Secrets (auto-detected)
      API_SECRET: "gateway_api_secret"
      AUTH_TOKEN: "gateway_auth_token"
      JWT_SECRET: "jwt_signing_key"
    network: "microservices"
    resources:
      max_memory: 512

  backend-service:
    command: "java"
    args: ["-jar", "service.jar"]
    environment:
      # Service config
      SERVER_PORT: "9090"
      SPRING_PROFILES_ACTIVE: "production"

      # Secrets (auto-detected)
      DATABASE_PASSWORD: "backend_db_password"
      REDIS_PASSWORD: "redis_secret"
      SECRET_KEY: "backend_secret_key"
    network: "microservices"
    requires:
      - api-gateway: "RUNNING"
```

### Example 4: GPU-Accelerated Workload

```yaml
jobs:
  distributed-training:
    command: "python"
    args: ["-m", "torch.distributed.launch", "train.py"]
    environment:
      # Distributed training
      MASTER_ADDR: "localhost"
      MASTER_PORT: "29500"
      WORLD_SIZE: "2"
      RANK: "0"

      # Model config
      MODEL_SIZE: "large"
      PRECISION: "fp16"
      GRADIENT_CHECKPOINTING: "true"

      # Secrets (auto-detected)
      CLUSTER_TOKEN: "secure_cluster_token"
      MONITORING_API_KEY: "monitoring_key"
      SECRET_MODEL_KEY: "model_encryption_key"
    resources:
      gpu_count: 2
      max_memory: 32768
    network: "distributed-training"
```

## Advanced Use Cases

### Variable Templating with Secrets

```yaml
jobs:
  deploy:
    command: "deploy.sh"
    environment:
      ENVIRONMENT: "production"
      SERVICE_NAME: "api"

      # Templating with secrets
      SECRET_KEY_PATH: "/secrets/${SERVICE_NAME}/${ENVIRONMENT}/key.pem"
      DATABASE_URL: "postgresql://user:${DATABASE_PASSWORD}@${DB_HOST}/db"

      # Secrets (auto-detected)
      DATABASE_PASSWORD: "prod_db_secret"
      DB_HOST: "db.production.internal"
      SECRET_DEPLOYMENT_KEY: "deploy_key_prod"
```

### Multi-Stage Pipeline

```yaml
jobs:
  build:
    command: "make"
    args: ["build"]
    environment:
      BUILD_ENV: "production"
      OPTIMIZE: "true"

      # Build secrets (auto-detected)
      NPM_TOKEN: "npm_registry_token"
      PRIVATE_KEY: "code_signing_key"

  test:
    command: "make"
    args: ["test"]
    environment:
      TEST_ENV: "ci"
      COVERAGE: "true"

      # Test secrets (auto-detected)
      TEST_DATABASE_PASSWORD: "test_db_secret"
    requires:
      - build: "COMPLETED"

  deploy:
    command: "make"
    args: ["deploy"]
    environment:
      DEPLOY_TARGET: "production"

      # Deployment secrets (auto-detected)
      DEPLOYMENT_KEY: "production_deploy_key"
      SSH_PRIVATE_KEY: "ssh_deploy_key"
    requires:
      - test: "COMPLETED"
```

## Migration from v4.x to v5.0.0

### Before (v4.x) - DEPRECATED

```yaml
# v4.x - No longer supported
jobs:
  my-job:
    command: "app"
    environment:
      PUBLIC_VAR: "value"
    secret_environment:        # ❌ REMOVED
      API_KEY: "secret"
```

### After (v5.0.0) - REQUIRED

```yaml
# v5.0.0 - Current
jobs:
  my-job:
    command: "app"
    environment:
      PUBLIC_VAR: "value"
      API_KEY: "secret"        # ✅ Auto-detected as secret
```

## Additional Resources

- [V5 Cleanup Summary](../V5_CLEANUP_SUMMARY.md) - Complete v5.0.0 changes
- [Deprecation Guide](./DEPRECATION.md) - Migration instructions
- [API Documentation](./API.md) - gRPC API reference
- [Security Guide](./SECURITY.md) - Security best practices

---

**Last Updated**: 2025-10-13
**Joblet Version**: v5.0.0
