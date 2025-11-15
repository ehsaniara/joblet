# Scaling Joblet on AWS

Quick guide for deploying Joblet with horizontal scaling on AWS using Secrets Manager for shared certificate management.

## Quick Start

### 1. Create IAM Policy

```bash
cat > joblet-secretsmanager-policy.json << 'EOF'
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
      "Resource": "arn:aws:secretsmanager:*:*:secret:joblet/*"
    }
  ]
}
EOF

# Create and attach to JobletEC2Role
POLICY_ARN=$(aws iam create-policy \
  --policy-name JobletSecretsManagerPolicy \
  --policy-document file://joblet-secretsmanager-policy.json \
  --query 'Policy.Arn' --output text)

aws iam attach-role-policy \
  --role-name JobletEC2Role \
  --policy-arn $POLICY_ARN
```

### 2. Launch Template with User Data

```bash
#!/bin/bash
set -e

export JOBLET_VERSION="latest"
export JOBLET_SERVER_PORT="443"
export ENABLE_CLOUDWATCH="true"

# Enable Secrets Manager for scaling
export USE_SECRETS_MANAGER="true"

curl -fsSL https://raw.githubusercontent.com/ehsaniara/joblet/main/scripts/ec2-user-data.sh \
  -o /tmp/joblet-install.sh
chmod +x /tmp/joblet-install.sh
/tmp/joblet-install.sh 2>&1 | tee /var/log/joblet-install.log
```

### 3. Create Auto Scaling Group

```bash
# Create launch template
aws ec2 create-launch-template \
  --launch-template-name joblet-template \
  --version-description "Joblet with Secrets Manager" \
  --launch-template-data '{
    "ImageId": "ami-xxxxxxxxx",
    "InstanceType": "t3.medium",
    "IamInstanceProfile": {"Name": "JobletEC2Role"},
    "SecurityGroupIds": ["sg-xxxxxxxxx"],
    "UserData": "'$(base64 -w0 user-data.sh)'"
  }'

# Create auto scaling group
aws autoscaling create-auto-scaling-group \
  --auto-scaling-group-name joblet-asg \
  --launch-template LaunchTemplateName=joblet-template,Version='$Latest' \
  --min-size 2 \
  --max-size 10 \
  --desired-capacity 2 \
  --vpc-zone-identifier "subnet-xxx,subnet-yyy" \
  --target-group-arns arn:aws:elasticloadbalancing:...
```

### 4. Verify Shared Certificates

```bash
# Check secrets were created by first instance
aws secretsmanager list-secrets \
  --filters Key=name,Values=joblet/ \
  --output table

# Expected output:
# joblet/ca-cert       - Shared CA certificate
# joblet/ca-key        - Shared CA private key
# joblet/client-cert   - Shared client certificate
# joblet/client-key    - Shared client private key
```

### 5. Download Client Config

```bash
# From ANY instance in the ASG
INSTANCE_IP=$(aws ec2 describe-instances \
  --filters "Name=tag:aws:autoscaling:groupName,Values=joblet-asg" \
  --query 'Reservations[0].Instances[0].PublicIpAddress' \
  --output text)

scp ec2-user@$INSTANCE_IP:/opt/joblet/config/rnx-config.yml ~/.rnx/

# This config works with ALL instances!
```

## Architecture

```
┌─────────────────────────────────────┐
│    AWS Secrets Manager              │
│  ┌────────────────────────────┐    │
│  │ joblet/ca-cert    (Shared) │    │
│  │ joblet/ca-key     (Shared) │    │
│  │ joblet/client-cert(Shared) │    │
│  │ joblet/client-key (Shared) │    │
│  └────────────────────────────┘    │
└────────────┬────────────────────────┘
             ↓
    ┌────────┴────────┐
    ↓                 ↓
┌─────────┐       ┌─────────┐
│ EC2 #1  │       │ EC2 #2  │
│ (Cert1) │       │ (Cert2) │
└────┬────┘       └────┬────┘
     └────────┬────────┘
              ↓
      ┌───────────────┐
      │   Clients     │
      │ (One Config)  │
      └───────────────┘
```

## Load Balancer Setup

### Network Load Balancer (Recommended)

```bash
# Create NLB with TLS passthrough
aws elbv2 create-load-balancer \
  --name joblet-nlb \
  --type network \
  --subnets subnet-xxx subnet-yyy

# Create target group (TCP)
aws elbv2 create-target-group \
  --name joblet-tg \
  --protocol TCP \
  --port 443 \
  --vpc-id vpc-xxx \
  --health-check-protocol TCP

# Create listener (TLS passthrough)
aws elbv2 create-listener \
  --load-balancer-arn arn:aws:elasticloadbalancing:... \
  --protocol TCP \
  --port 443 \
  --default-actions Type=forward,TargetGroupArn=arn:aws:...
```

### Application Load Balancer

```bash
# Note: Requires TLS termination at ALB, then re-encryption to backends
# More complex setup - NLB recommended for gRPC
```

## Scaling Scenarios

### Scenario 1: Manual Scale Up

```bash
# Increase desired capacity
aws autoscaling set-desired-capacity \
  --auto-scaling-group-name joblet-asg \
  --desired-capacity 5

# New instances automatically:
# 1. Retrieve CA/client certs from Secrets Manager
# 2. Generate unique server certificate
# 3. Start accepting requests
```

### Scenario 2: Auto-Scale on CPU

