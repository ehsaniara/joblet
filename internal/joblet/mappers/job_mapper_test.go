package mappers

import (
	"joblet/internal/joblet/domain"
	"testing"
	"time"
)

func TestDomainToProtobuf(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Second)

	job := &domain.Job{
		Id:      "test-job-1",
		Command: "echo",
		Args:    []string{"hello", "world"},
		Limits: domain.ResourceLimits{
			MaxCPU:    100,
			MaxMemory: 512,
			MaxIOBPS:  1000,
		},
		Status:    domain.StatusCompleted,
		StartTime: startTime,
		EndTime:   &endTime,
		ExitCode:  0,
	}

	pbJob := DomainToProtobuf(job)

	// Verify all fields are mapped correctly
	if pbJob.Id != job.Id {
		t.Errorf("Expected ID %v, got %v", job.Id, pbJob.Id)
	}
	if pbJob.Command != job.Command {
		t.Errorf("Expected command %v, got %v", job.Command, pbJob.Command)
	}
	if len(pbJob.Args) != len(job.Args) {
		t.Errorf("Expected %v args, got %v", len(job.Args), len(pbJob.Args))
	}
	for i, arg := range job.Args {
		if pbJob.Args[i] != arg {
			t.Errorf("Expected arg[%d] %v, got %v", i, arg, pbJob.Args[i])
		}
	}
	if pbJob.MaxCPU != job.Limits.MaxCPU {
		t.Errorf("Expected MaxCPU %v, got %v", job.Limits.MaxCPU, pbJob.MaxCPU)
	}
	if pbJob.MaxMemory != job.Limits.MaxMemory {
		t.Errorf("Expected MaxMemory %v, got %v", job.Limits.MaxMemory, pbJob.MaxMemory)
	}
	if pbJob.MaxIOBPS != job.Limits.MaxIOBPS {
		t.Errorf("Expected MaxIOBPS %v, got %v", job.Limits.MaxIOBPS, pbJob.MaxIOBPS)
	}
	if pbJob.Status != string(job.Status) {
		t.Errorf("Expected status %v, got %v", string(job.Status), pbJob.Status)
	}
	if pbJob.ExitCode != job.ExitCode {
		t.Errorf("Expected exit code %v, got %v", job.ExitCode, pbJob.ExitCode)
	}

	// Verify time formatting
	expectedStartTime := startTime.Format("2006-01-02T15:04:05Z07:00")
	if pbJob.StartTime != expectedStartTime {
		t.Errorf("Expected start time %v, got %v", expectedStartTime, pbJob.StartTime)
	}

	expectedEndTime := endTime.Format("2006-01-02T15:04:05Z07:00")
	if pbJob.EndTime != expectedEndTime {
		t.Errorf("Expected end time %v, got %v", expectedEndTime, pbJob.EndTime)
	}
}

func TestDomainToProtobuf_NoEndTime(t *testing.T) {
	job := &domain.Job{
		Id:        "running-job",
		Command:   "sleep",
		Args:      []string{"60"},
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
		EndTime:   nil, // Running job has no end time
	}

	pbJob := DomainToProtobuf(job)

	if pbJob.EndTime != "" {
		t.Errorf("Expected empty end time for running job, got %v", pbJob.EndTime)
	}
}

func TestDomainToProtobuf_EmptyArgs(t *testing.T) {
	job := &domain.Job{
		Id:        "no-args-job",
		Command:   "pwd",
		Args:      []string{}, // Empty args
		Status:    domain.StatusCompleted,
		StartTime: time.Now(),
	}

	pbJob := DomainToProtobuf(job)

	if len(pbJob.Args) != 0 {
		t.Errorf("Expected empty args, got %v", pbJob.Args)
	}
}

func TestDomainToRunJobResponse(t *testing.T) {
	job := &domain.Job{
		Id:      "run-job-test",
		Command: "echo",
		Args:    []string{"test"},
		Limits: domain.ResourceLimits{
			MaxCPU:    50,
			MaxMemory: 256,
			MaxIOBPS:  500,
		},
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
		ExitCode:  0,
	}

	response := DomainToRunJobResponse(job)

	// Verify it's a proper RunJobRes
	if response.Id != job.Id {
		t.Errorf("Expected ID %v, got %v", job.Id, response.Id)
	}
	if response.Command != job.Command {
		t.Errorf("Expected command %v, got %v", job.Command, response.Command)
	}
	if response.Status != string(job.Status) {
		t.Errorf("Expected status %v, got %v", string(job.Status), response.Status)
	}
}

