package gpu_test

import (
	"context"
	"os"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/gpu"
	"github.com/ehsaniara/joblet/internal/joblet/gpu/gpufakes"
)

func TestParseComputeCapability(t *testing.T) {
	tests := []struct {
		input         string
		expectedMajor int
		expectedMinor int
	}{
		{"7.0", 7, 0},
		{"8.6", 8, 6},
		{"9.0", 9, 0},
		{"3.5", 3, 5},
		{"", 0, 0},
		{"7", 7, 0},
		{"invalid", 0, 0},
	}

	for _, tt := range tests {
		major, minor := gpu.ParseComputeCapability(tt.input)
		if major != tt.expectedMajor || minor != tt.expectedMinor {
			t.Errorf("ParseComputeCapability(%q) = (%d, %d), want (%d, %d)",
				tt.input, major, minor, tt.expectedMajor, tt.expectedMinor)
		}
	}
}

func TestIsComputeCapabilityCompatible(t *testing.T) {
	tests := []struct {
		actual   string
		required string
		want     bool
	}{
		{"7.0", "7.0", true},
		{"8.6", "8.6", true},
		{"8.0", "7.0", true},
		{"9.0", "7.5", true},
		{"7.5", "7.0", true},
		{"8.9", "8.6", true},
		{"7.0", "8.0", false},
		{"6.0", "7.0", false},
		{"7.0", "7.5", false},
		{"8.0", "8.6", false},
		{"", "", true},
		{"8.6", "", true},
	}

	for _, tt := range tests {
		got := gpu.IsComputeCapabilityCompatible(tt.actual, tt.required)
		if got != tt.want {
			t.Errorf("IsComputeCapabilityCompatible(%q, %q) = %v, want %v",
				tt.actual, tt.required, got, tt.want)
		}
	}
}

func TestNewCUDAVerifier(t *testing.T) {
	v := gpu.NewCUDAVerifier()
	if v == nil {
		t.Fatal("NewCUDAVerifier returned nil")
	}
}

// TestCUDAVerifierInterface_WithFake demonstrates using counterfeiter fake
func TestCUDAVerifierInterface_WithFake(t *testing.T) {
	fake := &gpufakes.FakeCUDAVerifierInterface{}

	// Configure the fake
	fake.IsAvailableReturns(true)
	fake.VerifyReturns(&gpu.CUDAVerificationResult{
		Success:       true,
		DriverVersion: "535.104.05",
		CUDAVersion:   "12.2",
		DeviceCount:   2,
		Devices: []gpu.CUDADeviceInfo{
			{Index: 0, Name: "RTX 4090", ComputeCapability: "8.9", TotalMemoryMB: 24564, Usable: true},
			{Index: 1, Name: "RTX 3090", ComputeCapability: "8.6", TotalMemoryMB: 24576, Usable: true},
		},
		UsableDeviceCount: 2,
	}, nil)
	fake.CheckGPUsUsableReturns(nil)

	// Test IsAvailable
	if !fake.IsAvailable() {
		t.Error("expected IsAvailable to return true")
	}

	// Test Verify
	ctx := context.Background()
	result, err := fake.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success=true")
	}
	if result.DeviceCount != 2 {
		t.Errorf("DeviceCount = %d, want 2", result.DeviceCount)
	}

	// Test CheckGPUsUsable
	err = fake.CheckGPUsUsable(ctx, []int{0, 1})
	if err != nil {
		t.Errorf("CheckGPUsUsable failed: %v", err)
	}

	// Verify call counts
	if fake.IsAvailableCallCount() != 1 {
		t.Errorf("IsAvailable call count = %d, want 1", fake.IsAvailableCallCount())
	}
	if fake.VerifyCallCount() != 1 {
		t.Errorf("Verify call count = %d, want 1", fake.VerifyCallCount())
	}
	if fake.CheckGPUsUsableCallCount() != 1 {
		t.Errorf("CheckGPUsUsable call count = %d, want 1", fake.CheckGPUsUsableCallCount())
	}

	// Verify arguments passed to CheckGPUsUsable
	passedCtx, passedIndices := fake.CheckGPUsUsableArgsForCall(0)
	if passedCtx != ctx {
		t.Error("wrong context passed to CheckGPUsUsable")
	}
	if len(passedIndices) != 2 || passedIndices[0] != 0 || passedIndices[1] != 1 {
		t.Errorf("wrong indices passed: %v", passedIndices)
	}
}

// TestCUDAVerifierInterface_ErrorCase tests error handling with fake
func TestCUDAVerifierInterface_ErrorCase(t *testing.T) {
	fake := &gpufakes.FakeCUDAVerifierInterface{}

	// Configure fake to return unavailable
	fake.IsAvailableReturns(false)
	fake.VerifyReturns(&gpu.CUDAVerificationResult{
		Success: false,
		Error:   "nvidia-smi not found",
	}, nil)

	if fake.IsAvailable() {
		t.Error("expected IsAvailable to return false")
	}

	ctx := context.Background()
	result, _ := fake.Verify(ctx)
	if result.Success {
		t.Error("expected success=false")
	}
	if result.Error != "nvidia-smi not found" {
		t.Errorf("Error = %q, want %q", result.Error, "nvidia-smi not found")
	}
}

// Integration tests - only run if nvidia-smi is available
func TestCUDAVerifierIntegration(t *testing.T) {
	if os.Getenv("JOBLET_TEST_CUDA") == "" {
		t.Skip("Skipping CUDA integration test (set JOBLET_TEST_CUDA=1 to run)")
	}

	v := gpu.NewCUDAVerifier()
	if !v.IsAvailable() {
		t.Skip("nvidia-smi not available")
	}

	ctx := context.Background()
	result, err := v.Verify(ctx)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}

	t.Logf("Success: %v", result.Success)
	t.Logf("Driver: %s", result.DriverVersion)
	t.Logf("CUDA: %s", result.CUDAVersion)
	t.Logf("Devices: %d", result.DeviceCount)

	for _, dev := range result.Devices {
		t.Logf("  GPU %d: %s (CC %s, %d MB free, usable=%v)",
			dev.Index, dev.Name, dev.ComputeCapability, dev.FreeMemoryMB, dev.Usable)
	}
}
