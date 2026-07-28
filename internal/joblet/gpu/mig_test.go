package gpu

import (
	"testing"

	"github.com/ehsaniara/joblet/pkg/config"
)

func TestParseMIGDevices(t *testing.T) {
	out := `GPU 0: NVIDIA A100-SXM4-40GB (UUID: GPU-8f1a)
  MIG 1g.5gb      Device  0: (UUID: MIG-3eb1c2a0)
  MIG 2g.10gb     Device  1: (UUID: MIG-9c4d5e6f)
GPU 1: NVIDIA A100-SXM4-40GB (UUID: GPU-2b3c)
  MIG 3g.20gb     Device  0: (UUID: MIG-aa11bb22)`

	migs := ParseMIGDevices(out)
	if len(migs) != 3 {
		t.Fatalf("expected 3 MIG instances, got %d", len(migs))
	}

	// GPU 0 device 0 -> synthetic index 0*100+0
	if migs[0].Index != 0 || migs[0].MIGUUID != "MIG-3eb1c2a0" || !migs[0].IsMIG {
		t.Errorf("mig[0] = %+v", migs[0])
	}
	// GPU 0 device 1 -> 1
	if migs[1].Index != 1 || migs[1].MIGUUID != "MIG-9c4d5e6f" {
		t.Errorf("mig[1] = %+v", migs[1])
	}
	// GPU 1 device 0 -> 100
	if migs[2].Index != 100 || migs[2].MIGUUID != "MIG-aa11bb22" {
		t.Errorf("mig[2] = %+v", migs[2])
	}
	if migs[0].Name != "MIG 1g.5gb" {
		t.Errorf("profile name = %q", migs[0].Name)
	}
}

func TestParseMIGDevices_NoMIG(t *testing.T) {
	// A plain (non-MIG) listing yields no MIG instances.
	out := `GPU 0: NVIDIA GeForce RTX 4090 (UUID: GPU-abc)
GPU 1: NVIDIA GeForce RTX 4090 (UUID: GPU-def)`
	if migs := ParseMIGDevices(out); len(migs) != 0 {
		t.Fatalf("expected no MIG instances, got %d", len(migs))
	}
}

// A MIG allocation must carry the instance UUID so it can target the instance.
func TestManagerAllocatesMIG_CarriesUUID(t *testing.T) {
	m := NewManager(config.GPUConfig{Enabled: true}, NewSimulatedDiscovery(0), NewCUDADetector(nil))
	// Seed the pool directly with a MIG instance.
	m.enabled = true
	m.gpus[0] = &GPU{Index: 0, IsMIG: true, MIGUUID: "MIG-xyz", Name: "MIG 1g.5gb", MemoryMB: 5120}

	alloc, err := m.AllocateGPUs("job-mig", 1, 0)
	if err != nil {
		t.Fatalf("AllocateGPUs: %v", err)
	}
	if len(alloc.MIGUUIDs) != 1 || alloc.MIGUUIDs[0] != "MIG-xyz" {
		t.Fatalf("allocation MIGUUIDs = %v, want [MIG-xyz]", alloc.MIGUUIDs)
	}
}
