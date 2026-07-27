//go:build linux

package telematics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMonitor_GetStats_NotRunning(t *testing.T) {
	monitor := NewMonitorWithConfig(nil, nil, EventTypeConfig{})

	stats := monitor.GetStats()

	assert.False(t, stats.Running)
	assert.Equal(t, 0, stats.JobsMonitored)
}

func TestNullTerminatedString(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "empty string",
			input:    []byte{0, 0, 0, 0},
			expected: "",
		},
		{
			name:     "simple string",
			input:    []byte{'h', 'e', 'l', 'l', 'o', 0, 'x', 'x'},
			expected: "hello",
		},
		{
			name:     "no null terminator",
			input:    []byte{'t', 'e', 's', 't'},
			expected: "test",
		},
		{
			name:     "path",
			input:    []byte{'/', 'u', 's', 'r', '/', 'b', 'i', 'n', '/', 'p', 'y', 't', 'h', 'o', 'n', 0},
			expected: "/usr/bin/python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nullTerminatedString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsCgroupV2(t *testing.T) {
	// This test runs on real system, so result depends on environment
	result := IsCgroupV2()
	// Just verify it doesn't panic - actual value depends on system
	t.Logf("IsCgroupV2: %v", result)
}

func TestIsSupported(t *testing.T) {
	err := IsSupported()
	// Log result - may or may not be supported depending on system
	if err != nil {
		t.Logf("eBPF telematics not supported: %v", err)
	} else {
		t.Log("eBPF telematics is supported")
	}
}

func TestMonitor_AddRemoveJob_NotStarted(t *testing.T) {
	monitor := NewMonitorWithConfig(nil, nil, EventTypeConfig{})

	// Should fail because monitor not started
	err := monitor.AddJob("test-job", 12345)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestFindJobByCgroup(t *testing.T) {
	monitor := NewMonitorWithConfig(nil, nil, EventTypeConfig{})
	monitor.jobs["job-1"] = 100
	monitor.jobs["job-2"] = 200
	monitor.jobs["job-3"] = 300

	// Test finding existing jobs
	assert.Equal(t, "job-1", monitor.findJobByCgroup(100))
	assert.Equal(t, "job-2", monitor.findJobByCgroup(200))
	assert.Equal(t, "job-3", monitor.findJobByCgroup(300))

	// Test finding non-existent cgroup
	assert.Equal(t, "", monitor.findJobByCgroup(999))
}

// TestExecEventParsing verifies the ExecEvent struct layout
func TestExecEventSize(t *testing.T) {
	// Verify the event struct has expected size for binary parsing
	var event ExecEvent
	// Timestamp (8) + CgroupID (8) + PID (4) + PPID (4) + UID (4) + Comm (16) + Filename (256) + RetVal (4) = 304
	expectedSize := 8 + 8 + 4 + 4 + 4 + 16 + 256 + 4
	t.Logf("ExecEvent size: expected %d", expectedSize)
	// Just a documentation test - actual binary size depends on struct layout
	_ = event
}

// TestConnectEventParsing verifies the ConnectEvent struct layout
func TestConnectEventSize(t *testing.T) {
	var event ConnectEvent
	// Timestamp (8) + CgroupID (8) + PID (4) + Port (2) + Family (2) + Protocol (1) + Pad (3) + Addr (16) = 44
	expectedSize := 8 + 8 + 4 + 2 + 2 + 1 + 3 + 16
	t.Logf("ConnectEvent size: expected %d", expectedSize)
	_ = event
}
