package execution

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"joblet/internal/joblet/core/environment"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
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

// GetRuntimeInitPath is deprecated - runtime functionality is now handled by filesystem isolator
func (es *EnvironmentService) GetRuntimeInitPath(ctx context.Context, runtimeSpec string) (string, error) {
	return "", fmt.Errorf("runtime init path resolution is deprecated - handled by filesystem isolator")
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
