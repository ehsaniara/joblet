# Environment Variables

Joblet provides comprehensive support for environment variables in jobs. Environment
variables allow you to pass configuration, secrets, and runtime parameters to your jobs in a secure and flexible manner.

## Table of Contents

- [Overview](#overview)
- [CLI Usage](#cli-usage)
- [Secret Detection](#secret-detection)
- [Security Features](#security-features)
- [Validation](#validation)
- [Best Practices](#best-practices)
- [Examples](#examples)
- [Advanced Use Cases](#advanced-use-cases)

## Overview

Joblet provides two methods for setting secret environment variables:

1. **Explicit `--secret-env` flag**: Explicitly mark variables as secrets
2. **Automatic detection**: Variables matching naming patterns are auto-detected as secrets

### Key Features

- **Explicit secret flag** (`--secret-env` / `-s` for explicit secret marking)
- **Automatic secret detection** (based on variable naming conventions)
- **Multiple input methods** (command line flags)
- **Variable templating** (`${VAR_NAME}` syntax for referencing other variables)
- **Status display masking** (secret variables shown as `***` in status output)
- **Variable validation** (name format, value size, conflict detection)
- **Security design** (secret variables hidden from logs)
- **Reserved variable warnings** (system variables like PATH, HOME)

## CLI Usage

### Basic Syntax

```bash
# Regular environment variables (visible in logs)
rnx job run --env="KEY=value" command
rnx job run -e "KEY=value" command

# Secret environment variables - Method 1: Explicit flag (always hidden)
rnx job run --secret-env="API_KEY=secret_value" command
rnx job run -s "API_KEY=secret_value" command

# Secret environment variables - Method 2: Auto-detected by naming
rnx job run --env="SECRET_KEY=secret_value" command      # SECRET_ prefix
rnx job run --env="DATABASE_PASSWORD=pass123" command    # _PASSWORD suffix
rnx job run --env="API_TOKEN=token123" command           # _TOKEN suffix

# Multiple variables
rnx job run --env="VAR1=value1" --env="VAR2=value2" command

# Mixed usage
rnx job run --env="NODE_ENV=production" --secret-env="API_KEY=secret" command
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

```bash
rnx job run \
  --env="MODEL_NAME=gpt-2" \
  --env="EPOCHS=10" \
  --env="BATCH_SIZE=32" \
  --env="LEARNING_RATE=0.001" \
  --env="DATA_DIR=/volumes/datasets" \
  --env="OUTPUT_DIR=/volumes/models" \
  --env="WANDB_API_KEY=your_wandb_api_key_here" \
  --env="HF_TOKEN=huggingface_token_here" \
  --env="AWS_ACCESS_KEY=aws_key_for_s3_data" \
  --env="AWS_SECRET_KEY=aws_secret_for_s3_data" \
  --gpu=1 \
  --memory=16GB \
  python train.py
```

Variables `WANDB_API_KEY`, `HF_TOKEN`, `AWS_ACCESS_KEY`, and `AWS_SECRET_KEY` are automatically detected as secrets by
their naming patterns.

### Example 2: Data Processing

```bash
# Extract job with secrets
rnx job run \
  --env="SOURCE_TYPE=postgresql" \
  --env="OUTPUT_FORMAT=parquet" \
  --env="BATCH_SIZE=1000" \
  --env="LOG_LEVEL=INFO" \
  --env="API_KEY=your_api_key_here" \
  --env="DATABASE_PASSWORD=extraction_db_password" \
  python extract.py

# Transform job
rnx job run \
  --env="INPUT_FORMAT=parquet" \
  --env="OUTPUT_FORMAT=parquet" \
  --env="VALIDATION_MODE=strict" \
  python transform.py

# Load job with secrets
rnx job run \
  --env="TARGET_DATABASE=warehouse" \
  --env="BATCH_SIZE=500" \
  --env="RETRY_COUNT=3" \
  --env="WAREHOUSE_PASSWORD=warehouse_secret" \
  python load.py
```

### Example 3: API Service

```bash
rnx job run \
  --env="PORT=8080" \
  --env="NODE_ENV=production" \
  --env="LOG_LEVEL=info" \
  --env="RATE_LIMIT=100" \
  --env="API_SECRET=gateway_api_secret" \
  --env="AUTH_TOKEN=gateway_auth_token" \
  --env="JWT_SECRET=jwt_signing_key" \
  --memory=512MB \
  node gateway.js
```

### Example 4: GPU-Accelerated Workload

```bash
rnx job run \
  --env="MASTER_ADDR=localhost" \
  --env="MASTER_PORT=29500" \
  --env="WORLD_SIZE=2" \
  --env="RANK=0" \
  --env="MODEL_SIZE=large" \
  --env="PRECISION=fp16" \
  --env="GRADIENT_CHECKPOINTING=true" \
  --env="CLUSTER_TOKEN=secure_cluster_token" \
  --env="MONITORING_API_KEY=monitoring_key" \
  --env="SECRET_MODEL_KEY=model_encryption_key" \
  --gpu=2 \
  --memory=32GB \
  python -m torch.distributed.launch train.py
```

## Advanced Use Cases

### Variable Templating with Secrets

```bash
# Using shell variable expansion for templating
SERVICE_NAME="api"
ENVIRONMENT="production"

rnx job run \
  --env="ENVIRONMENT=${ENVIRONMENT}" \
  --env="SERVICE_NAME=${SERVICE_NAME}" \
  --env="SECRET_KEY_PATH=/secrets/${SERVICE_NAME}/${ENVIRONMENT}/key.pem" \
  --env="DATABASE_PASSWORD=prod_db_secret" \
  --env="DB_HOST=db.production.internal" \
  --env="SECRET_DEPLOYMENT_KEY=deploy_key_prod" \
  ./deploy.sh
```

### Build and Test Jobs

```bash
# Build job with secrets
rnx job run \
  --env="BUILD_ENV=production" \
  --env="OPTIMIZE=true" \
  --env="NPM_TOKEN=npm_registry_token" \
  --env="PRIVATE_KEY=code_signing_key" \
  make build

# Test job with secrets
rnx job run \
  --env="TEST_ENV=ci" \
  --env="COVERAGE=true" \
  --env="TEST_DATABASE_PASSWORD=test_db_secret" \
  make test

# Deploy job with secrets
rnx job run \
  --env="DEPLOY_TARGET=production" \
  --env="DEPLOYMENT_KEY=production_deploy_key" \
  --env="SSH_PRIVATE_KEY=ssh_deploy_key" \
  make deploy
```

## Secret Environment Options

You can use either explicit `--secret-env` or rely on automatic detection:

### Option 1: Explicit --secret-env Flag

```bash
# Use --secret-env for any variable you want hidden (regardless of name)
rnx job run --env="PUBLIC_VAR=value" --secret-env="MY_VAR=secret" app
```

### Option 2: Automatic Detection by Naming

```bash
# Variables with secret naming patterns are auto-detected
rnx job run --env="PUBLIC_VAR=value" --env="API_KEY=secret" app
# API_KEY auto-detected as secret by _KEY suffix
```

## Additional Resources

- [V5 Cleanup Summary](../V5_CLEANUP_SUMMARY.md) - Complete v5.0.0 changes
- [Deprecation Guide](./DEPRECATION.md) - Migration instructions
- [API Documentation](./API.md) - gRPC API reference
- [Security Guide](./SECURITY.md) - Security best practices

---

**Last Updated**: 2025-10-13
**Joblet Version**: v5.0.0
