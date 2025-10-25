# Joblet Examples

This directory contains example code and workflows for testing Joblet runtime environments.

## Directory Structure

```
examples/
├── java/           # Java example programs
├── python/         # Python example scripts
└── workflows/      # Multi-job workflow YAML files
```

## Java Examples

### JavaRuntimeTest.java

Java runtime test that validates:

- Java version and system properties
- File I/O operations
- Environment variables
- Basic data structures

Usage:

```bash
# Compile and run Java program
rnx job run --runtime=openjdk-21 --upload=examples/java/JavaRuntimeTest.java javac JavaRuntimeTest.java
rnx job run --runtime=openjdk-21 java JavaRuntimeTest

# Or using GraalVM
rnx job run --runtime=graalvmjdk-21 --upload=examples/java/JavaRuntimeTest.java javac JavaRuntimeTest.java
rnx job run --runtime=graalvmjdk-21 java JavaRuntimeTest
```

### SimpleTest.java

Simple "Hello World" Java program for quick testing.

## Python Examples

### comprehensive-python-test.py

Python runtime test that validates:

- Python version and system info
- NumPy, pandas, scikit-learn functionality
- File operations
- Network connectivity

Usage:

```bash
# Run with ML runtime (full NumPy, Pandas support)
rnx job run --runtime=python-3.11-ml --upload=examples/python/comprehensive-python-test.py python3 comprehensive-python-test.py

# Run with basic runtime (lightweight, essential packages only)
rnx job run --runtime=python-3.11 --upload=examples/python/comprehensive-python-test.py python3 comprehensive-python-test.py
```

### data-processor.py & data-analyzer.py

Example scripts for multi-job workflows demonstrating data processing pipelines.

## 🎯 Runtime Selection Guide

### **Choose `python-3.11` for:**

- ✅ Lightweight AI agents
- ✅ Quick utility scripts
- ✅ Fast startup required (1 second)
- ✅ Basic HTTP operations (with urllib)
- ✅ File processing and JSON handling

### **Choose `python-3.11-ml` for:**

- ✅ Machine learning workloads
- ✅ Data science with NumPy/Pandas
- ✅ Statistical analysis
- ✅ Scientific computing
- ✅ HTTP operations

### **Choose `openjdk-21` for:**

- ✅ Standard Java applications
- ✅ Enterprise development
- ✅ Spring Boot applications
- ✅ Web services

### **Choose `graalvmjdk-21` for:**

- ✅ Java applications
- ✅ Native binary compilation
- ✅ Microservices with fast startup
- ✅ Resource-constrained environments

## Workflow Examples

### java-complete-test.yaml

Multi-job workflow for compiling and executing Java code:

```bash
rnx workflow run examples/workflows/java-complete-test.yaml --upload=examples/java/JavaRuntimeTest.java
```

### python-e2e-test.yaml

End-to-end Python runtime test workflow:

```bash
rnx workflow run examples/workflows/python-e2e-test.yaml --upload=examples/python/comprehensive-python-test.py
```

### multi-job-test.yaml

Demonstrates volume sharing between Python jobs in a data processing pipeline:

```bash
rnx workflow run examples/workflows/multi-job-test.yaml --upload=examples/python/data-processor.py --upload=examples/python/data-analyzer.py
```
