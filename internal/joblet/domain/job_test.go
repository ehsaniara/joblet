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

	// test valid transition: INITIALIZING -> RUNNING
	err := job.MarkAsRunning(1234)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if job.Status != StatusRunning {
		t.Errorf("Expected status RUNNING, got %v", job.Status)
	}
	if job.Pid != 1234 {
		t.Errorf("Expected PID 1234, got %v", job.Pid)
	}

	// test invalid transition: RUNNING -> RUNNING
	err = job.MarkAsRunning(5678)
	if err == nil {
		t.Error("Expected error for invalid state transition")
	}

	// test valid transition: RUNNING -> COMPLETED
	job.Complete(0)
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
		expectError    bool
		expectedStatus JobStatus
	}{
		{
			name:           "RUNNING to FAILED",
			initialStatus:  StatusRunning,
			exitCode:       1,
			expectError:    false,
			expectedStatus: StatusFailed,
		},
		{
			name:           "INITIALIZING to FAILED",
			initialStatus:  StatusInitializing,
			exitCode:       -1,
			expectError:    false,
			expectedStatus: StatusFailed,
		},
		{
			name:          "COMPLETED to FAILED (invalid)",
			initialStatus: StatusCompleted,
			exitCode:      1,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{
				Id:     "test-fail",
				Status: tt.initialStatus,
			}

			job.Fail(tt.exitCode)

			if !tt.expectError {
				if job.Status != tt.expectedStatus {
					t.Errorf("Expected status %v, got %v", tt.expectedStatus, job.Status)
				}
				if job.ExitCode != tt.exitCode {
					t.Errorf("Expected exit code %v, got %v", tt.exitCode, job.ExitCode)
				}
				if job.EndTime == nil {
					t.Error("Expected end time to be set")
				}
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

	job.Stop()

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

func TestJobMarkAsRunningValidation(t *testing.T) {
	tests := []struct {
		name        string
		pid         int32
		expectError bool
	}{
		{"Valid PID", 1234, false},
		{"Zero PID", 0, true},
		{"Negative PID", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &Job{
				Id:     "test-validation",
				Status: StatusInitializing,
			}

			err := job.MarkAsRunning(tt.pid)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestJobDeepCopy(t *testing.T) {
	endTime := time.Now()
	original := &Job{
		Id:      "test-1",
		Command: "echo",
		Args:    []string{"hello", "world"},
		Status:  StatusRunning,
		Pid:     1234,
		Limits: ResourceLimits{
			MaxCPU:    100,
			MaxMemory: 512,
			MaxIOBPS:  1000,
		},
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
