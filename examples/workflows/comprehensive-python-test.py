#!/usr/bin/env python3
"""
Comprehensive Python runtime test
Tests Python environment, package availability, and basic functionality
"""

import json
import os
import sys


def test_python_environment():
    """Test basic Python environment"""
    print("=== Python Environment Test ===")
    print(f"Python version: {sys.version}")
    print(f"Python executable: {sys.executable}")
    print(f"Platform: {sys.platform}")
    print(f"Python path entries: {len(sys.path)}")

    # Show first few Python path entries
    print("Python path:")
    for i, path in enumerate(sys.path[:5]):
        print(f"  {i + 1}. {path}")
    if len(sys.path) > 5:
        print(f"  ... and {len(sys.path) - 5} more entries")


def test_environment_variables():
    """Test runtime environment variables"""
    print("\n=== Environment Variables ===")
    env_vars = ['PYTHON_HOME', 'PYTHONPATH', 'PATH', 'LD_LIBRARY_PATH']
    for var in env_vars:
        value = os.environ.get(var, 'NOT SET')
        if len(value) > 100:
            value = value[:100] + "..."
        print(f"{var}: {value}")


def test_package_imports():
    """Test importing available packages"""
    print("\n=== Package Import Test ===")
    packages_to_test = [
        'json', 'os', 'sys', 'math', 'datetime', 'urllib', 'http',
        'requests', 'numpy', 'pandas', 'sklearn', 'matplotlib', 'scipy'
    ]

    available_packages = []
    unavailable_packages = []

    for package in packages_to_test:
        try:
            __import__(package)
            available_packages.append(package)
            print(f"✓ {package}: Available")
        except ImportError:
            unavailable_packages.append(package)
            print(f"✗ {package}: Not available")

    return available_packages, unavailable_packages


def test_basic_functionality():
    """Test basic Python functionality"""
    print("\n=== Basic Functionality Test ===")

    # Test file operations
    test_file = "/tmp/runtime_test.txt"
    try:
        with open(test_file, 'w') as f:
            f.write("Runtime test successful!")
        with open(test_file, 'r') as f:
            content = f.read()
        os.remove(test_file)
        print("✓ File operations: Working")
    except Exception as e:
        print(f"✗ File operations: Failed - {e}")

    # Test JSON processing
    try:
        test_data = {"test": "data", "numbers": [1, 2, 3]}
        json_str = json.dumps(test_data)
        parsed = json.loads(json_str)
        assert parsed == test_data
        print("✓ JSON processing: Working")
    except Exception as e:
        print(f"✗ JSON processing: Failed - {e}")


def test_advanced_features():
    """Test advanced Python features if packages are available"""
    print("\n=== Advanced Features Test ===")

    # Test requests if available
    try:
        import requests
        print(f"✓ Requests version: {requests.__version__}")
        # Test basic requests functionality (but don't make actual HTTP calls in isolated env)
        print("✓ Requests module: Loaded successfully")
    except ImportError:
        print("✗ Requests: Not available")

    # Test numpy if available
    try:
        import numpy as np
        arr = np.array([1, 2, 3, 4, 5])
        mean_val = np.mean(arr)
        print(f"✓ NumPy: Working (mean of [1,2,3,4,5] = {mean_val})")
    except ImportError:
        print("✗ NumPy: Not available")


def main():
    """Run comprehensive runtime test"""
    print("🐍 Comprehensive Python Runtime Integration Test")
    print("=" * 50)

    test_python_environment()
    test_environment_variables()
    available, unavailable = test_package_imports()
    test_basic_functionality()
    test_advanced_features()

    print("\n=== Test Summary ===")
    print(f"Python version: {sys.version.split()[0]}")
    print(f"Available packages: {len(available)}")
    print(f"Unavailable packages: {len(unavailable)}")

    if available:
        print(f"✓ Working packages: {', '.join(available[:10])}")
        if len(available) > 10:
            print(f"  ... and {len(available) - 10} more")

    print("\n🎉 Runtime integration test completed successfully!")
    return 0


if __name__ == "__main__":
    sys.exit(main())
