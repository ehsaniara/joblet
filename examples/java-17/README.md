# Java 17 LTS Examples

Enterprise Java development with the `java:17` runtime environment - instant compilation and execution.

## ⚡ Runtime Features

- **Java Version**: OpenJDK 17.0.12 (Long Term Support)
- **Development Tools**: javac, jar, javap, jshell (interactive REPL)
- **Package Size**: ~193MB compressed
- **Memory**: 512MB-2GB recommended
- **Startup Time**: 2-3 seconds (vs 30-120 seconds traditional)

## 🚀 Quick Start

### Using YAML Workflows (NEW - Recommended)

```bash
# Run specific Java 17 example using the workflow
rnx workflow run jobs.yaml      # Compile and run HelloJoblet
rnx workflow run jobs.yaml     # Run with JVM optimization
rnx workflow run jobs.yaml   # Demonstrate Java 17 features
rnx workflow run jobs.yaml       # Package as JAR and run
rnx workflow run jobs.yaml  # Performance testing
rnx workflow run jobs.yaml  # Persistent storage example
```

### Prerequisites

**Option 1: Deploy Pre-built Package (Recommended)**

```bash
# Copy package from examples/packages/ (193MB)
scp examples/packages/java-17-runtime-complete.tar.gz admin@host:/tmp/

# Deploy on target host
ssh admin@host
sudo tar -xzf /tmp/java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/
sudo chown -R joblet:joblet /opt/joblet/runtimes/java/java-17
```

**Option 2: Build from Setup Script**

```bash
# On Joblet host (as root)
sudo /opt/joblet/runtimes/java-17/setup_java_17.sh
```

### Running Examples

```bash
# Compile and run the Hello example
rnx job run --runtime=java:17 --upload=HelloJoblet.java \
  bash -c "javac HelloJoblet.java && java HelloJoblet"

# Quick test
rnx job run --runtime=java:17 java -version

# Interactive Java REPL
rnx job run --runtime=java:17 jshell

# One-liner Java program
rnx job run --runtime=java:17 bash -c "echo 'public class Test { public static void main(String[] args) { System.out.println(\"Java 17 works!\"); } }' > Test.java && javac Test.java && java Test"
```

## 📁 Example Files

### `HelloJoblet.java`

```java
public class HelloJoblet {
    public static void main(String[] args) {
        System.out.println("Hello from Joblet!");
        System.out.println("Java Version: " + System.getProperty("java.version"));
        System.out.println("Java Home: " + System.getProperty("java.home"));
        System.out.println("Runtime: OpenJDK 17 LTS in complete isolation");
    }
}
```

## 🔧 Java Project Development

### Compile and Run Java Applications

```bash
# Compile a Java source file
rnx job run --runtime=java:17 --upload=MyApp.java javac MyApp.java

# Run the compiled class
rnx job run --runtime=java:17 java MyApp

# Compile and run in one command
rnx job run --runtime=java:17 --upload=MyApp.java \
  bash -c "javac MyApp.java && java MyApp"
```

### Create and Run JAR Files

```bash
# Create a JAR file with manifest
rnx job run --runtime=java:17 --upload-dir=src bash -c "
javac src/*.java && \
echo 'Main-Class: Main' > manifest.txt && \
jar cfm app.jar manifest.txt *.class
"

# Run the JAR file
rnx job run --runtime=java:17 --upload=app.jar java -jar app.jar
```

## 🎯 Common Use Cases

### Enterprise Application Development

```java
// EnterpriseApp.java

import java.util.*;
import java.util.stream.*;
import java.util.concurrent.*;

public class EnterpriseApp {
    public static void main(String[] args) {
        // Modern Java 17 features
        var employees = List.of("Alice", "Bob", "Charlie");

        // Stream processing
        employees.stream()
                .filter(e -> e.length() > 3)
                .map(String::toUpperCase)
                .forEach(System.out::println);

        // Concurrent processing
        try (var executor = Executors.newFixedThreadPool(4)) {
            var futures = employees.stream()
                    .map(e -> executor.submit(() -> process(e)))
                    .collect(Collectors.toList());

            futures.forEach(f -> {
                try {
                    f.get();
                } catch (Exception ex) {
                }
            });
        }
    }

    static void process(String employee) {
        System.out.println("Processing: " + employee);
    }
}
```

### JShell Interactive Development

```bash
# Start interactive session
rnx job run --runtime=java:17 jshell

# In JShell, experiment with Java 17 features:
jshell> var list = List.of(1, 2, 3, 4, 5)
jshell> list.stream().filter(x -> x > 2).collect(Collectors.toList())
jshell> record Person(String name, int age) {}
jshell> var person = new Person("Alice", 30)
jshell> System.out.println(person)
```

