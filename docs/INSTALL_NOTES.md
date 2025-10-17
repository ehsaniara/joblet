# Joblet Installation Notes

## Overview

This document provides detailed information about what the Joblet Debian package installer does, including all system modifications, security considerations, and troubleshooting guidance.

## System Requirements

### Minimum Requirements
- **OS**: Debian 11+ or Ubuntu 20.04+
- **Kernel**: Linux 4.6+ with cgroups v2 support
- **Architecture**: x86_64 (amd64) or ARM64
- **Memory**: 512MB RAM (2GB recommended)
- **Storage**: 1GB available disk space
- **Network**: Available 172.20.0.0/16 IP range (for bridge network)

### Required Packages
The installer will automatically verify and use:
- `openssl` (>= 1.1.1) - For TLS certificate generation
- `systemd` - For service management
- `iptables` or `nftables` - For firewall rules (auto-detected)
- `iproute2` - For network configuration
- `bridge-utils` - For bridge network creation

## System Modifications

### ⚠️ PERMANENT CHANGES

The installation makes the following **permanent** changes to your system:

#### 1. Network Configuration

**IP Forwarding (Permanent)**
```bash
# Added to /etc/sysctl.conf:
net.ipv4.ip_forward = 1
```
This enables packet forwarding between network interfaces, required for job networking.

**Kernel Modules (Auto-load on boot)**
```bash
# Added to /etc/modules:
br_netfilter
nf_conntrack
nf_nat
```

**Bridge Network**
```bash
# Created bridge interface:
Interface: joblet0
IP Range: 172.20.0.0/16
Gateway: 172.20.0.1
```

⚠️ **CONFLICT DETECTION**: The installer checks if 172.20.0.0/16 is already in use. If a conflict is detected, a warning is displayed but installation continues. You may need to manually reconfigure the bridge network.

**Firewall Rules**

The installer auto-detects your firewall backend:

**For iptables:**
```bash
# NAT rule for job networking
iptables -t nat -A POSTROUTING -s 172.20.0.0/16 -j MASQUERADE

# FORWARD rules (if FORWARD policy is DROP)
iptables -I FORWARD -i joblet0 -j ACCEPT
iptables -I FORWARD -o joblet0 -j ACCEPT
iptables -I FORWARD -i viso+ -j ACCEPT
iptables -I FORWARD -o viso+ -j ACCEPT
```

**For nftables:**
```bash
# Creates dedicated joblet table
nft add table inet joblet
nft add chain inet joblet postrouting { type nat hook postrouting priority 100 \; }
nft add rule inet joblet postrouting ip saddr 172.20.0.0/16 masquerade

nft add chain inet joblet forward { type filter hook forward priority 0 \; }
nft add rule inet joblet forward iifname "joblet0" accept
nft add rule inet joblet forward oifname "joblet0" accept
```

Rules are automatically persisted if `iptables-persistent` or `/etc/nftables.conf` is available.

#### 2. File System Structure

```
/opt/joblet/
├── bin/
│   ├── joblet              # Main server binary
│   ├── rnx                 # Client CLI
│   └── joblet-persist      # Persistence service (runs as subprocess)
├── config/
│   ├── joblet-config.yml   # Server config (chmod 600, contains private keys)
│   └── rnx-config.yml      # Client config (chmod 600, contains private keys)
├── logs/                   # Job logs and output
├── volumes/                # Persistent job volumes
├── network/                # Network state
├── jobs/                   # Job state
└── scripts/                # Helper scripts

/var/log/joblet/            # System logs (journald)
/etc/joblet/
└── rnx-config.yml          # Convenience copy (chmod 644, for local use)

/usr/bin/rnx                # Symlink to /opt/joblet/bin/rnx
/usr/local/bin/rnx          # Symlink to /opt/joblet/bin/rnx

/etc/systemd/system/
└── joblet.service          # Systemd service file
```

#### 3. Systemd Service

**Service Configuration:**
```ini
[Unit]
Description=Joblet Service - Process Isolation Platform
After=network.target

[Service]
User=root              # Required for namespace/cgroup operations
Group=root
ExecStart=/opt/joblet/bin/joblet
Restart=always
Delegate=yes           # Required for cgroup delegation

[Install]
WantedBy=multi-user.target
```

**Enabled by default**: The service is enabled but NOT automatically started. You must manually start it:
```bash
sudo systemctl start joblet
```

## Security Considerations

### Running as Root

⚠️ **Joblet runs as root** - This is **required** for:
- Creating and managing Linux namespaces (pid, net, mount, ipc, uts, cgroup)
- Configuring cgroups v2 for resource limits
- Creating veth pairs and managing bridge networking
- Setting up network isolation per job

