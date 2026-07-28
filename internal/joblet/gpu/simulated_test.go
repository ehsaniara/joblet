package gpu

import (
	"testing"

	"github.com/ehsaniara/joblet/pkg/config"
)

func TestSimulatedDiscovery(t *testing.T) {
	d := NewSimulatedDiscovery(3)
	gpus, err := d.DiscoverGPUs()
	if err != nil {
		t.Fatalf("DiscoverGPUs error: %v", err)
	}
	if len(gpus) != 3 {
		t.Fatalf("expected 3 simulated GPUs, got %d", len(gpus))
	}
	for i, g := range gpus {
		if g.Index != i {
			t.Errorf("gpu %d has Index %d", i, g.Index)
		}
		if g.MemoryMB <= 0 || g.UUID == "" {
			t.Errorf("gpu %d looks unpopulated: %+v", i, g)
		}
	}

	// Non-positive count defaults to 2.
	if got, _ := NewSimulatedDiscovery(0).DiscoverGPUs(); len(got) != 2 {
		t.Errorf("default count = %d, want 2", len(got))
	}
}

// The control plane must work end to end against simulated GPUs: the manager
// discovers them, reports them enabled, and allocates one to a job.
func TestManagerAllocatesSimulatedGPUs(t *testing.T) {
	m := NewManager(
		gpuConfigForTest(),
		NewSimulatedDiscovery(2),
		NewCUDADetector(nil),
	)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	if !m.IsEnabled() {
		t.Fatal("expected GPU manager to be enabled")
	}
	if m.GetGPUCount() != 2 {
		t.Fatalf("GetGPUCount = %d, want 2", m.GetGPUCount())
	}

	alloc, err := m.AllocateGPUs("job-sim", 1, 0)
	if err != nil {
		t.Fatalf("AllocateGPUs error: %v", err)
	}
	if len(alloc.GPUIndices) != 1 {
		t.Fatalf("allocated %v, want exactly 1 GPU", alloc.GPUIndices)
	}
}

func gpuConfigForTest() config.GPUConfig { return config.GPUConfig{Enabled: true} }
