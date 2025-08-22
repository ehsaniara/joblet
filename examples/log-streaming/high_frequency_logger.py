#!/usr/bin/env python3
"""
High-frequency logging script for demonstrating Joblet's async log system.

This script generates continuous log output at configurable intervals,
perfect for testing real-time log streaming and the rate-decoupled 
async log persistence system.

Features:
- Configurable count range and interval
- Progress indicators and timestamps
- Burst logging patterns to test overflow protection
- Memory usage simulation for HPC workload testing
"""

import time
import sys
import os
from datetime import datetime

def log_with_timestamp(message):
    """Log message with precise timestamp"""
    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M:%S.%f")[:-3]
    print(f"[{timestamp}] {message}", flush=True)

def simulate_burst_logging():
    """Simulate burst logging to test overflow strategies"""
    log_with_timestamp("🚀 Starting burst logging simulation...")
    
    # Generate 100 rapid log entries to test queue handling
    start_time = time.time()
    for i in range(100):
        log_with_timestamp(f"BURST-{i:03d}: Rapid log entry for async system testing")
    
    end_time = time.time()
    duration = end_time - start_time
    rate = 100 / duration if duration > 0 else float('inf')
    log_with_timestamp(f"✅ Burst complete: 100 entries in {duration:.3f}s ({rate:.1f} logs/sec)")

def simulate_memory_usage_pattern():
    """Simulate memory allocation patterns typical in HPC workloads"""
    log_with_timestamp("💾 Simulating HPC memory allocation pattern...")
    
    # Simulate memory-intensive operations with detailed logging
    for phase in ["initialization", "computation", "reduction", "cleanup"]:
        log_with_timestamp(f"PHASE: {phase.upper()} - Allocating memory blocks...")
        
        for block in range(10):
            # Simulate memory block allocation with metadata
            size_mb = (block + 1) * 64  # 64MB, 128MB, etc.
            log_with_timestamp(f"  ALLOC: Block-{block:02d} -> {size_mb}MB (phase: {phase})")
            time.sleep(0.05)  # Brief pause between allocations
        
        log_with_timestamp(f"PHASE: {phase.upper()} - Memory allocation complete")

def main():
    """Main logging loop with configurable parameters"""
    # Configuration from environment variables with sensible defaults
    start_num = int(os.getenv('START_NUM', '0'))
    end_num = int(os.getenv('END_NUM', '10000'))
    interval = float(os.getenv('INTERVAL', '0.1'))
    
    log_with_timestamp(f"🎯 Starting high-frequency logger")
    log_with_timestamp(f"📊 Configuration: range={start_num}-{end_num}, interval={interval}s")
    log_with_timestamp(f"🔧 Async log system performance test - rate-decoupled writes")
    
    # Initial burst to test async system startup
    simulate_burst_logging()
    
    # HPC simulation early in the sequence
    if start_num < 100:
        simulate_memory_usage_pattern()
    
    # Main counting loop with rich logging
    log_with_timestamp(f"🔄 Beginning main counting loop...")
    
    for i in range(start_num, end_num + 1):
        # Vary log content to test compression and different patterns
        if i % 1000 == 0:
            # Major milestone with detailed info
            elapsed = (i - start_num) * interval
            progress = ((i - start_num) / (end_num - start_num)) * 100
            log_with_timestamp(f"🎯 MILESTONE: {i:,} | Progress: {progress:.1f}% | Elapsed: {elapsed:.1f}s")
            
            # Periodic burst to test sustained high-rate logging
            if i % 5000 == 0 and i > 0:
                log_with_timestamp(f"⚡ Periodic burst test at count {i:,}")
                for burst in range(50):
                    log_with_timestamp(f"BURST-MILESTONE-{burst:02d}: High-rate logging test at {i:,}")
        
        elif i % 100 == 0:
            # Regular progress update
            log_with_timestamp(f"📍 COUNT: {i:,} | Async log system handling sustained writes")
        
        elif i % 10 == 0:
            # Frequent updates with varying content
            log_with_timestamp(f"COUNT: {i:,} | Rate-decoupled write #{i - start_num + 1}")
        
        else:
            # Basic counting with minimal overhead
            log_with_timestamp(f"COUNT: {i:,}")
        
        # Configurable sleep interval
        time.sleep(interval)
    
    # Final burst to test end-of-job behavior
    log_with_timestamp(f"🏁 Final burst test before completion...")
    for final in range(25):
        log_with_timestamp(f"FINAL-BURST-{final:02d}: Testing async system job completion handling")
    
    log_with_timestamp(f"✅ High-frequency logging complete!")
    log_with_timestamp(f"📈 Total entries: {end_num - start_num + 1:,}")
    log_with_timestamp(f"⏱️  Total duration: {(end_num - start_num + 1) * interval:.1f}s")
    log_with_timestamp(f"🚀 Average rate: {1/interval:.1f} logs/second")
    log_with_timestamp(f"💪 Async log system performance test completed successfully")

if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        log_with_timestamp("⚠️  Interrupted by user")
        sys.exit(1)
    except Exception as e:
        log_with_timestamp(f"❌ Error: {e}")
        sys.exit(1)