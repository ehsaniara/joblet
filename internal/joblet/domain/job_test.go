package domain

import (
	"testing"
	"time"
)

func TestJobStateTransitions(t *testing.T) {
	job := &Job{
		Id:      "test-1",
		Command: "echo",
		Args:    []string{"hello"},
		Status:  StatusInitializing,
	}

	// test transition: INITIALIZING -> RUNNING
	job.Status = StatusRunning
	job.Pid = 1234

	if job.Status != StatusRunning {
		t.Errorf("Expected status RUNNING, got %v", job.Status)
	}
	if job.Pid != 1234 {
		t.Errorf("Expected PID 1234, got %v", job.Pid)
	}

	// test transition: RUNNING -> COMPLETED
	job.Status = StatusCompleted
	job.ExitCode = 0
	endTime := time.Now()
	job.EndTime = &endTime

	if job.Status != StatusCompleted {
		t.Errorf("Expected status COMPLETED, got %v", job.Status)
	}
	if job.EndTime == nil {
		t.Error("Expected end time to be set")
	}
	if job.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %v", job.ExitCode)
	}
}

func TestJobFailTransitions(t *testing.T) {
	tests := []struct {
		name           string
		initialStatus  JobStatus
		exitCode       int32
		expectedStatus JobStatus
	}{
		{
			name:           "RUNNING to FAILED",
			initialStatus:  StatusRunning,
			exitCode:       1,
			expectedStatus: StatusFailed,
		},
		{
			name:           "INITIALIZING to FAILED",
			initialStatus:  StatusInitializing,
			exitCode:       -1,
			expectedStatus: StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{
				Id:     "test-fail",
				Status: tt.initialStatus,
			}

			// Simulate failure
			job.Status = StatusFailed
			job.ExitCode = tt.exitCode
			endTime := time.Now()
			job.EndTime = &endTime

			if job.Status != tt.expectedStatus {
				t.Errorf("Expected status %v, got %v", tt.expectedStatus, job.Status)
			}
			if job.ExitCode != tt.exitCode {
				t.Errorf("Expected exit code %v, got %v", tt.exitCode, job.ExitCode)
			}
			if job.EndTime == nil {
				t.Error("Expected end time to be set")
			}
		})
	}
}

func TestJobStopTransition(t *testing.T) {
	job := &Job{
		Id:     "test-stop",
		Status: StatusRunning,
		Pid:    1234,
	}

	// Simulate stop
	job.Status = StatusStopped
	job.ExitCode = -1
	endTime := time.Now()
	job.EndTime = &endTime

	if job.Status != StatusStopped {
		t.Errorf("Expected status STOPPED, got %v", job.Status)
	}
	if job.ExitCode != -1 {
		t.Errorf("Expected exit code -1, got %v", job.ExitCode)
	}
	if job.EndTime == nil {
		t.Error("Expected end time to be set")
	}
}

func TestJobDeepCopy(t *testing.T) {
	endTime := time.Now()

	// Create resource limits using simplified approach
	limits := NewResourceLimitsFromParams(100, "", 512, 1000)

	original := &Job{
		Id:         "test-1",
		Command:    "echo",
		Args:       []string{"hello", "world"},
		Status:     StatusRunning,
		Pid:        1234,
		Limits:     *limits,
		CgroupPath: "/sys/fs/cgroup/job-test-1",
		StartTime:  time.Now(),
		EndTime:    &endTime,
		ExitCode:   0,
	}

	cp := original.DeepCopy()

	// Verify all fields are copied
	if cp.Id != original.Id {
		t.Errorf("ID not copied correctly: expected %v, got %v", original.Id, cp.Id)
	}
	if cp.Command != original.Command {
		t.Errorf("Command not copied correctly")
	}
	if cp.Status != original.Status {
		t.Errorf("Status not copied correctly")
	}
	if cp.Pid != original.Pid {
		t.Errorf("PID not copied correctly")
	}
	if cp.ExitCode != original.ExitCode {
		t.Errorf("ExitCode not copied correctly")
	}

	// Test slice independence
	original.Args[0] = "goodbye"
	if cp.Args[0] != "hello" {
		t.Error("Deep copy failed: args slice was not properly copied")
	}

	// Test status independence
	original.Status = StatusCompleted
	if cp.Status != StatusRunning {
		t.Error("Deep copy failed: status was not properly copied")
	}

	// Test time pointer independence
	if original.EndTime == cp.EndTime {
		t.Error("EndTime should be different pointers")
	}
	if cp.EndTime == nil {
		t.Error("EndTime should not be nil")
	}
	if !cp.EndTime.Equal(*original.EndTime) {
		t.Error("EndTime values should be equal")
	}
}

func TestJobIsRunning(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected bool
	}{
		{StatusInitializing, false},
		{StatusRunning, true},
		{StatusCompleted, false},
		{StatusFailed, false},
		{StatusStopped, false},
	}

	for _, tt := range tests {
		job := &Job{Status: tt.status}
		if job.IsRunning() != tt.expected {
			t.Errorf("IsRunning() for status %v: expected %v, got %v",
				tt.status, tt.expected, job.IsRunning())
		}
	}
}

func TestJobIsCompleted(t *testing.T) {
	tests := []struct {
		status   JobStatus
		expected bool
	}{
		{StatusInitializing, false},
		{StatusRunning, false},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusStopped, true},
	}

	for _, tt := range tests {
		job := &Job{Status: tt.status}
		if job.IsCompleted() != tt.expected {
			t.Errorf("IsCompleted() for status %v: expected %v, got %v",
				tt.status, tt.expected, job.IsCompleted())
		}
	}
}