### JAR Creation and Execution

```bash
# Create and package JAR
rnx job run --runtime=java:17 --volume=java-artifacts bash -c "
echo 'public class App { 
    public static void main(String[] args) { 
        System.out.println(\"JAR Application Running!\"); 
    }
}' > App.java && \
javac App.java && \
jar cfe app.jar App App.class && \
cp app.jar /volumes/java-artifacts/
"

# Run JAR from volume
rnx job run --runtime=java:17 --volume=java-artifacts \
  java -jar /volumes/java-artifacts/app.jar
```

## 🌐 Network Capabilities

### HTTP Server

```java
// SimpleHttpServer.java

import com.sun.net.httpserver.*;

import java.io.*;
import java.net.InetSocketAddress;

public class SimpleHttpServer {
    public static void main(String[] args) throws IOException {
        HttpServer server = HttpServer.create(new InetSocketAddress(8000), 0);
        server.createContext("/", exchange -> {
            String response = "Hello from Java 17!";
            exchange.sendResponseHeaders(200, response.length());
            try (OutputStream os = exchange.getResponseBody()) {
                os.write(response.getBytes());
            }
        });
        server.start();
        System.out.println("Server running on port 8000");
    }
}
```

```bash
# Run HTTP server
rnx job run --runtime=java:17 --upload=SimpleHttpServer.java --network=java-api --port=8000:8000 \
  bash -c "javac SimpleHttpServer.java && java SimpleHttpServer"

# Test from another job
rnx job run --network=java-api curl http://10.200.0.2:8000
```

## 📦 Volume Persistence

```bash
# Create persistent volume for build artifacts
rnx volume create java-builds

# Compile to volume
rnx job run --runtime=java:17 --volume=java-builds --upload=HelloJoblet.java \
  bash -c "javac -d /volumes/java-builds HelloJoblet.java"

# Run from volume
rnx job run --runtime=java:17 --volume=java-builds \
  bash -c "cd /volumes/java-builds && java HelloJoblet"
```

## ⚡ Performance Comparison

| Operation        | Traditional    | Runtime     | Speedup   |
|------------------|----------------|-------------|-----------|
| JDK Installation | 30-120 seconds | 0 seconds   | ∞         |
| Job Startup      | 10-30 seconds  | 2-3 seconds | **5-10x** |
| JAR Creation     | 10-20 seconds  | 2-5 seconds | **4-5x**  |
| Compilation      | 5-10 seconds   | 1-2 seconds | **3-5x**  |

## 🔍 Debugging and Profiling

```bash
# Enable debugging
rnx job run --runtime=java:17 --upload=App.java \
  bash -c "javac -g App.java && java -agentlib:jdwp=transport=dt_socket,server=y,suspend=n,address=*:5005 App"

# JVM diagnostics
rnx job run --runtime=java:17 java -XshowSettings:vm -version

# Memory analysis
rnx job run --runtime=java:17 --max-memory=2048 \
  java -Xmx1g -XX:+PrintGCDetails -XX:+PrintGCTimeStamps App
```

## 🛠️ Troubleshooting

### Runtime Not Found

```bash
# Check if runtime is installed
rnx runtime list

# Deploy if missing
sudo tar -xzf java-17-runtime-complete.tar.gz -C /opt/joblet/runtimes/java/
```

### Classpath Issues

```bash
# Set classpath explicitly
rnx job run --runtime=java:17 --upload-dir=libs \
  java -cp "/volumes/libs/*:." MainClass
```

### Memory Errors

```bash
# Increase heap size
rnx job run --runtime=java:17 --max-memory=4096 \
  java -Xmx3g -Xms1g LargeApp
```

## 📚 Related Documentation

- [Java 21 Examples](../java-21/README.md) - Modern Java features
- [Runtime Setup Guide](../../runtimes/java-17/)
- [Package Documentation](../../packages/README.md)
- [Runtime System Overview](../../../docs/RUNTIME_SYSTEM.md)

## 🎯 Summary

The Java 17 runtime provides:

- ⚡ **Instant Startup**: 2-3 seconds vs minutes
- 🔒 **Complete Isolation**: Zero host contamination
- 📦 **Enterprise Ready**: Full JDK with all development tools
- 🚀 **Production Grade**: LTS support
- 💾 **Efficient**: ~193MB compressed package

Perfect for enterprise applications, microservices, and modern Java development!