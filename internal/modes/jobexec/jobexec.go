//go:build linux

package jobexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ehsaniara/joblet/internal/joblet/core/environment"
	"github.com/ehsaniara/joblet/internal/joblet/core/upload"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/errors"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

const (
	// UnprivilegedUID is the UID jobs run as (nobody = 65534)
	UnprivilegedUID = 65534
	// UnprivilegedGID is the GID jobs run as (nogroup = 65534)
	UnprivilegedGID = 65534
)

// JobExecutor handles job execution in init mode with consolidated environment handling
type JobExecutor struct {
	platform      platform.Platform
	logger        *logger.Logger
	envBuilder    *environment.Builder
	uploadManager *upload.Manager
	config        *config.Config
}

// NewJobExecutor creates a new job executor
func NewJobExecutor(platform platform.Platform, logger *logger.Logger, cfg *config.Config) *JobExecutor {
	// Create upload manager
	uploadManager := upload.NewManager(platform, logger)

	// Create environment builder with the correct parameters
	envBuilder := environment.NewBuilder(platform, uploadManager, cfg, logger)

	return &JobExecutor{
		platform:      platform,
		logger:        logger.WithField("component", "job-executor"),
		envBuilder:    envBuilder,
		uploadManager: uploadManager, // Store for direct access if needed
		config:        cfg,
	}
}

// ExecuteInInitMode executes a job in init mode
// Execute executes the job using consolidated environment handling
func Execute(logger *logger.Logger) error {
	p := platform.NewPlatform()
	// Runs inside the job filesystem: no server config exists here. The
	// server forwarded its effective filesystem settings via JOB_FS_* env
	// vars; everything else arrives via the other JOB_* variables.
	executor := NewJobExecutor(p, logger, config.InitConfig())
	return executor.ExecuteJob()
}

func (je *JobExecutor) ExecuteJob() error {
	// Load configuration from environment
	config, err := je.envBuilder.LoadJobConfigFromEnvironment()
	if err != nil {
		return errors.WrapConfigError("job", "configuration", err)
	}

	// Check which phase we're in
	phase := je.platform.Getenv("JOB_PHASE")

	switch phase {
	case "upload":
		// Upload phase is handled in server.go
		return fmt.Errorf("%w: upload phase should be handled by server.go", errors.ErrInvalidConfig)

	case "execute", "":
		// Execute phase - just run the command
		// Executing job command

		return je.executeCommand(config)

	default:
		return errors.WrapConfigError("job", "phase", fmt.Errorf("unknown phase: %s", phase))
	}
}

// executeCommand uses fork to create a child process while keeping init as PID 1
func (je *JobExecutor) executeCommand(config *environment.JobConfig) error {
	// Resolve command path
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return errors.WrapConfigError("job", "command", err)
	}

	// Change to workspace if uploads were processed (use os.Chdir since we're in isolated namespace)
	if je.platform.Getenv("JOB_HAS_UPLOADS") == "true" {
		workDir := je.config.Filesystem.WorkspaceDir
		if workDir == "" {
			return errors.WrapConfigError("job", "workspace", fmt.Errorf("directory not configured"))
		}
		if _, err := je.platform.Stat(workDir); err == nil {
			if err := os.Chdir(workDir); err != nil {
				return errors.WrapFilesystemError(workDir, "chdir", err)
			}
			// Changed to workspace directory
		}
	}

	// Get current environment (already set up by parent process)
	envv := je.platform.Environ()

	// Load and apply runtime environment variables from /etc/runtime.env if it exists
	runtimeEnv, err := je.loadRuntimeEnvironment()
	if err == nil && len(runtimeEnv) > 0 {
		// Apply runtime environment variables
		envv = append(envv, runtimeEnv...)
		je.logger.Debug("applied runtime environment variables", "count", len(runtimeEnv))
	}

	// Executing job command
	// About to exec to replace init process with job command

	// Prepare arguments for exec - argv[0] should be the command name
	argv := append([]string{commandPath}, config.Args...)

	// SECURITY: Drop privileges before exec (except for runtime-build jobs)
	// Runtime-build jobs need root to run apt install, configure runtimes, etc.
	// Standard jobs run as unprivileged user (nobody/65534) for security.
	jobType := je.platform.Getenv("JOB_TYPE")
	if jobType == "runtime-build" {
		je.logger.Info("skipping privilege drop for runtime-build job (needs root for apt)")
	} else {
		// Standard job - drop to unprivileged user
		if err := je.dropPrivileges(); err != nil {
			je.logger.Error("failed to drop privileges", "error", err)
			return fmt.Errorf("security: failed to drop privileges: %w", err)
		}
	}

	// Use exec to replace the current process (init) with the job command
	// This makes the job command become PID 1 in the namespace, providing proper isolation
	err = je.platform.Exec(commandPath, argv, envv)
	// If we reach this point, exec failed
	je.logger.Error("exec failed - job will not appear as PID 1", "error", err)
	return fmt.Errorf("execution failed: %w", err)
}

