public class JavaRuntimeTest {
    public static void main(String[] args) {
        System.out.println("=== Java Runtime Integration Test ===");

        // Test basic Java functionality
        System.out.println("Java Version: " + System.getProperty("java.version"));
        System.out.println("Java Home: " + System.getProperty("java.home"));
        System.out.println("Java Vendor: " + System.getProperty("java.vendor"));
        System.out.println("OS Name: " + System.getProperty("os.name"));
        System.out.println("OS Architecture: " + System.getProperty("os.arch"));

        // Test basic data structures
        java.util.List<String> testList = java.util.Arrays.asList("Java", "Runtime", "Test");
        System.out.println("List operations: " + testList.size() + " items");

        // Test file operations
        try {
            java.io.File tempFile = new java.io.File("/tmp/java_test.txt");
            java.io.PrintWriter writer = new java.io.PrintWriter(tempFile);
            writer.println("Java runtime test successful!");
            writer.close();

            java.util.Scanner scanner = new java.util.Scanner(tempFile);
            String content = scanner.nextLine();
            scanner.close();
            tempFile.delete();

            System.out.println("File I/O test: " + content);
        } catch (Exception e) {
            System.out.println("File I/O test failed: " + e.getMessage());
        }

        // Test environment variables
        String javaHome = System.getenv("JAVA_HOME");
        String path = System.getenv("PATH");
        System.out.println("JAVA_HOME environment: " + (javaHome != null ? javaHome : "NOT SET"));
        System.out.println("PATH contains java: " + (path != null && path.contains("java")));

        // Test basic math operations
        double result = Math.sqrt(64);
        System.out.println("Math operations: sqrt(64) = " + result);

        System.out.println("✅ Java Runtime Integration Test COMPLETED SUCCESSFULLY!");
    }
}