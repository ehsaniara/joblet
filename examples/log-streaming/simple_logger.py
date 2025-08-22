#!/usr/bin/env python3
"""
Simple high-frequency logger for quick rnx demo
"""
import time
import sys
import os
from datetime import datetime

def main():
    start_num = int(os.getenv('START_NUM', '0'))
    end_num = int(os.getenv('END_NUM', '20'))
    interval = float(os.getenv('INTERVAL', '0.2'))
    
    print(f"🎯 Starting simple high-frequency logger")
    print(f"📊 Configuration: range={start_num}-{end_num}, interval={interval}s")
    print(f"🚀 Async log system demo - rate-decoupled writes")
    
    # Quick burst test
    print("⚡ Burst test...")
    for i in range(10):
        print(f"BURST-{i:02d}: Rapid async log entry #{i}")
    
    print("🔄 Main counting loop...")
    for i in range(start_num, end_num + 1):
        timestamp = datetime.now().strftime("%H:%M:%S.%f")[:-3]
        print(f"[{timestamp}] COUNT: {i:,} | Async log system test")
        time.sleep(interval)
    
    print("✅ High-frequency logging complete!")
    print(f"📈 Total: {end_num - start_num + 1} entries")
    print(f"🚀 Rate: {1/interval:.1f} logs/second")

if __name__ == "__main__":
    main()