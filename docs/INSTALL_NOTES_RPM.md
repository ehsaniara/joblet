# Joblet RPM Installation Notes (RHEL/CentOS/Fedora)

## Overview

This document provides detailed information about the Joblet RPM package installation for RedHat-based systems (RHEL, CentOS, Fedora, Amazon Linux), including all system modifications, security considerations, and troubleshooting guidance.

## System Requirements

### Minimum Requirements
- **OS**: RHEL 8+, CentOS Stream 8+, Fedora 30+, Amazon Linux 2/2023
- **Kernel**: Linux 4.6+ with cgroups v2 support
- **Architecture**: x86_64 (amd64) or aarch64 (ARM64)
- **Memory**: 512MB RAM (2GB recommended)
- **Storage**: 1GB available disk space
- **Network**: Available 172.20.0.0/16 IP range (for bridge network)

### Required Packages
The installer will automatically use:
- `openssl` (>= 1.1.1) - For TLS certificate generation
- `systemd` - For service management
- `iptables` or `nftables` or `firewalld` - For firewall rules (auto-detected)
- `iproute` - For network configuration
- `bridge-utils` - For bridge network creation

## System Modifications

### ⚠️ PERMANENT CHANGES

The RPM installation makes the following **permanent** changes to your system:

#### 1. Network Configuration

**IP Forwarding (Permanent)**
```bash
# Created in /etc/sysctl.d/99-joblet.conf:
net.ipv4.ip_forward = 1
```
This enables packet forwarding between network interfaces, required for job networking.

**Kernel Modules (Auto-load on boot)**
```bash
# Created in /etc/modules-load.d/joblet.conf:
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

**For firewalld (default on RHEL/CentOS/Fedora):**
```bash
# Enable masquerading for NAT
firewall-cmd --permanent --add-masquerade

# Add direct rules for joblet traffic
firewall-cmd --permanent --direct --add-rule ipv4 nat POSTROUTING 0 -s 172.20.0.0/16 -j MASQUERADE
firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -i joblet0 -j ACCEPT
firewall-cmd --permanent --direct --add-rule ipv4 filter FORWARD 0 -o joblet0 -j ACCEPT

# Reload to apply
firewall-cmd --reload
```

**For nftables (modern systems):**
```bash
# Creates dedicated joblet table
nft add table inet joblet
nft add chain inet joblet postrouting { type nat hook postrouting priority 100 \; }
nft add rule inet joblet postrouting ip saddr 172.20.0.0/16 masquerade

nft add chain inet joblet forward { type filter hook forward priority 0 \; }
nft add rule inet joblet forward iifname "joblet0" accept
nft add rule inet joblet forward oifname "joblet0" accept
```

**For iptables (older systems):**
```bash
# NAT rule for job networking
iptables -t nat -A POSTROUTING -s 172.20.0.0/16 -j MASQUERADE

# FORWARD rules (if FORWARD policy is DROP)
iptables -I FORWARD -i joblet0 -j ACCEPT
iptables -I FORWARD -o joblet0 -j ACCEPT
```

Rules are automatically persisted based on the firewall system in use.

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
    └── common-install-functions.sh  # Shared installation functions

/var/log/joblet/            # System logs (journald)
/etc/joblet/
└── rnx-config.yml          # Convenience copy (chmod 644, for local use)

/usr/local/bin/rnx          # Symlink to /opt/joblet/bin/rnx
/usr/local/bin/certs_gen_embedded.sh  # Certificate generation script

/etc/systemd/system/
└── joblet.service          # Systemd service file

/etc/sysctl.d/
└── 99-joblet.conf          # Sysctl configuration

/etc/modules-load.d/
└── joblet.conf             # Kernel module auto-loading
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

### SELinux Compatibility

Joblet is designed to work with SELinux in enforcing mode:
- All binaries are placed in standard locations
- Service runs with standard systemd context
- Network operations use standard Linux kernel interfaces

If you encounter SELinux issues:
```bash
# Check for SELinux denials
sudo ausearch -m avc -ts recent | grep joblet

# If needed, create custom policy (advanced)
# See RHEL SELinux documentation
```

### Firewall Security

**Default behavior**:
- Joblet opens port 50051 for gRPC (configurable)
- Bridge network allows outbound connections from jobs
- Jobs can access the internet if host has internet access

**Recommendations**:
```bash
# Restrict joblet access to specific zones (firewalld)
sudo firewall-cmd --permanent --zone=internal --add-port=50051/tcp
sudo firewall-cmd --permanent --zone=public --remove-port=50051/tcp
sudo firewall-cmd --reload

