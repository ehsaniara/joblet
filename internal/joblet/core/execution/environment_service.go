package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ehsaniara/joblet/internal/joblet/core/environment"
	"github.com/ehsaniara/joblet/internal/joblet/core/upload"
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// EnvironmentService handles job environment setup and management
type EnvironmentService struct {
	envBuilder    *environment.Builder
	uploadManager *upload.Manager
	platform      platform.Platform
	config        *config.Config
	logger        *logger.Logger
}

// NewEnvironmentService creates a new environment service
func NewEnvironmentService(
	envBuilder *environment.Builder,
	uploadManager *upload.Manager,
	platform platform.Platform,
	config *config.Config,
	logger *logger.Logger,
) *EnvironmentService {
	return &EnvironmentService{
		envBuilder:    envBuilder,
		uploadManager: uploadManager,
		platform:      platform,
		config:        config,
		logger:        logger.WithField("component", "environment-service"),
	}
}

// BuildEnvironment builds the environment variables for a job
func (es *EnvironmentService) BuildEnvironment(job *domain.Job, phase string) []string {
	baseEnv := es.platform.Environ()

	jobEnv := []string{
		"JOBLET_MODE=init",
		fmt.Sprintf("JOB_PHASE=%s", phase),
		fmt.Sprintf("JOB_ID=%s", job.Uuid),
		fmt.Sprintf("JOB_TYPE=%s", job.Type.String()),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.CPU.Value()),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.Memory.Megabytes()),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.IOBandwidth.BytesPerSecond()),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
	}

	for i, arg := range job.Args {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	if !job.Limits.CPUCores.IsEmpty() {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_CPU_CORES=%s", job.Limits.CPUCores.String()))
	}

	if len(job.Volumes) > 0 {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_VOLUMES_COUNT=%d", len(job.Volumes)))
		for i, volume := range job.Volumes {
			jobEnv = append(jobEnv, fmt.Sprintf("JOB_VOLUME_%d=%s", i, volume))
		}
	}

	if job.Runtime != "" {
		jobEnv = append(jobEnv, fmt.Sprintf("JOB_RUNTIME=%s", job.Runtime))
	}

	// Combine all environment variables
	env := append(baseEnv, jobEnv...)

	if job.Runtime != "" {
		runtimeEnv, err := es.getRuntimeEnvironment(job.Runtime)
		if err != nil {
			es.logger.Warn("failed to load runtime environment", "runtime", job.Runtime, "error", err)
		} else {
			env = append(env, runtimeEnv...)
		}
	}

	// Add GPU environment variables if GPUs are allocated
	if job.HasGPURequirement() && job.IsGPUAllocated() {
		gpuEnv := es.buildGPUEnvironment(job)
		env = append(env, gpuEnv...)
	}

	for key, value := range job.Environment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	for key, value := range job.SecretEnvironment {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	return env
}

// PrepareWorkspace prepares the workspace and processes uploads
func (es *EnvironmentService) PrepareWorkspace(jobID string, uploads []domain.FileUpload) (string, error) {
	// Create job base directory
	jobDir := filepath.Join(es.config.Filesystem.BaseDir, jobID)
	workDir := filepath.Join(jobDir, "work")

	if err := es.platform.MkdirAll(workDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create work directory: %w", err)
	}

	// Process uploads if any
	if len(uploads) > 0 {
		if err := es.processUploads(uploads, workDir); err != nil {
			return "", fmt.Errorf("failed to process uploads: %w", err)
		}
	}

	return workDir, nil
}

// CleanupWorkspace cleans up the job workspace
func (es *EnvironmentService) CleanupWorkspace(jobID string) error {
	jobDir := filepath.Join(es.config.Filesystem.BaseDir, jobID)

	if err := os.RemoveAll(jobDir); err != nil {
		es.logger.Warn("failed to remove job directory", "jobID", jobID, "path", jobDir, "error", err)
		return err
	}

	return nil
}

// processUploads processes file uploads to the work directory
func (es *EnvironmentService) processUploads(uploads []domain.FileUpload, workDir string) error {
	es.logger.Debug("processing uploads", "count", len(uploads), "workDir", workDir)

	for _, upload := range uploads {
		fullPath := filepath.Join(workDir, upload.Path)

		if upload.IsDirectory {
			if err := os.MkdirAll(fullPath, os.FileMode(upload.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", upload.Path, err)
			}
		} else {
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", upload.Path, err)
			}

			if err := os.WriteFile(fullPath, upload.Content, os.FileMode(upload.Mode)); err != nil {
				return fmt.Errorf("failed to write file %s: %w", upload.Path, err)
			}
		}
	}

	return nil
}

// DetectCUDA detects available CUDA installations on the system
func (es *EnvironmentService) DetectCUDA() ([]string, error) {
	es.logger.Debug("detecting CUDA installations")

	var cudaPaths []string

	// Common CUDA installation paths
	commonPaths := []string{
		"/usr/local/cuda",
		"/opt/cuda",
		"/usr/cuda",
	}

	// Check for CUDA_HOME environment variable
	if cudaHome := es.platform.Getenv("CUDA_HOME"); cudaHome != "" {
		commonPaths = append([]string{cudaHome}, commonPaths...)
	}

	// Check each potential CUDA path
	for _, path := range commonPaths {
		if es.isCUDAPath(path) {
			cudaPaths = append(cudaPaths, path)
			es.logger.Debug("found CUDA installation", "path", path)
		}
	}

	if len(cudaPaths) == 0 {
		return nil, fmt.Errorf("no CUDA installations found")
	}

	es.logger.Info("detected CUDA installations", "count", len(cudaPaths), "paths", cudaPaths)
	return cudaPaths, nil
}

// GetCUDAEnvironment returns environment variables needed for CUDA runtime
func (es *EnvironmentService) GetCUDAEnvironment(cudaPath string) map[string]string {
	env := make(map[string]string)

	// Set CUDA_HOME
	env["CUDA_HOME"] = cudaPath

	// Set PATH to include CUDA binaries
	cudaBinPath := filepath.Join(cudaPath, "bin")
	if currentPath := es.platform.Getenv("PATH"); currentPath != "" {
		env["PATH"] = cudaBinPath + ":" + currentPath
	} else {
		env["PATH"] = cudaBinPath
	}

	// Set LD_LIBRARY_PATH to include CUDA libraries
	cudaLibPaths := []string{
		filepath.Join(cudaPath, "lib64"),
		filepath.Join(cudaPath, "lib"),
	}

	var existingLibPaths []string
	for _, libPath := range cudaLibPaths {
		if es.pathExists(libPath) {
			existingLibPaths = append(existingLibPaths, libPath)
		}
	}

	if len(existingLibPaths) > 0 {
		libPathStr := strings.Join(existingLibPaths, ":")
		if currentLdPath := es.platform.Getenv("LD_LIBRARY_PATH"); currentLdPath != "" {
			env["LD_LIBRARY_PATH"] = libPathStr + ":" + currentLdPath
		} else {
			env["LD_LIBRARY_PATH"] = libPathStr
		}
	}

	return env
}

// isCUDAPath checks if the given path is a valid CUDA installation
func (es *EnvironmentService) isCUDAPath(path string) bool {
	// Check for key CUDA files/directories
	requiredPaths := []string{
		filepath.Join(path, "bin", "nvcc"),       // CUDA compiler
		filepath.Join(path, "include", "cuda.h"), // CUDA headers
	}

	// Check for library directories
	libPaths := []string{
		filepath.Join(path, "lib64"),
		filepath.Join(path, "lib"),
	}

	// All required files must exist
	for _, reqPath := range requiredPaths {
		if !es.pathExists(reqPath) {
			return false
		}
	}

	// At least one library directory must exist
	hasLibDir := false
	for _, libPath := range libPaths {
		if es.pathExists(libPath) {
			hasLibDir = true
			break
		}
	}

	return hasLibDir
}

// pathExists checks if a file or directory exists
func (es *EnvironmentService) pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// getRuntimeEnvironment reads runtime.yml and returns environment variables
func (es *EnvironmentService) getRuntimeEnvironment(runtimeSpec string) ([]string, error) {
	if runtimeSpec == "" {
		return nil, nil
	}

	// Runtime name directly maps to directory name (e.g., "openjdk-21" -> "{base_path}/openjdk-21")
	runtimeDirName := runtimeSpec
	runtimeDir := filepath.Join(es.config.Runtime.BasePath, runtimeDirName)

	// Load runtime.yml file
	configPath := filepath.Join(runtimeDir, "runtime.yml")
	configData, err := es.platform.ReadFile(configPath)
	if err != nil {
		// Runtime config not found - this is not an error for simple runtimes
		es.logger.Debug("runtime config not found, using empty environment", "configPath", configPath)
		return nil, nil
	}

	// Parse environment section from YAML
	environmentVars := make(map[string]string)
	lines := strings.Split(string(configData), "\n")
	inEnvironment := false

	for _, line := range lines {
		originalLine := line
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "environment:") {
			inEnvironment = true
			continue
		}

		if inEnvironment {
			// Check if this is still part of environment section (has indentation)
			hasIndentation := strings.HasPrefix(originalLine, " ") || strings.HasPrefix(originalLine, "\t")
			if !hasIndentation && strings.Contains(line, ":") {
				// New top-level section started
				inEnvironment = false
				continue
			}

			// Parse environment variable: "  PYTHONPATH: "/usr/local/lib/python3.11/site-packages""
			if hasIndentation && strings.Contains(line, ":") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])
					// Remove quotes if present
					value = strings.Trim(value, "\"'")
					environmentVars[key] = value
				}
			}
		}
	}

	// Convert to environment variable format
	var envVars []string
	for key, value := range environmentVars {
		// Handle special PATH_PREPEND by prepending to existing PATH
		if key == "PATH_PREPEND" {
			currentPath := os.Getenv("PATH")
			if currentPath != "" {
				envVars = append(envVars, fmt.Sprintf("PATH=%s:%s", value, currentPath))
			} else {
				envVars = append(envVars, fmt.Sprintf("PATH=%s", value))
			}
		} else {
			envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
		}
	}

	es.logger.Debug("loaded runtime environment variables", "runtime", runtimeSpec, "count", len(envVars), "vars", envVars)
	return envVars, nil
}

