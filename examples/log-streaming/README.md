# Log Streaming Example - Async Log System Performance Demo

This example demonstrates Joblet's **rate-decoupled async log persistence system** optimized for HPC workloads. Experience real-time log streaming with microsecond write latency and 5M+ writes/second capability.

## 🎯 What This Example Demonstrates

### **Rate-Decoupled Async Architecture**
- **Microsecond Writes**: Jobs write to channels instantly, never waiting for disk I/O
- **Producer-Consumer Pattern**: Background disk writer handles batching and optimization
- **Overflow Protection**: Four strategies (compress/spill/sample/alert) prevent data loss
- **HPC Optimization**: Handles 1000+ concurrent jobs with GB-scale logs

### **Real-Time Log Streaming**
- **Live Updates**: Watch logs appear instantly with `rnx log -f`
- **Historical Access**: View complete job logs from start to finish
- **Multiple Clients**: Concurrent streaming to multiple terminals
- **Backpressure Handling**: Automatic cleanup of slow clients

### **Performance Validation**
- **High-Frequency Logging**: 10-100 logs/second sustained rates
- **Burst Patterns**: Rapid log generation to test overflow strategies
- **Concurrent Jobs**: Multiple simultaneous high-frequency loggers
- **Memory Management**: Bounded memory usage with configurable limits

## 🚀 Quick Start

### **Option 1: Interactive Demo Script**
```bash
cd examples/log-streaming
./run_demo.sh
```

Choose from:
- **Quick Demo** (10 seconds): Basic functionality showcase
- **Standard Demo** (~5 minutes): Comprehensive features  
- **Full Demo** (~10 minutes): All features including stress tests

### **Option 2: Individual Jobs**
```bash
# Quick 10-second demo (100 counts at 10 logs/second)
rnx run --workflow=jobs.yaml:quick-demo

# Standard demo (1000 counts at 10 logs/second) 
rnx run --workflow=jobs.yaml:standard-demo

# High-frequency test (20 logs/second)
rnx run --workflow=jobs.yaml:high-frequency

# Burst test (50 logs/second - tests async overflow)
rnx run --workflow=jobs.yaml:burst-test

# HPC simulation (10,000 counts over ~16 minutes)
rnx run --workflow=jobs.yaml:hpc-simulation

# Stress test (100 logs/second)
rnx run --workflow=jobs.yaml:stress-test
```

### **Option 3: Real-Time Streaming**
```bash
# Start a logging job
JOB_ID=$(rnx run --workflow=jobs.yaml:standard-demo | grep -o '[0-9a-f\-]*')

# Stream logs in real-time (watch async system in action)
rnx log -f $JOB_ID

# View complete logs after job finishes
rnx log --follow=false $JOB_ID
```

## 📊 Available Logging Patterns

### **Pre-configured Jobs**

| Job Name | Rate | Count | Duration | Purpose |
|----------|------|-------|----------|---------|
| `quick-demo` | 10/sec | 100 | 10s | Basic functionality demo |
| `standard-demo` | 10/sec | 1,000 | ~100s | Standard performance test |
| `high-frequency` | 20/sec | 1,000 | ~50s | High-rate sustained logging |
| `burst-test` | 50/sec | 500 | ~10s | Async overflow testing |
| `hpc-simulation` | 10/sec | 10,000 | ~16min | HPC workload simulation |
| `stress-test` | 100/sec | 2,000 | ~20s | Maximum rate validation |

### **Custom Configuration**
```bash
# Configure your own logging pattern
START_NUM=0 END_NUM=5000 INTERVAL=0.05 rnx run --workflow=jobs.yaml:custom-range
```

Environment variables:
- `START_NUM`: Starting count (default: 0)
- `END_NUM`: Ending count (default: 10000)  
- `INTERVAL`: Seconds between logs (default: 0.1)

## 🔄 Concurrent Logging Demo

Test multiple high-frequency loggers simultaneously:

```bash
# Run multiple concurrent loggers
rnx run --workflow=concurrent-logging.yaml

# Or start them individually
rnx run --workflow=jobs.yaml:quick-demo &
rnx run --workflow=jobs.yaml:quick-demo &
rnx run --workflow=jobs.yaml:quick-demo &

# Monitor all jobs
rnx list
```

## 📈 Performance Monitoring

### **Real-Time System Monitoring**
```bash
# Monitor system performance during logging
rnx monitor status

# View all active jobs
rnx list

# Check specific job status
rnx status <job-uuid>
```

### **Log Analysis**
```bash
# Count total log entries
rnx log --follow=false <job-uuid> | wc -l

# Analyze timestamp precision
rnx log --follow=false <job-uuid> | head -20

# Check for burst patterns
rnx log --follow=false <job-uuid> | grep "BURST"

# Verify HPC simulation patterns
rnx log --follow=false <job-uuid> | grep "PHASE\|ALLOC"
```

## 🎛️ Logging Script Features

The `high_frequency_logger.py` script includes:

### **Burst Testing**
- Initial burst of 100 rapid log entries
- Periodic bursts every 5,000 counts
- Final burst before completion
- Tests async system overflow handling

