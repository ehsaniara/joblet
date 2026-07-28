package execution

import (
	"context"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// stubEnvManager is a minimal EnvironmentManager for GPU-forwarding tests; only
// the CUDA methods are exercised, the rest satisfy the interface.
type stubEnvManager struct {
	cudaPaths []string
}

func (s *stubEnvManager) BuildEnvironment(*domain.Job, string) []string { return nil }
func (s *stubEnvManager) PrepareWorkspace(string, []domain.FileUpload) (string, error) {
	return "", nil
}
func (s *stubEnvManager) CleanupWorkspace(string) error               { return nil }
func (s *stubEnvManager) DetectCUDA() ([]string, error)               { return s.cudaPaths, nil }
func (s *stubEnvManager) GetCUDAEnvironment(string) map[string]string { return nil }

// setupGPUEnvironment must forward the allocated GPU indices and detected CUDA
// mount paths to the init process via the JOB_GPU_* environment variables, in
// addition to the CUDA_VISIBLE_DEVICES visibility hints.
func TestSetupGPUEnvironment_ForwardsIndicesAndCUDA(t *testing.T) {
	ec := &ExecutionCoordinator{
		environmentManager: &stubEnvManager{cudaPaths: []string{"/usr/local/cuda"}},
		logger:             logger.New(),
	}

	job := &domain.Job{Uuid: "job-1", GPUIndices: []int32{0, 1}}
	if err := ec.setupGPUEnvironment(context.Background(), job); err != nil {
		t.Fatalf("setupGPUEnvironment error: %v", err)
	}

	checks := map[string]string{
		"JOB_GPU_INDICES":      "0,1",
		"JOB_GPU_CUDA_MOUNTS":  "/usr/local/cuda",
		"CUDA_VISIBLE_DEVICES": "0,1",
	}
	for k, want := range checks {
		if got := job.Environment[k]; got != want {
			t.Errorf("job.Environment[%q] = %q, want %q", k, got, want)
		}
	}
}

// With no GPUs allocated, setupGPUEnvironment is a no-op and sets nothing.
func TestSetupGPUEnvironment_NoGPUsNoOp(t *testing.T) {
	ec := &ExecutionCoordinator{
		environmentManager: &stubEnvManager{},
		logger:             logger.New(),
	}
	job := &domain.Job{Uuid: "job-2"}
	if err := ec.setupGPUEnvironment(context.Background(), job); err != nil {
		t.Fatalf("setupGPUEnvironment error: %v", err)
	}
	if _, ok := job.Environment["JOB_GPU_INDICES"]; ok {
		t.Errorf("expected no GPU env for a job without GPUs, got %v", job.Environment)
	}
}
