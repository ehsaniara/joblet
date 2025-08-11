#!/bin/bash
# Java with optimized JVM settings

javac HelloJoblet.java
echo "Running with optimized JVM settings..."
java -Xms256m -Xmx512m -XX:+UseG1GC -XX:MaxGCPauseMillis=100 HelloJoblet