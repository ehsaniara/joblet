package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	log := New()
	if log == nil {
		t.Error("New() returned nil logger")
	}

	if log.logger == nil {
		t.Error("New() created logger with nil internal logger")
	}

	if log.level != INFO {
		t.Errorf("Expected default level INFO, got %v", log.level)
	}
}

func TestNewWithConfig(t *testing.T) {
	buf := bytes.NewBuffer(nil)

	log := NewWithConfig(Config{
		Level:  DEBUG,
		Output: buf,
		Format: "text",
		Mode:   "test",
	})

	if log == nil {
		t.Error("NewWithConfig() returned nil logger")
	}

	if log.level != DEBUG {
		t.Errorf("Expected level DEBUG, got %v", log.level)
	}

	if log.mode != "test" {
		t.Errorf("Expected mode 'test', got %s", log.mode)
	}

	// Test logging
	log.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "test message") {
		t.Error("Log output doesn't contain message")
	}
}

func TestWithField(t *testing.T) {
	log := New()

	newLog := log.WithField("component", "test")
	if newLog == nil {
		t.Error("WithField() returned nil logger")
	}

	if len(newLog.fields) != 1 {
		t.Errorf("Expected 1 field, got %d", len(newLog.fields))
	}

	if newLog.fields["component"] != "test" {
		t.Errorf("Expected field value 'test', got %v", newLog.fields["component"])
	}

	// Original logger should remain unchanged
	if len(log.fields) != 0 {
		t.Error("Original logger was modified")
	}
}

func TestWithFields(t *testing.T) {
	log := New()

	newLog := log.WithFields("key1", "value1", "key2", 42)
	if newLog == nil {
		t.Error("WithFields() returned nil logger")
	}

	if len(newLog.fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(newLog.fields))
	}
}

func TestWithMode(t *testing.T) {
	log := New()

	serverLog := log.WithMode("server")
	if serverLog == nil {
		t.Error("WithMode() returned nil logger")
	}

	if serverLog.mode != "server" {
		t.Errorf("Expected mode 'server', got %s", serverLog.mode)
	}

	// Original logger should remain unchanged
	if log.mode != "" {
		t.Error("Original logger was modified")
	}
}

func TestSetMode(t *testing.T) {
	log := New()

	log.SetMode("init")
	if log.mode != "init" {
		t.Errorf("Expected mode 'init', got %s", log.mode)
	}
}

func TestGetMode(t *testing.T) {
	log := New()
	log.SetMode("server")

	mode := log.GetMode()
	if mode != "server" {
		t.Errorf("Expected mode 'server', got %s", mode)
	}
}

func TestLogLevels(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	log := NewWithConfig(Config{
		Level:  DEBUG,
		Output: buf,
	})

	// Test Debug
	log.Debug("debug message")
	if !strings.Contains(buf.String(), "DEBUG") {
		t.Error("Debug log doesn't contain DEBUG level")
	}
	buf.Reset()

	// Test Info
	log.Info("info message")
	if !strings.Contains(buf.String(), "INFO") {
		t.Error("Info log doesn't contain INFO level")
	}
	buf.Reset()

	// Test Warn
	log.Warn("warn message")
	if !strings.Contains(buf.String(), "WARN") {
		t.Error("Warn log doesn't contain WARN level")
	}
	buf.Reset()

	// Test Error
	log.Error("error message")
	if !strings.Contains(buf.String(), "ERROR") {
		t.Error("Error log doesn't contain ERROR level")
	}
}

func TestLogLevelFiltering(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	log := NewWithConfig(Config{
		Level:  WARN, // Only WARN and ERROR
		Output: buf,
	})

	// Debug should not appear
	log.Debug("debug message")
	if strings.Contains(buf.String(), "debug message") {
		t.Error("Debug message appeared despite level being WARN")
	}

	// Info should not appear
	log.Info("info message")
	if strings.Contains(buf.String(), "info message") {
		t.Error("Info message appeared despite level being WARN")
	}

	// Warn should appear
	log.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("Warn message didn't appear")
	}

	// Error should appear
	log.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Error("Error message didn't appear")
	}
}

func TestLogWithFields(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	log := NewWithConfig(Config{
		Level:  INFO,
		Output: buf,
	})

	log.Info("test message", "key1", "value1", "key2", 42)

	output := buf.String()
	if !strings.Contains(output, "key1=value1") {
		t.Error("Log doesn't contain key1=value1")
	}
	if !strings.Contains(output, "key2=42") {
		t.Error("Log doesn't contain key2=42")
	}
}

func TestSetLevel(t *testing.T) {
	log := New()

	log.SetLevel(ERROR)
	if log.GetLevel() != ERROR {
		t.Errorf("Expected level ERROR after SetLevel, got %v", log.GetLevel())
	}
}

func TestGetLevel(t *testing.T) {
	log := New()

	if log.GetLevel() != INFO {
		t.Errorf("Expected default level INFO, got %v", log.GetLevel())
	}
}

func TestIsDebugEnabled(t *testing.T) {
	log := NewWithConfig(Config{Level: DEBUG})
	if !log.IsDebugEnabled() {
		t.Error("IsDebugEnabled() should return true for DEBUG level")
	}

	log.SetLevel(INFO)
	if log.IsDebugEnabled() {
		t.Error("IsDebugEnabled() should return false for INFO level")
	}
}

func TestIsInfoEnabled(t *testing.T) {
	log := NewWithConfig(Config{Level: INFO})
	if !log.IsInfoEnabled() {
		t.Error("IsInfoEnabled() should return true for INFO level")
	}

	log.SetLevel(ERROR)
	if log.IsInfoEnabled() {
		t.Error("IsInfoEnabled() should return false for ERROR level")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
		wantErr  bool
	}{
		{"DEBUG", DEBUG, false},
		{"debug", DEBUG, false},
		{"INFO", INFO, false},
		{"info", INFO, false},
		{"WARN", WARN, false},
		{"warn", WARN, false},
		{"WARNING", WARN, false},
		{"ERROR", ERROR, false},
		{"error", ERROR, false},
		{"invalid", INFO, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLevel(tt.input)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if level != tt.expected {
				t.Errorf("Expected level %v, got %v", tt.expected, level)
			}
		})
	}
}

func TestLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DEBUG, "DEBUG"},
		{INFO, "INFO"},
		{WARN, "WARN"},
		{ERROR, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestConcurrentLogging(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	log := NewWithConfig(Config{
		Level:  INFO,
		Output: buf,
	})

	done := make(chan bool)

	// Test concurrent logging
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				log.Info("concurrent log", "id", id, "iteration", j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should have 100 log lines
	lines := strings.Count(buf.String(), "\n")
	if lines != 100 {
		t.Errorf("Expected 100 log lines, got %d", lines)
	}
}

func TestModeInLogOutput(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	log := NewWithConfig(Config{
		Level:  INFO,
		Output: buf,
		Mode:   "server",
	})

	log.Info("test message")

	output := buf.String()
	if !strings.Contains(output, "[server]") {
		t.Error("Log output doesn't contain mode [server]")
	}
}

func TestFieldPreservation(t *testing.T) {
	log := New()

	log1 := log.WithField("field1", "value1")
	log2 := log1.WithField("field2", "value2")
	log3 := log2.WithMode("test")

	// log3 should have both fields
	if len(log3.fields) != 2 {
		t.Errorf("Expected 2 fields in log3, got %d", len(log3.fields))
	}

	// Mode should be preserved
	if log3.mode != "test" {
		t.Errorf("Expected mode 'test', got %s", log3.mode)
	}
}
