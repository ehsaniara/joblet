package server

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/internal/joblet/gpu"
	jobletruntime "github.com/ehsaniara/joblet/internal/joblet/runtime"
	pkgerrors "github.com/ehsaniara/joblet/pkg/errors"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTestRuntime(t *testing.T, basePath, name, yml string) {
	t.Helper()
	dir := filepath.Join(basePath, name)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "runtime.yml"), []byte(yml), 0644))
}

func newTestJobService(runtimesPath string) *JobServiceServer {
	p := platform.NewPlatform()
	return &JobServiceServer{
		runtimeResolver: jobletruntime.NewResolver(runtimesPath, p),
		cudaDetector:    gpu.NewCUDADetector(p),
		logger:          logger.WithField("component", "job-grpc-test"),
	}
}

func TestValidateRuntime_NotFoundRejectedAtSubmission(t *testing.T) {
	s := newTestJobService(t.TempDir())

	err := s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python-3.99"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, pkgerrors.ErrRuntimeNotFound), "expected ErrRuntimeNotFound, got: %v", err)
}

func TestValidateRuntime_ExistingRuntimeAccepted(t *testing.T) {
	base := t.TempDir()
	writeTestRuntime(t, base, "python-3.11", `name: python-3.11
language: python
version: "1.0.0"
`)
	s := newTestJobService(base)

	err := s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python-3.11"})
	assert.NoError(t, err)
}

func TestValidateRuntime_ColonSpecNotResolvable(t *testing.T) {
	// Runtime directories are looked up by name[@version]; a colon-form spec
	// never matches one, so submission rejects it as not found.
	base := t.TempDir()
	writeTestRuntime(t, base, "python-3.11", `name: python-3.11
language: python
version: "1.0.0"
`)
	s := newTestJobService(base)

	err := s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python:3.11"})

	require.Error(t, err)
	assert.True(t, errors.Is(err, pkgerrors.ErrRuntimeNotFound), "expected ErrRuntimeNotFound, got: %v", err)
}

func TestValidateRuntime_IncompatibleArchitectureRejected(t *testing.T) {
	base := t.TempDir()
	writeTestRuntime(t, base, "python-3.11", `name: python-3.11
language: python
version: "1.0.0"
requirements:
  architectures: ["mips64"]
`)
	s := newTestJobService(base)

	err := s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python-3.11"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "architecture")
}

func TestValidateRuntime_UnameStyleArchitectureAccepted(t *testing.T) {
	// x86_64/aarch64 (uname style) must match Go's amd64/arm64
	base := t.TempDir()
	writeTestRuntime(t, base, "python-3.11", `name: python-3.11
language: python
version: "1.0.0"
requirements:
  architectures: ["x86_64", "aarch64"]
`)
	s := newTestJobService(base)

	err := s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python-3.11"})
	assert.NoError(t, err)
}

func TestValidateRuntime_GPURequiredButNotRequested(t *testing.T) {
	base := t.TempDir()
	writeTestRuntime(t, base, "python-3.11-gpu", `name: python-3.11-gpu
language: python
version: "1.0.0"
requirements:
  gpu: true
`)
	s := newTestJobService(base)

	err := s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python-3.11-gpu"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a GPU")

	err = s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python-3.11-gpu", GPUCount: 1})
	assert.NoError(t, err)
}

func TestValidateRuntime_CUDARequirementUnsatisfiable(t *testing.T) {
	base := t.TempDir()
	writeTestRuntime(t, base, "python-3.11-gpu", `name: python-3.11-gpu
language: python
version: "1.0.0"
requirements:
  gpu: true
  cuda_version: "99.9"
`)
	s := newTestJobService(base)

	err := s.validateJobRequest(&interfaces.StartJobRequest{Runtime: "python-3.11-gpu", GPUCount: 1})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "CUDA")
}

func TestValidateJobRequest_NegativeGPUCount(t *testing.T) {
	s := newTestJobService(t.TempDir())

	err := s.validateJobRequest(&interfaces.StartJobRequest{GPUCount: -1})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "gpuCount")
}

func TestValidateJobRequest_RejectsTraversalVolumeNames(t *testing.T) {
	// Volume names reach a root-side bind-mount path in the job init process;
	// a name containing "../" or a slash could escape the volumes base, so
	// submission must reject anything that isn't a plain volume name.
	s := newTestJobService(t.TempDir())

	bad := []string{
		"../etc",
		"../../etc/passwd",
		"foo/bar",
		"/etc",
		"..",
		"foo/../bar",
		"",
	}
	for _, name := range bad {
		err := s.validateJobRequest(&interfaces.StartJobRequest{Volumes: []string{name}})
		require.Error(t, err, "expected rejection for volume name %q", name)
		assert.Contains(t, err.Error(), "volume name", "name %q", name)
	}
}

func TestValidateJobRequest_AcceptsPlainVolumeNames(t *testing.T) {
	s := newTestJobService(t.TempDir())

	for _, name := range []string{"data", "my-vol", "vol_1", "cache123"} {
		err := s.validateJobRequest(&interfaces.StartJobRequest{Volumes: []string{name}})
		assert.NoError(t, err, "expected plain volume name %q to be accepted", name)
	}
}
