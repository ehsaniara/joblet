package gpu

import "fmt"

// SimulatedDiscovery reports a fixed number of fake GPUs so the GPU control plane
// runs on hosts without NVIDIA hardware. The devices are non-functional (CUDA
// cannot run); it exists to test the GPU plumbing.
type SimulatedDiscovery struct {
	count    int
	memoryMB int64
}

// NewSimulatedDiscovery returns a discovery that reports count fake GPUs. A
// non-positive count is treated as 2.
func NewSimulatedDiscovery(count int) *SimulatedDiscovery {
	if count <= 0 {
		count = 2
	}
	return &SimulatedDiscovery{count: count, memoryMB: 16384}
}

// DiscoverGPUs returns the simulated GPU inventory.
func (s *SimulatedDiscovery) DiscoverGPUs() ([]*GPU, error) {
	gpus := make([]*GPU, 0, s.count)
	for i := 0; i < s.count; i++ {
		gpus = append(gpus, &GPU{
			Index:    i,
			UUID:     fmt.Sprintf("GPU-SIM-%08d", i),
			Name:     "Simulated GPU",
			MemoryMB: s.memoryMB,
		})
	}
	return gpus, nil
}

// RefreshGPUs is a no-op for simulated devices.
func (s *SimulatedDiscovery) RefreshGPUs() error { return nil }
