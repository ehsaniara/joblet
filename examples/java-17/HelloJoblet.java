/**
 * Example Java program for testing the java:17 runtime
 * Demonstrates basic Java functionality and environment info
 */
public class HelloJoblet {
    public static void main(String[] args) {
        System.out.println("☕ Java Runtime Test");
        System.out.println("=".repeat(40));

        // Java version info
        System.out.println("Java Version: " + System.getProperty("java.version"));
        System.out.println("Java Vendor: " + System.getProperty("java.vendor"));
        System.out.println("Java Home: " + System.getProperty("java.home"));
        System.out.println("OS: " + System.getProperty("os.name") + " " + System.getProperty("os.version"));
        System.out.println();

        // Runtime information
        Runtime runtime = Runtime.getRuntime();
        long maxMemory = runtime.maxMemory();
        long totalMemory = runtime.totalMemory();
        long freeMemory = runtime.freeMemory();

        System.out.println("🖥️  Runtime Information:");
        System.out.printf("  Max Memory: %d MB%n", maxMemory / (1024 * 1024));
        System.out.printf("  Total Memory: %d MB%n", totalMemory / (1024 * 1024));
        System.out.printf("  Free Memory: %d MB%n", freeMemory / (1024 * 1024));
        System.out.printf("  Used Memory: %d MB%n", (totalMemory - freeMemory) / (1024 * 1024));
        System.out.println();

        // Test basic functionality
        System.out.println("🧮 Basic Functionality Test:");

        // Array processing
        int[] numbers = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10};
        int sum = 0;
        for (int num : numbers) {
            sum += num;
        }
        System.out.println("  Array sum: " + sum);

        // String processing
        String message = "Hello from Joblet Java Runtime!";
        System.out.println("  Message: " + message);
        System.out.println("  Message length: " + message.length());
        System.out.println("  Uppercase: " + message.toUpperCase());

        // Command line arguments
        if (args.length > 0) {
            System.out.println("🔧 Command Line Arguments:");
            for (int i = 0; i < args.length; i++) {
                System.out.println("  Arg " + i + ": " + args[i]);
            }
        } else {
            System.out.println("🔧 No command line arguments provided");
        }

        System.out.println();
        System.out.println("🎉 Java runtime test completed successfully!");
        System.out.println("✨ Java environment is isolated and working correctly!");
    }
}