# Log Streaming Examples

Demonstrates Joblet's real-time log streaming capabilities.

## Quick Start

### Run the Demo

```bash
cd examples/log-streaming
./run_demo.sh
```

The demo script offers:

- **Quick Demo** (10 seconds): Basic functionality
- **Standard Demo** (~5 minutes): Comprehensive features
- **Full Demo** (~10 minutes): All features including stress tests

### Manual Examples

```bash
# Start a logging job
JOB_ID=$(rnx job run python3 examples/log-streaming/simple_logger.py 2>&1 | grep -oP 'ID: \K[a-f0-9-]+')

# Stream logs in real-time
rnx job log -f $JOB_ID

# High-frequency logging test
rnx job run --upload=examples/log-streaming/high_frequency_logger.py \
  python3 high_frequency_logger.py --count=1000 --rate=50
```

## Features Demonstrated

### Real-Time Streaming

- Live log updates with `rnx job log -f`
- Multiple concurrent log viewers
- Backpressure handling for slow clients

### Rate-Decoupled Architecture

- Microsecond write latency
- Background disk writer with batching
- Overflow protection strategies

### Performance Testing

- High-frequency logging (10-100+ logs/second)
- Burst patterns for overflow testing
- Concurrent job logging

## Example Files

### simple_logger.py

Basic logging script that outputs timestamped messages:

```python
import time
import datetime

for i in range(100):
    print(f"[{datetime.datetime.now()}] Log message {i}")
    time.sleep(0.1)
```

### high_frequency_logger.py

Configurable high-frequency logging for performance testing:

```bash
# 1000 logs at 50/second
python3 high_frequency_logger.py --count=1000 --rate=50

# Burst mode
python3 high_frequency_logger.py --count=500 --rate=100
```

## Viewing Logs

```bash
# Follow logs in real-time
rnx job log -f <job-id>

# View complete logs
rnx job log <job-id>

# View last N lines
rnx job log --tail=50 <job-id>
```

## Related

- [Basic Usage Examples](../basic-usage/README.md)
- [Advanced Examples](../advanced/README.md)
