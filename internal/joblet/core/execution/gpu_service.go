package execution

import (
	"context"
	"fmt"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/gpu"
	"github.com/ehsaniara/joblet/pkg/logger"
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
	gpuManager   gpu.GPUManagerInterface
	cudaVerifier gpu.CUDAVerifierInterface
	logger       *logger.Logger
}

// NewGPUService creates a new GPU service
func NewGPUService(gpuManager gpu.GPUManagerInterface, logger *logger.Logger) *GPUService {
	return &GPUService{
		gpuManager: gpuManager,
		logger:     logger.WithField("component", "gpu-service"),
	}
}

// NewGPUServiceWithVerifier creates a GPU service with CUDA verification
func NewGPUServiceWithVerifier(gpuManager gpu.GPUManagerInterface, verifier gpu.CUDAVerifierInterface, logger *logger.Logger) *GPUService {
	return &GPUService{
		gpuManager:   gpuManager,
		cudaVerifier: verifier,
		logger:       logger.WithField("component", "gpu-service"),
	}
}

// AllocateGPU allocates GPUs for a job, verifies CUDA runtime, and updates the job with allocation details
func (gs *GPUService) AllocateGPU(ctx context.Context, job *domain.Job) (*gpu.GPUAllocation, error) {
	if !gs.gpuManager.IsEnabled() {
		if job.HasGPURequirement() {
			gs.logger.Error("GPU requested but GPU support is disabled", "job_uuid", job.Uuid, "gpuCount", job.GPUCount)
			return nil, fmt.Errorf("job requires %d GPU(s) but GPU support is disabled", job.GPUCount)
		}
		return nil, nil
	}

	if !job.HasGPURequirement() {
		gs.logger.Debug("job does not require GPU", "job_uuid", job.Uuid)
		return nil, nil
	}

	log := gs.logger.WithField("job_uuid", job.Uuid)
	log.Info("allocating GPUs for job", "requestedGPUs", job.GPUCount, "memoryRequirement", job.GPUMemoryMB)

	allocation, err := gs.gpuManager.AllocateGPUs(job.Uuid, int(job.GPUCount), job.GPUMemoryMB)
	if err != nil {
		log.Error("GPU allocation failed", "error", err)
		return nil, err
	}

	if allocation == nil {
		log.Warn("GPU allocation returned nil, no GPUs allocated")
		return nil, nil
	}

	// Update job with allocated GPU information
	job.GPUIndices = make([]int32, len(allocation.GPUIndices))
	for i, gpuIndex := range allocation.GPUIndices {
		job.GPUIndices[i] = int32(gpuIndex)
	}

	log.Info("GPUs allocated", "allocatedGPUs", allocation.GPUIndices)

	// Verify CUDA runtime if verifier is available
	if gs.cudaVerifier != nil && gs.cudaVerifier.IsAvailable() {
		log.Debug("verifying CUDA runtime for allocated GPUs")

		if err := gs.cudaVerifier.CheckGPUsUsable(ctx, allocation.GPUIndices); err != nil {
			// CUDA verification failed - release GPUs and return error
			log.Error("CUDA runtime verification failed", "error", err)
			if releaseErr := gs.gpuManager.ReleaseGPUs(job.Uuid); releaseErr != nil {
				log.Warn("failed to release GPUs after verification failure", "error", releaseErr)
			}
			return nil, fmt.Errorf("CUDA verification failed: %w", err)
		}

		log.Info("CUDA runtime verified successfully")
	}

	return allocation, nil
}

// ReleaseGPU releases GPUs allocated to a job
func (gs *GPUService) ReleaseGPU(ctx context.Context, jobID string) error {
	if !gs.gpuManager.IsEnabled() {
		return nil
	}

	log := gs.logger.WithField("job_uuid", jobID)
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
