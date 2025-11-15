# AWS Secrets Manager Integration for Certificate Management

## Overview

This document describes how Joblet integrates with AWS Secrets Manager to enable horizontal scaling across multiple EC2 instances while maintaining a shared Certificate Authority (CA) and client certificates.

## The Scaling Challenge

**Without Secrets Manager (Current Default):**
- Each EC2 instance generates its own CA certificate
- Each EC2 instance generates its own client certificates
- Clients need different config files for each server
- No way to load balance across multiple joblet servers
- Certificate rotation requires updating all clients

**With Secrets Manager:**
- First instance generates CA + client certs → Stores in Secrets Manager
- Subsequent instances retrieve shared CA + client certs → Generate only server cert
- All instances share the same CA and client certificates
- One client config file works with all servers (ALB/NLB compatible)
- Certificate rotation happens centrally

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                  AWS Secrets Manager                        │
│  ┌─────────────────────────────────────────────────────┐   │
│  │ joblet/ca-cert      (Shared - Generated Once)      │   │
│  │ joblet/ca-key       (Shared - Generated Once)      │   │
│  │ joblet/client-cert  (Shared - Generated Once)      │   │
│  │ joblet/client-key   (Shared - Generated Once)      │   │
│  └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
                          ↓ ↓ ↓
        ┌─────────────────┼─┼─┼─────────────────┐
        ↓                 ↓   ↓                 ↓
┌───────────────┐  ┌───────────────┐  ┌───────────────┐
│  EC2 Instance │  │  EC2 Instance │  │  EC2 Instance │
│      #1       │  │      #2       │  │      #3       │
├───────────────┤  ├───────────────┤  ├───────────────┤
│ Server Cert A │  │ Server Cert B │  │ Server Cert C │
│  (Local)      │  │  (Local)      │  │  (Local)      │
└───────────────┘  └───────────────┘  └───────────────┘
        ↑                 ↑                 ↑
        └─────────────────┼─────────────────┘
                          ↓
                  ┌───────────────┐
                  │    Clients    │
                  │ (One config)  │
                  └───────────────┘
```

## Use Cases

### 1. Auto-Scaling Groups

Deploy joblet in an ASG with shared certificates:

```bash
# First instance (or launch template)
USE_SECRETS_MANAGER=true /usr/local/bin/certs_gen_with_secretsmanager.sh

# All subsequent instances automatically:
# - Retrieve shared CA/client certs from Secrets Manager
# - Generate unique server certificates
# - Work with the same client config
```

### 2. Multi-Region Deployment

Replicate secrets across regions for DR:

```bash
# Primary region (us-east-1)
USE_SECRETS_MANAGER=true EC2_REGION=us-east-1 ./certs_gen_with_secretsmanager.sh

# Replicate to DR region (us-west-2)
aws secretsmanager replicate-secret-to-regions \
  --secret-id joblet/ca-cert \
  --add-replica-regions Region=us-west-2

# Deploy in DR region
USE_SECRETS_MANAGER=true EC2_REGION=us-west-2 ./certs_gen_with_secretsmanager.sh
```

### 3. Blue-Green Deployment

Both environments share certificates:

```bash
# Blue environment
USE_SECRETS_MANAGER=true JOBLET_SERVER_ADDRESS=blue.internal ./script.sh

# Green environment (reuses CA/client)
USE_SECRETS_MANAGER=true JOBLET_SERVER_ADDRESS=green.internal ./script.sh

# Clients can connect to either with same config
```

### 4. Load Balancer Integration

```
                    ┌─────────────────┐
                    │  ALB / NLB      │
                    │  (TLS Passthru) │
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         ↓                   ↓                   ↓
    ┌─────────┐         ┌─────────┐         ┌─────────┐
    │ Server1 │         │ Server2 │         │ Server3 │
    │ Cert A  │         │ Cert B  │         │ Cert C  │
    └─────────┘         └─────────┘         └─────────┘
         All signed by same CA
         All accept same client cert
```

## Setup Instructions

### Step 1: Create IAM Policy for Secrets Manager

Update your existing `JobletEC2Role` with Secrets Manager permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "SecretsManagerAccess",
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret",
        "secretsmanager:CreateSecret",
        "secretsmanager:UpdateSecret",
        "secretsmanager:PutSecretValue",
        "secretsmanager:TagResource"
      ],
      "Resource": [
        "arn:aws:secretsmanager:*:*:secret:joblet/*"
      ]
    }
  ]
}
```

Apply the policy:

```bash
# Create policy
POLICY_ARN=$(aws iam create-policy \
  --policy-name JobletSecretsManagerPolicy \
  --policy-document file://secretsmanager-policy.json \
  --query 'Policy.Arn' \
  --output text)

# Attach to existing role
aws iam attach-role-policy \
  --role-name JobletEC2Role \
  --policy-arn $POLICY_ARN
```

### Step 2: Update EC2 User Data Script

Modify the EC2 user data to enable Secrets Manager:

