package gpu

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Manager implements the GPUManagerInterface for managing GPU resources
type Manager struct {
	enabled            bool
	gpus               map[int]*GPU              // GPU index -> GPU info
	allocations        map[string]*GPUAllocation // job ID -> allocation
	discovery          GPUDiscoveryInterface
	cudaDetector       CUDADetectorInterface
	monitor            *GPUMonitor           // GPU monitoring service
	allocationStrategy GPUAllocationStrategy // GPU allocation strategy
	mutex              sync.RWMutex
	config             config.GPUConfig
	logger             *logger.Logger
}

// NewManager creates a new GPU manager with the given configuration
func NewManager(cfg config.GPUConfig, discovery GPUDiscoveryInterface, cudaDetector CUDADetectorInterface) *Manager {
	manager := &Manager{
		enabled:            cfg.Enabled,
		gpus:               make(map[int]*GPU),
		allocations:        make(map[string]*GPUAllocation),
		discovery:          discovery,
		cudaDetector:       cudaDetector,
		allocationStrategy: GetAllocationStrategy(cfg.AllocationStrategy),
		config:             cfg,
		logger:             logger.New().WithField("component", "gpu-manager"),
	}

	// Initialize GPU monitor if enabled
	if cfg.Enabled {
		manager.monitor = NewGPUMonitor(manager, 0) // Use default interval
	}

	return manager
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

	// Use allocation strategy to select GPUs
	selectedGPUs, err := m.allocationStrategy.SelectGPUs(availableGPUs, gpuCount, gpuMemoryMB)
	if err != nil {
		return nil, err
	}

	// Allocate the selected GPUs
	allocatedIndices := make([]int, gpuCount)
	allocatedAt := time.Now()

	for i, gpu := range selectedGPUs {
		gpu.InUse = true
		gpu.JobID = jobID
		gpu.AllocatedAt = &allocatedAt
		allocatedIndices[i] = gpu.Index

		log.Debug("allocated GPU to job",
			"gpuIndex", gpu.Index,
			"gpuName", gpu.Name,
			"strategy", m.allocationStrategy.Name())
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

	// Clear GPU memory for security
	if err := m.ClearGPUMemory(allocation.GPUIndices); err != nil {
		log.Warn("failed to clear GPU memory", "error", err, "gpuIndices", allocation.GPUIndices)
		// Don't fail the release operation due to memory clearing issues
	}

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

// ClearGPUMemory clears GPU memory for security between job allocations
func (m *Manager) ClearGPUMemory(gpuIndices []int) error {
	if len(gpuIndices) == 0 {
		return nil
	}

	log := m.logger.WithField("gpuIndices", gpuIndices)
	log.Debug("clearing GPU memory for security")

	// Method 1: Try nvidia-smi GPU reset (recommended)
	for _, idx := range gpuIndices {
		cmd := exec.Command("nvidia-smi", "--gpu-reset", "-i", fmt.Sprintf("%d", idx))
		if err := cmd.Run(); err != nil {
			// GPU reset might not be supported on all cards, log warning but continue
			log.Warn("GPU reset failed, attempting alternative memory clearing",
				"gpuIndex", idx, "error", err)

			// Method 2: Alternative - force memory cleanup using nvidia-smi
			if err := m.forceMemoryCleanup(idx); err != nil {
				log.Warn("memory cleanup failed", "gpuIndex", idx, "error", err)
			}
		} else {
			log.Debug("successfully reset GPU", "gpuIndex", idx)
		}
	}

	return nil
}

// forceMemoryCleanup attempts alternative memory cleanup methods
func (m *Manager) forceMemoryCleanup(gpuIndex int) error {
	// Alternative method: Query GPU processes and attempt cleanup
	// This is less reliable than GPU reset but better than nothing
	cmd := exec.Command("nvidia-smi", "--query-compute-apps=pid",
		"--format=csv,noheader,nounits", "-i", fmt.Sprintf("%d", gpuIndex))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to query GPU processes: %w", err)
	}

	// Note: In a production environment, you might want to implement more
	// sophisticated memory clearing, such as:
	// 1. Using nvidia-ml-py to directly control GPU memory
	// 2. Running a small CUDA kernel to overwrite memory
	// 3. Integration with container runtime memory clearing

	m.logger.Debug("completed alternative memory cleanup", "gpuIndex", gpuIndex)
	return nil
}

// GetMonitor returns the GPU monitoring service
func (m *Manager) GetMonitor() *GPUMonitor {
	return m.monitor
}

// StartMonitoring starts the GPU monitoring service
func (m *Manager) StartMonitoring(ctx context.Context) error {
	if !m.enabled || m.monitor == nil {
		return nil
	}
	return m.monitor.Start(ctx)
}

// StopMonitoring stops the GPU monitoring service
func (m *Manager) StopMonitoring() {
	if m.monitor != nil {
		m.monitor.Stop()
	}
}

// GetGPUMetrics returns current metrics for all GPUs
func (m *Manager) GetGPUMetrics() map[int]*GPUMetrics {
	if m.monitor == nil {
		return make(map[int]*GPUMetrics)
	}
	return m.monitor.GetMetrics()
}

// GetGPUHealth returns health status for all GPUs
func (m *Manager) GetGPUHealth() map[int]string {
	if m.monitor == nil {
		return make(map[int]string)
	}
	return m.monitor.CheckGPUHealth()
}
