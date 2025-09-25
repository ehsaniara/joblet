package gpu

import (
	"fmt"
	"sync"
	"time"

	"joblet/pkg/config"
	"joblet/pkg/logger"
)

// Manager implements the GPUManagerInterface for managing GPU resources
type Manager struct {
	enabled      bool
	gpus         map[int]*GPU              // GPU index -> GPU info
	allocations  map[string]*GPUAllocation // job ID -> allocation
	discovery    GPUDiscoveryInterface
	cudaDetector CUDADetectorInterface
	mutex        sync.RWMutex
	config       config.GPUConfig
	logger       *logger.Logger
}

// NewManager creates a new GPU manager with the given configuration
func NewManager(cfg config.GPUConfig, discovery GPUDiscoveryInterface, cudaDetector CUDADetectorInterface) *Manager {
	return &Manager{
		enabled:      cfg.Enabled,
		gpus:         make(map[int]*GPU),
		allocations:  make(map[string]*GPUAllocation),
		discovery:    discovery,
		cudaDetector: cudaDetector,
		config:       cfg,
		logger:       logger.New().WithField("component", "gpu-manager"),
	}
}

// Initialize sets up the GPU manager and discovers available GPUs
func (m *Manager) Initialize() error {
	if !m.enabled {
		m.logger.Debug("GPU support is disabled")
		return nil
	}

	m.logger.Info("initializing GPU manager")

	// Discover GPUs
	discoveredGPUs, err := m.discovery.DiscoverGPUs()
	if err != nil {
		return fmt.Errorf("failed to discover GPUs: %w", err)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Initialize GPU map
	for _, gpu := range discoveredGPUs {
		m.gpus[gpu.Index] = gpu
	}

	m.logger.Info("GPU discovery completed",
		"gpuCount", len(m.gpus),
		"enabled", m.enabled)

	// Log discovered GPUs
	for _, gpu := range m.gpus {
		m.logger.Info("discovered GPU",
			"index", gpu.Index,
			"name", gpu.Name,
			"uuid", gpu.UUID,
			"memoryMB", gpu.MemoryMB)
	}

	return nil
}

// GetAvailableGPUs returns all currently available (not allocated) GPUs
func (m *Manager) GetAvailableGPUs() ([]*GPU, error) {
	if !m.enabled {
		return []*GPU{}, nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	available := make([]*GPU, 0)
	for _, gpu := range m.gpus {
		if !gpu.InUse {
			available = append(available, gpu)
		}
	}

	return available, nil
}

// GetAllGPUs returns all GPUs (allocated and available)
func (m *Manager) GetAllGPUs() ([]*GPU, error) {
	if !m.enabled {
		return []*GPU{}, nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	all := make([]*GPU, 0, len(m.gpus))
	for _, gpu := range m.gpus {
		all = append(all, gpu)
	}

	return all, nil
}

// AllocateGPUs attempts to allocate the requested number of GPUs for a job
func (m *Manager) AllocateGPUs(jobID string, gpuCount int, gpuMemoryMB int64) (*GPUAllocation, error) {
	if !m.enabled {
		return nil, fmt.Errorf("GPU support is disabled")
	}

	if gpuCount <= 0 {
		return nil, fmt.Errorf("invalid GPU count: %d", gpuCount)
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	log := m.logger.WithFields("jobID", jobID, "gpuCount", gpuCount, "gpuMemoryMB", gpuMemoryMB)
	log.Debug("attempting to allocate GPUs")

	// Check if job already has allocation
	if existing, exists := m.allocations[jobID]; exists {
		log.Warn("job already has GPU allocation", "existingGPUs", existing.GPUIndices)
		return existing, nil
	}

	// Find available GPUs that meet memory requirements
	availableGPUs := make([]*GPU, 0)
	for _, gpu := range m.gpus {
		if !gpu.InUse {
			// Check memory requirement if specified
			if gpuMemoryMB > 0 && gpu.MemoryMB < gpuMemoryMB {
				log.Debug("skipping GPU due to insufficient memory",
					"gpuIndex", gpu.Index,
					"availableMemory", gpu.MemoryMB,
					"requiredMemory", gpuMemoryMB)
				continue
			}
			availableGPUs = append(availableGPUs, gpu)
		}
	}

	// Check if we have enough GPUs
	if len(availableGPUs) < gpuCount {
		return nil, fmt.Errorf("insufficient GPUs available: need %d, have %d", gpuCount, len(availableGPUs))
	}

	// Allocate the first N available GPUs
	allocatedIndices := make([]int, gpuCount)
	allocatedAt := time.Now()

	for i := 0; i < gpuCount; i++ {
		gpu := availableGPUs[i]
		gpu.InUse = true
		gpu.JobID = jobID
		gpu.AllocatedAt = &allocatedAt
		allocatedIndices[i] = gpu.Index

		log.Debug("allocated GPU to job",
			"gpuIndex", gpu.Index,
			"gpuName", gpu.Name)
	}

	// Create allocation record
	allocation := &GPUAllocation{
		JobID:       jobID,
		GPUIndices:  allocatedIndices,
		GPUCount:    gpuCount,
		GPUMemoryMB: gpuMemoryMB,
		AllocatedAt: allocatedAt,
	}

	m.allocations[jobID] = allocation

	log.Info("successfully allocated GPUs to job",
		"allocatedGPUs", allocatedIndices,
		"totalAllocated", len(m.allocations))

	return allocation, nil
}

// ReleaseGPUs releases all GPUs allocated to a job
func (m *Manager) ReleaseGPUs(jobID string) error {
	if !m.enabled {
		return nil // Nothing to do if GPU support is disabled
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	log := m.logger.WithField("jobID", jobID)
	log.Debug("releasing GPUs for job")

	allocation, exists := m.allocations[jobID]
	if !exists {
		log.Debug("no GPU allocation found for job")
		return nil // Not an error - job might not have used GPUs
	}

	// Release each allocated GPU
	for _, gpuIndex := range allocation.GPUIndices {
		if gpu, exists := m.gpus[gpuIndex]; exists {
			gpu.InUse = false
			gpu.JobID = ""
			gpu.AllocatedAt = nil

			log.Debug("released GPU",
				"gpuIndex", gpuIndex,
				"gpuName", gpu.Name)
		} else {
			log.Warn("GPU not found during release", "gpuIndex", gpuIndex)
		}
	}

	// Remove allocation record
	delete(m.allocations, jobID)

	// TODO: Clear GPU memory for security (design requirement)
	// This would require nvidia-ml-go library or nvidia-smi calls

	log.Info("successfully released GPUs for job",
		"releasedGPUs", allocation.GPUIndices,
		"remainingAllocations", len(m.allocations))

	return nil
}

// GetJobAllocation returns the GPU allocation for a job
func (m *Manager) GetJobAllocation(jobID string) (*GPUAllocation, error) {
	if !m.enabled {
		return nil, nil
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if allocation, exists := m.allocations[jobID]; exists {
		return allocation, nil
	}

	return nil, nil // No allocation found (not an error)
}

// IsEnabled returns whether GPU support is enabled
func (m *Manager) IsEnabled() bool {
	return m.enabled
}

// GetGPUCount returns the total number of GPUs available
func (m *Manager) GetGPUCount() int {
	if !m.enabled {
		return 0
	}

	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.gpus)
}

// RefreshGPUInfo refreshes GPU information from the discovery service
func (m *Manager) RefreshGPUInfo() error {
	if !m.enabled {
		return nil
	}

	return m.discovery.RefreshGPUs()
}