func TestDomainToGetJobStatusResponse(t *testing.T) {
	endTime := time.Now()
	job := &domain.Job{
		Id:        "status-job-test",
		Command:   "ls",
		Args:      []string{"-la"},
		Status:    domain.StatusCompleted,
		StartTime: time.Now().Add(-1 * time.Minute),
		EndTime:   &endTime,
		ExitCode:  0,
	}

	response := DomainToGetJobStatusResponse(job)

	// Verify it's a proper GetJobStatusRes
	if response.Id != job.Id {
		t.Errorf("Expected ID %v, got %v", job.Id, response.Id)
	}
	if response.Status != string(job.Status) {
		t.Errorf("Expected status %v, got %v", string(job.Status), response.Status)
	}
	if response.ExitCode != job.ExitCode {
		t.Errorf("Expected exit code %v, got %v", job.ExitCode, response.ExitCode)
	}
	if response.EndTime == "" {
		t.Error("Expected end time to be set for completed job")
	}
}

func TestDomainToStopJobResponse(t *testing.T) {
	endTime := time.Now()
	job := &domain.Job{
		Id:       "stop-job-test",
		Status:   domain.StatusStopped,
		EndTime:  &endTime,
		ExitCode: -1,
	}

	response := DomainToStopJobResponse(job)

	// Verify it's a proper StopJobRes
	if response.Id != job.Id {
		t.Errorf("Expected ID %v, got %v", job.Id, response.Id)
	}
	if response.Status != string(job.Status) {
		t.Errorf("Expected status %v, got %v", string(job.Status), response.Status)
	}
	if response.ExitCode != job.ExitCode {
		t.Errorf("Expected exit code %v, got %v", job.ExitCode, response.ExitCode)
	}
	if response.EndTime == "" {
		t.Error("Expected end time to be set for stopped job")
	}
}

func TestDomainToStopJobResponse_NoEndTime(t *testing.T) {
	job := &domain.Job{
		Id:       "stop-job-no-end",
		Status:   domain.StatusStopped,
		EndTime:  nil, // No end time set
		ExitCode: -1,
	}

	response := DomainToStopJobResponse(job)

	if response.EndTime != "" {
		t.Errorf("Expected empty end time, got %v", response.EndTime)
	}
}

// Test all status values mapping correctly
func TestStatusMapping(t *testing.T) {
	statuses := []domain.JobStatus{
		domain.StatusInitializing,
		domain.StatusRunning,
		domain.StatusCompleted,
		domain.StatusFailed,
		domain.StatusStopped,
	}

	for _, status := range statuses {
		job := &domain.Job{
			Id:        "status-test",
			Command:   "echo",
			Status:    status,
			StartTime: time.Now(),
		}

		// Test all mapper functions
		pbJob := DomainToProtobuf(job)
		runJobRes := DomainToRunJobResponse(job)
		statusRes := DomainToGetJobStatusResponse(job)
		stopRes := DomainToStopJobResponse(job)

		expectedStatus := string(status)

		if pbJob.Status != expectedStatus {
			t.Errorf("DomainToProtobuf: Expected status %v, got %v", expectedStatus, pbJob.Status)
		}
		if runJobRes.Status != expectedStatus {
			t.Errorf("DomainToRunJobResponse: Expected status %v, got %v", expectedStatus, runJobRes.Status)
		}
		if statusRes.Status != expectedStatus {
			t.Errorf("DomainToGetJobStatusResponse: Expected status %v, got %v", expectedStatus, statusRes.Status)
		}
		if stopRes.Status != expectedStatus {
			t.Errorf("DomainToStopJobResponse: Expected status %v, got %v", expectedStatus, stopRes.Status)
		}
	}
}

