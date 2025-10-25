# Java 21 LTS Examples

Modern Java development with the `java:21` runtime environment - featuring Virtual Threads, Pattern Matching, and
cutting-edge Java features.

## ⚡ Runtime Features

- **Java Version**: OpenJDK 21.0.4 (Long Term Support)
- **Modern Features**:
    - Virtual Threads (Project Loom) - massive concurrency
    - Pattern Matching for switch expressions
    - String Templates (Preview feature)
    - Record Patterns
    - Foreign Function & Memory API
- **Package Size**: ~208MB compressed
- **Memory**: 512MB-2GB recommended
- **Startup Time**: 2-3 seconds (vs 30-120 seconds traditional)

## 🚀 Quick Start

### Using YAML Workflows (NEW - Recommended)

```bash
# Run specific Java 21 example using the workflow
rnx workflow run jobs.yaml      # Virtual threads demo
rnx workflow run jobs.yaml      # Java 21 language features
rnx workflow run jobs.yaml       # HTTP server with virtual threads
rnx workflow run jobs.yaml    # Platform vs Virtual threads comparison
rnx workflow run jobs.yaml  # Structured concurrency patterns
rnx workflow run jobs.yaml        # GraalVM-optimized application
```

### Prerequisites

**Option 1: Deploy Pre-built Package (Recommended)**

```bash
# Copy package from examples/packages/ (208MB)
scp examples/packages/java-21-runtime-complete.tar.gz admin@host:/tmp/

# Deploy on target host
ssh admin@host
sudo tar -xzf /tmp/java-21-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/
sudo chown -R joblet:joblet /opt/joblet/runtimes/java/java-21
```

**Option 2: Build from Setup Script**

```bash
# On Joblet host (as root)
sudo /opt/joblet/runtimes/java-21/setup_java_21.sh
```

### Running Examples

```bash
# Run Virtual Threads example
rnx job run --runtime=java:21 --upload=VirtualThreadExample.java \
  bash -c "javac VirtualThreadExample.java && java VirtualThreadExample"

# Quick Virtual Thread test
rnx job run --runtime=java:21 jshell -s - << 'EOF'
Thread.startVirtualThread(() -> System.out.println("Virtual Thread works!")).join();
System.out.println("Created virtual thread successfully!");
EOF

# Pattern Matching demonstration
rnx job run --runtime=java:21 bash -c "cat > PatternTest.java << 'EOF'
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
EOF
javac PatternTest.java && java PatternTest"
```

## 📁 Example Files

### `VirtualThreadExample.java`

```java
import java.time.Duration;
import java.util.concurrent.Executors;
import java.util.stream.IntStream;

public class VirtualThreadExample {
    public static void main(String[] args) throws InterruptedException {
        System.out.println("🚀 Java 21 Virtual Threads Demo");
        System.out.println("================================");
        
        // Traditional threads vs Virtual threads comparison
        long startTime = System.currentTimeMillis();
        
        // Create 10,000 virtual threads
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
        System.out.println("\n✅ Created and executed 10,000 virtual threads");
        System.out.println("⏱️  Time taken: " + (endTime - startTime) + "ms");
        System.out.println("💡 Traditional threads would use ~10GB RAM, virtual threads use <100MB!");
    }
}
```

## 🎯 Modern Java 21 Features

### Virtual Threads (Project Loom)

```java
// MassiveConcurrency.java
import java.util.concurrent.*;
import java.util.stream.IntStream;

public class MassiveConcurrency {
    public static void main(String[] args) {
        // Create 1 million virtual threads!
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            var futures = IntStream.range(0, 1_000_000)
                .mapToObj(i -> executor.submit(() -> {
                    // Simulate work
                    return "Task " + i;
                }))
                .toList();
            
            // Wait for all to complete
            var results = futures.stream()
                .map(f -> {
                    try { return f.get(); }
                    catch (Exception e) { return "Failed"; }
                })
                .toList();
            
            System.out.println("Completed " + results.size() + " tasks!");
        }
    }
}
```

### Pattern Matching with Records

```java
// ModernPatterns.java
public class ModernPatterns {
    record Point(int x, int y) {}
    record Circle(Point center, int radius) {}
    record Rectangle(Point topLeft, Point bottomRight) {}
    
    public static void main(String[] args) {
        Object shape = new Circle(new Point(10, 20), 5);
        
        String description = switch (shape) {
            case Circle(Point(var x, var y), var r) -> 
                "Circle at (" + x + "," + y + ") with radius " + r;
            case Rectangle(Point(var x1, var y1), Point(var x2, var y2)) ->
                "Rectangle from (" + x1 + "," + y1 + ") to (" + x2 + "," + y2 + ")";
            case null -> "No shape";
            default -> "Unknown shape";
        };
        
        System.out.println(description);
    }
}
```

### String Templates (Preview)

```java
// StringTemplates.java
public class StringTemplates {
    public static void main(String[] args) {
        // Enable with --enable-preview flag
        String name = "Alice";
        int age = 30;
        
        // Traditional
        String old = "Name: " + name + ", Age: " + age;
        
        // String format
        String format = String.format("Name: %s, Age: %d", name, age);
        
        // Future: String templates (preview)
        // String template = STR."Name: \{name}, Age: \{age}";
        
        System.out.println(old);
        System.out.println(format);
    }
}
```

