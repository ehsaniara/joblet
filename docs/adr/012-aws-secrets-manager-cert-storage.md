# ADR-012: AWS Secrets Manager for Certificate Management

## Status

Proposed

## Context

Joblet currently generates a new Certificate Authority (CA) for every installation. While this is secure, it creates
operational challenges:

**Current behavior (embedded certs only):**

- Every EC2 instance generates a new CA
- Every EC2 instance generates new client certificates
- Each instance requires a different client config file
- Cannot easily add more instances

**Problems:**

1. **Multiple client configs:** Users need different `rnx-config.yml` files for each server
2. **Certificate sprawl:** No centralized CA management
3. **No compliance:** No audit trail of certificate generation/access
4. **Manual rotation:** Each instance must be updated independently

**What we need:**

- Shared Root CA across all EC2 instances
- Shared Admin Client certificate across all EC2 instances
- Unique Server certificate per EC2 instance
- One client config file works with all instances
- Compliance: audit logs, encryption at rest, IAM access control

## Decision

Use AWS Secrets Manager to store and share Root CA and Admin Client certificates across EC2 instances, while generating
unique Server certificates per instance.

### Architecture

```
┌─────────────────────────────────────┐
│    AWS Secrets Manager              │
│  ┌────────────────────────────┐    │
│  │ joblet/ca-cert    (Shared) │    │  Generated ONCE by first instance
│  │ joblet/ca-key     (Shared) │    │  Reused by ALL instances
│  │ joblet/client-cert(Shared) │    │
│  │ joblet/client-key (Shared) │    │
│  └────────────────────────────┘    │
└────────────┬────────────────────────┘
             │
    ┌────────┼────────┐
    ↓        ↓        ↓
┌─────┐  ┌─────┐  ┌─────┐
│EC2#1│  │EC2#2│  │EC2#3│  Each generates unique server cert
│Srv-A│  │Srv-B│  │Srv-C│  All signed by shared CA
└──┬──┘  └──┬──┘  └──┬──┘
   └────────┼────────┘
            ↓
    ┌───────────────┐
    │   Clients     │  One config works with all servers
    │ (One Config)  │
    └───────────────┘
```

### Certificate Flow

**First EC2 Instance:**

```bash
1. Check Secrets Manager for joblet/ca-cert
   → Not found
2. Generate Root CA certificate and private key
3. Store in Secrets Manager (joblet/ca-cert, joblet/ca-key)
4. Generate Admin Client certificate (signed by CA)
5. Store in Secrets Manager (joblet/client-cert, joblet/client-key)
6. Generate Server certificate (signed by CA) - UNIQUE to this instance
7. Embed all certificates in config files
```

**Subsequent EC2 Instances:**

```bash
1. Check Secrets Manager for joblet/ca-cert
   → Found!
2. Retrieve Root CA certificate and private key
3. Validate (not expired, properly signed)
4. Retrieve Admin Client certificate and private key
5. Validate (not expired, properly signed)
6. Generate NEW Server certificate (signed by shared CA) - UNIQUE to this instance
7. Embed all certificates in config files
```

### Secret Naming

**Shared secrets (generated once):**

```
joblet/ca-cert          # Root CA certificate
joblet/ca-key           # Root CA private key
joblet/client-cert      # Admin client certificate
joblet/client-key       # Admin client private key
```

No per-instance paths. All instances share the same CA and client certificates.

### Behavior

**On EC2 with IAM permissions:**

```bash
# Auto-detect EC2, check Secrets Manager
./certs_gen_with_secretsmanager.sh

# First instance output:
✨ CA Certificate: Generated and stored in Secrets Manager (shared)
✨ Client Certificate: Generated and stored in Secrets Manager (shared)
🆕 Server Certificate: Generated locally (instance-specific)

# Second instance output:
✅ CA Certificate: Retrieved from Secrets Manager (shared)
✅ Client Certificate: Retrieved from Secrets Manager (shared)
🆕 Server Certificate: Generated locally (instance-specific)
```

**Result:**

- All instances can use the same Root CA
- All instances accept the same client certificate
- Each instance has a unique server certificate (security best practice)
- One `rnx-config.yml` works with all instances

## Consequences

### The Good

**Operational Benefits:**

- ✅ One client config for all EC2 instances
- ✅ Can add new instances easily (reuse existing CA)
- ✅ Simplified certificate management
- ✅ Foundation for auto-scaling (future)
- ✅ Foundation for load balancers (future)

**Security Benefits:**

- ✅ Secrets encrypted with AWS KMS
- ✅ CloudTrail logs all access
- ✅ IAM policies control access
- ✅ Certificate validation and expiration checks
- ✅ Each server gets unique certificate

**Compliance:**

- ✅ SOC 2: Centralized secret management, audit logs
- ✅ HIPAA: Encrypted storage, access controls
- ✅ PCI DSS: Key management requirements

**Certificate Lifecycle:**

```bash
# View all shared certificates
aws secretsmanager list-secrets --filters Key=name,Values=joblet/

# Check CA expiration
aws secretsmanager get-secret-value --secret-id joblet/ca-cert \
  --query SecretString --output text | openssl x509 -noout -dates

# Audit access
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=ResourceName,AttributeValue=joblet/ca-cert
```

