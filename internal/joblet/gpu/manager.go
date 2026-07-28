package gpu

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
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

	// verifyMemUsedMB returns a GPU's used memory in MB; used by the strict scrub
	// policy to confirm memory was cleared. Injectable for testing.
	verifyMemUsedMB func(gpuIndex int) (int64, error)
}

// scrubResidualThresholdMB is the used-memory ceiling (MB) below which a GPU is
// considered clean after a reset. A freshly reset GPU with no processes reports
// near zero; anything above this under the strict policy triggers quarantine.
const scrubResidualThresholdMB = 64

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
	manager.verifyMemUsedMB = manager.queryMemoryUsedMB

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
		if !gpu.InUse && !gpu.Quarantined {
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

	log := m.logger.WithFields("job_uuid", jobID, "gpuCount", gpuCount, "gpuMemoryMB", gpuMemoryMB)
	log.Debug("attempting to allocate GPUs")

	// Check if job already has allocation
	if existing, exists := m.allocations[jobID]; exists {
		log.Warn("job already has GPU allocation", "existingGPUs", existing.GPUIndices)
		return existing, nil
	}

	// Find available GPUs that meet memory requirements
	availableGPUs := make([]*GPU, 0)
	for _, gpu := range m.gpus {
		if !gpu.InUse && !gpu.Quarantined {
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
	var migUUIDs []string
	allocatedAt := time.Now()

	for i, gpu := range selectedGPUs {
		gpu.InUse = true
		gpu.JobUUID = jobID
		gpu.AllocatedAt = &allocatedAt
		allocatedIndices[i] = gpu.Index
		if gpu.IsMIG {
			migUUIDs = append(migUUIDs, gpu.MIGUUID)
		}

		log.Debug("allocated GPU to job",
			"gpuIndex", gpu.Index,
			"gpuName", gpu.Name,
			"strategy", m.allocationStrategy.Name())
	}

	// Create allocation record
	allocation := &GPUAllocation{
		JobUUID:     jobID,
		GPUIndices:  allocatedIndices,
		MIGUUIDs:    migUUIDs,
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

	log := m.logger.WithField("job_uuid", jobID)
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
			gpu.JobUUID = ""
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

	// Scrub GPU memory before the GPU can be reallocated. Under the strict policy
	// a GPU whose memory cannot be verified clean is quarantined rather than
	// handed to another job.
	m.scrubOnRelease(allocation.GPUIndices)

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
// scrubOnRelease clears GPU memory according to the configured scrub policy.
// Caller must hold m.mutex.
func (m *Manager) scrubOnRelease(gpuIndices []int) {
	policy := m.config.ScrubPolicy
	if policy == "" {
		policy = "reset"
	}
	if policy == "off" || len(gpuIndices) == 0 {
		return
	}

	for _, idx := range gpuIndices {
		log := m.logger.WithField("gpuIndex", idx)
		if err := m.resetGPU(idx); err != nil {
			log.Warn("GPU reset failed during scrub", "error", err)
		}

		if policy != "strict" {
			// Best effort: the GPU returns to the pool regardless.
			continue
		}

		// Strict: only return the GPU to the pool if its memory is verifiably
		// clear; otherwise quarantine it so no later job can read stale data.
		usedMB, err := m.verifyMemUsedMB(idx)
		if err != nil {
			m.quarantine(idx, fmt.Sprintf("could not verify memory after scrub: %v", err))
			continue
		}
		if usedMB > scrubResidualThresholdMB {
			m.quarantine(idx, fmt.Sprintf("%dMB still in use after scrub (> %dMB)", usedMB, scrubResidualThresholdMB))
			continue
		}
		log.Debug("GPU memory verified clear after scrub", "usedMB", usedMB)
	}
}

// quarantine marks a GPU unavailable for allocation. Caller must hold m.mutex.
func (m *Manager) quarantine(gpuIndex int, reason string) {
	gpu, ok := m.gpus[gpuIndex]
	if !ok {
		return
	}
	gpu.Quarantined = true
	gpu.QuarantineReason = reason
	m.logger.Error("quarantining GPU after unverified scrub; excluded from allocation until cleared",
		"gpuIndex", gpuIndex, "reason", reason)
}

// ClearQuarantine returns a quarantined GPU to the allocatable pool. For operator
// use once the GPU has been verified or reset out of band.
func (m *Manager) ClearQuarantine(gpuIndex int) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	gpu, ok := m.gpus[gpuIndex]
	if !ok {
		return fmt.Errorf("GPU %d not found", gpuIndex)
	}
	gpu.Quarantined = false
	gpu.QuarantineReason = ""
	m.logger.Info("cleared GPU quarantine", "gpuIndex", gpuIndex)
	return nil
}

// resetGPU attempts a best-effort GPU reset via nvidia-smi.
func (m *Manager) resetGPU(gpuIndex int) error {
	return exec.Command("nvidia-smi", "--gpu-reset", "-i", fmt.Sprintf("%d", gpuIndex)).Run()
}

// queryMemoryUsedMB returns a GPU's used memory in MB via nvidia-smi.
func (m *Manager) queryMemoryUsedMB(gpuIndex int) (int64, error) {
	out, err := exec.Command("nvidia-smi", "--query-gpu=memory.used",
		"--format=csv,noheader,nounits", "-i", fmt.Sprintf("%d", gpuIndex)).Output()
	if err != nil {
		return 0, fmt.Errorf("failed to query GPU memory: %w", err)
	}
	txt := strings.TrimSpace(string(out))
	used, err := strconv.ParseInt(txt, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse memory.used %q: %w", txt, err)
	}
	return used, nil
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
