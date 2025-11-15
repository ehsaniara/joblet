# Certificate Management: Embedded vs. Secrets Manager

Quick comparison guide to help you choose the right certificate management approach for your Joblet deployment.

## TL;DR

| Scenario                   | Recommendation             |
|----------------------------|----------------------------|
| **Single EC2 instance**    | Embedded (default)         |
| **Multiple EC2 instances** | Secrets Manager            |
| **Auto Scaling Groups**    | Secrets Manager (required) |
| **Load Balancer**          | Secrets Manager (required) |
| **On-premises**            | Embedded (only option)     |
| **Dev/Test**               | Embedded (simpler)         |
| **Production (AWS)**       | Secrets Manager (scalable) |
| **Air-gapped**             | Embedded (only option)     |
| **Kubernetes**             | Embedded + cert-manager    |

## Detailed Comparison

| Feature                  | Embedded Certs          | Secrets Manager      |
|--------------------------|-------------------------|----------------------|
| **Setup**                | ✅ One command           | ⚠️ IAM + one command |
| **Horizontal Scaling**   | ❌ No                    | ✅ Yes                |
| **Load Balancer**        | ❌ No                    | ✅ Yes                |
| **One Client Config**    | ❌ No                    | ✅ Yes                |
| **Auto-scaling**         | ❌ No                    | ✅ Yes                |
| **Certificate Rotation** | ⚠️ Manual, per-instance | ✅ Centralized        |
| **Audit Logs**           | ❌ No                    | ✅ CloudTrail         |
| **Encryption at Rest**   | ⚠️ File permissions     | ✅ KMS                |
| **Multi-region**         | ⚠️ Manual               | ✅ Replicate secrets  |
| **Disaster Recovery**    | ⚠️ EBS snapshots        | ✅ Secret replication |
| **On-premises Support**  | ✅ Yes                   | ❌ AWS only           |
| **Air-gapped**           | ✅ Yes                   | ❌ No                 |
| **Internet Required**    | ❌ No                    | ✅ Yes                |
| **Dependencies**         | None                    | AWS CLI, IAM         |

## Use Case Matrix

### ✅ Use Embedded Certs

**Best for:**

- Single-instance deployments
- Development and testing environments
- On-premises installations
- Air-gapped environments
- Single instance deployments (<2 instances)
- No scaling requirements
- Quick proof-of-concept

**Example scenarios:**

```bash
# Development laptop
./certs_gen_embedded.sh

# Single production server (on-prem)
./certs_gen_embedded.sh

# Lab environment
./certs_gen_embedded.sh

# Air-gapped secure network
./certs_gen_embedded.sh
```

### ✅ Use Secrets Manager

**Best for:**

- AWS EC2 deployments with 2+ instances
- Auto Scaling Groups
- Load balancer setups (ALB/NLB)
- Blue-green deployments
- Multi-AZ high availability
- Production environments requiring scaling
- Compliance requirements (audit logs)
- Enterprise deployments

**Example scenarios:**

```bash
# Auto-scaling group
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh

# Multi-instance production (AWS)
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh

# Behind load balancer
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh

# Blue-green deployment
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh
```

## Scaling Scenarios

### Scenario 1: Small Startup (1-2 instances)

**Recommendation:** Embedded Certs

**Why:**

- Low cost (no Secrets Manager fees)
- Simple setup
- Can manually manage 1-2 servers
- May not need auto-scaling yet

**Migration path:**
When you grow to 3+ instances, migrate to Secrets Manager.

### Scenario 2: Growing Company (3-10 instances)

**Recommendation:** Secrets Manager

**Why:**

- Simplified certificate management
- One client config for all servers
- Can add/remove instances easily
- Foundation for future auto-scaling

### Scenario 3: Enterprise (10+ instances)

**Recommendation:** Secrets Manager (required)

**Why:**

- Auto-scaling is essential
- Load balancers required for reliability
- Certificate rotation needs to be centralized
- Compliance requires audit logs
- DR/multi-region likely needed

## Migration Guide

### From Embedded → Secrets Manager

**When to migrate:**

- Scaling beyond 2 instances
- Adding auto-scaling
- Need load balancer
- Want centralized cert management

**How to migrate:**

```bash
# Step 1: Enable on one new instance
USE_SECRETS_MANAGER=true ./certs_gen_with_secretsmanager.sh

# Step 2: Verify secrets created
aws secretsmanager list-secrets --filters Key=name,Values=joblet/

# Step 3: Terminate old instances

# Step 4: Launch new instances (auto-enable on EC2)

# Step 5: Distribute new client config
scp ec2-user@new-instance:/opt/joblet/config/rnx-config.yml ~/.rnx/
```

