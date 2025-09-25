package execution_test

import (
	"context"
	"errors"
	"testing"

	"joblet/internal/joblet/core/execution"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/gpu"
	"joblet/internal/joblet/gpu/gpufakes"
	"joblet/pkg/logger"
)

func TestGPUService_AllocateGPU_DisabledGPU(t *testing.T) {
	// Setup fake GPU manager
	fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
	fakeGPUManager.IsEnabledReturns(false)

	// Create GPU service
	service := execution.NewGPUService(fakeGPUManager, logger.New())

	// Create job with GPU requirement
	job := &domain.Job{
		Uuid:        "test-job",
		GPUCount:    1,
		GPUMemoryMB: 4096,
	}

	// Try to allocate GPU
	allocation, err := service.AllocateGPU(context.Background(), job)

	// Should return nil (no allocation) when GPU disabled
	if err != nil {
		t.Errorf("Expected no error for disabled GPU, got: %v", err)
	}
	if allocation != nil {
		t.Error("Expected nil allocation when GPU disabled")
	}

	// Verify IsEnabled was called
	if fakeGPUManager.IsEnabledCallCount() != 1 {
		t.Error("Expected IsEnabled to be called once")
	}
}

func TestGPUService_AllocateGPU_Success(t *testing.T) {
	// Setup fake GPU manager
	fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
	fakeGPUManager.IsEnabledReturns(true)

	// Setup successful allocation
	expectedAllocation := &gpu.GPUAllocation{
		JobID:       "test-job",
		GPUIndices:  []int{0, 1},
		GPUMemoryMB: 8192,
	}
	fakeGPUManager.AllocateGPUsReturns(expectedAllocation, nil)

	// Create GPU service
	service := execution.NewGPUService(fakeGPUManager, logger.New())

	// Create job with GPU requirement
	job := &domain.Job{
		Uuid:        "test-job",
		GPUCount:    2,
		GPUMemoryMB: 8192,
	}

	// Allocate GPU
	allocation, err := service.AllocateGPU(context.Background(), job)

	// Should succeed
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if allocation == nil {
		t.Fatal("Expected allocation, got nil")
	}
	if allocation.JobID != "test-job" {
		t.Errorf("Expected job ID 'test-job', got '%s'", allocation.JobID)
	}
	if len(allocation.GPUIndices) != 2 {
		t.Errorf("Expected 2 GPU indices, got %d", len(allocation.GPUIndices))
	}

	// Verify job was updated with GPU indices
	if len(job.GPUIndices) != 2 {
		t.Errorf("Expected job to have 2 GPU indices, got %d", len(job.GPUIndices))
	}
	if job.GPUIndices[0] != 0 || job.GPUIndices[1] != 1 {
		t.Errorf("Job GPU indices not updated correctly: %v", job.GPUIndices)
	}

	// Verify AllocateGPUs was called with correct parameters
	if fakeGPUManager.AllocateGPUsCallCount() != 1 {
		t.Error("Expected AllocateGPUs to be called once")
	}
	jobID, gpuCount, memoryMB := fakeGPUManager.AllocateGPUsArgsForCall(0)
	if jobID != "test-job" {
		t.Errorf("AllocateGPUs called with wrong job ID: %s", jobID)
	}
	if gpuCount != 2 {
		t.Errorf("AllocateGPUs called with wrong GPU count: %d", gpuCount)
	}
	if memoryMB != 8192 {
		t.Errorf("AllocateGPUs called with wrong memory: %d", memoryMB)
	}
}

func TestGPUService_AllocateGPU_NoGPURequired(t *testing.T) {
	// Setup fake GPU manager
	fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
	fakeGPUManager.IsEnabledReturns(true)

	// Create GPU service
	service := execution.NewGPUService(fakeGPUManager, logger.New())

	// Create job without GPU requirement
	job := &domain.Job{
		Uuid:     "test-job",
		GPUCount: 0, // No GPU required
	}

	// Try to allocate GPU
	allocation, err := service.AllocateGPU(context.Background(), job)

	// Should return nil (no allocation needed)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if allocation != nil {
		t.Error("Expected nil allocation when no GPU required")
	}

	// AllocateGPUs should not be called
	if fakeGPUManager.AllocateGPUsCallCount() != 0 {
		t.Error("AllocateGPUs should not be called when no GPU required")
	}
}

