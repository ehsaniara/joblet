# Java 21 LTS Runtime

OpenJDK 21 runtime with Virtual Threads, Pattern Matching, and modern Java features.

## Quick Start

### 1. Build the Runtime

```bash
# Build the runtime (requires root)
sudo rnx runtime build examples/java-21/runtime.yaml

# Or preview without building
rnx runtime build --dry-run examples/java-21/runtime.yaml
```

### 2. Verify Installation

```bash
# List available runtimes
rnx runtime list

# Test the runtime
rnx runtime test openjdk-21

# Check Java version
rnx job run --runtime=openjdk-21 java -version
```

### 3. Run Examples

```bash
# Compile and run Virtual Threads example
rnx job run --runtime=openjdk-21 --upload=examples/java-21/VirtualThreadExample.java \
  bash -c "javac VirtualThreadExample.java && java VirtualThreadExample"

# Quick Virtual Thread test with jshell
rnx job run --runtime=openjdk-21 jshell -s - << 'EOF'
Thread.startVirtualThread(() -> System.out.println("Virtual Thread works!")).join();
System.out.println("Created virtual thread successfully!");
EOF

# Pattern Matching example
rnx job run --runtime=openjdk-21 bash -c "cat > PatternTest.java << 'JAVA'
public class PatternTest {
    public static void main(String[] args) {
        Object obj = \"Hello\";
        String result = switch (obj) {
            case String s -> \"String: \" + s;
            case Integer i -> \"Integer: \" + i;
            case null -> \"Null value\";
            default -> \"Unknown type\";
        };
        System.out.println(result);
    }
}
JAVA
javac PatternTest.java && java PatternTest"
```

## Runtime Features

- **Java Version**: OpenJDK 21 LTS
- **Virtual Threads**: Project Loom for massive concurrency
- **Pattern Matching**: Switch expressions with type patterns
- **Record Patterns**: Destructuring for records
- **String Templates**: Preview feature

## Example Files

### VirtualThreadExample.java

Demonstrates creating 10,000 virtual threads with minimal memory overhead:

```java
import java.time.Duration;
import java.util.concurrent.Executors;
import java.util.stream.IntStream;

public class VirtualThreadExample {
    public static void main(String[] args) throws InterruptedException {
        System.out.println("Java 21 Virtual Threads Demo");

        long startTime = System.currentTimeMillis();

        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            IntStream.range(0, 10_000).forEach(i -> {
                executor.submit(() -> {
                    try {
                        Thread.sleep(Duration.ofMillis(100));
                        if (i % 1000 == 0) {
                            System.out.println("Virtual thread " + i + " completed");
                        }
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                    }
                });
            });
        }

        long endTime = System.currentTimeMillis();
        System.out.println("Created 10,000 virtual threads in " + (endTime - startTime) + "ms");
    }
}
```

## Performance Benefits

| Feature           | Traditional Threads | Virtual Threads |
|-------------------|---------------------|-----------------|
| Memory per thread | ~1MB stack          | ~1KB            |
| Max concurrent    | ~4,000 (4GB RAM)    | ~4,000,000      |
| Context switch    | ~1-10 μs            | ~0.1-1 μs       |

## Related

- [Java 17 Examples](../java-17/README.md)
- [Runtime System Guide](../../docs/RUNTIME_SYSTEM.md)