// Test edge cases with resource limits
func TestResourceLimitsMapping(t *testing.T) {
	tests := []struct {
		name   string
		limits domain.ResourceLimits
	}{
		{
			name: "zero limits",
			limits: domain.ResourceLimits{
				MaxCPU:    0,
				MaxMemory: 0,
				MaxIOBPS:  0,
			},
		},
		{
			name: "negative limits",
			limits: domain.ResourceLimits{
				MaxCPU:    -1,
				MaxMemory: -1,
				MaxIOBPS:  -1,
			},
		},
		{
			name: "max values",
			limits: domain.ResourceLimits{
				MaxCPU:    2147483647, // int32 max
				MaxMemory: 2147483647,
				MaxIOBPS:  2147483647,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{
				Id:        "limits-test",
				Command:   "echo",
				Limits:    tt.limits,
				Status:    domain.StatusRunning,
				StartTime: time.Now(),
			}

			pbJob := DomainToProtobuf(job)

			if pbJob.MaxCPU != tt.limits.MaxCPU {
				t.Errorf("Expected MaxCPU %v, got %v", tt.limits.MaxCPU, pbJob.MaxCPU)
			}
			if pbJob.MaxMemory != tt.limits.MaxMemory {
				t.Errorf("Expected MaxMemory %v, got %v", tt.limits.MaxMemory, pbJob.MaxMemory)
			}
			if pbJob.MaxIOBPS != tt.limits.MaxIOBPS {
				t.Errorf("Expected MaxIOBPS %v, got %v", tt.limits.MaxIOBPS, pbJob.MaxIOBPS)
			}
		})
	}
}

// Test time formatting edge cases
func TestTimeFormatting(t *testing.T) {
	// Test with timezone
	location, _ := time.LoadLocation("America/New_York")
	timeInTZ := time.Date(2023, 12, 25, 15, 30, 45, 123456789, location)

	job := &domain.Job{
		Id:        "time-test",
		Command:   "echo",
		Status:    domain.StatusCompleted,
		StartTime: timeInTZ,
		EndTime:   &timeInTZ,
	}

	pbJob := DomainToProtobuf(job)

	// Verify the time format includes timezone
	expectedFormat := timeInTZ.Format("2006-01-02T15:04:05Z07:00")
	if pbJob.StartTime != expectedFormat {
		t.Errorf("Expected start time %v, got %v", expectedFormat, pbJob.StartTime)
	}
	if pbJob.EndTime != expectedFormat {
		t.Errorf("Expected end time %v, got %v", expectedFormat, pbJob.EndTime)
	}
}

// Test args slice handling
func TestArgsSliceHandling(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "single arg",
			args: []string{"hello"},
		},
		{
			name: "multiple args",
			args: []string{"echo", "-n", "hello world"},
		},
		{
			name: "args with special characters",
			args: []string{"echo", "hello\nworld", "test\ttab", "quote\"test"},
		},
		{
			name: "empty string args",
			args: []string{"", "not-empty", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{
				Id:        "args-test",
				Command:   "echo",
				Args:      tt.args,
				Status:    domain.StatusRunning,
				StartTime: time.Now(),
			}

			pbJob := DomainToProtobuf(job)

			if len(pbJob.Args) != len(tt.args) {
				t.Errorf("Expected %d args, got %d", len(tt.args), len(pbJob.Args))
			}

			for i, expectedArg := range tt.args {
				if i < len(pbJob.Args) && pbJob.Args[i] != expectedArg {
					t.Errorf("Expected arg[%d] = %q, got %q", i, expectedArg, pbJob.Args[i])
				}
			}
		})
	}
}

// Benchmark tests
func BenchmarkDomainToProtobuf(b *testing.B) {
	job := &domain.Job{
		Id:      "benchmark-job",
		Command: "echo",
		Args:    []string{"hello", "world", "from", "benchmark"},
		Limits: domain.ResourceLimits{
			MaxCPU:    100,
			MaxMemory: 512,
			MaxIOBPS:  1000,
		},
		Status:    domain.StatusCompleted,
		StartTime: time.Now(),
		ExitCode:  0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DomainToProtobuf(job)
	}
}

func BenchmarkDomainToRunJobResponse(b *testing.B) {
	job := &domain.Job{
		Id:        "benchmark-run-job",
		Command:   "echo",
		Args:      []string{"test"},
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DomainToRunJobResponse(job)
	}
}
