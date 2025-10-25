package jobs

import (
	"strings"
	"testing"
	"time"
)

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		timestamp string
		expected  string
	}{
		{
			name:      "valid timestamp",
			timestamp: "2025-09-02T03:46:21Z",
			expected:  "2025-09-02 03:46:21 UTC",
		},
		{
			name:      "empty timestamp",
			timestamp: "",
			expected:  "Not available",
		},
		{
			name:      "invalid timestamp",
			timestamp: "invalid-timestamp",
			expected:  "invalid-timestamp", // Should return as-is if parsing fails
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatTimestamp(tt.timestamp)
			if result != tt.expected {
				t.Errorf("formatTimestamp(%s) = %s, want %s", tt.timestamp, result, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "seconds only",
			duration: 30 * time.Second,
			expected: "30.0s",
		},
		{
			name:     "minutes only",
			duration: 5 * time.Minute,
			expected: "5.0m",
		},
		{
			name:     "hours only",
			duration: 2 * time.Hour,
			expected: "2h",
		},
		{
			name:     "hours and minutes",
			duration: 2*time.Hour + 30*time.Minute,
			expected: "2h30m",
		},
		{
			name:     "days only",
			duration: 3 * 24 * time.Hour,
			expected: "3d",
		},
		{
			name:     "days and hours",
			duration: 2*24*time.Hour + 5*time.Hour,
			expected: "2d5h",
		},
		{
			name:     "less than a second",
			duration: 500 * time.Millisecond,
			expected: "0.5s",
		},
		{
			name:     "exactly one minute",
			duration: time.Minute,
			expected: "1.0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.expected {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestGetStatusColor(t *testing.T) {
	// Test that getStatusColor function exists and returns consistent values
	// This is a basic test since we can't test actual terminal colors easily

	tests := []struct {
		name   string
		status string
	}{
		{"running status", "RUNNING"},
		{"completed status", "COMPLETED"},
		{"failed status", "FAILED"},
		{"pending status", "PENDING"},
		{"scheduled status", "SCHEDULED"},
		{"stopped status", "STOPPED"},
		{"unknown status", "UNKNOWN_STATUS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the function doesn't panic
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("getStatusColor(%s) panicked: %v", tt.status, r)
				}
			}()

			statusColor, resetColor := getStatusColor(tt.status)

			// Basic validation - colors should be strings (even if empty)
			// We don't assert specific values since colors may be disabled in test environments

			// Test that we get consistent results
			statusColor2, resetColor2 := getStatusColor(tt.status)
			if statusColor != statusColor2 || resetColor != resetColor2 {
				t.Errorf("getStatusColor(%s) returned inconsistent results", tt.status)
			}
		})
	}
}

func TestStatusCommandValidUUIDs(t *testing.T) {
	// Test UUID validation and processing
	tests := []struct {
		name    string
		uuid    string
		isValid bool
	}{
		{
			name:    "full valid UUID",
			uuid:    "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			isValid: true,
		},
		{
			name:    "short UUID",
			uuid:    "f47ac10b",
			isValid: true,
		},
		{
			name:    "very short UUID",
			uuid:    "f47a",
			isValid: true, // Short UUIDs are supported
		},
		{
			name:    "empty UUID",
			uuid:    "",
			isValid: false, // cobra.ExactArgs(1) should reject empty args
		},
		{
			name:    "invalid characters",
			uuid:    "invalid-uuid-with-special-chars!",
			isValid: true, // Let server handle validation
		},
	}

	cmd := NewStatusCmd()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args []string
			if tt.uuid == "" {
				args = []string{} // No arguments for empty UUID test
			} else {
				args = []string{tt.uuid}
			}

			err := cmd.Args(cmd, args)

			if tt.isValid && err != nil {
				t.Errorf("Expected UUID %s to be valid, but got error: %v", tt.uuid, err)
			}
			if !tt.isValid && err == nil {
				t.Errorf("Expected UUID %s to be invalid, but got no error", tt.uuid)
			}
		})
	}
}

func TestStatusCommandFlagCombinations(t *testing.T) {
	cmd := NewStatusCmd()

	// Verify that workflow and detail flags are removed (now use: rnx workflow status)
	workflowFlag := cmd.Flags().Lookup("workflow")
	detailFlag := cmd.Flags().Lookup("detail")

	if workflowFlag != nil {
		t.Fatal("workflow flag should not exist (removed - use rnx workflow status)")
	}
	if detailFlag != nil {
		t.Fatal("detail flag should not exist (removed - use rnx workflow status --detail)")
	}

	// No flag shortcuts to test anymore - flags were removed in favor of dedicated workflow commands
}

func TestStatusDisplaySections(t *testing.T) {
	// Test that all expected status display sections are documented in help
	cmd := NewStatusCmd()
	helpContent := cmd.Long

	expectedSections := []string{
		"Basic Info:",
		"Timing:",
		"Resource Limits:",
		"Runtime Environment:",
		"Network:",
		"Storage:",
		"Working Directory:",
		"Uploaded Files:",
		"Environment:",
		"Secrets:",
		"Workflow Context:",
		"Results:",
		"Actions:",
	}

	for _, section := range expectedSections {
		if !strings.Contains(helpContent, section) {
			t.Errorf("Help content should document section: '%s'", section)
		}
	}
}

func BenchmarkFormatDuration(b *testing.B) {
	duration := 2*time.Hour + 30*time.Minute + 45*time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatDuration(duration)
	}
}

func BenchmarkFormatTimestamp(b *testing.B) {
	timestamp := "2025-09-02T03:46:21Z"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = formatTimestamp(timestamp)
	}
}