**Mitigation**:
- Jobs run in isolated namespaces with restricted capabilities
- Resource limits enforced via cgroups
- Network isolation via bridge networking
- TLS mutual authentication for all client connections

### Certificate Security

**Private keys are embedded** in configuration files:
- `/opt/joblet/config/joblet-config.yml` (chmod 600)
- `/opt/joblet/config/rnx-config.yml` (chmod 600)

⚠️ **IMPORTANT**:
- These files contain TLS private keys - protect them accordingly
- Do NOT commit these files to version control
- Do NOT share them publicly
- Back them up securely if needed

**Certificate Generation:**
Certificates are auto-generated during installation with:
- 4096-bit RSA keys (CA)
- 2048-bit RSA keys (server/client)
- 3-year CA validity, 1-year certificate validity
- Subject Alternative Names (SAN) for all configured IPs/domains

### Firewall Security

**Default behavior**:
- Joblet opens port 50051 for gRPC (configurable)
- Bridge network allows outbound connections from jobs
- Jobs can access the internet if host has internet access

**Recommendations**:
```bash
# Restrict joblet access to specific networks
sudo ufw allow from 192.168.0.0/16 to any port 50051

# Or use iptables
sudo iptables -I INPUT -p tcp --dport 50051 -s 192.168.0.0/16 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 50051 -j DROP
```

## Configuration

### Configuration Precedence

The installer follows this precedence order (highest to lowest):

1. **Environment Variables** (for automation)
   ```bash
   export JOBLET_SERVER_ADDRESS="0.0.0.0"
   export JOBLET_SERVER_PORT="50051"
   export JOBLET_CERT_INTERNAL_IP="192.168.1.100"
   export JOBLET_CERT_PUBLIC_IP="203.0.113.10"
   export JOBLET_CERT_DOMAIN="joblet.example.com"
   sudo dpkg -i joblet.deb
   ```

2. **Debconf** (for interactive installations)
   ```bash
   sudo dpkg-reconfigure joblet
   ```

3. **Auto-detection** (fallback)
   - Detects primary network interface IP
   - Uses 0.0.0.0 for server bind address
   - Uses 50051 for default port

### AWS EC2 Automatic Configuration

On AWS EC2 instances, the installer can automatically:
- Detect EC2 environment via metadata service
- Configure CloudWatch Logs backend
- Set up certificates with instance public/private IPs
- Enable IAM role-based authentication

**Requirements**:
- Create `/tmp/joblet-ec2-info` file before installation:
  ```bash
  cat > /tmp/joblet-ec2-info << EOF
  IS_EC2=true
  EC2_INSTANCE_ID="i-1234567890abcdef0"
  EC2_REGION="us-east-1"
  EC2_INTERNAL_IP="10.0.1.100"
  EC2_PUBLIC_IP="203.0.113.10"
  EOF
  ```

- Attach IAM role with CloudWatch Logs permissions

## Uninstallation

### Remove Package (Keep Data)

```bash
sudo apt remove joblet
```

**What is removed**:
- Binaries and symlinks
- Systemd service
- Cgroup directories

**What is preserved**:
- Job logs in `/opt/joblet/logs/`
- Volumes in `/opt/joblet/volumes/`
- Configuration in `/opt/joblet/config/`
- Network configuration (IP forwarding, firewall rules, bridge)

### Complete Removal (Purge)

```bash
sudo apt purge joblet
```

**What is removed**:
- Everything from `remove` above
- All data in `/opt/joblet/`
- All data in `/var/log/joblet/`
- Configuration in `/etc/joblet/`

⚠️ **Manual cleanup still required**:
```bash
# Remove IP forwarding (if no longer needed)
sudo sed -i '/net.ipv4.ip_forward = 1/d' /etc/sysctl.conf
sudo sysctl -w net.ipv4.ip_forward=0

# Remove kernel modules from auto-load
sudo sed -i '/^br_netfilter$/d' /etc/modules
sudo sed -i '/^nf_conntrack$/d' /etc/modules
sudo sed -i '/^nf_nat$/d' /etc/modules

# Remove bridge network
sudo ip link delete joblet0

# Remove firewall rules (iptables)
sudo iptables -t nat -D POSTROUTING -s 172.20.0.0/16 -j MASQUERADE
sudo iptables -D FORWARD -i joblet0 -j ACCEPT
sudo iptables -D FORWARD -o joblet0 -j ACCEPT
# Save if using iptables-persistent
sudo netfilter-persistent save

# Remove firewall rules (nftables)
sudo nft delete table inet joblet
# Remove from /etc/nftables.conf if persisted
```

## Troubleshooting

### Installation Issues

#### Network Conflict Detected

