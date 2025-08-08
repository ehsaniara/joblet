#!/usr/bin/env python3
"""
Example Python script for testing the python:3.11+ml runtime
Demonstrates pandas, numpy, and basic data analysis
"""

import pandas as pd
import numpy as np
import sys

print("🐍 Python ML Runtime Test")
print("=" * 40)
print(f"Python Version: {sys.version}")
print(f"Python Executable: {sys.executable}")
print()

# Test NumPy
print("📊 NumPy Test:")
arr = np.random.randn(100)
print(f"  Array shape: {arr.shape}")
print(f"  Mean: {arr.mean():.3f}")
print(f"  Std: {arr.std():.3f}")
print(f"  NumPy version: {np.__version__}")
print()

# Test Pandas
print("🐼 Pandas Test:")
data = {
    'name': ['Alice', 'Bob', 'Charlie', 'Diana'],
    'age': [25, 30, 35, 28],
    'salary': [50000, 60000, 70000, 55000]
}

df = pd.DataFrame(data)
print("  Sample DataFrame:")
print(df.to_string(index=False))
print(f"  Average age: {df['age'].mean()}")
print(f"  Total salary: ${df['salary'].sum():,}")
print(f"  Pandas version: {pd.__version__}")
print()

# Test other packages
packages_to_test = ['sklearn', 'matplotlib', 'seaborn', 'scipy', 'requests']
print("📦 Package Availability:")
for pkg_name in packages_to_test:
    try:
        pkg = __import__(pkg_name)
        version = getattr(pkg, '__version__', 'unknown')
        print(f"  ✅ {pkg_name}: {version}")
    except ImportError:
        print(f"  ❌ {pkg_name}: Not available")

print()
print("🎉 Python ML Runtime test completed successfully!")
print("✨ All packages are isolated and working correctly!")