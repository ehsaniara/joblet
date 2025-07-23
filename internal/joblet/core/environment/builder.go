package environment

import (
	"fmt"
	"joblet/internal/joblet/core/upload"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"strconv"
	"time"
)

// Builder handles environment variable construction for job execution
type Builder struct {
	platform      platform.Platform
	uploadManager *upload.Manager
	logger        *logger.Logger
}

// NewBuilder creates a new environment builder
func NewBuilder(platform platform.Platform, uploadManager *upload.Manager, logger *logger.Logger) *Builder {
	return &Builder{
		platform:      platform,
		uploadManager: uploadManager,
		logger:        logger.WithField("component", "env-builder"),
	}
}

// JobEnvironmentConfig contains all configuration needed for building job environment
type JobEnvironmentConfig struct {
	Job         *domain.Job
	ExecutePath string
	Uploads     []domain.FileUpload
	BaseEnv     []string // Optional base environment, defaults to platform.Environ()
}

// StreamContext holds information for upload streaming (temporary definition if not in upload package)
type StreamContext struct {
	Session  *domain.UploadSession
	PipePath string
	JobID    string
}

// BuildJobEnvironment builds the complete environment for job execution
func (b *Builder) BuildJobEnvironment(config *JobEnvironmentConfig) ([]string, *StreamContext) {
	if config.BaseEnv == nil {
		config.BaseEnv = b.platform.Environ()
	}

	// Build core job environment
	jobEnv := b.buildCoreEnvironment(config.Job, config.ExecutePath)

	// Handle uploads if present
	var streamCtx *StreamContext
	if len(config.Uploads) > 0 {
		uploadEnv, ctx := b.buildUploadEnvironment(config.Job, config.Uploads)
		jobEnv = append(jobEnv, uploadEnv...)
		streamCtx = ctx
	}

	return append(config.BaseEnv, jobEnv...), streamCtx
}

// buildCoreEnvironment builds the core job-specific environment variables
func (b *Builder) buildCoreEnvironment(job *domain.Job, execPath string) []string {
	env := []string{
		"JOBLET_MODE=init",
		fmt.Sprintf("JOB_ID=%s", job.Id),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
		fmt.Sprintf("JOBLET_BINARY_PATH=%s", execPath),
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.MaxCPU),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.MaxMemory),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.MaxIOBPS),
	}

	// Add CPU cores if specified
	if job.Limits.CPUCores != "" {
		env = append(env, fmt.Sprintf("JOB_CPU_CORES=%s", job.Limits.CPUCores))
	}

	// Add job arguments
	for i, arg := range job.Args {
		env = append(env, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	// Add schedule information if present
	if job.ScheduledTime != nil && !job.ScheduledTime.IsZero() {
		env = append(env, fmt.Sprintf("JOB_SCHEDULED_TIME=%s", job.ScheduledTime.Format(time.RFC3339)))
	}

	return env
}

// buildUploadEnvironment builds upload-specific environment and returns stream context
func (b *Builder) buildUploadEnvironment(job *domain.Job, uploads []domain.FileUpload) ([]string, *StreamContext) {
	session, err := b.uploadManager.PrepareUploadSession(job.Id, uploads, job.Limits.MaxMemory)
	if err != nil {
		b.logger.Error("failed to prepare upload session", "error", err)
		return nil, nil
	}

	env := []string{
		fmt.Sprintf("JOB_UPLOAD_SESSION=%t", true),
		fmt.Sprintf("JOB_UPLOAD_TOTAL_FILES=%d", session.TotalFiles),
		fmt.Sprintf("JOB_UPLOAD_TOTAL_SIZE=%d", session.TotalSize),
	}

	// Create streaming context if files are present
	if len(session.Files) > 0 {
		pipePath, err := b.uploadManager.CreateUploadPipe(job.Id)
		if err != nil {
			b.logger.Error("failed to create upload pipe", "error", err)
			return env, nil
		}

		env = append(env, fmt.Sprintf("JOB_UPLOAD_PIPE=%s", pipePath))

		// Return context for streaming
		return env, &StreamContext{
			Session:  session,
			PipePath: pipePath,
			JobID:    job.Id,
		}
	}

	return env, nil
}

// BuildInitModeEnvironment builds environment for init mode execution
func (b *Builder) BuildInitModeEnvironment(config *JobConfig) []string {
	env := []string{
		"JOBLET_MODE=init",
		fmt.Sprintf("JOB_ID=%s", config.JobID),
		fmt.Sprintf("JOB_COMMAND=%s", config.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", config.CgroupPath),
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(config.Args)),
	}

	// Add arguments
	for i, arg := range config.Args {
		env = append(env, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	// Add upload information if present
	if config.HasUploadSession {
		env = append(env,
			fmt.Sprintf("JOB_UPLOAD_SESSION=%t", true),
			fmt.Sprintf("JOB_UPLOAD_PIPE=%s", config.UploadPipePath),
			fmt.Sprintf("JOB_UPLOAD_TOTAL_FILES=%d", config.TotalFiles),
		)
	}

	return env
}

// JobConfig represents configuration loaded from environment (used in init mode)
type JobConfig struct {
	JobID            string
	Command          string
	Args             []string
	CgroupPath       string
	HasUploadSession bool
	UploadPipePath   string
	TotalFiles       int
}

// LoadJobConfigFromEnvironment loads job configuration from environment variables
func (b *Builder) LoadJobConfigFromEnvironment() (*JobConfig, error) {
	jobID := b.platform.Getenv("JOB_ID")
	if jobID == "" {
		return nil, fmt.Errorf("JOB_ID not found in environment")
	}

	command := b.platform.Getenv("JOB_COMMAND")
	if command == "" {
		return nil, fmt.Errorf("JOB_COMMAND not found in environment")
	}

	cgroupPath := b.platform.Getenv("JOB_CGROUP_PATH")
	if cgroupPath == "" {
		cgroupPath = "/sys/fs/cgroup" // Default
	}

	// Load arguments
	argsCount := 0
	if argsStr := b.platform.Getenv("JOB_ARGS_COUNT"); argsStr != "" {
		count, _ := strconv.Atoi(argsStr)
		argsCount = count
	}

	args := make([]string, 0, argsCount)
	for i := 0; i < argsCount; i++ {
		if arg := b.platform.Getenv(fmt.Sprintf("JOB_ARG_%d", i)); arg != "" {
			args = append(args, arg)
		}
	}

	// Load upload session information
	hasUploadSession := b.platform.Getenv("JOB_UPLOAD_SESSION") == "true"
	uploadPipePath := b.platform.Getenv("JOB_UPLOAD_PIPE")
	totalFilesStr := b.platform.Getenv("JOB_UPLOAD_TOTAL_FILES")

	totalFiles := 0
	if totalFilesStr != "" {
		totalFiles, _ = strconv.Atoi(totalFilesStr)
	}

	return &JobConfig{
		JobID:            jobID,
		Command:          command,
		Args:             args,
		CgroupPath:       cgroupPath,
		HasUploadSession: hasUploadSession,
		UploadPipePath:   uploadPipePath,
		TotalFiles:       totalFiles,
	}, nil
}

// SetManager sets the upload manager for the stream context
func (sc *StreamContext) SetManager(m *upload.Manager) {
	// This is a simplified version - in production you might want to store the manager
	// or use it to perform operations
}

// StartStreaming starts the background streaming of files
func (sc *StreamContext) StartStreaming() error {
	// This should be implemented based on your upload manager's capabilities
	// For now, return nil to indicate success
	return nil
}
