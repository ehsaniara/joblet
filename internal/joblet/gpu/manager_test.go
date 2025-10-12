package gpu_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/gpu"
	"github.com/ehsaniara/joblet/internal/joblet/gpu/gpufakes"
	"github.com/ehsaniara/joblet/pkg/config"
)

func TestManager_AllocateGPUs_NoGPUsAvailable(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	// Return empty GPU list
	fakeDiscovery.DiscoverGPUsReturns([]*gpu.GPU{}, nil)

	// Create manager
	cfg := config.GPUConfig{Enabled: true}
	manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

	// Initialize manager (discovers GPUs)
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize manager: %v", err)
	}

	// Try to allocate 1 GPU
	allocation, err := manager.AllocateGPUs("test-job", 1, 1024)

	// Should fail when no GPUs available
	if err == nil {
		t.Error("Expected error when no GPUs available, got nil")
	}
	if allocation != nil {
		t.Error("Expected nil allocation when no GPUs available")
	}

	// Verify discovery was called
	if fakeDiscovery.DiscoverGPUsCallCount() == 0 {
		t.Error("Expected DiscoverGPUs to be called")
	}
}

func TestManager_AllocateGPUs_Success(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	// Return available GPUs
	gpus := []*gpu.GPU{
		{Index: 0, UUID: "GPU-12345", Name: "RTX 4090", MemoryMB: 24576, InUse: false},
		{Index: 1, UUID: "GPU-67890", Name: "RTX 4080", MemoryMB: 16384, InUse: false},
	}
	fakeDiscovery.DiscoverGPUsReturns(gpus, nil)

	// Create manager
	cfg := config.GPUConfig{Enabled: true}
	manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

	// Initialize manager
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize manager: %v", err)
	}

	// Allocate 1 GPU with 8GB memory requirement
	allocation, err := manager.AllocateGPUs("test-job", 1, 8192)

	// Should succeed
	if err != nil {
		t.Errorf("Expected successful allocation, got error: %v", err)
	}
	if allocation == nil {
		t.Fatal("Expected allocation, got nil")
	}
	if len(allocation.GPUIndices) != 1 {
		t.Errorf("Expected 1 GPU allocated, got %d", len(allocation.GPUIndices))
	}
	if allocation.JobID != "test-job" {
		t.Errorf("Expected job ID 'test-job', got '%s'", allocation.JobID)
	}
}

func TestManager_AllocateGPUs_InsufficientMemory(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	// Return low-memory GPU
	gpus := []*gpu.GPU{
		{Index: 0, UUID: "GPU-12345", Name: "GTX 1650", MemoryMB: 4096, InUse: false},
	}
	fakeDiscovery.DiscoverGPUsReturns(gpus, nil)

	// Create manager
	cfg := config.GPUConfig{Enabled: true}
	manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

	// Initialize manager
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize manager: %v", err)
	}

	// Try to allocate GPU with more memory than available
	allocation, err := manager.AllocateGPUs("test-job", 1, 8192)

	// Should fail for insufficient memory
	if err == nil {
		t.Error("Expected error for insufficient GPU memory, got nil")
	}
	if allocation != nil {
		t.Error("Expected nil allocation for insufficient memory")
	}
}

func TestManager_ReleaseGPUs_Success(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	// Return available GPU
	gpus := []*gpu.GPU{
		{Index: 0, UUID: "GPU-12345", Name: "RTX 4090", MemoryMB: 24576, InUse: false},
	}
	fakeDiscovery.DiscoverGPUsReturns(gpus, nil)

	// Create manager
	cfg := config.GPUConfig{Enabled: true}
	manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

	// Initialize manager
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize manager: %v", err)
	}

	// First allocate
	allocation, err := manager.AllocateGPUs("test-job", 1, 8192)
	if err != nil {
		t.Fatalf("Failed to allocate GPU: %v", err)
	}
	if allocation == nil {
		t.Fatal("Expected allocation, got nil")
	}

	// Then release
	err = manager.ReleaseGPUs("test-job")
	if err != nil {
		t.Errorf("Failed to release GPU: %v", err)
	}

	// Try to allocate again - should succeed if properly released
	allocation2, err := manager.AllocateGPUs("another-job", 1, 8192)
	if err != nil {
		t.Errorf("Failed to allocate GPU after release: %v", err)
	}
	if allocation2 == nil {
		t.Error("Expected successful allocation after release")
	}
}

