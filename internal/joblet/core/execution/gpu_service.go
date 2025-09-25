package execution

import (
	"context"

	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/gpu"
	"joblet/pkg/logger"
)

// GPUManager defines the interface for GPU resource management within the execution coordinator
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . GPUManager
type GPUManager interface {
	// AllocateGPU allocates GPUs for a job and returns allocation details
	AllocateGPU(ctx context.Context, job *domain.Job) (*gpu.GPUAllocation, error)
	// ReleaseGPU releases GPUs allocated to a job
	ReleaseGPU(ctx context.Context, jobID string) error
	// IsGPUEnabled returns whether GPU support is available
	IsGPUEnabled() bool
}

// GPUService implements GPU management for job execution
type GPUService struct {
	gpuManager gpu.GPUManagerInterface
	logger     *logger.Logger
}

// NewGPUService creates a new GPU service
func NewGPUService(gpuManager gpu.GPUManagerInterface, logger *logger.Logger) *GPUService {
	return &GPUService{
		gpuManager: gpuManager,
		logger:     logger.WithField("component", "gpu-service"),
	}
}

// AllocateGPU allocates GPUs for a job and updates the job with allocation details
func (gs *GPUService) AllocateGPU(ctx context.Context, job *domain.Job) (*gpu.GPUAllocation, error) {
	if !gs.gpuManager.IsEnabled() {
		if job.HasGPURequirement() {
			gs.logger.Warn("GPU requested but GPU support is disabled", "jobID", job.Uuid, "gpuCount", job.GPUCount)
			return nil, nil // Return nil to indicate no GPU allocation (job will run without GPU)
		}
		return nil, nil
	}

	if !job.HasGPURequirement() {
		gs.logger.Debug("job does not require GPU", "jobID", job.Uuid)
		return nil, nil
	}

	log := gs.logger.WithField("jobID", job.Uuid)
	log.Info("allocating GPUs for job", "requestedGPUs", job.GPUCount, "memoryRequirement", job.GPUMemoryMB)

	allocation, err := gs.gpuManager.AllocateGPUs(job.Uuid, int(job.GPUCount), job.GPUMemoryMB)
	if err != nil {
		log.Error("GPU allocation failed", "error", err)
		return nil, err
	}

	if allocation != nil {
		// Update job with allocated GPU information
		job.GPUIndices = make([]int32, len(allocation.GPUIndices))
		for i, gpuIndex := range allocation.GPUIndices {
			job.GPUIndices[i] = int32(gpuIndex)
		}

		log.Info("GPUs allocated successfully", "allocatedGPUs", allocation.GPUIndices)
	}

	return allocation, nil
}

// ReleaseGPU releases GPUs allocated to a job
func (gs *GPUService) ReleaseGPU(ctx context.Context, jobID string) error {
	if !gs.gpuManager.IsEnabled() {
		return nil
	}

	log := gs.logger.WithField("jobID", jobID)
	log.Debug("releasing GPUs for job")

	err := gs.gpuManager.ReleaseGPUs(jobID)
	if err != nil {
		log.Error("GPU release failed", "error", err)
		return err
	}

	log.Info("GPUs released successfully")
	return nil
}

// IsGPUEnabled returns whether GPU support is available
func (gs *GPUService) IsGPUEnabled() bool {
	return gs.gpuManager.IsEnabled()
}
