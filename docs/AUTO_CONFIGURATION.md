# Auto-Configuration Guide

Joblet automatically detects your deployment environment and configures optimal defaults for logging.

## Overview

When you install Joblet, it automatically:

1. **Detects** if you're running on AWS EC2 or on-premises
2. **Configures** logging based on your environment
3. **Optimizes** for cost and performance

## Environment Detection

### AWS EC2 Detection

Joblet uses multiple methods to detect AWS EC2 environments:

1. **IMDSv2 (Primary)** - Queries AWS Instance Metadata Service v2
   - Endpoint: `http://169.254.169.254/latest/api/token`
   - Retrieves: Instance ID, Region, Availability Zone
   - Timeout: 2 seconds

2. **System Files (Fallback)** - Checks hypervisor signatures
   - File: `/sys/hypervisor/uuid`
   - Pattern: Starts with `ec2`

3. **DMI Information (Additional)** - Checks system vendor
   - Files: `/sys/class/dmi/id/sys_vendor`, `/sys/class/dmi/id/bios_vendor`
   - Pattern: Contains `Amazon` or `EC2`

### Detection Timing

- Detection runs **once** during installation
- Takes < 2 seconds (IMDSv2 timeout)
- Results are cached in the generated configuration

## Auto-Configuration Behavior

### AWS EC2 Environment

When Joblet detects AWS EC2, it automatically applies **cloud-optimized** settings:

```yaml
buffers:
  # Local logs DISABLED for EC2
  log_persistence:
    enabled: false                # Disabled for EC2 (using CloudWatch)

  # CloudWatch ENABLED for EC2
  aws_cloudwatch:
    enabled: true                # Auto-enabled for EC2 deployment
    region: "us-west-2"         # Auto-detected from EC2 metadata
    log_group: "/aws/joblet/i-1234567890abcdef0" # Instance-specific
    auth_method: "iam_role"      # Uses EC2 instance profile
    compression: true            # Optimize bandwidth
    batch_max_events: 10000      # Maximize batching
    batch_interval: "1s"         # Balance latency/cost
```

**Benefits:**
- ✅ Reduces EBS storage costs (no local logs)
- ✅ Centralized log management via CloudWatch
- ✅ Automatic log retention policies
- ✅ Better scalability for autoscaling groups
- ✅ Zero configuration required

**Installation Output:**
```
🔍 Detecting environment...
✅ AWS EC2 environment detected!
   Instance ID: i-1234567890abcdef0
   Region: us-west-2
   Type: t3.medium

✅ Applying cloud-optimized configuration...
   ✓ Local disk logs: DISABLED (optimize EBS usage)
   ✓ CloudWatch Logs: ENABLED
   ✓ Region: us-west-2
   ✓ Log Group: /aws/joblet/i-1234567890abcdef0
```

### On-Premises Environment

When Joblet detects on-premises deployment, it uses **standard** settings:

```yaml
buffers:
  # Local logs ENABLED for on-premises
  log_persistence:
    enabled: true                # Enable local disk persistence
    directory: "/opt/joblet/logs"
    retention_days: 7
    rotation_size_bytes: 2097152  # 2MB

  # CloudWatch DISABLED for on-premises
  aws_cloudwatch:
    enabled: false               # Disabled by default
```

**Benefits:**
- ✅ Fast local log access
- ✅ No cloud dependencies
- ✅ Works in air-gapped environments
- ✅ Lower latency for log retrieval

**Installation Output:**
```
🔍 Detecting environment...
🏢 On-premises environment detected

Using standard configuration:
   ✓ Local disk logs: ENABLED (/opt/joblet/logs)
   ✗ CloudWatch Logs: DISABLED
```

## Manual Overrides

You can override auto-detection at any time by editing the configuration file.

### Enable CloudWatch on On-Premises

Edit `/opt/joblet/config/joblet-config.yml`:

```yaml
buffers:
  aws_cloudwatch:
    enabled: true
    region: "us-west-2"         # Specify your region
    auth_method: "credentials"   # Use AWS credentials
    log_group: "/aws/joblet/my-datacenter"
```