func TestManager_IsEnabled(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	t.Run("Enabled", func(t *testing.T) {
		cfg := config.GPUConfig{Enabled: true}
		manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

		if !manager.IsEnabled() {
			t.Error("Expected manager to be enabled")
		}
	})

	t.Run("Disabled", func(t *testing.T) {
		cfg := config.GPUConfig{Enabled: false}
		manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

		if manager.IsEnabled() {
			t.Error("Expected manager to be disabled")
		}
	})
}

func TestManager_DiscoveryError(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	// Return error from discovery
	fakeDiscovery.DiscoverGPUsReturns(nil, errors.New("discovery failed"))

	// Create manager
	cfg := config.GPUConfig{Enabled: true}
	manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

	// Initialize should handle discovery error gracefully
	_ = manager.Initialize()
	// Note: Initialize might not return error to allow system to run without GPU
	// but GPUs should not be available

	// Try to allocate - should fail
	allocation, err := manager.AllocateGPUs("test-job", 1, 1024)
	if err == nil {
		t.Error("Expected error when discovery fails")
	}
	if allocation != nil {
		t.Error("Expected nil allocation when discovery fails")
	}
}

func TestManager_ConcurrentAllocation(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	// Return 2 available GPUs
	gpus := []*gpu.GPU{
		{Index: 0, UUID: "GPU-1", Name: "GPU 1", MemoryMB: 8192, InUse: false},
		{Index: 1, UUID: "GPU-2", Name: "GPU 2", MemoryMB: 8192, InUse: false},
	}
	fakeDiscovery.DiscoverGPUsReturns(gpus, nil)

	// Create manager
	cfg := config.GPUConfig{Enabled: true}
	manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

	// Initialize manager
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize manager: %v", err)
	}

	// Run concurrent allocations
	results := make(chan error, 3)
	for i := 0; i < 3; i++ {
		go func(jobID int) {
			_, err := manager.AllocateGPUs(fmt.Sprintf("job-%d", jobID), 1, 4096)
			results <- err
		}(i)
	}

	// Collect results
	var successes, failures int
	for i := 0; i < 3; i++ {
		err := <-results
		if err == nil {
			successes++
		} else {
			failures++
		}
	}

	// Should have 2 successes (2 GPUs) and 1 failure
	if successes != 2 {
		t.Errorf("Expected 2 successful allocations, got %d", successes)
	}
	if failures != 1 {
		t.Errorf("Expected 1 failed allocation, got %d", failures)
	}
}

func TestManager_CUDADetection(t *testing.T) {
	// Setup fakes
	fakeDiscovery := &gpufakes.FakeGPUDiscoveryInterface{}
	fakeCuda := &gpufakes.FakeCUDADetectorInterface{}

	// Setup GPU discovery to return GPUs (needed to trigger CUDA detection)
	gpus := []*gpu.GPU{
		{Index: 0, UUID: "GPU-1", Name: "Test GPU", MemoryMB: 8192},
	}
	fakeDiscovery.DiscoverGPUsReturns(gpus, nil)

	// Setup CUDA detection
	cudaPaths := []string{"/usr/local/cuda", "/opt/cuda"}
	fakeCuda.DetectCUDAReturns(cudaPaths, nil)

	// Create manager with CUDA paths configured
	cfg := config.GPUConfig{
		Enabled:   true,
		CUDAPaths: []string{"/usr/local/cuda"},
	}
	manager := gpu.NewManager(cfg, fakeDiscovery, fakeCuda)

	// Initialize to trigger discovery
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Verify GPU discovery was called
	if fakeDiscovery.DiscoverGPUsCallCount() == 0 {
		t.Error("Expected DiscoverGPUs to be called during initialization")
	}

	// Note: CUDA detection happens during job allocation with GPU requirement
	// The manager only stores the CUDA detector for later use
}