# Or restrict to specific networks
sudo firewall-cmd --permanent --zone=internal --add-source=192.168.0.0/16
sudo firewall-cmd --permanent --zone=internal --add-port=50051/tcp
sudo firewall-cmd --reload
```

## Configuration

### Configuration Precedence

The RPM installer follows this precedence order (highest to lowest):

1. **Environment Variables** (for automation)
   ```bash
   export JOBLET_SERVER_ADDRESS="0.0.0.0"
   export JOBLET_SERVER_PORT="50051"
   export JOBLET_CERT_INTERNAL_IP="192.168.1.100"
   export JOBLET_CERT_PUBLIC_IP="203.0.113.10"
   export JOBLET_CERT_DOMAIN="joblet.example.com"
   sudo yum localinstall joblet-*.rpm
   ```

2. **Auto-detection** (fallback)
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
# RHEL 8+, CentOS Stream, Fedora 22+, Amazon Linux 2023
sudo dnf remove joblet

# RHEL 7, CentOS 7, Amazon Linux 2
sudo yum remove joblet
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

### Complete Removal

The RPM %postun script handles cleanup automatically on uninstall, but you may want to verify:

```bash
# Remove the package
sudo dnf remove joblet

# Verify cleanup (should be done automatically)
# Check bridge is removed
ip link show joblet0  # Should not exist

# Check firewall rules are removed
firewall-cmd --list-all | grep joblet  # Should be empty

# Check sysctl config is removed
cat /etc/sysctl.d/99-joblet.conf  # Should not exist

# Manual cleanup if needed
sudo rm -f /etc/sysctl.d/99-joblet.conf
sudo rm -f /etc/modules-load.d/joblet.conf
sudo rm -rf /opt/joblet
sudo rm -rf /var/log/joblet
sudo rm -rf /etc/joblet
```

## Troubleshooting

### Installation Issues

#### Firewall Backend Not Detected

**Symptom**: "No firewall backend detected"

**Solution**:
```bash
# Check which firewall is active
systemctl status firewalld
systemctl status nftables
systemctl status iptables

# Enable firewalld (recommended for RHEL/CentOS/Fedora)
sudo systemctl enable firewalld
sudo systemctl start firewalld

# Then reinstall
sudo dnf reinstall joblet
```

#### SELinux Denials

**Symptom**: Service fails to start with AVC denials

**Solution**:
```bash
# Check for denials
sudo ausearch -m avc -ts recent | grep joblet

# Temporarily set to permissive to test
sudo setenforce 0
sudo systemctl start joblet

# If it works, re-enable and check logs
sudo setenforce 1

# Generate custom policy if needed (advanced)
sudo audit2allow -a -M joblet_custom
sudo semodule -i joblet_custom.pp
```

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

# 4. SELinux (check above)
```

#### Jobs Can't Access Network

```bash
# Check bridge
ip link show joblet0
ip addr show joblet0

# Check firewall rules (firewalld)
sudo firewall-cmd --list-all
sudo firewall-cmd --direct --get-all-rules

# Check firewall rules (nftables)
sudo nft list table inet joblet

# Check firewall rules (iptables)
sudo iptables -t nat -L -n -v | grep 172.20
sudo iptables -L FORWARD -n -v | grep joblet

# Check IP forwarding
cat /proc/sys/net/ipv4/ip_forward  # Should be 1
```

#### Connection Refused from Clients

```bash
# Check server is listening
sudo ss -tlnp | grep 50051

# Check firewall allows connections
sudo firewall-cmd --list-ports
sudo firewall-cmd --zone=public --query-port=50051/tcp

# Add port if needed
sudo firewall-cmd --permanent --add-port=50051/tcp
sudo firewall-cmd --reload

# Check certificate matches
openssl x509 -in /opt/joblet/config/joblet-config.yml -text -noout
# Look for Subject Alternative Name with your IP

# Test with rnx
sudo rnx --config /opt/joblet/config/rnx-config.yml job list
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
sudo firewall-cmd --list-all  # or
sudo nft list tables  # or
sudo iptables -t nat -L -n

# 5. Check certificates
ls -la /opt/joblet/config/

# 6. Check modules
lsmod | grep br_netfilter
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

## Distribution-Specific Notes

### RHEL 8 / CentOS Stream 8
- Uses firewalld by default
- cgroups v2 by default
- Requires subscriptions for RHEL

### RHEL 7 / CentOS 7
- Uses iptables-services
- cgroups v1 (v2 requires kernel upgrade)
- May need additional configuration

### Fedora 30+
- Uses firewalld or nftables
- Latest features supported
- Bleeding edge kernel

### Amazon Linux 2
- Uses iptables-services
- Similar to RHEL 7
- AWS-optimized kernel

### Amazon Linux 2023
- Uses firewalld
- Similar to RHEL 8
- AWS-optimized kernel

## Security Best Practices

1. **Restrict Network Access**
   - Use firewall zones to limit access
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
   sudo dnf upgrade joblet
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