```bash
#!/bin/bash
set -e

# Configuration
export JOBLET_VERSION="latest"
export JOBLET_SERVER_PORT="443"
export ENABLE_CLOUDWATCH="true"

# Enable Secrets Manager integration
export USE_SECRETS_MANAGER="true"  # NEW!
export SECRETS_PREFIX="joblet"      # Optional: custom prefix

# Download and run installation script
curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh \
  -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

### Step 3: Launch First Instance

The first instance will:
1. Generate CA certificate and key
2. Generate admin client certificate and key
3. Store them in Secrets Manager
4. Generate instance-specific server certificate

```bash
# Verify secrets were created
aws secretsmanager list-secrets \
  --filters Key=name,Values=joblet/ \
  --query 'SecretList[*].[Name,Description]' \
  --output table
```

Expected output:
```
----------------------------------------
|            ListSecrets                |
+----------------------+----------------+
|  joblet/ca-cert      | Joblet Root CA Certificate - shared across all instances  |
|  joblet/ca-key       | Joblet Root CA Private Key - shared across all instances  |
|  joblet/client-cert  | Joblet Admin Client Certificate - shared across all clients |
|  joblet/client-key   | Joblet Admin Client Private Key - shared across all clients |
+----------------------+----------------+
```

### Step 4: Launch Additional Instances

Subsequent instances will:
1. Retrieve existing CA from Secrets Manager
2. Retrieve existing client certificates from Secrets Manager
3. Generate NEW instance-specific server certificate
4. Use the same client config

No additional steps needed - just use the same user data script!

### Step 5: Distribute Client Config

Download the client config from **any** instance:

```bash
# From instance 1
scp ec2-user@instance1:/opt/joblet/config/rnx-config.yml ~/.rnx/

# This config works with ALL instances!
rnx --node=default job list
```

## Certificate Lifecycle

### Initial Generation (First Instance)

```bash
# Instance 1
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh

Output:
  ✨ CA Certificate: Generated and stored in Secrets Manager (shared)
  ✨ Client Certificate: Generated and stored in Secrets Manager (shared)
  🆕 Server Certificate: Generated locally (instance-specific)
```

### Subsequent Instances

```bash
# Instance 2, 3, 4...
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh

Output:
  ✅ CA Certificate: Retrieved from Secrets Manager (shared)
  ✅ Client Certificate: Retrieved from Secrets Manager (shared)
  🆕 Server Certificate: Generated locally (instance-specific)
```

### Certificate Rotation

#### Rotate Server Certificates (Per Instance)

```bash
# On each instance
sudo /usr/local/bin/certs_gen_with_secretsmanager.sh
sudo systemctl restart joblet
```

This regenerates only the server certificate. CA and client certs remain unchanged.

#### Rotate CA and Client Certificates (All Instances)

```bash
# Force regeneration on one instance
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true \
  /usr/local/bin/certs_gen_with_secretsmanager.sh

# This creates NEW CA and client certs in Secrets Manager
# Then restart all other instances to pick up new certs
# Finally, distribute new client config to all clients
```

⚠️ **Warning:** Rotating CA requires:
1. Regenerating server certs on all instances
2. Distributing new client config to all clients
3. Brief service interruption during rollout

## Configuration Options

### Environment Variables

| Variable               | Default | Description                                    |
|------------------------|---------|------------------------------------------------|
| `USE_SECRETS_MANAGER`  | `auto`  | Enable Secrets Manager (`true`, `false`, `auto`) |
| `SECRETS_PREFIX`       | `joblet`| Prefix for secret names                        |
| `FORCE_REGENERATE`     | `false` | Force regenerate even if secrets exist         |
| `EC2_REGION`           | auto    | AWS region (auto-detected from EC2 metadata)   |

### Examples

**Enable explicitly:**
```bash
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh
```

**Disable explicitly:**
```bash
USE_SECRETS_MANAGER=false ./certs_gen_with_secretsmanager.sh
```

**Auto-detect (default on EC2):**
```bash
./certs_gen_with_secretsmanager.sh
# Automatically enables if running on EC2 and AWS CLI is available
```

**Custom prefix:**
```bash
USE_SECRETS_MANAGER=true SECRETS_PREFIX=my-app/joblet \
  ./certs_gen_with_secretsmanager.sh
```

**Force regenerate everything:**
```bash
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true \
  ./certs_gen_with_secretsmanager.sh
```


### Secrets Manager Benefits

1. **Encryption at Rest**: All secrets encrypted with AWS KMS
2. **Encryption in Transit**: TLS 1.2+ for API calls
3. **Access Control**: IAM policies control who can access secrets
4. **Audit Logging**: CloudTrail logs all secret access
5. **Automatic Rotation**: Can enable automatic rotation (advanced)
6. **Cross-Region Replication**: DR support built-in

### IAM Best Practices

**Principle of Least Privilege:**

```json
{
  "Effect": "Allow",
  "Action": [
    "secretsmanager:GetSecretValue",
    "secretsmanager:DescribeSecret"
  ],
  "Resource": "arn:aws:secretsmanager:*:*:secret:joblet/*"
}
```

For read-only instances (workers that don't create/update secrets).

**Full Access (for first instance):**

```json
{
  "Effect": "Allow",
  "Action": [
    "secretsmanager:GetSecretValue",
    "secretsmanager:DescribeSecret",
    "secretsmanager:CreateSecret",
    "secretsmanager:UpdateSecret"
  ],
  "Resource": "arn:aws:secretsmanager:*:*:secret:joblet/*"
}
```

### Server Certificate Security

Each instance gets a **unique** server certificate:
- Compromising one server doesn't compromise others
- Can revoke individual server certs without affecting the fleet
- Follows security best practice of instance-specific credentials

### CloudTrail Monitoring

Monitor secret access:

```bash
# Who accessed CA certificates?
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=ResourceName,AttributeValue=joblet/ca-cert \
  --query 'Events[*].[EventTime,Username,EventName]' \
  --output table

