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

Comprehensive Java runtime test that validates:

- Java version and system properties
- File I/O operations
- Environment variables
- Basic data structures

Usage:

```bash
rnx run --runtime=openjdk:21 --upload=examples/java/JavaRuntimeTest.java javac JavaRuntimeTest.java
rnx run --runtime=openjdk:21 java JavaRuntimeTest
```

### SimpleTest.java

Simple "Hello World" Java program for quick testing.

## Python Examples

### comprehensive-python-test.py

Complete Python runtime test that validates:

- Python version and system info
- NumPy, pandas, scikit-learn functionality
- File operations
- Network connectivity

Usage:

```bash
rnx run --runtime=python-3.11-ml --upload=examples/python/comprehensive-python-test.py python3 comprehensive-python-test.py
```

### data-processor.py & data-analyzer.py

Example scripts for multi-job workflows demonstrating data processing pipelines.

## Workflow Examples

### java-complete-test.yaml

Multi-job workflow for compiling and executing Java code:

```bash
rnx run --workflow=examples/workflows/java-complete-test.yaml --upload=examples/java/JavaRuntimeTest.java
```

### python-e2e-test.yaml

End-to-end Python runtime test workflow:

```bash
rnx run --workflow=examples/workflows/python-e2e-test.yaml --upload=examples/python/comprehensive-python-test.py
```

### multi-job-test.yaml

Demonstrates volume sharing between Python jobs in a data processing pipeline:

```bash
rnx run --workflow=examples/workflows/multi-job-test.yaml --upload=examples/python/data-processor.py --upload=examples/python/data-analyzer.py
```
