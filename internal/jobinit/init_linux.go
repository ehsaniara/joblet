//go:build linux

package jobinit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"worker/pkg/logger"
	"worker/pkg/platform"
)

type jobInitializer struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewJobInitializer creates a job initializer
func NewJobInitializer() JobInitializer {
	return &jobInitializer{
		platform: platform.NewPlatform(),
		logger:   logger.New().WithField("component", "job-init"),
	}
}

func (j *jobInitializer) Run() error {
	j.logger.Info("job-init starting",
		"platform", runtime.GOOS,
		"goVersion", runtime.Version(),
		"pid", os.Getpid())

	// Setup namespace isolation
	if err := j.setupIsolation(); err != nil {
		return fmt.Errorf("isolation setup failed: %w", err)
	}

	// Load and execute job
	config, err := j.LoadConfigFromEnv()
	if err != nil {
		return fmt.Errorf("config loading failed: %w", err)
	}

	return j.ExecuteJob(config)
}

// setupIsolation uses pure Go syscalls for namespace setup
func (j *jobInitializer) setupIsolation() error {
	pid := os.Getpid()
	j.logger.Info("setting up Go isolation", "pid", pid, "approach", "pure-syscalls")

	// Only PID 1 should setup isolation
	if pid != 1 {
		j.logger.Debug("not PID 1, skipping isolation setup", "pid", pid)
		return nil
	}

	// Use Go to make mounts private
	if err := j.makePrivate(); err != nil {
		j.logger.Warn("could not make mounts private", "error", err)
		// Continue - not always required
	}

	// Use to remount /proc
	if err := j.remountProc(); err != nil {
		j.logger.Error("failed to remount /proc with", "error", err)
		return fmt.Errorf("proc remount failed: %w", err)
	}

	// Verify isolation worked
	if err := j.verifyIsolation(); err != nil {
		j.logger.Warn("isolation verification failed", "error", err)
		// Continue - isolation might still be partial
	}

	j.logger.Info("native Go isolation setup completed successfully")
	return nil
}

// makePrivate uses syscall package to make mounts private
func (j *jobInitializer) makePrivate() error {
	j.logger.Debug("making mounts private using native Go syscalls")

	// Convert Go strings to C strings for syscall
	source := []byte("")
	target := []byte("/\x00")
	fstype := []byte("")

	// Use Go's native syscall.Mount
	err := syscall.Mount(
		string(source),
		string(target[:len(target)-1]), // Remove null terminator
		string(fstype),
		syscall.MS_PRIVATE|syscall.MS_REC,
		"",
	)

	if err != nil {
		return fmt.Errorf("native mount syscall failed: %w", err)
	}

	j.logger.Debug("mounts made private using native Go")
	return nil
}

// remountProc uses pure Go to remount /proc
func (j *jobInitializer) remountProc() error {
	j.logger.Info("remounting /proc using native Go syscalls")

	// Step 1: Lazy unmount existing /proc using native Go
	if err := syscall.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		j.logger.Debug("existing /proc unmount", "error", err)
		// Continue
	}

	// Step 2: Mount new proc using native Go syscall
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		j.logger.Error("native proc mount failed", "error", err)
		return fmt.Errorf("native proc mount syscall failed: %w", err)
	}

	j.logger.Info("/proc successfully remounted using native Go")
	return nil
}

// verifyIsolation checks that our native isolation worked
func (j *jobInitializer) verifyIsolation() error {
	j.logger.Debug("verifying native Go isolation effectiveness")

	// Check PID 1 in our namespace
	if comm, err := os.ReadFile("/proc/1/comm"); err == nil {
		pid1Process := string(comm)
		j.logger.Info("PID 1 in namespace", "process", pid1Process)

		// In isolated namespace, PID 1 should be job-init
		if pid1Process != "job-init\n" {
			j.logger.Warn("PID 1 is not job-init, isolation may be incomplete",
				"actualPid1", pid1Process)
		}
	}

	// Count visible processes
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return fmt.Errorf("cannot read /proc: %w", err)
	}

	pidCount := 0
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err == nil {
			pidCount++
		}
	}

	j.logger.Info("isolation verification",
		"visibleProcesses", pidCount,
		"isolationQuality", j.assessIsolationQuality(pidCount))

	return nil
}

// assessIsolationQuality provides feedback on isolation effectiveness
func (j *jobInitializer) assessIsolationQuality(pidCount int) string {
	switch {
	case pidCount <= 5:
		return "excellent"
	case pidCount <= 20:
		return "good"
	case pidCount <= 50:
		return "partial"
	default:
		return "poor"
	}
}

// LoadConfigFromEnv loads job configuration (same as before)
func (j *jobInitializer) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := os.Getenv("JOB_ID")
	command := os.Getenv("JOB_COMMAND")
	argsCountStr := os.Getenv("JOB_ARGS_COUNT")

	if jobID == "" || command == "" {
		return nil, fmt.Errorf("missing required environment: JOB_ID=%s, JOB_COMMAND=%s", jobID, command)
	}

	var args []string
	if argsCountStr != "" {
		if argsCount, err := strconv.Atoi(argsCountStr); err == nil {
			args = make([]string, argsCount)
			for i := 0; i < argsCount; i++ {
				args[i] = os.Getenv(fmt.Sprintf("JOB_ARG_%d", i))
			}
		}
	}

	j.logger.Info("loaded job config using native Go",
		"jobId", jobID,
		"command", command,
		"argsCount", len(args))

	return &JobConfig{
		JobID:   jobID,
		Command: command,
		Args:    args,
	}, nil
}

// ExecuteJob executes the job using native Go
func (j *jobInitializer) ExecuteJob(config *JobConfig) error {
	if config == nil {
		return fmt.Errorf("job config cannot be nil")
	}

	j.logger.Info("executing job with native Go",
		"jobId", config.JobID, "command", config.Command)

	// Resolve command path using native Go
	commandPath, err := j.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("command resolution failed: %w", err)
	}

	// Execute using native Go syscall.Exec
	execArgs := append([]string{config.Command}, config.Args...)
	envVars := os.Environ()

	j.logger.Info("executing command with native Go syscall.Exec",
		"commandPath", commandPath, "args", execArgs)

	// Native Go exec syscall
	if e := syscall.Exec(commandPath, execArgs, envVars); e != nil {
		return fmt.Errorf("native exec syscall failed: %w", e)
	}

	// This line should never be reached due to exec replacing the process
	return nil
}

// resolveCommandPath resolves command using native Go
func (j *jobInitializer) resolveCommandPath(command string) (string, error) {
	if filepath.IsAbs(command) {
		if _, e := os.Stat(command); e != nil {
			return "", fmt.Errorf("absolute command not found: %w", e)
		}
		return command, nil
	}

	// Use native Go exec.LookPath equivalent
	if resolved, e := j.lookPath(command); e == nil {
		return resolved, nil
	}

	// Check common paths manually
	commonPaths := []string{
		"/bin/" + command,
		"/usr/bin/" + command,
		"/usr/local/bin/" + command,
		"/sbin/" + command,
		"/usr/sbin/" + command,
	}

	for _, path := range commonPaths {
		if _, e := os.Stat(path); e == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// lookPath is a native Go implementation of PATH lookup
func (j *jobInitializer) lookPath(command string) (string, error) {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "", fmt.Errorf("PATH environment variable not set")
	}

	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}

		path := filepath.Join(dir, command)
		if info, e := os.Stat(path); e == nil && !info.IsDir() {
			// Check if executable
			if info.Mode()&0111 != 0 {
				return path, nil
			}
		}
	}

	return "", fmt.Errorf("command %s not found in PATH", command)
}
