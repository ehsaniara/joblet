# Troubleshooting Guide

Comprehensive troubleshooting guide for common Joblet issues, error messages, and solutions.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Installation Issues](#installation-issues)
- [Connection Problems](#connection-problems)
- [Job Execution Issues](#job-execution-issues)
- [Resource and Performance](#resource-and-performance)
- [Volume Problems](#volume-problems)
- [Network Issues](#network-issues)
- [Certificate and Security](#certificate-and-security)
- [Log Analysis](#log-analysis)
- [Getting Help](#getting-help)

## Quick Diagnostics

### Health Check Commands

```bash
# 1. Check server status
sudo systemctl status joblet

# 2. Test client connectivity
rnx list

# 3. Check server logs
sudo journalctl -u joblet -f

# 4. Verify listening port
sudo ss -tlnp | grep 50051

# 5. Test simple job
rnx run echo "health check"

# 6. Check system resources
rnx monitor status
```

### Common Error Patterns

| Error Pattern                      | Likely Cause         | Quick Fix                     |
|------------------------------------|----------------------|-------------------------------|
| `connection refused`               | Server not running   | `sudo systemctl start joblet` |
| `certificate verify failed`        | Certificate mismatch | Regenerate certificates       |
| `permission denied`                | Role/auth issue      | Check client certificate OU   |
| `no such file or directory`        | Missing upload       | Verify file exists            |
| `resource temporarily unavailable` | Resource limit       | Increase limits               |

## Installation Issues

### Binary Not Found

```bash
# Error: "joblet: command not found"

# Check if binary exists
ls -la /usr/local/bin/joblet

# If missing, reinstall
curl -L https://github.com/ehsaniara/joblet/releases/latest/download/joblet-linux-amd64.tar.gz | tar xz
sudo mv joblet /usr/local/bin/
sudo chmod +x /usr/local/bin/joblet
```

### Permission Denied

```bash
# Error: "permission denied" when starting joblet

# Fix binary permissions
sudo chmod +x /usr/local/bin/joblet

# Fix directory permissions
sudo mkdir -p /opt/joblet/{config,state,jobs,volumes}
sudo chown -R root:root /opt/joblet
sudo chmod -R 755 /opt/joblet
```

### Systemd Service Issues

```bash
# Service fails to start

# Check service file
sudo systemctl cat joblet

# Fix common service file issues
sudo tee /etc/systemd/system/joblet.service > /dev/null <<EOF
[Unit]
Description=Joblet Job Execution Service
After=network-online.target

[Service]
Type=simple
User=root
Group=root
ExecStart=/usr/local/bin/joblet
Restart=always
Environment="JOBLET_CONFIG_PATH=/opt/joblet/config/joblet-config.yml"

[Install]
WantedBy=multi-user.target
EOF

# Reload and restart
sudo systemctl daemon-reload
sudo systemctl enable joblet
sudo systemctl start joblet
```

### Cgroups Issues

```bash
# Error: "cgroup not available" or "operation not supported"

# Check cgroups version
mount | grep cgroup

# For cgroups v2 (required)
# Check if enabled
cat /proc/cgroups | grep memory

# Enable cgroups v2 (requires reboot)
sudo grubby --update-kernel=ALL --args="systemd.unified_cgroup_hierarchy=1"
sudo reboot

# For older systems, enable cgroups v1 compatibility
echo 'GRUB_CMDLINE_LINUX="cgroup_enable=memory cgroup_memory=1"' | sudo tee -a /etc/default/grub
sudo update-grub
sudo reboot
```

## Connection Problems

### Cannot Connect to Server

```bash
# Error: "connection refused" or "connection timeout"

# 1. Check if server is running
sudo systemctl status joblet

# 2. Check if port is listening
sudo ss -tlnp | grep 50051
# Should show: LISTEN 0 4096 *:50051 *:*

# 3. Check firewall
sudo ufw status
sudo ufw allow 50051/tcp

# 4. Test local connection
telnet localhost 50051

# 5. Check server logs
sudo journalctl -u joblet -n 50
```

### DNS/Hostname Issues

```bash
# Error: "no such host" or "hostname resolution failed"

# 1. Test hostname resolution
ping joblet-server

# 2. Use IP address instead
rnx --config=<(sed 's/joblet-server/192.168.1.100/' ~/.rnx/rnx-config.yml) list

# 3. Add to /etc/hosts
echo "192.168.1.100 joblet-server" | sudo tee -a /etc/hosts

# 4. Check DNS configuration
nslookup joblet-server
```

### Network Connectivity

```bash
# Test network path to server
traceroute joblet-server 50051

# Check for proxy issues
unset http_proxy https_proxy HTTP_PROXY HTTPS_PROXY

# Test with curl (if server has debug endpoint)
curl -k https://joblet-server:50051

# Check for MTU issues
ping -M do -s 1472 joblet-server
```

## Job Execution Issues

### Job Fails Immediately

```bash
# Error: Job exits with code 127 (command not found)

# Check command availability
rnx run which python3
rnx run echo $PATH

# Install required packages
rnx run apt update && apt install -y python3

# Use full path
rnx run /usr/bin/python3 script.py

# Check uploaded files
rnx run ls -la /work
```

### Command Not Found

```bash
# Error: "bash: command not found"

# Check available commands
rnx run ls -la /usr/bin/
rnx run echo $PATH

# Use alternative commands
rnx run sh -c "echo test"  # Instead of bash
rnx run python3 -c "print('test')"  # Instead of python

# Install packages if allowed
rnx run apt update && apt install -y curl
```

### File Upload Issues

```bash
# Error: "no such file or directory" for uploaded files

# 1. Verify file exists locally
ls -la script.py

# 2. Check upload was specified
rnx run --upload=script.py ls -la /work

# 3. Use correct filename
rnx run --upload=script.py python3 script.py  # Not ./script.py

# 4. Check file permissions
rnx run --upload=script.py ls -la script.py
```

### Environment Issues

```bash
# Error: "environment variable not set"

# 1. Set environment variable
rnx run --env=DATABASE_URL=postgres://localhost/db python app.py

# 2. Check environment
rnx run env | grep DATABASE

# 3. Use defaults in script
rnx run python -c "import os; print(os.getenv('VAR', 'default'))"
```

### Working Directory Issues

```bash
# Error: "no such file or directory" for relative paths

# 1. Check current directory
rnx run pwd

# 2. Set working directory
rnx run --workdir=/work/app npm start

# 3. Use absolute paths
rnx run python3 /work/script.py

# 4. Check uploaded directory structure
rnx run --upload-dir=./project find /work -type f
```

## Resource and Performance

### Out of Memory (OOM)

```bash
# Error: "Killed" or "Out of memory"

# 1. Check current memory limit
rnx status <job-id> | grep "Max Memory"

# 2. Increase memory limit
rnx run --max-memory=2048 python memory_intensive.py

# 3. Monitor actual usage
rnx run --max-memory=1024 python -c "
import psutil
print(f'Memory usage: {psutil.virtual_memory().percent}%')
"

# 4. Optimize memory usage
rnx run python -c "
import gc
# Process data in chunks
# Use generators instead of lists
gc.collect()
"
```

### CPU Throttling

```bash
# Job running slowly or timing out

# 1. Check CPU limits
rnx status <job-id> | grep "Max CPU"

# 2. Increase CPU allocation
rnx run --max-cpu=200 compute_intensive.py

# 3. Monitor CPU usage
rnx monitor

# 4. Use multiple cores efficiently
rnx run --cpu-cores="0-3" make -j4
```

### I/O Bandwidth Limits

```bash
# Error: "Operation timed out" for file operations

# 1. Check I/O limits
rnx status <job-id> | grep "Max IO"

# 2. Increase I/O bandwidth
rnx run --max-iobps=52428800 rsync -av /large/dataset/

# 3. Use memory volumes for temporary I/O
rnx volume create temp-io --size=2GB --type=memory
rnx run --volume=temp-io process_data.py

# 4. Optimize I/O patterns
rnx run python -c "
# Read in larger chunks
# Use buffered I/O
# Minimize random access
"
```

### Resource Exhaustion

```bash
# Server becomes unresponsive

# 1. Check system resources
rnx monitor status

# 2. List resource-heavy jobs
rnx list --json | jq '.[] | select(.max_memory > 8192)'

# 3. Stop problematic jobs
rnx list --json | jq -r '.[] | select(.status == "RUNNING") | .id' | head -5 | xargs rnx stop

# 4. Adjust default limits
# Edit /opt/joblet/config/joblet-config.yml
sudo systemctl restart joblet
```

## Volume Problems

### Volume Creation Fails

```bash
# Error: "failed to create volume: operation not permitted"

# 1. Check server permissions
sudo ls -la /opt/joblet/volumes

# 2. Ensure sufficient disk space
df -h /opt/joblet/volumes

# 3. Check volume limits
rnx volume list --json | jq 'length'

# 4. Try smaller volume
rnx volume create test-small --size=100MB
```

### Volume Not Found

```bash
# Error: "volume mydata not found"

# 1. List existing volumes
rnx volume list

# 2. Check volume name spelling
rnx volume create my-data --size=1GB  # Note: hyphens vs underscores

# 3. Recreate volume if needed
rnx volume create mydata --size=1GB --type=filesystem
```

### Volume Mount Issues

```bash
# Error: "failed to mount volume"

# 1. Check volume exists
rnx volume list | grep mydata

# 2. Verify mount point
rnx run --volume=mydata ls -la /volumes/

# 3. Check volume permissions
rnx run --volume=mydata ls -la /volumes/mydata

# 4. Fix permissions if needed
rnx run --volume=mydata chmod 755 /volumes/mydata
```

### Volume Space Issues

```bash
# Error: "No space left on device"

# 1. Check volume usage
rnx run --volume=full-vol df -h /volumes/full-vol

# 2. Clean up files
rnx run --volume=full-vol rm -f /volumes/full-vol/*.tmp

# 3. Increase volume size (recreate)
rnx volume remove full-vol
rnx volume create full-vol --size=10GB

# 4. Use cleanup script
rnx run --volume=data bash -c '
find /volumes/data -mtime +7 -type f -delete
'
```

## Network Issues

### Network Creation Fails

```bash
# Error: "CIDR overlaps with existing network"

# 1. List existing networks
rnx network list

# 2. Use different CIDR
rnx network create mynet --cidr=10.99.0.0/24

# 3. Check system network interfaces
ip addr show
```

### No Internet Access

```bash
# Job cannot reach external sites

# 1. Test from bridge network
rnx run --network=bridge ping 8.8.8.8

# 2. Check DNS resolution
rnx run --network=bridge nslookup google.com

# 3. Use host network for debugging
rnx run --network=host curl https://google.com

# 4. Check custom network config
# Custom networks may not have NAT enabled
```

### Inter-Job Communication Fails

```bash
# Jobs in same network cannot communicate

# 1. Verify same network
rnx run --network=mynet ip addr show

# 2. Test connectivity
rnx run --network=mynet --name=server nc -l 8080 &
rnx run --network=mynet nc server 8080

# 3. Check firewall/iptables
rnx run --network=mynet iptables -L

# 4. Use IP addresses instead of hostnames
SERVER_IP=$(rnx run --network=mynet hostname -I | awk '{print $1}')
rnx run --network=mynet nc $SERVER_IP 8080
```

## Certificate and Security

### Certificate Verification Failed

```bash
# Error: "certificate verify failed" or "x509: certificate signed by unknown authority"

# 1. Check certificate files
ls -la ~/.rnx/rnx-config.yml

# 2. Regenerate certificates
sudo /usr/local/bin/certs_gen_embedded.sh

# 3. Copy new client config
scp server:/opt/joblet/config/rnx-config.yml ~/.rnx/

# 4. Verify certificate chain
openssl verify -CAfile ca-cert.pem client-cert.pem
```

### Permission Denied (RBAC)

```bash
# Error: "permission denied" for admin operations

# 1. Check client role
openssl x509 -in client-cert.pem -noout -subject

# 2. Should show OU=admin for admin operations
# Generate admin certificate if needed
openssl req -new -key client-key.pem -out admin.csr \
  -subj "/CN=admin-client/OU=admin"

# 3. Use viewer certificate for read-only
openssl req -new -key client-key.pem -out viewer.csr \
  -subj "/CN=viewer-client/OU=viewer"
```

### TLS Handshake Failure

```bash
# Error: "TLS handshake failed"

# 1. Check TLS version compatibility
openssl s_client -connect joblet-server:50051 -tls1_3

# 2. Check cipher compatibility
openssl ciphers -v 'ECDHE+AESGCM:ECDHE+CHACHA20'

# 3. Disable TLS temporarily for debugging (NOT for production)
# Edit server config to allow lower TLS versions
```

### Certificate Expiration

```bash
# Check certificate expiration
openssl x509 -in server-cert.pem -noout -dates

# Renew certificates before expiration
sudo /usr/local/bin/certs_gen_embedded.sh

# Update all clients with new certificates
for client in client1 client2 client3; do
  scp /opt/joblet/config/rnx-config.yml $client:~/.rnx/
done
```

## Log Analysis

### Server Logs

```bash
# View real-time logs
sudo journalctl -u joblet -f

# Filter by severity
sudo journalctl -u joblet -p err

# Search for specific errors
sudo journalctl -u joblet | grep "certificate"
sudo journalctl -u joblet | grep "permission denied"

# Export logs for analysis
sudo journalctl -u joblet --since="1 hour ago" > joblet.log
```

### Job Logs

```bash
# View job logs with timestamps
rnx log --timestamps <job-id>

# Search job logs
rnx log <job-id> | grep ERROR

# Save logs for analysis
rnx log <job-id> > job-output.log

# Monitor running job
rnx log -f <job-id>
```

### Audit Logs

```bash
# View security audit logs
sudo tail -f /var/log/joblet/audit.log

# Filter authentication events
sudo grep "auth" /var/log/joblet/audit.log

# Find failed operations
sudo jq 'select(.status == "failed")' /var/log/joblet/audit.log
```

### Debug Logging

```bash
# Enable debug logging temporarily
# Edit /opt/joblet/config/joblet-config.yml
logging:
  level: "debug"

# Restart server
sudo systemctl restart joblet

# Check debug logs
sudo journalctl -u joblet | tail -100
```

## Getting Help

### Information to Collect

When reporting issues, include:

```bash
# 1. Version information
joblet --version
rnx --version

# 2. System information
uname -a
lsb_release -a

# 3. Configuration (sanitized - remove certificates)
cat /opt/joblet/config/joblet-config.yml | grep -v "BEGIN CERTIFICATE"

# 4. Recent logs
sudo journalctl -u joblet --since="1 hour ago"

# 5. Resource usage
rnx monitor status

# 6. Network configuration
ip addr show
iptables -L

# 7. Job details (if job-related)
rnx status <job-id>
rnx log <job-id>
```

### Debugging Steps

1. **Reproduce the issue** with minimal example
2. **Check logs** at all levels (system, server, job)
3. **Test connectivity** between components
4. **Verify configuration** syntax and values
5. **Test with known-good configuration**
6. **Isolate variables** (network, certificates, resources)

### Support Channels

- **GitHub Issues**: https://github.com/ehsaniara/joblet/issues
- **Documentation**: Check all relevant docs first
- **Community Forum**: For general questions
- **Security Issues**: Report privately to maintainers

### Creating Bug Reports

```markdown
## Issue Description

Brief description of the problem

## Expected Behavior

What should happen

## Actual Behavior

What actually happens

## Steps to Reproduce

1. Run `rnx run echo test`
2. Observe error
3. Check logs

## Environment

- Joblet version: 1.x.x
- OS: Ubuntu 22.04
- Architecture: x86_64

## Logs
```

sudo journalctl -u joblet --since="1 hour ago"

```

## Configuration
```yaml
# Sanitized config (remove certificates)
```

## Additional Context

Any other relevant information

```

## Common Solutions Summary

| Problem | Solution |
|---------|----------|
| Server won't start | Check permissions, cgroups, config syntax |
| Connection refused | Start server, check firewall, verify port |
| Certificate errors | Regenerate certificates, check time sync |
| Permission denied | Check RBAC role, verify certificate OU |
| Job fails immediately | Check command exists, verify uploads |
| Out of memory | Increase memory limit or optimize usage |
| Volume issues | Check disk space, permissions, naming |
| Network problems | Verify CIDR ranges, check isolation |
| Performance issues | Adjust resource limits, monitor usage |