### The Trade-offs

**Cost:**

- Not per-instance (all instances share the same 4 secrets)
- Compared to EC2 costs (~$30/month), this is negligible

**Dependencies:**

- Requires AWS Secrets Manager availability
- Requires IAM permissions
- First instance must succeed in storing secrets

**Still Using Embedded Certs:**

- Service reads certificates from config files (not directly from Secrets Manager)
- Secrets Manager is a storage/sharing layer
- If Secrets Manager is down, existing instances continue working

### When to Use

**✅ Use this approach:**

- AWS EC2 deployments
- Need multiple instances with same CA
- Want one client config for all servers
- Compliance requirements
- Planning future scaling/load balancing

**❌ Don't use:**

- Single instance deployment (no benefit)
- On-premises deployment (no AWS Secrets Manager)
- Development/testing (unnecessary cost)

## Implementation

### IAM Policy Required

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
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

### Usage

**Auto-detect (default on EC2):**

```bash
./certs_gen_with_secretsmanager.sh
```

**Explicit enable:**

```bash
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh
```

**Disable (embedded only):**

```bash
USE_SECRETS_MANAGER=false ./certs_gen_embedded.sh
```

**Force regenerate CA (rotation):**

```bash
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true ./certs_gen_with_secretsmanager.sh
```

### Certificate Rotation

**Rotate Server Certificates (per instance, easy):**

```bash
# On each instance, regenerate server cert (CA/client stay the same)
sudo /usr/local/bin/certs_gen_with_secretsmanager.sh
sudo systemctl restart joblet
```

**Rotate CA and Client Certificates (all instances, coordinated):**

```bash
# Step 1: Force regenerate on one instance
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true \
  /usr/local/bin/certs_gen_with_secretsmanager.sh

# Step 2: Restart all other instances to pick up new shared certs
# Step 3: Distribute new client config to all users
```

## Alternatives Considered

### Alternative 1: Keep Embedded Only (Status Quo)

**Pros:** Simple, no cost
**Cons:** Each instance has different CA, multiple client configs, no compliance

**Decision:** Rejected for multi-instance AWS deployments.

### Alternative 2: Per-Instance Secret Storage

Store each instance's certificates separately:

```
joblet/instances/i-xxx/ca-cert
joblet/instances/i-xxx/client-cert
```

**Pros:** Isolation, easy cleanup
**Cons:** Doesn't solve the "different CA per instance" problem

**Decision:** Rejected. Need shared CA.

### Alternative 3: AWS Systems Manager Parameter Store

**Pros:** Free for standard parameters
**Cons:** Not designed for secrets, 4KB limit, no rotation

**Decision:** Rejected. Secrets Manager is purpose-built.

### Alternative 4: Service Loads from Secrets Manager

Instead of embedding certs in config files, load directly from Secrets Manager.

**Pros:** True secret management, refresh without restart
**Cons:** Breaking change, service depends on Secrets Manager availability

**Decision:** Deferred to future. This release keeps embedded certs working.

## Success Criteria

- [x] First instance creates 4 shared secrets in Secrets Manager
- [x] Second instance retrieves shared CA/client from Secrets Manager
- [x] Each instance generates unique server certificate
- [x] One client config works with all instances
- [x] Graceful fallback if Secrets Manager unavailable
- [x] Certificate validation and expiration checks
- [x] CloudTrail audit logs
- [x] IAM access control

## Use Cases Enabled

### Use Case 1: Adding a Second Instance

```bash
# Already have one instance with CA in Secrets Manager
# Launch second instance with same user data
# → Automatically retrieves shared CA/client
# → Generates new server cert
# → Same client config works!
```

### Use Case 2: Blue-Green Deployment

```bash
# Blue environment running (CA in Secrets Manager)
# Deploy Green environment
# → Reuses same CA/client certs
# → Clients can connect to either environment
# → No cert distribution needed
```

### Use Case 3: Certificate Rotation

```bash
# Check all certs across instances
aws secretsmanager list-secrets --filters Key=name,Values=joblet/

# Rotate CA (coordinated)
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true ./certs_gen_with_secretsmanager.sh
# Restart all instances, distribute new client config
```

## Future Enhancements

### Phase 2: Auto-Scaling Groups

- Create launch template with Secrets Manager enabled
- ASG automatically reuses shared CA/client
- Scale up/down seamlessly

### Phase 3: Load Balancer Support

- NLB with TLS passthrough
- All servers accept same client cert
- Distribute traffic across instances

### Phase 4: Automatic Rotation

- Lambda function triggered by Secrets Manager
- Generates new certificates
- Updates secrets
- Notifies instances to restart

## References

- [ADR-006: Embedded Certificate Architecture](./006-embedded-certificates.md)
- [AWS Secrets Manager Documentation](https://docs.aws.amazon.com/secretsmanager/)
- [Certificate Management Best Practices](https://csrc.nist.gov/publications/detail/sp/800-57-part-1/rev-5/final)

## Decision Log

- **2025-01-29**: ADR proposed
- **Pending**: Team review
- **Pending**: Security review
- **Pending**: Implementation testing
- **Pending**: Production deployment

---

