//go:build linux

package builder

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNewBuildLogger(t *testing.T) {
	logger := NewBuildLogger(true)
	if logger == nil {
		t.Fatal("NewBuildLogger returned nil")
	}
	if !logger.verbose {
		t.Error("expected verbose=true")
	}

	logger2 := NewBuildLogger(false)
	if logger2.verbose {
		t.Error("expected verbose=false")
	}
}

func TestDefaultBuildLogger_Info(t *testing.T) {
	logger := NewBuildLogger(false)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger.Info("test message %s", "arg")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "INFO") {
		t.Errorf("expected output to contain 'INFO', got: %s", output)
	}
	if !strings.Contains(output, "test message arg") {
		t.Errorf("expected output to contain 'test message arg', got: %s", output)
	}
}

func TestDefaultBuildLogger_Debug_Verbose(t *testing.T) {
	logger := NewBuildLogger(true)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger.Debug("debug message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "DEBUG") {
		t.Errorf("expected output to contain 'DEBUG', got: %s", output)
	}
	if !strings.Contains(output, "debug message") {
		t.Errorf("expected output to contain 'debug message', got: %s", output)
	}
}

func TestDefaultBuildLogger_Debug_NotVerbose(t *testing.T) {
	logger := NewBuildLogger(false)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger.Debug("debug message")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	// Should be empty when verbose=false
	if output != "" {
		t.Errorf("expected empty output when verbose=false, got: %s", output)
	}
}

func TestDefaultBuildLogger_Warn(t *testing.T) {
	logger := NewBuildLogger(false)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger.Warn("warning message")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "WARN") {
		t.Errorf("expected output to contain 'WARN', got: %s", output)
	}
	if !strings.Contains(output, "warning message") {
		t.Errorf("expected output to contain 'warning message', got: %s", output)
	}
}

func TestDefaultBuildLogger_Error(t *testing.T) {
	logger := NewBuildLogger(false)

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	logger.Error("error message")

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "ERROR") {
		t.Errorf("expected output to contain 'ERROR', got: %s", output)
	}
	if !strings.Contains(output, "error message") {
		t.Errorf("expected output to contain 'error message', got: %s", output)
	}
}

func TestDefaultBuildLogger_Phase(t *testing.T) {
	logger := NewBuildLogger(false)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	logger.Phase(3, 14, "Install Packages", "Installing pip packages")

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "PHASE 3/14") {
		t.Errorf("expected output to contain 'PHASE 3/14', got: %s", output)
	}
	if !strings.Contains(output, "Install Packages") {
		t.Errorf("expected output to contain 'Install Packages', got: %s", output)
	}
	if !strings.Contains(output, "Installing pip packages") {
		t.Errorf("expected output to contain 'Installing pip packages', got: %s", output)
	}
}

func TestBuildLogger_Interface(t *testing.T) {
	// Verify DefaultBuildLogger implements BuildLogger interface
	var _ BuildLogger = (*DefaultBuildLogger)(nil)
}

// MockBuildLogger for testing purposes
type MockBuildLogger struct {
	InfoMessages  []string
	DebugMessages []string
	WarnMessages  []string
	ErrorMessages []string
	PhaseMessages []string
}

func NewMockBuildLogger() *MockBuildLogger {
	return &MockBuildLogger{}
}

func (l *MockBuildLogger) Info(format string, args ...interface{}) {
	l.InfoMessages = append(l.InfoMessages, strings.TrimSpace(format))
}

func (l *MockBuildLogger) Debug(format string, args ...interface{}) {
	l.DebugMessages = append(l.DebugMessages, strings.TrimSpace(format))
}

func (l *MockBuildLogger) Warn(format string, args ...interface{}) {
	l.WarnMessages = append(l.WarnMessages, strings.TrimSpace(format))
}

func (l *MockBuildLogger) Error(format string, args ...interface{}) {
	l.ErrorMessages = append(l.ErrorMessages, strings.TrimSpace(format))
}

func (l *MockBuildLogger) Phase(phase int, totalPhases int, name string, message string) {
	l.PhaseMessages = append(l.PhaseMessages, name)
}

func TestMockBuildLogger(t *testing.T) {
	mock := NewMockBuildLogger()

	mock.Info("info 1")
	mock.Info("info 2")
	mock.Debug("debug 1")
	mock.Warn("warn 1")
	mock.Error("error 1")
	mock.Phase(1, 14, "Phase1", "Starting")

	if len(mock.InfoMessages) != 2 {
		t.Errorf("expected 2 info messages, got %d", len(mock.InfoMessages))
	}
	if len(mock.DebugMessages) != 1 {
		t.Errorf("expected 1 debug message, got %d", len(mock.DebugMessages))
	}
	if len(mock.WarnMessages) != 1 {
		t.Errorf("expected 1 warn message, got %d", len(mock.WarnMessages))
	}
	if len(mock.ErrorMessages) != 1 {
		t.Errorf("expected 1 error message, got %d", len(mock.ErrorMessages))
	}
	if len(mock.PhaseMessages) != 1 {
		t.Errorf("expected 1 phase message, got %d", len(mock.PhaseMessages))
	}
}