**Downtime:** ~5 minutes (rolling restart)

### From Secrets Manager → Embedded

**When to migrate:**

- Moving off AWS
- Moving to (single instance)
- Simplification (no longer scaling)
- Air-gapped deployment

**How to migrate:**

```bash
# Step 1: Download certs from Secrets Manager
aws secretsmanager get-secret-value \
  --secret-id joblet/ca-cert \
  --query SecretString --output text > ca-cert.pem

# Step 2: Launch with embedded mode
USE_SECRETS_MANAGER=false ./certs_gen_embedded.sh

# Step 3: Optional - delete secrets
aws secretsmanager delete-secret --secret-id joblet/ca-cert
```

**Downtime:** ~5 minutes (restart)

## Decision Tree

```
Are you deploying on AWS EC2?
├─ No → Use Embedded Certs
│       (On-premises, laptop, air-gapped)
│
└─ Yes → How many instances?
    ├─ 1 instance
    │  └─ Will you scale in next 6 months?
    │     ├─ No → Use Embedded Certs
    │     └─ Yes → Use Secrets Manager
    │              (Easier to start with scaling support)
    │
    ├─ 2-3 instances
    │  └─ Need auto-scaling or load balancer?
    │     ├─ No → Use Embedded Certs
    │     └─ Yes → Use Secrets Manager (required)
    │
    └─ 4+ instances
       └─ Use Secrets Manager (required)
          (Manual cert management too complex)
```

### ❌ Using Embedded Certs for ASG

**Problem:**

- Each instance has different CA
- Client config doesn't work
- Load balancer fails TLS validation

**Solution:** Use Secrets Manager

### ❌ Using Secrets Manager for Single Instance

**Problem:**

- Extra complexity (IAM setup)
- No scaling benefit

**Solution:** Use Embedded Certs

### ❌ Mixed Mode (Some instances with, some without)

**Problem:**

- Certificate incompatibility
- Client confusion
- Operational nightmare

**Solution:** Choose one approach for entire deployment

### ❌ Not Planning for Scale

**Problem:**

- Start with embedded certs
- Need to scale at 3am during incident
- Can't add instances quickly
- Manual migration during crisis

**Solution:** If there's any chance of scaling, start with Secrets Manager

## Recommendations by Environment

### Development

**Use:** Embedded Certs
**Reason:** Simplicity, no AWS costs

### Staging/QA

**Use:** Secrets Manager (if production uses it)
**Reason:** Test production setup

### Production (Single Instance)

**Use:** Embedded Certs
**Reason:** No scaling needed

### Production (2-3 Instances)

**Use:** Secrets Manager

### Production (4+ Instances)

**Use:** Secrets Manager
**Reason:** Required for management

### On-Premises

**Use:** Embedded Certs
**Reason:** Only option (no AWS)

### Hybrid (AWS + On-prem)

**Use:** Both (Secrets Manager on AWS, Embedded on-prem)
**Reason:** Different clients for each environment

## FAQs

**Q: Can I start with Embedded and migrate later?**
A: Yes! Migration takes ~15 minutes.

**Q: What if I'm not sure if I'll scale?**
A: Start with Secrets Manager if on AWS. Minimal cost, maximum flexibility.

**Q: Does Secrets Manager work on-premises?**
A: No, AWS only. Use Embedded for on-premises.

**Q: Can I use both?**
A: Not recommended for same environment. Choose one per deployment.

**Q: What's the break-even point?**

**Q: Is Secrets Manager required for load balancers?**
A: Yes, for TLS passthrough with mTLS. All servers need same CA/client cert.

**Q: What about Kubernetes?**
A: Use Embedded + cert-manager. Kubernetes has its own secret management.

**Q: Can I use HashiCorp Vault instead?**
A: Yes, but you'll need to implement it yourself. This guide is for AWS Secrets Manager.

## See Also

- [Secrets Manager Integration Guide](./SECRETS_MANAGER_INTEGRATION.md)
- [Scaling on AWS Guide](./SCALING_ON_AWS.md)
- [ADR-006: Embedded Certificates](./adr/006-embedded-certificates.md)
- [ADR-012: Secrets Manager for Horizontal Scaling](./adr/012-secrets-manager-for-horizontal-scaling.md)