// buildGPUEnvironment creates GPU-related environment variables for jobs with GPU allocations
func (es *EnvironmentService) buildGPUEnvironment(job *domain.Job) []string {
	var gpuEnv []string

	// Set CUDA_VISIBLE_DEVICES to allocated GPU indices
	if len(job.GPUIndices) > 0 {
		gpuIndicesStr := make([]string, len(job.GPUIndices))
		for i, gpuIndex := range job.GPUIndices {
			gpuIndicesStr[i] = fmt.Sprintf("%d", gpuIndex)
		}
		cudaVisibleDevices := strings.Join(gpuIndicesStr, ",")
		gpuEnv = append(gpuEnv, fmt.Sprintf("CUDA_VISIBLE_DEVICES=%s", cudaVisibleDevices))

		es.logger.Debug("setting GPU environment variables",
			"jobID", job.Uuid,
			"gpuIndices", job.GPUIndices,
			"CUDA_VISIBLE_DEVICES", cudaVisibleDevices)
	}

	// Add GPU metadata environment variables
	gpuEnv = append(gpuEnv,
		fmt.Sprintf("JOBLET_GPU_COUNT=%d", job.GPUCount),
		fmt.Sprintf("JOBLET_GPU_MEMORY_MB=%d", job.GPUMemoryMB),
		fmt.Sprintf("JOBLET_GPU_ALLOCATED_COUNT=%d", len(job.GPUIndices)),
	)

	// If CUDA paths are available, add CUDA environment variables
	if es.config.GPU.Enabled && len(es.config.GPU.CUDAPaths) > 0 {
		// Use the first CUDA path as default
		cudaPath := es.config.GPU.CUDAPaths[0]

		// Basic CUDA environment setup
		gpuEnv = append(gpuEnv,
			fmt.Sprintf("CUDA_HOME=%s", cudaPath),
			fmt.Sprintf("CUDA_ROOT=%s", cudaPath),
		)

		// Add CUDA lib path to LD_LIBRARY_PATH
		cudaLibPath := filepath.Join(cudaPath, "lib64")
		if es.platform.Getenv("LD_LIBRARY_PATH") != "" {
			gpuEnv = append(gpuEnv, fmt.Sprintf("LD_LIBRARY_PATH=%s:%s", cudaLibPath, es.platform.Getenv("LD_LIBRARY_PATH")))
		} else {
			gpuEnv = append(gpuEnv, fmt.Sprintf("LD_LIBRARY_PATH=%s", cudaLibPath))
		}

		// Add CUDA bin path to PATH
		cudaBinPath := filepath.Join(cudaPath, "bin")
		if es.platform.Getenv("PATH") != "" {
			gpuEnv = append(gpuEnv, fmt.Sprintf("PATH=%s:%s", cudaBinPath, es.platform.Getenv("PATH")))
		} else {
			gpuEnv = append(gpuEnv, fmt.Sprintf("PATH=%s", cudaBinPath))
		}
	}

	es.logger.Debug("built GPU environment variables",
		"jobID", job.Uuid,
		"gpuEnvCount", len(gpuEnv))

	return gpuEnv
}
