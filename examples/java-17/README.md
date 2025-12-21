# Java 17 LTS Runtime

OpenJDK 17 LTS runtime for enterprise Java applications.

## Quick Start

### 1. Build the Runtime

```bash
# Build the runtime (requires root)
sudo rnx runtime build examples/java-17/runtime.yaml

# Or preview without building
rnx runtime build --dry-run examples/java-17/runtime.yaml
```

### 2. Verify Installation

```bash
# List available runtimes
rnx runtime list

# Test the runtime
rnx runtime test openjdk-17

# Check Java version
rnx job run --runtime=openjdk-17 java -version
```

### 3. Run Examples

```bash
# Compile and run HelloJoblet
rnx job run --runtime=openjdk-17 --upload=examples/java-17/HelloJoblet.java \
  bash -c "javac HelloJoblet.java && java HelloJoblet"

# Run Java 17 features demo
rnx job run --runtime=openjdk-17 --upload=examples/java-17/Java17Features.java \
  bash -c "javac Java17Features.java && java Java17Features"

# Quick test with jshell
rnx job run --runtime=openjdk-17 jshell -s - << 'EOF'
System.out.println("Java 17 is working!");
record Point(int x, int y) {}
var p = new Point(10, 20);
System.out.println("Record: " + p);
EOF
```

## Runtime Features

- **Java Version**: OpenJDK 17 LTS
- **Records**: Immutable data classes
- **Sealed Classes**: Restricted class hierarchies
- **Pattern Matching**: instanceof patterns
- **Text Blocks**: Multi-line strings

## Example Files

### HelloJoblet.java

Simple test program:

```java
public class HelloJoblet {
    public static void main(String[] args) {
        System.out.println("Hello from Joblet!");
        System.out.println("Java version: " + System.getProperty("java.version"));
    }
}
```

### Java17Features.java

Demonstrates Java 17 language features like records and pattern matching.

## Scripts

The `scripts/` directory contains helper scripts:

- `compile-and-run.sh` - Compile and run Java files
- `java17-features.sh` - Demo Java 17 features
- `optimized-jvm.sh` - JVM optimization examples

## Related

- [Java 21 Examples](../java-21/README.md) - Virtual Threads and newer features
- [Runtime YAML Reference](../../docs/design/RUNTIME_YAML_QUICKREF.md)
