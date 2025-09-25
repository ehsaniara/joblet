package gpu

import "time"

// GPU represents information about a single GPU device
type GPU struct {
	Index       int        `json:"index"`                  // GPU number (0, 1, 2, etc.)
	UUID        string     `json:"uuid"`                   // Unique GPU identifier
	Name        string     `json:"name"`                   // GPU model name
	MemoryMB    int64      `json:"memory_mb"`              // Total GPU memory in MB
	InUse       bool       `json:"in_use"`                 // Is currently allocated to a job
	JobID       string     `json:"job_id"`                 // Which job is using this GPU (empty if not in use)
	AllocatedAt *time.Time `json:"allocated_at,omitempty"` // When GPU was allocated
}

// GPUAllocation represents a GPU allocation for a job
type GPUAllocation struct {
	JobID       string    `json:"job_id"`
	GPUIndices  []int     `json:"gpu_indices"`   // Which GPUs are allocated
	GPUCount    int       `json:"gpu_count"`     // Number of GPUs requested
	GPUMemoryMB int64     `json:"gpu_memory_mb"` // Memory requirement (0 = any)
	AllocatedAt time.Time `json:"allocated_at"`
}

// GPUDiscoveryInterface defines methods for discovering GPU devices
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . GPUDiscoveryInterface
type GPUDiscoveryInterface interface {
	// DiscoverGPUs discovers all available GPU devices
	DiscoverGPUs() ([]*GPU, error)
	// RefreshGPUs refreshes GPU information
	RefreshGPUs() error
}

// GPUManagerInterface defines the main GPU management interface
//
//counterfeiter:generate . GPUManagerInterface
type GPUManagerInterface interface {
	// Initialize sets up the GPU manager and discovers GPUs
	Initialize() error
	// GetAvailableGPUs returns all currently available (not allocated) GPUs
	GetAvailableGPUs() ([]*GPU, error)
	// GetAllGPUs returns all GPUs (allocated and available)
	GetAllGPUs() ([]*GPU, error)
	// AllocateGPUs attempts to allocate the requested number of GPUs for a job
	AllocateGPUs(jobID string, gpuCount int, gpuMemoryMB int64) (*GPUAllocation, error)
	// ReleaseGPUs releases all GPUs allocated to a job
	ReleaseGPUs(jobID string) error
	// GetJobAllocation returns the GPU allocation for a job
	GetJobAllocation(jobID string) (*GPUAllocation, error)
	// IsEnabled returns whether GPU support is enabled
	IsEnabled() bool
	// GetGPUCount returns the total number of GPUs available
	GetGPUCount() int
}

// CUDADetectorInterface defines methods for detecting CUDA installations
//
//counterfeiter:generate . CUDADetectorInterface
type CUDADetectorInterface interface {
	// DetectCUDA finds CUDA installation paths (legacy method)
	DetectCUDA() ([]string, error)
	// GetCUDAEnvironment returns environment variables for CUDA
	GetCUDAEnvironment(cudaPath string) map[string]string
	// DetectCUDAInstallations finds all CUDA installations with version information
	DetectCUDAInstallations() ([]CUDAInstallation, error)
	// FindCompatibleCUDA finds CUDA installations compatible with the required version
	FindCompatibleCUDA(requiredVersion CUDAVersion) ([]CUDAInstallation, error)
	// GetBestCUDA returns the best CUDA installation for a given requirement
	GetBestCUDA(requiredVersion CUDAVersion) (CUDAInstallation, error)
}