```bash
# Scale up when CPU > 70%
aws autoscaling put-scaling-policy \
  --auto-scaling-group-name joblet-asg \
  --policy-name scale-up \
  --scaling-adjustment 2 \
  --adjustment-type ChangeInCapacity \
  --metric-aggregation-type Average

# Scale down when CPU < 30%
aws autoscaling put-scaling-policy \
  --auto-scaling-group-name joblet-asg \
  --policy-name scale-down \
  --scaling-adjustment -1 \
  --adjustment-type ChangeInCapacity \
  --metric-aggregation-type Average
```

### Scenario 3: Blue-Green Deployment

```bash
# Current: Green ASG with 5 instances (using joblet v1.0)
# New: Blue ASG with 5 instances (using joblet v1.1)

# Both ASGs use the same Secrets Manager secrets
# Clients can connect to either ASG without config changes

# Traffic shift:
# 1. Create Blue ASG with new version
# 2. Attach to same target group (50/50 split)
# 3. Monitor for errors
# 4. Shift 100% to Blue
# 5. Terminate Green ASG
```

## Certificate Management

### View Current Certificates

```bash
# Check CA certificate expiration
aws secretsmanager get-secret-value \
  --secret-id joblet/ca-cert \
  --query SecretString --output text | \
  openssl x509 -noout -dates

# Check client certificate expiration
aws secretsmanager get-secret-value \
  --secret-id joblet/client-cert \
  --query SecretString --output text | \
  openssl x509 -noout -dates
```

### Rotate Server Certificates (Per Instance)

```bash
# SSH to instance
ssh ec2-user@<instance-ip>

# Regenerate server cert (CA/client stay the same)
sudo /usr/local/bin/certs_gen_with_secretsmanager.sh

# Restart service
sudo systemctl restart joblet
```

### Rotate CA and Client Certificates (Full Fleet)

```bash
# Step 1: Force regenerate on one instance
ssh ec2-user@<instance-ip>
sudo USE_SECRETS_MANAGER=true FORCE_REGENERATE=true \
  /usr/local/bin/certs_gen_with_secretsmanager.sh

# Step 2: Rolling instance refresh
aws autoscaling start-instance-refresh \
  --auto-scaling-group-name joblet-asg

# Step 3: Download new client config
scp ec2-user@<instance-ip>:/opt/joblet/config/rnx-config.yml ~/.rnx/

# Step 4: Distribute to all clients
```


### CloudWatch Metrics

```bash
# Monitor secret access
aws cloudwatch get-metric-statistics \
  --namespace AWS/SecretsManager \
  --metric-name SecretAccess \
  --dimensions Name=SecretId,Value=joblet/ca-cert \
  --start-time $(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%S) \
  --end-time $(date -u +%Y-%m-%dT%H:%M:%S) \
  --period 300 \
  --statistics Sum

# Alert on unexpected access
aws cloudwatch put-metric-alarm \
  --alarm-name JobletUnexpectedSecretAccess \
  --metric-name SecretAccess \
  --namespace AWS/SecretsManager \
  --statistic Sum \
  --period 300 \
  --threshold 100 \
  --comparison-operator GreaterThanThreshold
```

### CloudTrail Audit

```bash
# Who accessed CA certificate?
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=ResourceName,AttributeValue=joblet/ca-cert \
  --query 'Events[*].[EventTime,Username,EventName,SourceIPAddress]' \
  --output table
```

## Troubleshooting

### Problem: New instances can't get certificates

**Symptom:**
```
ERROR: Failed to retrieve secret joblet/ca-cert
ERROR: Permission denied
```

**Solution:**
```bash
# Check IAM role
aws ec2 describe-instances --instance-ids i-xxx \
  --query 'Reservations[0].Instances[0].IamInstanceProfile'

# Attach missing role
aws ec2 associate-iam-instance-profile \
  --instance-id i-xxx \
  --iam-instance-profile Name=JobletEC2Role

# Terminate instance (ASG will launch new one)
aws ec2 terminate-instances --instance-ids i-xxx
```

### Problem: Certificates expired

**Symptom:**
```
ERROR: Certificate expired
ERROR: Certificate verification failed
```

**Solution:**
```bash
# Force regenerate
USE_SECRETS_MANAGER=true FORCE_REGENERATE=true \
  /usr/local/bin/certs_gen_with_secretsmanager.sh

# Trigger instance refresh
aws autoscaling start-instance-refresh \
  --auto-scaling-group-name joblet-asg
```

### Problem: Mixed certificate versions

**Symptom:**
Some instances work, others don't. Client gets intermittent connection errors.

**Solution:**
```bash
# This means some instances have old certs
# Force refresh all instances
aws autoscaling start-instance-refresh \
  --auto-scaling-group-name joblet-asg \
  --preferences MinHealthyPercentage=50
```

## Best Practices

1. **Always use Secrets Manager for production ASGs**
   - Required for auto-scaling

2. **Use Network Load Balancer (NLB)**
   - Better for gRPC/TLS passthrough
   - Lower latency than ALB

3. **Enable CloudTrail logging**
   - Audit who accesses secrets
   - Compliance requirement

4. **Set up certificate expiration alerts**
   - CloudWatch alarm 30 days before expiry
   - Automated rotation (future)

5. **Test your scaling before production**
   - Verify new instances get shared certs
   - Verify client can connect to all instances
   - Test auto-scaling triggers

6. **Plan certificate rotation**
   - Server certs: Annually (per-instance, easy)
   - Client certs: Every 2 years (requires client updates)
   - CA cert: Every 3 years (full fleet update)

## See Also

- [Complete Secrets Manager Guide](./SECRETS_MANAGER_INTEGRATION.md)
- [AWS Deployment Guide](./AWS_DEPLOYMENT.md)
- [ADR-012: Secrets Manager for Horizontal Scaling](./adr/012-secrets-manager-for-horizontal-scaling.md)
- [Security Guide](./SECURITY.md)