**Symptom**: Warning about 172.20.0.0/16 already in use

**Solution**:
```bash
# Check what's using the range
ip route | grep 172.20

# Option 1: Remove conflicting network
# (depends on what's using it)

# Option 2: Reconfigure joblet after install
# Edit /opt/joblet/config/joblet-config.yml
# Change bridge network settings
```

#### Firewall Backend Not Detected

**Symptom**: "No firewall backend detected"

**Solution**:
```bash
# Install iptables
sudo apt install iptables

# OR install nftables
sudo apt install nftables

# Then reconfigure
sudo dpkg-reconfigure joblet
```

#### Certificate Generation Failed

**Symptom**: "Certificate generation failed"

**Solution**:
```bash
# Check OpenSSL is installed
openssl version

# Manually generate certificates
export JOBLET_SERVER_ADDRESS="192.168.1.100"
sudo /usr/local/bin/certs_gen_embedded.sh

# Restart service
sudo systemctl restart joblet
```

### Runtime Issues

#### Service Won't Start

```bash
# Check status
sudo systemctl status joblet -l

# Check logs
sudo journalctl -u joblet -n 50 --no-pager

# Common issues:
# 1. Port already in use
sudo ss -tlnp | grep 50051

# 2. Certificate issues
ls -la /opt/joblet/config/

# 3. Cgroup delegation
cat /sys/fs/cgroup/joblet.slice/cgroup.controllers
```

#### Jobs Can't Access Network

```bash
# Check bridge
ip link show joblet0
ip addr show joblet0

# Check firewall rules (iptables)
sudo iptables -t nat -L -n -v | grep 172.20
sudo iptables -L FORWARD -n -v | grep joblet

# Check firewall rules (nftables)
sudo nft list table inet joblet

# Check IP forwarding
cat /proc/sys/net/ipv4/ip_forward  # Should be 1
```

#### Connection Refused from Clients

```bash
# Check server is listening
sudo ss -tlnp | grep 50051

# Check firewall allows connections
sudo ufw status  # or
sudo iptables -L INPUT -n -v | grep 50051

# Check certificate matches
openssl x509 -in /opt/joblet/config/joblet-config.yml -text -noout
# Look for Subject Alternative Name with your IP

# Test with rnx
rnx --config /opt/joblet/config/rnx-config.yml job list
```

## Post-Installation Verification

### Verify Installation

```bash
# 1. Check binaries
which rnx
rnx --version

# 2. Check service
sudo systemctl status joblet

# 3. Check network
ip link show joblet0
ip route | grep 172.20

# 4. Check firewall
sudo iptables -t nat -L -n | grep 172.20  # or
sudo nft list table inet joblet

# 5. Check certificates
ls -la /opt/joblet/config/
```

### First Job Test

```bash
# Start service
sudo systemctl start joblet

# Run test job
sudo rnx job run echo "Hello from Joblet"

# Check logs
sudo rnx job list
sudo rnx job log <job-id>
```

## Advanced Configuration

### Custom Network Range

Edit `/opt/joblet/config/joblet-config.yml`:
```yaml
network:
  bridgeName: "joblet0"
  bridgeSubnet: "10.100.0.0/16"  # Change from 172.20.0.0/16
  bridgeIP: "10.100.0.1"
```

Update firewall rules accordingly.

### Multiple Joblet Instances

Not recommended, but possible:
- Use different ports
- Use different bridge networks
- Use different config directories
- Create separate systemd services

### Certificate Rotation

```bash
# Backup current certificates
sudo cp -r /opt/joblet/config /opt/joblet/config.backup

# Regenerate
export JOBLET_SERVER_ADDRESS="your.server.ip"
sudo /usr/local/bin/certs_gen_embedded.sh

# Restart service
sudo systemctl restart joblet

# Distribute new client config
# Copy /opt/joblet/config/rnx-config.yml to clients
```

## Security Best Practices

1. **Restrict Network Access**
   - Use firewall rules to limit access to trusted networks
   - Don't expose port 50051 to the internet without additional protection

2. **Protect Configuration Files**
   - Keep `/opt/joblet/config/` permissions at 600
   - Don't commit to version control
   - Use encrypted backups

3. **Monitor Logs**
   ```bash
   sudo journalctl -u joblet -f
   ```

4. **Regular Updates**
   ```bash
   sudo apt update
   sudo apt upgrade joblet
   ```

5. **Audit Job Execution**
   - Review job logs regularly
   - Monitor resource usage
   - Check for unusual network activity

## Support

For issues, documentation, and updates:
- GitHub: https://github.com/ehsaniara/joblet
- Documentation: /opt/joblet/docs/
- Logs: `sudo journalctl -u joblet -f`