func TestGPUService_AllocateGPU_AllocationFailure(t *testing.T) {
	// Setup fake GPU manager
	fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
	fakeGPUManager.IsEnabledReturns(true)

	// Setup allocation failure
	expectedError := errors.New("no GPUs available")
	fakeGPUManager.AllocateGPUsReturns(nil, expectedError)

	// Create GPU service
	service := execution.NewGPUService(fakeGPUManager, logger.New())

	// Create job with GPU requirement
	job := &domain.Job{
		Uuid:        "test-job",
		GPUCount:    1,
		GPUMemoryMB: 4096,
	}

	// Try to allocate GPU
	allocation, err := service.AllocateGPU(context.Background(), job)

	// Should return error
	if err == nil {
		t.Error("Expected error for allocation failure")
	}
	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
	if allocation != nil {
		t.Error("Expected nil allocation on failure")
	}
}

func TestGPUService_ReleaseGPU_Success(t *testing.T) {
	// Setup fake GPU manager
	fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
	fakeGPUManager.IsEnabledReturns(true)
	fakeGPUManager.ReleaseGPUsReturns(nil)

	// Create GPU service
	service := execution.NewGPUService(fakeGPUManager, logger.New())

	// Release GPU
	err := service.ReleaseGPU(context.Background(), "test-job")

	// Should succeed
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Verify ReleaseGPUs was called
	if fakeGPUManager.ReleaseGPUsCallCount() != 1 {
		t.Error("Expected ReleaseGPUs to be called once")
	}
	jobID := fakeGPUManager.ReleaseGPUsArgsForCall(0)
	if jobID != "test-job" {
		t.Errorf("ReleaseGPUs called with wrong job ID: %s", jobID)
	}
}

func TestGPUService_ReleaseGPU_Disabled(t *testing.T) {
	// Setup fake GPU manager
	fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
	fakeGPUManager.IsEnabledReturns(false)

	// Create GPU service
	service := execution.NewGPUService(fakeGPUManager, logger.New())

	// Release GPU
	err := service.ReleaseGPU(context.Background(), "test-job")

	// Should succeed (no-op when disabled)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// ReleaseGPUs should not be called
	if fakeGPUManager.ReleaseGPUsCallCount() != 0 {
		t.Error("ReleaseGPUs should not be called when GPU disabled")
	}
}

func TestGPUService_ReleaseGPU_Failure(t *testing.T) {
	// Setup fake GPU manager
	fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
	fakeGPUManager.IsEnabledReturns(true)

	// Setup release failure
	expectedError := errors.New("release failed")
	fakeGPUManager.ReleaseGPUsReturns(expectedError)

	// Create GPU service
	service := execution.NewGPUService(fakeGPUManager, logger.New())

	// Release GPU
	err := service.ReleaseGPU(context.Background(), "test-job")

	// Should return error
	if err == nil {
		t.Error("Expected error for release failure")
	}
	if err != expectedError {
		t.Errorf("Expected error '%v', got '%v'", expectedError, err)
	}
}

func TestGPUService_IsGPUEnabled(t *testing.T) {
	t.Run("Enabled", func(t *testing.T) {
		fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
		fakeGPUManager.IsEnabledReturns(true)

		service := execution.NewGPUService(fakeGPUManager, logger.New())

		if !service.IsGPUEnabled() {
			t.Error("Expected GPU to be enabled")
		}
	})

	t.Run("Disabled", func(t *testing.T) {
		fakeGPUManager := &gpufakes.FakeGPUManagerInterface{}
		fakeGPUManager.IsEnabledReturns(false)

		service := execution.NewGPUService(fakeGPUManager, logger.New())

		if service.IsGPUEnabled() {
			t.Error("Expected GPU to be disabled")
		}
	})
}