## 🌐 High-Performance Server with Virtual Threads

```java
// VirtualThreadServer.java

import com.sun.net.httpserver.*;

import java.io.*;
import java.net.InetSocketAddress;
import java.util.concurrent.Executors;

public class VirtualThreadServer {
    public static void main(String[] args) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress(8000), 0);

        // Use virtual threads for handling requests
        server.setExecutor(Executors.newVirtualThreadPerTaskExecutor());

        server.createContext("/", exchange -> {
            String response = "Handled by virtual thread: " +
                    Thread.currentThread().toString();
            exchange.sendResponseHeaders(200, response.length());
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(response.getBytes());
            }
        });

        server.start();
        System.out.println("Server running on port 8000 with virtual threads");
    }
}
```

```bash
# Run high-performance server
rnx job run --runtime=java:21 --upload=VirtualThreadServer.java --network=vthread-server --port=8000:8000 \
  bash -c "javac VirtualThreadServer.java && java VirtualThreadServer"

# Load test with virtual threads
rnx job run --runtime=java:21 --network=vthread-server bash -c "
for i in {1..1000}; do
  curl -s http://10.200.0.2:8000 &
done
wait
echo 'Handled 1000 concurrent requests with virtual threads!'
"
```

## 🔧 Advanced Java 21 Projects

### Building Complex Applications with Virtual Threads

```bash
# Create a multi-file Java project using virtual threads
rnx job run --runtime=java:21 --volume=java21-project bash -c "
mkdir -p /volumes/java21-project/src/com/example && \
cd /volumes/java21-project && \
cat > src/com/example/Server.java << 'EOF'
package com.example;

import java.util.concurrent.Executors;

public class Server {
    public static void main(String[] args) throws Exception {
        System.out.println(\"Starting server with virtual threads...\");
        
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            for (int i = 0; i < 1000; i++) {
                final int id = i;
                executor.submit(() -> {
                    System.out.println(\"Virtual thread \" + id + \": \" + Thread.currentThread());
                    try { Thread.sleep(1000); } catch (Exception e) {}
                });
            }
        }
        System.out.println(\"Handled 1000 virtual threads!\");
    }
}
EOF
"

# Compile and run the application
rnx job run --runtime=java:21 --volume=java21-project bash -c "
cd /volumes/java21-project && \
javac -d out src/com/example/*.java && \
java -cp out com.example.Server
"
```

## ⚡ Performance Comparison

| Feature                | Traditional Threads   | Virtual Threads         | Improvement |
|------------------------|-----------------------|-------------------------|-------------|
| Thread Creation        | ~1MB stack per thread | ~1KB per virtual thread | **1000x**   |
| Max Concurrent Threads | ~4,000 (4GB RAM)      | ~4,000,000 (4GB RAM)    | **1000x**   |
| Context Switch         | ~1-10 μs              | ~0.1-1 μs               | **10x**     |
| HTTP Server Capacity   | ~1K concurrent        | ~100K concurrent        | **100x**    |

## 🔍 JVM Diagnostics and Monitoring

```bash
# Virtual thread statistics
rnx job run --runtime=java:21 jcmd 1 Thread.dump_to_file -format=json /tmp/threads.json

# JVM flags for virtual threads
rnx job run --runtime=java:21 java -XX:+ShowCodeDetailsInExceptionMessages \
  -Djdk.virtualThreadScheduler.parallelism=4 \
  -Djdk.virtualThreadScheduler.maxPoolSize=256 \
  VirtualThreadApp

# Memory analysis with virtual threads
rnx job run --runtime=java:21 --max-memory=4096 \
  java -Xmx2g -XX:+UseZGC -XX:+ZGenerational App
```

## 🛠️ Troubleshooting

### Virtual Threads Not Working

```bash
# Verify Java 21
rnx job run --runtime=java:21 java -version

# Check virtual thread support
rnx job run --runtime=java:21 jshell -s - << 'EOF'
try {
    Thread.ofVirtual().start(() -> {}).join();
    System.out.println("Virtual threads supported!");
} catch (Exception e) {
    System.out.println("Virtual threads not available: " + e);
}
EOF
```

### Preview Features

```bash
# Enable preview features
rnx job run --runtime=java:21 javac --enable-preview --release 21 ModernApp.java
rnx job run --runtime=java:21 java --enable-preview ModernApp
```

## 📚 Related Documentation

- [Java 17 Examples](../java-17/README.md) - Enterprise Java LTS
- [Runtime Setup Guide](../../runtimes/java-21/)
- [Package Documentation](../../packages/README.md)
- [Runtime System Overview](../../../docs/RUNTIME_SYSTEM.md)
- [Virtual Threads Guide](https://openjdk.org/jeps/444)

## 🎯 Summary

The Java 21 runtime provides:

- ⚡ **Virtual Threads**: Million-scale concurrency
- 🔄 **Pattern Matching**: Cleaner, more expressive code
- 📦 **Modern Features**: Latest Java innovations
- 🚀 **Instant Startup**: 2-3 seconds deployment
- 💾 **Efficient Package**: ~208MB compressed

Perfect for high-concurrency applications, modern Java development, and exploring cutting-edge JVM features!