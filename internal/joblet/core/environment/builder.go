package environment

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// FilterServerConfigEnv removes server config path variables from an
// environment destined for a job: they reference host files that do not
// exist inside the job filesystem, and the init process must fall back to
// built-in defaults instead of failing on them.
func FilterServerConfigEnv(env []string) []string {
	filtered := env[:0:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "JOBLET_CONFIG_PATH=") || strings.HasPrefix(kv, "JOBLET_RUNTIME_CONFIG_PATH=") {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}

// ForwardFilesystemEnv renders the server's effective filesystem settings as
// JOB_FS_* environment variables for the job init process. Init cannot (and
// must not) read the server config file; these variables carry the loaded
// in-memory values across the process boundary instead.
func ForwardFilesystemEnv(fs *config.FilesystemConfig) []string {
	return []string{
		fmt.Sprintf("JOB_FS_WORKSPACE_DIR=%s", fs.WorkspaceDir),
		fmt.Sprintf("JOB_FS_BASE_DIR=%s", fs.BaseDir),
		fmt.Sprintf("JOB_FS_TMP_DIR=%s", fs.TmpDir),
	}
}

// ForwardRuntimeEnv renders the server's effective runtime settings that the
// job init process consumes - the runtime base path and the list of host dirs
// allowed to be mounted into the sandbox - as JOB_RT_* environment variables.
// Init cannot read the server config file, so without these it falls back to
// the truncated built-in AllowedMounts and jobs lose the mounts the operator's
// distro runtime config provides (e.g. /usr/sbin, /usr/lib, /etc/ssl, the TLS
// trust store). AllowedMounts is joined with the OS path-list separator.
func ForwardRuntimeEnv(rt *config.RuntimeConfig) []string {
	env := []string{
		fmt.Sprintf("JOB_RT_BASE_PATH=%s", rt.BasePath),
	}
	if len(rt.AllowedMounts) > 0 {
		env = append(env, fmt.Sprintf("JOB_RT_ALLOWED_MOUNTS=%s",
			strings.Join(rt.AllowedMounts, string(os.PathListSeparator))))
	}
	return env
}

// Builder handles environment variable construction for job execution
type Builder struct {
	platform      platform.Platform
	uploadManager domain.UploadManager
	config        *config.Config
	logger        *logger.Logger
}

// NewBuilder creates a new environment builder
func NewBuilder(
	platform platform.Platform,
	uploadManager domain.UploadManager,
	cfg *config.Config,
	logger *logger.Logger,
) *Builder {
	return &Builder{
		platform:      platform,
		uploadManager: uploadManager,
		config:        cfg,
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

// BuildJobEnvironment builds the complete environment for job execution
func (b *Builder) BuildJobEnvironment(config *JobEnvironmentConfig) ([]string, domain.UploadStreamer) {
	if config.BaseEnv == nil {
		config.BaseEnv = b.platform.Environ()
	}

	config.BaseEnv = FilterServerConfigEnv(config.BaseEnv)

	// Build core job environment
	jobEnv := b.buildCoreEnvironment(config.Job, config.ExecutePath)

	// Handle uploads if present
	var streamer domain.UploadStreamer
	if len(config.Uploads) > 0 {
		uploadEnv, str := b.buildUploadEnvironment(config.Job, config.Uploads)
		jobEnv = append(jobEnv, uploadEnv...)
		streamer = str
	}

	return append(config.BaseEnv, jobEnv...), streamer
}

// buildCoreEnvironment builds the core job-specific environment variables
func (b *Builder) buildCoreEnvironment(job *domain.Job, execPath string) []string {
	env := []string{
		"JOBLET_MODE=init",
		fmt.Sprintf("JOB_ID=%s", job.Uuid),
		fmt.Sprintf("JOB_COMMAND=%s", job.Command),
		fmt.Sprintf("JOB_CGROUP_PATH=%s", "/sys/fs/cgroup"),
		fmt.Sprintf("JOB_CGROUP_HOST_PATH=%s", job.CgroupPath),
		fmt.Sprintf("JOB_ARGS_COUNT=%d", len(job.Args)),
		fmt.Sprintf("JOBLET_BINARY_PATH=%s", execPath),
		fmt.Sprintf("JOB_MAX_CPU=%d", job.Limits.CPU.Value()),
		fmt.Sprintf("JOB_MAX_MEMORY=%d", job.Limits.Memory.Megabytes()),
		fmt.Sprintf("JOB_MAX_IOBPS=%d", job.Limits.IOBandwidth.BytesPerSecond()),
	}

	// Forward the server's effective filesystem settings so init uses the
	// loaded config values rather than assuming built-in defaults
	if b.config != nil {
		env = append(env, ForwardFilesystemEnv(&b.config.Filesystem)...)
		env = append(env, ForwardRuntimeEnv(&b.config.Runtime)...)
	}

	if !job.Limits.CPUCores.IsEmpty() {
		env = append(env, fmt.Sprintf("JOB_CPU_CORES=%s", job.Limits.CPUCores.String()))
	}

	for i, arg := range job.Args {
		env = append(env, fmt.Sprintf("JOB_ARG_%d=%s", i, arg))
	}

	if job.ScheduledTime != nil && !job.ScheduledTime.IsZero() {
		env = append(env, fmt.Sprintf("JOB_SCHEDULED_TIME=%s", job.ScheduledTime.Format(time.RFC3339)))
	}

	return env
}

// buildUploadEnvironment builds upload-specific environment and returns stream context
func (b *Builder) buildUploadEnvironment(job *domain.Job, uploads []domain.FileUpload) ([]string, domain.UploadStreamer) {
	var env []string

	// Prepare upload session
	session, err := b.uploadManager.PrepareUploadSession(job.Uuid, uploads, job.Limits.Memory.Megabytes())
	if err != nil {
		b.logger.Error("failed to prepare upload session", "error", err)
		return env, nil
	}

	// Set basic upload info
	env = append(env,
		fmt.Sprintf("JOB_UPLOAD_SESSION=%t", true),
		fmt.Sprintf("JOB_UPLOAD_TOTAL_FILES=%d", session.TotalFiles),
		fmt.Sprintf("JOB_UPLOAD_TOTAL_SIZE=%d", session.TotalSize),
	)

	// Create streaming context if files are present
	if len(session.Files) > 0 {
		transport, err := b.uploadManager.CreateTransport(job.Uuid)
		if err != nil {
			b.logger.Error("failed to create upload transport", "error", err)
			return env, nil
		}

		// For backward compatibility, try to get pipe path if it's a pipe transport
		if pipeTransport, ok := transport.(*domain.PipeTransport); ok {
			env = append(env, fmt.Sprintf("JOB_UPLOAD_PIPE=%s", pipeTransport.GetPath()))
		}

		// No streamer is returned here; the upload env vars above carry the transport info.
		return env, nil
	}

	return env, nil
}

// JobConfig represents configuration loaded from environment (used in init mode)
type JobConfig struct {
	JobUUID          string
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
		JobUUID:          jobID,
		Command:          command,
		Args:             args,
		CgroupPath:       cgroupPath,
		HasUploadSession: hasUploadSession,
		UploadPipePath:   uploadPipePath,
		TotalFiles:       totalFiles,
	}, nil
}
