package errors

import (
	"errors"
	"fmt"
	"testing"
)

// Test custom error types
func TestJobError(t *testing.T) {
	originalErr := errors.New("process exited with code 1")
	jobErr := &JobError{
		JobUUID:   "job-123",
		Operation: "execute",
		Err:       originalErr,
	}

	expectedMsg := "job job-123: operation execute: process exited with code 1"
	if jobErr.Error() != expectedMsg {
		t.Errorf("JobError.Error() = %v, want %v", jobErr.Error(), expectedMsg)
	}

	// Test Unwrap
	if unwrapped := jobErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("JobError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestRuntimeError(t *testing.T) {
	originalErr := errors.New("runtime not found")
	runtimeErr := &RuntimeError{
		Runtime:   "python:3.11",
		Operation: "install",
		Err:       originalErr,
	}

	expectedMsg := "runtime python:3.11: operation install: runtime not found"
	if runtimeErr.Error() != expectedMsg {
		t.Errorf("RuntimeError.Error() = %v, want %v", runtimeErr.Error(), expectedMsg)
	}

	// Test Unwrap
	if unwrapped := runtimeErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("RuntimeError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestNetworkError(t *testing.T) {
	originalErr := errors.New("IP allocation failed")
	networkErr := &NetworkError{
		Network:   "my-network",
		Operation: "allocate_ip",
		Err:       originalErr,
	}

	expectedMsg := "network my-network: operation allocate_ip: IP allocation failed"
	if networkErr.Error() != expectedMsg {
		t.Errorf("NetworkError.Error() = %v, want %v", networkErr.Error(), expectedMsg)
	}

	// Test Unwrap
	if unwrapped := networkErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("NetworkError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestVolumeError(t *testing.T) {
	originalErr := errors.New("mount failed")
	volumeErr := &VolumeError{
		Volume:    "data-vol",
		Operation: "mount",
		Err:       originalErr,
	}

	expectedMsg := "volume data-vol: operation mount: mount failed"
	if volumeErr.Error() != expectedMsg {
		t.Errorf("VolumeError.Error() = %v, want %v", volumeErr.Error(), expectedMsg)
	}

	// Test Unwrap
	if unwrapped := volumeErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("VolumeError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

// Test sentinel errors
func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrJobNotFound", ErrJobNotFound, "job not found"},
		{"ErrJobAlreadyExists", ErrJobAlreadyExists, "job already exists"},
		{"ErrJobNotRunning", ErrJobNotRunning, "job is not running"},
		{"ErrJobAlreadyRunning", ErrJobAlreadyRunning, "job is already running"},
		{"ErrInvalidJobSpec", ErrInvalidJobSpec, "invalid job specification"},
		{"ErrResourceExhausted", ErrResourceExhausted, "resource exhausted"},
		{"ErrInvalidRuntime", ErrInvalidRuntime, "invalid runtime specification"},
		{"ErrRuntimeNotFound", ErrRuntimeNotFound, "runtime not found"},
		{"ErrNetworkNotFound", ErrNetworkNotFound, "network not found"},
		{"ErrNetworkConflict", ErrNetworkConflict, "network conflict"},
		{"ErrVolumeNotFound", ErrVolumeNotFound, "volume not found"},
		{"ErrVolumeInUse", ErrVolumeInUse, "volume is in use"},
		{"ErrPermissionDenied", ErrPermissionDenied, "permission denied"},
		{"ErrTimeout", ErrTimeout, "operation timed out"},
		{"ErrInvalidConfig", ErrInvalidConfig, "invalid configuration"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.msg {
				t.Errorf("Error message = %v, want %v", tt.err.Error(), tt.msg)
			}
		})
	}
}

// Test error classification
func TestIsJobError(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		isJob bool
	}{
		{"JobError", &JobError{JobUUID: "123", Operation: "start", Err: errors.New("test")}, true},
		{"Wrapped JobError", fmt.Errorf("wrapped: %w", &JobError{JobUUID: "123", Operation: "start", Err: errors.New("test")}), true},
		{"Regular error", errors.New("not a job error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsJobError(tt.err)
			if result != tt.isJob {
				t.Errorf("IsJobError() = %v, want %v", result, tt.isJob)
			}
		})
	}
}

func TestIsRuntimeError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		isRuntime bool
	}{
		{"RuntimeError", &RuntimeError{Runtime: "python:3.11", Operation: "install", Err: errors.New("test")}, true},
		{"Wrapped RuntimeError", fmt.Errorf("wrapped: %w", &RuntimeError{Runtime: "python:3.11", Operation: "install", Err: errors.New("test")}), true},
		{"Regular error", errors.New("not a runtime error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRuntimeError(tt.err)
			if result != tt.isRuntime {
				t.Errorf("IsRuntimeError() = %v, want %v", result, tt.isRuntime)
			}
		})
	}
}

func TestIsResourceError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		isResource bool
	}{
		{"ErrResourceExhausted", ErrResourceExhausted, true},
		{"Wrapped resource error", fmt.Errorf("context: %w", ErrResourceExhausted), true},
		{"Regular error", errors.New("not a resource error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsResourceError(tt.err)
			if result != tt.isResource {
				t.Errorf("IsResourceError() = %v, want %v", result, tt.isResource)
			}
		})
	}
}

