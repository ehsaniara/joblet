package gpu

import (
	"fmt"
	"testing"

	"github.com/ehsaniara/joblet/pkg/config"
)

// managerWithGPUs builds a manager preloaded with n simulated GPUs and a
// stubbed memory verifier, so scrub/quarantine logic is testable without a GPU.
func managerWithGPUs(t *testing.T, policy string, n int, usedMB int64, verifyErr error) *Manager {
	t.Helper()
	m := NewManager(
		config.GPUConfig{Enabled: true, ScrubPolicy: policy},
		NewSimulatedDiscovery(n),
		NewCUDADetector(nil),
	)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	// resetGPU shells to nvidia-smi (absent here); the strict path only depends on
	// the verifier, which we stub. reset failures are non-fatal by design.
	m.verifyMemUsedMB = func(int) (int64, error) { return usedMB, verifyErr }
	return m
}

func allocAndRelease(t *testing.T, m *Manager, job string) {
	t.Helper()
	if _, err := m.AllocateGPUs(job, 1, 0); err != nil {
		t.Fatalf("AllocateGPUs: %v", err)
	}
	if err := m.ReleaseGPUs(job); err != nil {
		t.Fatalf("ReleaseGPUs: %v", err)
	}
}

func TestScrubStrict_QuarantinesWhenMemoryDirty(t *testing.T) {
	m := managerWithGPUs(t, "strict", 2, 4096, nil) // 4GB still used after scrub
	allocAndRelease(t, m, "job-1")

	// The GPU that was used is quarantined; a fresh allocation must avoid it.
	avail, _ := m.GetAvailableGPUs()
	if len(avail) != 1 {
		t.Fatalf("expected 1 available GPU after quarantine, got %d", len(avail))
	}
	// Allocating again should succeed only on the clean GPU, never the quarantined one.
	alloc, err := m.AllocateGPUs("job-2", 1, 0)
	if err != nil {
		t.Fatalf("AllocateGPUs job-2: %v", err)
	}
	if m.gpus[alloc.GPUIndices[0]].Quarantined {
		t.Fatal("allocated a quarantined GPU")
	}
}

func TestScrubStrict_QuarantinesWhenVerifyFails(t *testing.T) {
	m := managerWithGPUs(t, "strict", 1, 0, fmt.Errorf("nvidia-smi unavailable"))
	allocAndRelease(t, m, "job-1")
	if !m.gpus[0].Quarantined {
		t.Fatal("expected GPU quarantined when memory cannot be verified")
	}
	// Nothing allocatable now.
	if _, err := m.AllocateGPUs("job-2", 1, 0); err == nil {
		t.Fatal("expected allocation to fail with the only GPU quarantined")
	}
}

func TestScrubStrict_CleanGPUReturnsToPool(t *testing.T) {
	m := managerWithGPUs(t, "strict", 1, 8, nil) // 8MB < threshold => clean
	allocAndRelease(t, m, "job-1")
	if m.gpus[0].Quarantined {
		t.Fatal("clean GPU should not be quarantined")
	}
	if _, err := m.AllocateGPUs("job-2", 1, 0); err != nil {
		t.Fatalf("clean GPU should be reallocatable: %v", err)
	}
}

func TestScrubReset_NeverQuarantines(t *testing.T) {
	// Even with dirty memory, the non-strict "reset" policy returns GPUs to the pool.
	m := managerWithGPUs(t, "reset", 1, 999999, nil)
	allocAndRelease(t, m, "job-1")
	if m.gpus[0].Quarantined {
		t.Fatal("reset policy must not quarantine")
	}
}

func TestClearQuarantine(t *testing.T) {
	m := managerWithGPUs(t, "strict", 1, 4096, nil)
	allocAndRelease(t, m, "job-1")
	if !m.gpus[0].Quarantined {
		t.Fatal("precondition: GPU should be quarantined")
	}
	if err := m.ClearQuarantine(0); err != nil {
		t.Fatalf("ClearQuarantine: %v", err)
	}
	if _, err := m.AllocateGPUs("job-2", 1, 0); err != nil {
		t.Fatalf("GPU should be allocatable after clearing quarantine: %v", err)
	}
}