Then restart the service:
```bash
sudo systemctl restart joblet
```

### Enable Local Logs on EC2 (Hybrid Mode)

Edit `/opt/joblet/config/joblet-config.yml`:

```yaml
buffers:
  log_persistence:
    enabled: true                # Re-enable local logs

  aws_cloudwatch:
    enabled: true                # Keep CloudWatch too
```

This enables **hybrid mode** where logs go to both local disk AND CloudWatch.

## IAM Requirements for EC2

For CloudWatch to work on EC2, your instance needs an IAM role with these permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents",
        "logs:DescribeLogGroups"
      ],
      "Resource": "arn:aws:logs:*:*:log-group:/aws/joblet/*"
    }
  ]
}
```

**Attach this policy to your EC2 instance profile** before installing Joblet.

## Verification

### Check Current Configuration

```bash
# View the active configuration
sudo cat /opt/joblet/config/joblet-config.yml | grep -A5 "log_persistence:"
sudo cat /opt/joblet/config/joblet-config.yml | grep -A5 "aws_cloudwatch:"
```

### View CloudWatch Logs (EC2 only)

```bash
# Using AWS CLI
aws logs tail /aws/joblet/$(ec2-metadata --instance-id | cut -d' ' -f2) --follow

# Or via AWS Console
# Navigate to CloudWatch > Log groups > /aws/joblet/i-xxxxxxxxxxxxx
```

### View Local Logs (On-Premises)

```bash
# View logs for a specific job
sudo ls -lh /opt/joblet/logs/
sudo cat /opt/joblet/logs/job-<job-id>.log
```

## Troubleshooting

### CloudWatch Not Working on EC2

1. **Check IAM role permissions:**
   ```bash
   aws sts get-caller-identity
   ```

2. **Verify CloudWatch is enabled:**
   ```bash
   grep "aws_cloudwatch:" -A3 /opt/joblet/config/joblet-config.yml
   ```

3. **Check service logs:**
   ```bash
   sudo journalctl -u joblet -n 100 | grep -i cloudwatch
   ```

4. **Test AWS connectivity:**
   ```bash
   curl http://169.254.169.254/latest/meta-data/instance-id
   ```

### Force Re-detection

To force re-detection and reconfiguration:

```bash
# Backup current config
sudo cp /opt/joblet/config/joblet-config.yml /opt/joblet/config/joblet-config.yml.backup

# Regenerate configuration
sudo JOBLET_SERVER_ADDRESS=$(hostname -I | awk '{print $1}') \
     /usr/local/bin/certs_gen_embedded.sh

# Restart service
sudo systemctl restart joblet
```

## Cost Optimization

### CloudWatch Costs

Default configuration optimizes for cost:

- **Sampling**: Debug logs sampled at 10%, Trace at 1%
- **Compression**: Enabled by default
- **Batching**: Up to 10,000 events per batch
- **Retention**: 30 days (configurable)

**Estimated costs** (us-west-2):
- Ingestion: $0.50 per GB
- Storage: $0.03 per GB/month
- Typical job (1000 logs): ~$0.001

### Disable Sampling

For critical jobs, disable sampling:

```yaml
aws_cloudwatch:
  sampling_enabled: false  # Keep all logs
```

## Migration Scenarios

### Migrating from On-Premises to EC2

1. **Deploy on EC2** with instance profile
2. **Install Joblet** - Auto-detects EC2
3. **CloudWatch enabled automatically**
4. Migrate workloads

### Migrating from EC2 to On-Premises

1. **Backup CloudWatch logs** before decommission
2. **Install on new server** - Auto-detects on-premises
3. **Local logs enabled automatically**
4. Migrate workloads

## Advanced Configuration

See [CONFIGURATION.md](CONFIGURATION.md) for detailed configuration options including:
- Custom log groups
- Multiple CloudWatch regions
- Cross-account logging
- Hybrid logging strategies
- Custom sampling rates