func TestIsTimeoutError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		isTimeout bool
	}{
		{"ErrTimeout", ErrTimeout, true},
		{"Wrapped timeout error", fmt.Errorf("operation failed: %w", ErrTimeout), true},
		{"Regular error", errors.New("not a timeout error"), false},
		{"Nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTimeoutError(tt.err)
			if result != tt.isTimeout {
				t.Errorf("IsTimeoutError() = %v, want %v", result, tt.isTimeout)
			}
		})
	}
}

// Test error joining
func TestJoinErrors(t *testing.T) {
	err1 := errors.New("first error")
	err2 := errors.New("second error")
	err3 := errors.New("third error")

	tests := []struct {
		name  string
		errs  []error
		want  string
		isNil bool
	}{
		{
			name:  "No errors",
			errs:  []error{},
			isNil: true,
		},
		{
			name:  "Single error",
			errs:  []error{err1},
			want:  "first error",
			isNil: false,
		},
		{
			name:  "Multiple errors",
			errs:  []error{err1, err2, err3},
			want:  "first error; second error; third error",
			isNil: false,
		},
		{
			name:  "Errors with nils",
			errs:  []error{err1, nil, err2},
			want:  "first error; second error",
			isNil: false,
		},
		{
			name:  "Only nils",
			errs:  []error{nil, nil, nil},
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinErrors(tt.errs...)
			if tt.isNil {
				if result != nil {
					t.Errorf("JoinErrors() = %v, want nil", result)
				}
			} else {
				if result == nil {
					t.Error("JoinErrors() = nil, want non-nil")
				} else if result.Error() != tt.want {
					t.Errorf("JoinErrors() = %v, want %v", result.Error(), tt.want)
				}
			}
		})
	}
}

// Test error wrapping helpers
func TestWrapJobError(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WrapJobError("job-123", "start", originalErr)

	jobErr, ok := wrappedErr.(*JobError)
	if !ok {
		t.Fatalf("WrapJobError() returned %T, want *JobError", wrappedErr)
	}

	if jobErr.JobUUID != "job-123" {
		t.Errorf("JobID = %v, want job-123", jobErr.JobUUID)
	}
	if jobErr.Operation != "start" {
		t.Errorf("Operation = %v, want start", jobErr.Operation)
	}
	if jobErr.Err != originalErr {
		t.Errorf("Err = %v, want %v", jobErr.Err, originalErr)
	}
}

func TestWrapRuntimeError(t *testing.T) {
	originalErr := errors.New("original error")
	wrappedErr := WrapRuntimeError("python:3.11", "install", originalErr)

	runtimeErr, ok := wrappedErr.(*RuntimeError)
	if !ok {
		t.Fatalf("WrapRuntimeError() returned %T, want *RuntimeError", wrappedErr)
	}

	if runtimeErr.Runtime != "python:3.11" {
		t.Errorf("Runtime = %v, want python:3.11", runtimeErr.Runtime)
	}
	if runtimeErr.Operation != "install" {
		t.Errorf("Operation = %v, want install", runtimeErr.Operation)
	}
	if runtimeErr.Err != originalErr {
		t.Errorf("Err = %v, want %v", runtimeErr.Err, originalErr)
	}
}

// Test error cause extraction
func TestGetJobID(t *testing.T) {
	tests := []struct {
		name  string
		err   error
		jobID string
		hasID bool
	}{
		{
			name:  "Direct JobError",
			err:   &JobError{JobUUID: "job-123", Operation: "start", Err: errors.New("test")},
			jobID: "job-123",
			hasID: true,
		},
		{
			name:  "Wrapped JobError",
			err:   fmt.Errorf("context: %w", &JobError{JobUUID: "job-456", Operation: "stop", Err: errors.New("test")}),
			jobID: "job-456",
			hasID: true,
		},
		{
			name:  "Non-JobError",
			err:   errors.New("regular error"),
			jobID: "",
			hasID: false,
		},
		{
			name:  "Nil error",
			err:   nil,
			jobID: "",
			hasID: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobID, hasID := GetJobID(tt.err)
			if jobID != tt.jobID {
				t.Errorf("GetJobID() jobID = %v, want %v", jobID, tt.jobID)
			}
			if hasID != tt.hasID {
				t.Errorf("GetJobID() hasID = %v, want %v", hasID, tt.hasID)
			}
		})
	}
}

// Test error chain operations
func TestErrorChain(t *testing.T) {
	baseErr := errors.New("base error")
	jobErr := WrapJobError("job-123", "start", baseErr)
	wrappedErr := fmt.Errorf("context: %w", jobErr)

	// Test that we can unwrap to the base error
	if !errors.Is(wrappedErr, baseErr) {
		t.Error("errors.Is() should find base error in chain")
	}

	// Test that we can find JobError in chain
	var je *JobError
	if !errors.As(wrappedErr, &je) {
		t.Error("errors.As() should find JobError in chain")
	}
	if je.JobUUID != "job-123" {
		t.Errorf("Found JobError has JobID = %v, want job-123", je.JobUUID)
	}
}

// Benchmark tests
func BenchmarkJobError_Error(b *testing.B) {
	err := &JobError{
		JobUUID:   "job-12345678-1234-1234-1234-123456789012",
		Operation: "execute_command",
		Err:       errors.New("process failed with exit code 1"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = err.Error()
	}
}

func BenchmarkIsJobError(b *testing.B) {
	err := fmt.Errorf("wrapped: %w", &JobError{
		JobUUID:   "job-123",
		Operation: "start",
		Err:       errors.New("test"),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = IsJobError(err)
	}
}

// Tests use the actual implementation from errors.go
