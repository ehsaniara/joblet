# Java Microservices Workflow Examples

This directory contains workflow examples for Java-based microservices architecture.

## Files

### `java-services.yaml`

Contains six Java service examples:

1. **api-gateway** - Spring Cloud Gateway service
2. **user-service** - User management microservice
3. **order-service** - Order processing microservice
4. **payment-service** - Payment processing with high resources
5. **db-migration** - Flyway database migration job
6. **batch-processor** - Spring Batch job for data processing

## Usage Examples

```bash
# Run individual services
rnx job run --workflow=java-services.yaml:api-gateway
rnx job run --workflow=java-services.yaml:user-service
rnx job run --workflow=java-services.yaml:order-service
rnx job run --workflow=java-services.yaml:payment-service

# Run database migration
rnx job run --workflow=java-services.yaml:db-migration

# Run batch processor
rnx job run --workflow=java-services.yaml:batch-processor
```

## Prerequisites

### Required Volumes

```bash
rnx volume create config
rnx volume create logs
rnx volume create user-data
rnx volume create order-data
rnx volume create payment-data
rnx volume create secure-keys
rnx volume create batch-input
rnx volume create batch-output
```

### Required JAR Files

Ensure these JAR files are available in the current directory:

- `api-gateway.jar`
- `user-service.jar`
- `order-service.jar`
- `payment-service.jar`
- `flyway.jar`
- `batch-processor.jar`

### Configuration Files

- `application.yml` - Spring configuration
- `payment-config.properties` - Payment service configuration
- `migrations/` - Database migration scripts

## Runtime Requirements

- Java 17 runtime for most services
- Java 21 runtime for batch-processor (enhanced performance)