// dropPrivileges drops root privileges to unprivileged user (nobody/65534)
// This is a critical security measure - even if the job escapes the chroot,
// it runs as an unprivileged user and cannot damage the host system.
// Order matters: must set GID before UID (can't change groups after dropping root)
func (je *JobExecutor) dropPrivileges() error {
	je.logger.Debug("dropping privileges to unprivileged user",
		"targetUID", UnprivilegedUID,
		"targetGID", UnprivilegedGID)

	// Set supplementary groups to empty (drop all group memberships)
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("failed to clear supplementary groups: %w", err)
	}

	// Set GID first (must be done before dropping root UID)
	if err := syscall.Setgid(UnprivilegedGID); err != nil {
		return fmt.Errorf("failed to set GID to %d: %w", UnprivilegedGID, err)
	}

	// Set UID last (after this, we're no longer root)
	if err := syscall.Setuid(UnprivilegedUID); err != nil {
		return fmt.Errorf("failed to set UID to %d: %w", UnprivilegedUID, err)
	}

	// Verify we actually dropped privileges
	if syscall.Getuid() != UnprivilegedUID || syscall.Getgid() != UnprivilegedGID {
		return fmt.Errorf("privilege drop verification failed: uid=%d gid=%d",
			syscall.Getuid(), syscall.Getgid())
	}

	je.logger.Debug("privileges dropped successfully",
		"uid", syscall.Getuid(),
		"gid", syscall.Getgid())

	return nil
}

// resolveCommandPath resolves the full path for a command
func (je *JobExecutor) resolveCommandPath(command string) (string, error) {
	// Check if absolute path
	if filepath.IsAbs(command) {
		return command, nil
	}

	// Try common locations first - check /usr/local/bin first for runtime binaries
	// We check these first because PATH may not be set correctly in the chroot environment
	commonPaths := []string{
		filepath.Join("/usr/local/bin", command), // Check runtime location first
		filepath.Join("/usr/bin", command),
		filepath.Join("/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	for _, path := range commonPaths {
		if _, err := je.platform.Stat(path); err == nil {
			// Resolved command at path
			return path, nil
		}
	}

	// Fall back to PATH lookup if not found in common locations
	if path, err := je.platform.LookPath(command); err == nil {
		return path, nil
	}

	// Log what we checked for debugging
	je.logger.Debug("command not found in any location", "command", command, "checked", commonPaths)
	return "", fmt.Errorf("%w: %s", errors.ErrRuntimeNotFound, command)
}

// loadRuntimeEnvironment loads runtime environment variables from /joblet/runtime.env
func (je *JobExecutor) loadRuntimeEnvironment() ([]string, error) {
	runtimeEnvPath := "/joblet/runtime.env"

	// Check if file exists
	if _, err := je.platform.Stat(runtimeEnvPath); err != nil {
		// File doesn't exist - no runtime environment variables
		return nil, err
	}

	// Read file content
	content, err := je.platform.ReadFile(runtimeEnvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read runtime environment file: %w", err)
	}

	// Parse KEY=VALUE lines
	var env []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		env = append(env, line)
	}

	return env, nil
}
