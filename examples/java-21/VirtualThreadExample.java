/**
 * Java 21 Virtual Threads Example
 * Demonstrates Project Loom's Virtual Threads for high-concurrency applications
 */

import java.time.Duration;
import java.time.Instant;
import java.util.concurrent.Executors;
import java.util.stream.IntStream;

public class VirtualThreadExample {
    public static void main(String[] args) {
        System.out.println("☕ Java 21 Virtual Threads Demo");
        System.out.println("=".repeat(40));
        System.out.println("Java Version: " + System.getProperty("java.version"));
        System.out.println("Available Processors: " + Runtime.getRuntime().availableProcessors());
        System.out.println();
        
        // Traditional platform threads vs Virtual threads comparison
        int taskCount = 1000;
        
        System.out.println("🧵 Comparing Platform Threads vs Virtual Threads");
        System.out.println("Task count: " + taskCount);
        System.out.println();
        
        // Test with Virtual Threads (Java 21)
        System.out.println("🚀 Testing Virtual Threads:");
        testWithVirtualThreads(taskCount);
        
        System.out.println();
        
        // Test with Platform Threads
        System.out.println("🐌 Testing Platform Threads:");
        testWithPlatformThreads(Math.min(taskCount, 100)); // Limit platform threads
        
        System.out.println();
        System.out.println("🎉 Virtual Threads Demo Complete!");
        System.out.println("✨ Virtual threads enable massive concurrency with minimal overhead");
    }
    
    private static void testWithVirtualThreads(int taskCount) {
        Instant start = Instant.now();
        
        try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
            // Submit many virtual threads
            var futures = IntStream.range(0, taskCount)
                .mapToObj(i -> executor.submit(() -> {
                    // Simulate I/O-bound work
                    try {
                        Thread.sleep(100); // 100ms delay
                        return "Virtual Task " + i + " completed by " + Thread.currentThread();
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        return "Task " + i + " interrupted";
                    }
                }))
                .toList();
            
            // Wait for completion and count results
            long completed = futures.stream()
                .mapToLong(future -> {
                    try {
                        String result = future.get();
                        return 1;
                    } catch (Exception e) {
                        System.err.println("Task failed: " + e.getMessage());
                        return 0;
                    }
                })
                .sum();
            
            Duration elapsed = Duration.between(start, Instant.now());
            System.out.println("✅ Completed " + completed + "/" + taskCount + " virtual thread tasks");
            System.out.println("⏱️  Time taken: " + elapsed.toMillis() + "ms");
            System.out.println("🧵 Virtual threads are lightweight and efficient!");
        }
    }
    
    private static void testWithPlatformThreads(int taskCount) {
        Instant start = Instant.now();
        
        // Use a limited thread pool to avoid overwhelming the system
        try (var executor = Executors.newFixedThreadPool(10)) {
            var futures = IntStream.range(0, taskCount)
                .mapToObj(i -> executor.submit(() -> {
                    try {
                        Thread.sleep(100); // 100ms delay
                        return "Platform Task " + i + " completed by " + Thread.currentThread();
                    } catch (InterruptedException e) {
                        Thread.currentThread().interrupt();
                        return "Task " + i + " interrupted";
                    }
                }))
                .toList();
            
            // Wait for completion
            long completed = futures.stream()
                .mapToLong(future -> {
                    try {
                        String result = future.get();
                        return 1;
                    } catch (Exception e) {
                        System.err.println("Task failed: " + e.getMessage());
                        return 0;
                    }
                })
                .sum();
            
            Duration elapsed = Duration.between(start, Instant.now());
            System.out.println("✅ Completed " + completed + "/" + taskCount + " platform thread tasks");
            System.out.println("⏱️  Time taken: " + elapsed.toMillis() + "ms");
            System.out.println("🧵 Platform threads limited by system resources");
        }
    }
    
    // Demonstrate virtual thread creation patterns
    private static void demonstrateVirtualThreadCreation() {
        System.out.println("🔧 Virtual Thread Creation Patterns:");
        
        // Pattern 1: Direct creation
        Thread virtualThread1 = Thread.ofVirtual().start(() -> {
            System.out.println("Virtual thread 1: " + Thread.currentThread());
        });
        
        // Pattern 2: With name
        Thread virtualThread2 = Thread.ofVirtual()
            .name("my-virtual-thread")
            .start(() -> {
                System.out.println("Named virtual thread: " + Thread.currentThread());
            });
        
        // Pattern 3: Factory pattern
        var factory = Thread.ofVirtual().factory();
        Thread virtualThread3 = factory.newThread(() -> {
            System.out.println("Factory virtual thread: " + Thread.currentThread());
        });
        virtualThread3.start();
        
        // Wait for completion
        try {
            virtualThread1.join();
            virtualThread2.join();
            virtualThread3.join();
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
        }
    }
}