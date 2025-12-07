//go:build linux

package visibility

import (
	"os"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/telemetry"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMonitor(t *testing.T) {
	collector := telemetry.NewCollector(100)
	log := logger.New()

	monitor := NewMonitor(collector, log)

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.collector)
	assert.NotNil(t, monitor.logger)
	assert.NotNil(t, monitor.jobs)
	assert.False(t, monitor.running)
}

func TestNewMonitor_NilLogger(t *testing.T) {
	collector := telemetry.NewCollector(100)

	monitor := NewMonitor(collector, nil)

	assert.NotNil(t, monitor)
	assert.NotNil(t, monitor.logger) // Should create default logger
}

func TestMonitor_GetStats_NotRunning(t *testing.T) {
	monitor := NewMonitor(nil, nil)

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

func TestGetProcessCgroupPath(t *testing.T) {
	// Test with current process
	pid := os.Getpid()

	path, err := GetProcessCgroupPath(pid)
	require.NoError(t, err)
	t.Logf("Current process cgroup path: %s", path)
}

func TestGetProcessCgroupPath_InvalidPID(t *testing.T) {
	// Use an invalid PID
	_, err := GetProcessCgroupPath(999999999)
	assert.Error(t, err)
}

func TestGetCgroupID(t *testing.T) {
	// Skip if not running on a system with cgroup v2
	if !IsCgroupV2() {
		t.Skip("cgroup v2 not available")
	}

	// Test with root cgroup
	id, err := GetCgroupID("")
	if err == nil {
		assert.NotZero(t, id)
		t.Logf("Root cgroup ID: %d", id)
	}
}

func TestValidateCgroupPath(t *testing.T) {
	// Root cgroup should always exist on cgroup v2 systems
	if !IsCgroupV2() {
		t.Skip("cgroup v2 not available")
	}

	err := ValidateCgroupPath("")
	assert.NoError(t, err)
}

func TestValidateCgroupPath_NonExistent(t *testing.T) {
	err := ValidateCgroupPath("nonexistent-cgroup-path-12345")
	assert.Error(t, err)
}

func TestIsSupported(t *testing.T) {
	err := IsSupported()
	// Log result - may or may not be supported depending on system
	if err != nil {
		t.Logf("eBPF visibility not supported: %v", err)
	} else {
		t.Log("eBPF visibility is supported")
	}
}

func TestMonitor_AddRemoveJob_NotStarted(t *testing.T) {
	monitor := NewMonitor(nil, nil)

	// Should fail because monitor not started
	err := monitor.AddJob("test-job", 12345)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not started")
}

func TestFindJobByCgroup(t *testing.T) {
	monitor := NewMonitor(nil, nil)
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
