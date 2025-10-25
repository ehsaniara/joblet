# Joblet Port Configuration for AWS

## Summary

All installation documentation has been updated to use **port 443** for Joblet on AWS EC2, following the official Joblet AWS deployment recommendations.

---

## Port Configuration

### On EC2 (Server Side)
**Joblet listens on port 443**

```yaml
# /opt/joblet/joblet-config.yml
server:
  host: "0.0.0.0"
  port: 443  # Standard HTTPS port - firewall-friendly
```

### On MacBook (Client Side)
**SSH tunnel maps to port 8443 locally**

```bash
# SSH tunnel configuration
LOCAL_PORT="8443"   # Local (your MacBook) - no sudo needed
REMOTE_PORT="443"   # Remote (EC2) - where Joblet listens

ssh -L 8443:localhost:443 ubuntu@ec2-host
```

### Client Configuration
**Connect to localhost:8443 via tunnel**

```yaml
# ~/.rnx/rnx-config.yml
default:
  server: "localhost:8443"  # Via SSH tunnel
  # OR
  # server: "ec2-host:443"  # Direct connection (if in same VPC)
```

---

## Why These Ports?

### Why Port 443 on EC2?

✅ **Firewall-Friendly** - Port 443 (HTTPS) is allowed by most corporate firewalls
✅ **Standard Practice** - No special firewall rules needed
✅ **Proxy Compatible** - Works through HTTP proxies and NAT
✅ **AWS Recommendation** - Official Joblet AWS deployment guide uses 443

From the official Joblet AWS documentation:
> "Port 443 (HTTPS) instead of default 50051 - better firewall compatibility"

### Why Port 8443 Locally?

✅ **No Sudo Required** - Ports below 1024 require root on macOS/Linux
✅ **Easy to Use** - Just run the tunnel script, no special permissions
✅ **Still Secure** - SSH tunnel provides encryption
✅ **Standard Alternative** - 8443 is a common alternative HTTPS port

If you want to use 443 locally:
```bash
sudo ~/bin/joblet-tunnel.sh  # Requires sudo
```

---

## Security Group Configuration

Update your EC2 security group:

```
Type: Custom TCP
Protocol: TCP
Port: 443
Source: Your IP/32
Description: Joblet gRPC API (HTTPS port for firewall compatibility)
```

---

## Updated Files

All tutorial documentation has been updated:

### Installation Guides
- ✅ `docs/installation/EC2_INSTALLATION.md`
- ✅ `docs/installation/EC2_INSTALLATION_MEDIUM.md`
- ✅ `docs/installation/README.md`

### ML Demo Tutorials
- ✅ `examples/ml-demo/README.md`
- ✅ `examples/ml-demo/QUICKSTART.md`
- ✅ `examples/ml-demo/MEDIUM_ARTICLE.md`
- ✅ `examples/ml-demo/INDEX.md`

---

## Migration from Port 50051

If you have existing documentation or scripts using port 50051:

### Update Server Config
```yaml
# OLD
server:
  port: 50051

# NEW
server:
  port: 443
```

### Update Security Group
```
# OLD: Port 50051
# NEW: Port 443
```

### Update SSH Tunnel
```bash
# OLD
ssh -L 50051:localhost:50051 ubuntu@ec2-host

# NEW
ssh -L 8443:localhost:443 ubuntu@ec2-host
```

### Update Client Config
```yaml
# OLD
server: "localhost:50051"

# NEW
server: "localhost:8443"  # Via tunnel
# OR
server: "ec2-host:443"    # Direct
```

---

## Testing Your Configuration

### 1. Verify Server Port

SSH to EC2:
```bash
sudo netstat -tlnp | grep 443
```

Should show:
```
tcp  0  0  0.0.0.0:443  0.0.0.0:*  LISTEN  12345/joblet
```

### 2. Test SSH Tunnel

On MacBook:
```bash
# Start tunnel
~/bin/joblet-tunnel.sh

# In another terminal, test connection
nc -zv localhost 8443
```

Should output:
```
Connection to localhost port 8443 [tcp/*] succeeded!
```

### 3. Test Joblet Connection

```bash
rnx version
```

Should show:
```
rnx version: v1.0.0
Server version: v1.0.0
```

---

## Troubleshooting

### Error: "bind: Permission denied"

**Problem:** Trying to use port 443 locally without sudo

**Solution:** Use port 8443 locally instead:
```bash
LOCAL_PORT="8443"  # In your tunnel script
```

### Error: "Connection refused"

**Problem:** Tunnel not running or wrong port

**Solution:**
1. Verify tunnel is running: `ps aux | grep ssh`
2. Check tunnel is using correct ports
3. Verify Joblet is listening on EC2: `sudo netstat -tlnp | grep 443`

### Error: "certificate verify failed"

**Problem:** TLS certificate mismatch

**Solution:**
- Ensure you copied `ca-cert.pem` from EC2
- Check it's in `~/.rnx/certs/ca-cert.pem`
- Verify path in `~/.rnx/rnx-config.yml`

---

## Quick Reference

| Location | Port | Why |
|----------|------|-----|
| **EC2 Server** | 443 | Standard HTTPS, firewall-friendly |
| **SSH Tunnel (Local)** | 8443 | No sudo needed on MacBook |
| **SSH Tunnel (Remote)** | 443 | Connects to Joblet on EC2 |
| **Client Config** | 8443 or 443 | Depends on connection method |

### Connection Methods

**Via SSH Tunnel (Recommended for security):**
```
MacBook:8443 → SSH Tunnel → EC2:443
```

**Direct Connection (Same VPC/VPN):**
```
MacBook → EC2:443
```

---

## Summary

✅ Joblet runs on **port 443** on EC2 (firewall-friendly HTTPS port)
✅ SSH tunnel uses **port 8443** locally (no sudo needed)
✅ Client connects to **localhost:8443** via tunnel
✅ All documentation updated to reflect AWS best practices

This configuration provides the best balance of security, ease of use, and firewall compatibility.
