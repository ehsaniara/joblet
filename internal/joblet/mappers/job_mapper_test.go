package mappers

import (
	"joblet/internal/joblet/domain"
	"testing"
	"time"
)

func TestDomainToProtobuf(t *testing.T) {
	startTime := time.Now()
	endTime := startTime.Add(5 * time.Second)

	// Create resource limits using simplified approach
	limits := domain.NewResourceLimitsFromParams(100, "", 512, 1000)

	job := &domain.Job{
		Id:        "test-job-1",
		Command:   "echo",
		Args:      []string{"hello", "world"},
		Limits:    *limits,
		Status:    domain.StatusCompleted,
		StartTime: startTime,
		EndTime:   &endTime,
		ExitCode:  0,
	}

	mapper := NewJobMapper()
	pbJob := mapper.DomainToProtobuf(job)

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
	if pbJob.MaxCPU != job.Limits.CPU.Value() {
		t.Errorf("Expected MaxCPU %v, got %v", job.Limits.CPU.Value(), pbJob.MaxCPU)
	}
	if pbJob.MaxMemory != job.Limits.Memory.Megabytes() {
		t.Errorf("Expected MaxMemory %v, got %v", job.Limits.Memory.Megabytes(), pbJob.MaxMemory)
	}
	if pbJob.MaxIOBPS != int32(job.Limits.IOBandwidth.BytesPerSecond()) {
		t.Errorf("Expected MaxIOBPS %v, got %v", job.Limits.IOBandwidth.BytesPerSecond(), pbJob.MaxIOBPS)
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
	limits := domain.NewResourceLimits()

	job := &domain.Job{
		Id:        "running-job",
		Command:   "sleep",
		Args:      []string{"60"},
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
		EndTime:   nil, // Running job has no end time
		Limits:    *limits,
	}

	mapper := NewJobMapper()
	pbJob := mapper.DomainToProtobuf(job)

	if pbJob.EndTime != "" {
		t.Errorf("Expected empty end time for running job, got %v", pbJob.EndTime)
	}
}

func TestDomainToProtobuf_EmptyArgs(t *testing.T) {
	limits := domain.NewResourceLimits()

	job := &domain.Job{
		Id:        "no-args-job",
		Command:   "pwd",
		Args:      []string{}, // Empty args
		Status:    domain.StatusCompleted,
		StartTime: time.Now(),
		Limits:    *limits,
	}

	mapper := NewJobMapper()
	pbJob := mapper.DomainToProtobuf(job)

	if len(pbJob.Args) != 0 {
		t.Errorf("Expected empty args, got %v", pbJob.Args)
	}
}

func TestDomainToRunJobResponse(t *testing.T) {
	limits := domain.NewResourceLimitsFromParams(50, "", 256, 500)

	job := &domain.Job{
		Id:        "run-job-test",
		Command:   "echo",
		Args:      []string{"test"},
		Limits:    *limits,
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
		ExitCode:  0,
	}

	mapper := NewJobMapper()
	response := mapper.DomainToRunJobResponse(job)

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

// Test all status values mapping correctly
func TestStatusMapping(t *testing.T) {
	statuses := []domain.JobStatus{
		domain.StatusInitializing,
		domain.StatusRunning,
		domain.StatusCompleted,
		domain.StatusFailed,
		domain.StatusStopped,
	}

	mapper := NewJobMapper()
	limits := domain.NewResourceLimits()

	for _, status := range statuses {
		job := &domain.Job{
			Id:        "status-test",
			Command:   "echo",
			Status:    status,
			StartTime: time.Now(),
			Limits:    *limits,
		}

		// Test mapper function
		pbJob := mapper.DomainToProtobuf(job)
		runJobRes := mapper.DomainToRunJobResponse(job)

		expectedStatus := string(status)

		if pbJob.Status != expectedStatus {
			t.Errorf("DomainToProtobuf: Expected status %v, got %v", expectedStatus, pbJob.Status)
		}
		if runJobRes.Status != expectedStatus {
			t.Errorf("DomainToRunJobResponse: Expected status %v, got %v", expectedStatus, runJobRes.Status)
		}
	}
}

// Test edge cases with resource limits
func TestResourceLimitsMapping(t *testing.T) {
	tests := []struct {
		name   string
		cpu    int32
		memory int32
		iobps  int64
	}{
		{
			name:   "zero limits",
			cpu:    0,
			memory: 0,
			iobps:  0,
		},
		{
			name:   "default limits",
			cpu:    50,
			memory: 256,
			iobps:  1000000,
		},
		{
			name:   "high limits",
			cpu:    1000,
			memory: 32768,
			iobps:  10000000,
		},
	}

	mapper := NewJobMapper()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limits := domain.NewResourceLimitsFromParams(tt.cpu, "", tt.memory, tt.iobps)

			job := &domain.Job{
				Id:        "limits-test",
				Command:   "echo",
				Limits:    *limits,
				Status:    domain.StatusRunning,
				StartTime: time.Now(),
			}

			pbJob := mapper.DomainToProtobuf(job)

			actualCPU := job.Limits.CPU.Value()
			actualMemory := job.Limits.Memory.Megabytes()
			actualIOBPS := job.Limits.IOBandwidth.BytesPerSecond()

			if pbJob.MaxCPU != actualCPU {
				t.Errorf("Expected MaxCPU %v, got %v", actualCPU, pbJob.MaxCPU)
			}
			if pbJob.MaxMemory != actualMemory {
				t.Errorf("Expected MaxMemory %v, got %v", actualMemory, pbJob.MaxMemory)
			}
			if pbJob.MaxIOBPS != int32(actualIOBPS) {
				t.Errorf("Expected MaxIOBPS %v, got %v", actualIOBPS, pbJob.MaxIOBPS)
			}
		})
	}
}

// Test time formatting edge cases
func TestTimeFormatting(t *testing.T) {
	// Test with timezone
	location, _ := time.LoadLocation("America/New_York")
	timeInTZ := time.Date(2023, 12, 25, 15, 30, 45, 123456789, location)

	limits := domain.NewResourceLimits()

	job := &domain.Job{
		Id:        "time-test",
		Command:   "echo",
		Status:    domain.StatusCompleted,
		StartTime: timeInTZ,
		EndTime:   &timeInTZ,
		Limits:    *limits,
	}

	mapper := NewJobMapper()
	pbJob := mapper.DomainToProtobuf(job)

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

	mapper := NewJobMapper()
	limits := domain.NewResourceLimits()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := &domain.Job{
				Id:        "args-test",
				Command:   "echo",
				Args:      tt.args,
				Status:    domain.StatusRunning,
				StartTime: time.Now(),
				Limits:    *limits,
			}

			pbJob := mapper.DomainToProtobuf(job)

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
	limits := domain.NewResourceLimitsFromParams(100, "", 512, 1000)

	job := &domain.Job{
		Id:        "benchmark-job",
		Command:   "echo",
		Args:      []string{"hello", "world", "from", "benchmark"},
		Limits:    *limits,
		Status:    domain.StatusCompleted,
		StartTime: time.Now(),
		ExitCode:  0,
	}

	mapper := NewJobMapper()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper.DomainToProtobuf(job)
	}
}

func BenchmarkDomainToRunJobResponse(b *testing.B) {
	limits := domain.NewResourceLimits()

	job := &domain.Job{
		Id:        "benchmark-run-job",
		Command:   "echo",
		Args:      []string{"test"},
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
		Limits:    *limits,
	}

	mapper := NewJobMapper()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper.DomainToRunJobResponse(job)
	}
}