# Alert on unexpected access
aws cloudwatch put-metric-alarm \
  --alarm-name JobletSecretAccess \
  --metric-name SecretAccess \
  --namespace AWS/SecretsManager \
  --threshold 100
```

## Troubleshooting

### Secrets Not Found

```bash
# Check if secrets exist
aws secretsmanager list-secrets --filters Key=name,Values=joblet/

# If empty, force regenerate
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true ./certs_gen_with_secretsmanager.sh
```

### Permission Denied

```bash
# Check IAM role
aws sts get-caller-identity

# Test secret access
aws secretsmanager get-secret-value --secret-id joblet/ca-cert

# If permission denied, update IAM policy
```

### Certificate Expired

```bash
# Check certificate expiration
aws secretsmanager get-secret-value \
  --secret-id joblet/ca-cert \
  --query SecretString \
  --output text | openssl x509 -noout -dates

# Force regenerate if expired
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true ./certs_gen_with_secretsmanager.sh
```

### Mixed Mode (Some instances with, some without)

This is **not supported**. You must choose one approach:

- **Option A**: All instances use Secrets Manager
- **Option B**: All instances use embedded certs (no scaling)

Migration path:
1. Generate initial certs with Secrets Manager
2. Terminate all old instances
3. Launch new instances with `USE_SECRETS_MANAGER=true`

## Migration Guide

### From Embedded Certs to Secrets Manager

**Step 1: Extract existing certs (optional)**

If you want to keep your existing CA:

```bash
# On existing instance
sudo cat /opt/joblet/config/joblet-config.yml | \
  yq '.security.caCert' > /tmp/ca-cert.pem

# Upload to Secrets Manager
aws secretsmanager create-secret \
  --name joblet/ca-cert \
  --secret-string file:///tmp/ca-cert.pem
```

**Step 2: Enable on new instances**

Launch new instances with `USE_SECRETS_MANAGER=true`.

**Step 3: Update clients**

Download new client config and distribute.

### From Secrets Manager to Embedded Certs

**Step 1: Download certs**

```bash
# Get certs from Secrets Manager
aws secretsmanager get-secret-value \
  --secret-id joblet/ca-cert \
  --query SecretString \
  --output text > ca-cert.pem
```

**Step 2: Launch without Secrets Manager**

```bash
USE_SECRETS_MANAGER=false ./certs_gen_embedded.sh
```

**Step 3: Delete secrets (optional)**

```bash
aws secretsmanager delete-secret \
  --secret-id joblet/ca-cert \
  --force-delete-without-recovery
```

## Comparison: Embedded vs. Secrets Manager

| Feature                          | Embedded Certs | Secrets Manager |
|----------------------------------|----------------|-----------------|
| Setup Complexity                 | ⭐⭐⭐⭐⭐       | ⭐⭐⭐⭐          |
| Horizontal Scaling               | ❌             | ✅              |
| Load Balancer Support            | ❌             | ✅              |
| One Client Config                | ❌             | ✅              |
| Certificate Rotation             | Manual, complex | Centralized     |
| Compliance (Audit Logs)          | ❌             | ✅              |
| Disaster Recovery                | Manual backup  | Built-in        |
| Multi-Region                     | Manual         | Replicate       |
| Offline / No Internet            | ✅             | ❌              |

## Best Practices

1. **Enable Secrets Manager for production EC2 deployments**
   - Required for auto-scaling

2. **Use embedded certs for:**
   - Development/testing
   - On-premises deployments
   - Single-instance deployments
   - Air-gapped environments

3. **Protect your secrets:**
   - Use IAM policies to restrict access
   - Enable CloudTrail logging
   - Set up alerts for unexpected access
   - Regular certificate rotation

4. **Plan for rotation:**
   - Server certs: Rotate annually per instance
   - Client certs: Rotate every 2 years (requires client updates)
   - CA cert: Rotate every 3 years (requires full fleet update)

5. **Test your DR plan:**
   - Verify cross-region replication
   - Test recovery from secret deletion
   - Document rollback procedures

## See Also

- [AWS Secrets Manager Documentation](https://docs.aws.amazon.com/secretsmanager/)
- [Joblet AWS Deployment Guide](./AWS_DEPLOYMENT.md)
- [Joblet Security Guide](./SECURITY.md)
- [ADR-006: Embedded Certificate Architecture](./adr/006-embedded-certificates.md)
