public class Java17Features {
    public static void main(String[] args) {
        // Text blocks (Java 15+)
        String textBlock = """
                This is a text block
                introduced in Java 15
                and fully supported in Java 17
                """;
        System.out.println("Text Block Demo:");
        System.out.println(textBlock);

        // instanceof pattern matching (Java 16+)
        Object obj = "Hello";
        String result;
        if (obj instanceof String s) {
            result = "String: " + s;
        } else if (obj instanceof Integer i) {
            result = "Integer: " + i;
        } else if (obj == null) {
            result = "Null value";
        } else {
            result = "Unknown type";
        }
        System.out.println("Pattern Matching: " + result);

        // Records (Java 14+)
        record Person(String name, int age) {
        }
        Person person = new Person("Alice", 30);
        System.out.println("Record: " + person);
    }
}