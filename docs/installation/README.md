# Joblet EC2 Installation Guides

Two guides for installing Joblet on AWS EC2 - choose the one that fits your needs.

## 📄 Available Guides

### 1. Technical Reference Guide

**File:** `EC2_INSTALLATION.md`

**Best for:**

- System administrators
- DevOps engineers
- Production deployments
- Reference documentation

**Style:** Step-by-step technical documentation with commands, configurations, and troubleshooting.

**What's Covered:**

- Complete EC2 setup with specifications
- Security group configuration
- IAM roles and permissions
- TLS certificate generation
- Systemd service setup
- CloudWatch integration
- SSH tunnel configuration
- Production checklist
- Cost optimization
- Comprehensive troubleshooting

---

### 2. Medium Article (Conversational)

**File:** `EC2_INSTALLATION_MEDIUM.md`

**Best for:**

- Blog posts / Medium articles
- Tutorial writers
- Beginners learning the process
- Sharing with non-technical stakeholders

**Style:** Personal, conversational tone with explanations of "why" behind each step.

**What's Covered:**

- Same technical content as reference guide
- Written as a personal journey
- More context and explanations
- Real-world experiences and mistakes
- Beginner-friendly language
- Ready to publish to Medium/blog

---

## Quick Comparison

| Aspect       | Technical Guide                 | Medium Article                           |
|--------------|---------------------------------|------------------------------------------|
| **Tone**     | Formal, precise                 | Conversational, personal                 |
| **Audience** | Engineers, admins               | General developers, learners             |
| **Format**   | Reference manual                | Tutorial narrative                       |
| **Use Case** | Production setup, documentation | Learning, blog content                   |
| **Example**  | "Configure the security group"  | "Here's what I did with security groups" |

---

## What Gets Installed

Both guides result in the same setup:

```
┌─────────────┐         SSH Tunnel          ┌─────────────────┐
│   MacBook   │◄────────(localhost:443)───┤   EC2 Instance  │
│   (Local)   │                              │  (Joblet Server)│
│             │                              │                 │
│  rnx CLI    │                              │  joblet daemon  │
└─────────────┘                              └────────┬────────┘
                                                      │
                                                      ▼
                                              ┌──────────────┐
                                              │  CloudWatch  │
                                              │  Logs/Metrics│
                                              └──────────────┘
```

**Components:**

- ✅ EC2 instance (Ubuntu 22.04)
- ✅ Joblet server with TLS
- ✅ Docker for job isolation
- ✅ CloudWatch logging and metrics
- ✅ SSH tunnel for secure access
- ✅ Joblet CLI on MacBook
- ✅ Systemd service for auto-start

---

## Prerequisites

Both guides assume you have:

- AWS account with EC2 access
- MacBook or Unix terminal
- Basic command line knowledge
- SSH key pair for EC2
- ~30 minutes of time
- ~$60/month budget for EC2

---

## After Installation

Once you complete either guide, you'll have:

1. **Joblet server running on EC2**
   ```bash
   # From your MacBook
   rnx version
   rnx nodes list
   ```

2. **Secure access via SSH tunnel**
   ```bash
   ~/bin/joblet-tunnel.sh
   ```

3. **CloudWatch monitoring**
    - Logs in `/aws/joblet/server`
    - Metrics in `Joblet/Server` namespace

4. **Ready for workloads**
   ```bash
   rnx runtime install python-3.11-ml
   rnx volume create ml-data --size=10GB
   rnx job run echo "Hello from cloud!"
   ```

---

## Next Steps

After installation, continue to:

### ML Demo Tutorial

Run your first machine learning pipeline with Joblet.

- [ML Demo README](../../examples/ml-demo/README.md)
- [ML Demo Quick Start](../../examples/ml-demo/QUICKSTART.md)
- [ML Demo Medium Article](../../examples/ml-demo/MEDIUM_ARTICLE.md)

### Install Runtimes

Get Python, Java, and other execution environments.

```bash
# Python for ML
rnx runtime install python-3.11-ml

# Java for enterprise
rnx runtime install openjdk-21

# Check what's available
rnx runtime list
```

### Create Workflows

Build your own job pipelines.

```yaml
version: "3.0"
jobs:
  my-job:
    command: "python3"
    args: [ "script.py" ]
    runtime: "python-3.11-ml"
```

---

## Which Guide Should I Use?

### Use the Technical Guide if you:

- Are setting up production infrastructure
- Need a reference to come back to
- Want all commands in one place
- Prefer formal documentation

### Use the Medium Article if you:

- Are learning how Joblet works
- Want to understand the "why" behind each step
- Are writing your own tutorial
- Prefer a narrative style

### Use Both if you:

- Want to understand context (Medium) while having commands reference (Technical)
- Are writing documentation for your team
- Need different versions for different audiences

---

## Publishing the Medium Article

If you're using the Medium article guide for your blog:

1. **Copy Content:** `EC2_INSTALLATION_MEDIUM.md`

2. **Add Code Highlighting:** Medium supports syntax highlighting

3. **Add Images:** Screenshots of AWS Console steps help readers

4. **Update Links:** Change relative links to absolute GitHub URLs

5. **Add Your Voice:** Customize with your own experiences

6. **Cross-Reference:** Link to the ML demo article as a follow-up

---

## Support

- [Joblet GitHub](https://github.com/ehsaniara/joblet)
- [Submit Issues](https://github.com/ehsaniara/joblet/issues)
- [AWS EC2 Docs](https://docs.aws.amazon.com/ec2/)
- [CloudWatch Docs](https://docs.aws.amazon.com/cloudwatch/)

---

## Files in This Directory

```
installation/
├── README.md                    # This file
├── EC2_INSTALLATION.md          # Technical reference guide
└── EC2_INSTALLATION_MEDIUM.md   # Medium article version
```

---

**Ready to install?** Pick your guide and let's get Joblet running on AWS! 🚀