### **HPC Simulation**
- Memory allocation phase logging
- Computational phase simulation
- Resource cleanup tracking
- Realistic HPC workload patterns

### **Rich Log Content**
- Precise timestamps with millisecond precision
- Progress indicators and milestones
- Varying content patterns for compression testing
- Performance metrics and rates

### **Configurable Parameters**
- Adjustable count ranges
- Variable logging intervals
- Burst pattern configuration
- Resource limit testing

## 🔧 Async Log System Configuration

The example uses these settings to demonstrate optimal performance:

```yaml
# Resource limits for different test patterns
max_cpu: 25-80     # CPU percentage limit
max_memory: 128-512 # Memory limit in MB

# Logging rates for different scenarios
interval: 0.01-0.1  # Seconds between logs (10-100 logs/second)
```

### **System Configuration**
For production HPC workloads, tune these async log system parameters:

```yaml
log_persistence:
  queue_size: 100000              # Large queue for burst handling
  memory_limit: 1073741824        # 1GB overflow protection
  batch_size: 100                 # Efficient disk batching
  flush_interval: "100ms"         # Low-latency periodic flush
  overflow_strategy: "compress"   # Memory-efficient default
```

## 🎯 Expected Output Examples

### **Quick Demo Output**
```
[2025-01-22 10:30:00.123] 🎯 Starting high-frequency logger
[2025-01-22 10:30:00.124] 📊 Configuration: range=0-100, interval=0.1s
[2025-01-22 10:30:00.125] 🚀 Starting burst logging simulation...
[2025-01-22 10:30:00.126] BURST-001: Rapid log entry for async system testing
[2025-01-22 10:30:00.127] BURST-002: Rapid log entry for async system testing
...
[2025-01-22 10:30:00.234] ✅ Burst complete: 100 entries in 0.108s (925.9 logs/sec)
[2025-01-22 10:30:00.235] 🔄 Beginning main counting loop...
[2025-01-22 10:30:00.336] COUNT: 1
[2025-01-22 10:30:00.437] COUNT: 2
...
[2025-01-22 10:30:10.123] ✅ High-frequency logging complete!
```

### **Streaming Display**
```bash
$ rnx log -f f47ac10b-58cc-4372-a567-0e02b2c3d479

Logs for job f47ac10b-58cc-4372-a567-0e02b2c3d479 (Press Ctrl+C to exit):
[2025-01-22 10:30:00.123] 🎯 Starting high-frequency logger
[2025-01-22 10:30:00.124] 📊 Configuration: range=0-1000, interval=0.1s
[2025-01-22 10:30:00.125] 🚀 Starting burst logging simulation...
[2025-01-22 10:30:00.126] BURST-001: Rapid log entry for async system testing
# ... logs continue streaming in real-time ...
```

## 🏆 What Makes This Special

### **Async Log System Benefits**
1. **Zero Job Impact**: Jobs never wait for disk I/O regardless of log volume
2. **Complete Data Integrity**: All logs preserved with overflow protection  
3. **Real-Time Streaming**: Instant log availability for monitoring
4. **HPC Optimized**: Handles extreme workloads (1000+ jobs, GB logs)
5. **Configurable Protection**: Multiple overflow strategies for different scenarios

### **Rate Decoupling**
- **Producer Side**: Jobs write to channels instantly (microseconds)
- **Consumer Side**: Background worker optimizes disk I/O with batching
- **Overflow Protection**: Multiple strategies prevent data loss under load
- **Monitoring**: Real-time metrics and performance tracking

### **Production Ready**
- Tested with 5M+ writes/second sustained throughput
- Memory usage bounded by configuration (1GB default)
- Comprehensive overflow handling and recovery
- Complete integration with job lifecycle and streaming

## 🔍 Troubleshooting

### **Common Issues**

**Logs not appearing in real-time:**
```bash
# Check job status
rnx status <job-uuid>

# Verify async log system configuration
cat /opt/joblet/config/joblet-config.yml | grep -A 10 log_persistence
```

**High CPU during burst tests:**
```bash
# Normal for stress tests - verify limits
rnx status <job-uuid>

# Monitor system resources
rnx monitor status
```

**Jobs finishing too quickly:**
```bash
# Check interval configuration
rnx status <job-uuid>

# Use longer-running jobs
rnx run --workflow=jobs.yaml:hpc-simulation
```

### **Performance Validation**
```bash
# Verify async system is handling load
grep "async" /var/log/joblet/joblet.log

# Check overflow protection activation
grep "overflow" /var/log/joblet/joblet.log

# Monitor memory usage
cat /sys/fs/cgroup/joblet.slice/joblet.service/memory.current
```

## 🌟 Next Steps

After running this example:

1. **Explore Real Workloads**: Apply these patterns to your actual HPC jobs
2. **Tune Configuration**: Adjust async system parameters for your workload
3. **Monitor Production**: Use these techniques to monitor production jobs
4. **Scale Testing**: Test with more concurrent jobs and higher rates
5. **Custom Scripts**: Create your own high-frequency logging applications

The async log system ensures your critical HPC workloads maintain optimal performance while providing complete observability and real-time monitoring capabilities.