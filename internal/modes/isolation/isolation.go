//go:build linux

package isolation

import (
	"fmt"
	"joblet/internal/joblet/core/filesystem"
	"runtime"
	"strconv"
	"strings"

	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// Isolator provides job isolation functionality
type Isolator struct {
	platform   platform.Platform
	filesystem *filesystem.Isolator
	logger     *logger.Logger
}

// NewIsolator creates a new isolator with the given platform
func NewIsolator(p platform.Platform, logger *logger.Logger) *Isolator {
	cfg, _, _ := config.LoadConfig()

	return &Isolator{
		platform:   p,
		filesystem: filesystem.NewIsolator(cfg.Filesystem, p),
		logger:     logger.WithField("component", "isolator"),
	}
}

// Setup sets up platform-specific job isolation
func Setup(logger *logger.Logger) error {
	p := platform.NewPlatform()
	isolator := NewIsolator(p, logger)
	return isolator.Setup()
}

// Setup sets up platform-specific job isolation using the platform abstraction
func (i *Isolator) Setup() error {
	switch runtime.GOOS {
	case "linux":
		return i.setupLinux()
	case "darwin":
		return i.setupDarwin()
	default:
		return fmt.Errorf("unsupported platform for job isolation: %s", runtime.GOOS)
	}
}

// setupLinux sets up Linux-specific isolation using platform abstraction
func (i *Isolator) setupLinux() error {
	pid := i.platform.Getpid()
	i.logger.Debug("setting up Linux isolation with filesystem isolation", "pid", pid, "approach", "platform-abstraction")

	// Only PID 1 should set up isolation
	if pid != 1 {
		i.logger.Debug("not PID 1, skipping isolation setup", "pid", pid)
		return nil
	}

	// Make mounts private
	if err := i.makePrivate(); err != nil {
		i.logger.Warn("could not make mounts private", "error", err)
		// Continue - not always required
	}

	// Setup filesystem isolation BEFORE remounting /proc
	if err := i.setupFilesystemIsolation(); err != nil {
		i.logger.Error("filesystem isolation setup failed", "error", err)
		return fmt.Errorf("filesystem isolation failed: %w", err)
	}

	// Remount /proc (this will now be inside the chroot)
	if err := i.remountProc(); err != nil {
		i.logger.Error("failed to remount /proc", "error", err)
		return fmt.Errorf("proc remount failed: %w", err)
	}

	// Verify isolation
	if err := i.verifyIsolation(); err != nil {
		i.logger.Warn("isolation verification failed", "error", err)
		// Continue - isolation might still be partial
	}

	i.logger.Debug("Linux isolation setup completed successfully")
	return nil
}

// setupDarwin sets up macOS-specific isolation (minimal)
func (i *Isolator) setupDarwin() error {
	i.logger.Debug("macOS isolation setup (minimal - no namespaces available)")
	// macOS doesn't have Linux namespaces, so this is mostly a no-op
	return nil
}

// setupFilesystemIsolation sets up filesystem isolation for the job
func (i *Isolator) setupFilesystemIsolation() error {
	i.logger.Debug("setting up filesystem isolation")

	jobID := i.platform.Getenv("JOB_ID")
	if jobID == "" {
		return fmt.Errorf("JOB_ID not set - cannot setup filesystem isolation")
	}

	// Create isolated filesystem for this job
	jobFS, e := i.filesystem.CreateJobFilesystem(jobID)
	if e != nil {
		return fmt.Errorf("failed to create job filesystem: %w", e)
	}

	// Set up the filesystem isolation (chroot, mounts, etc.)
	// Note: jobFS.Setup() handles runtime mounting internally before chroot
	if err := jobFS.Setup(); err != nil {
		return fmt.Errorf("failed to setup filesystem isolation: %w", err)
	}

	i.logger.Debug("filesystem isolation setup completed successfully", "jobID", jobID)
	return nil
}

// makePrivate makes mounts private using platform abstraction
func (i *Isolator) makePrivate() error {
	i.logger.Debug("making mounts private using platform abstraction")

	// Use platform constants and helper method
	err := i.platform.Mount("", "/", "", 0x40000|0x4000, "") // 0x40000|0x4000 for platform.MountPrivate|platform.MountRecursive
	if err != nil {
		return fmt.Errorf("platform mount syscall failed: %w", err)
	}

	i.logger.Debug("mounts made private using platform abstraction")
	return nil
}

// remountProc remounts /proc using platform abstraction (now within chroot)
func (i *Isolator) remountProc() error {
	i.logger.Debug("remounting /proc within isolated filesystem")

	// We're now inside the chroot, so /proc refers to the chrooted /proc
	// Ensure /proc directory exists
	if err := i.platform.MkdirAll("/proc", 0755); err != nil {
		i.logger.Debug("failed to create /proc directory", "error", err)
		// Continue anyway - it might already exist
	}

	// Lazy unmount existing /proc using platform helper
	if err := i.platform.Unmount("/proc", 0x2); err != nil { // 0x2 for platform.UnmountDetach
		i.logger.Debug("existing /proc unmount (within chroot)", "error", err)
		// Continue
	}

	// Mount new proc using platform abstraction
	if err := i.platform.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		i.logger.Error("platform proc mount failed (within chroot)", "error", err)
		return fmt.Errorf("platform proc mount failed: %w", err)
	}

	i.logger.Debug("/proc successfully remounted within chrooted environment")
	return nil
}

// verifyIsolation checks that isolation worked using platform abstraction
func (i *Isolator) verifyIsolation() error {
	i.logger.Debug("verifying isolation effectiveness")

	// Check PID 1 in our namespace using platform abstraction
	if comm, err := i.platform.ReadFile("/proc/1/comm"); err == nil {
		pid1Process := strings.TrimSpace(string(comm))
		i.logger.Debug("PID 1 in namespace", "process", pid1Process)

		// In isolated namespace, PID 1 should be either:
		// - "joblet" (original binary name)
		// - "init" (renamed for security)
		// - The actual command after exec (e.g., "ps", "bash", etc.)
		validPid1Names := []string{"joblet", "init", "systemd"}

		isValid := false
		for _, validName := range validPid1Names {
			if strings.Contains(pid1Process, validName) {
				isValid = true
				break
			}
		}

		if !isValid {
			// Check if it's the user's command (which is actually good!)
			// If PID 1 is the user's command, it means exec worked perfectly
			i.logger.Debug("PID 1 is user command (exec successful)",
				"process", pid1Process)
		}
	}

	// Count visible processes using platform abstraction
	entries, err := i.readProcDir()
	if err != nil {
		return fmt.Errorf("cannot read /proc: %w", err)
	}

	pidCount := 0
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry); err == nil {
			pidCount++
		}
	}

	i.logger.Debug("isolation verification", "visibleProcesses", pidCount)

	// If we can only see a few processes, isolation is working
	if pidCount > 100 {
		i.logger.Warn("many processes visible, isolation may be incomplete",
			"count", pidCount)
	}

	return nil
}

// readProcDir reads /proc directory entries using platform abstraction
func (i *Isolator) readProcDir() ([]string, error) {
	// For now, we'll use a simple approach - this could be extended to the platform interface
	entries := []string{}

	// Try to read common PID ranges to get an estimate
	for pid := 1; pid <= 1000; pid++ {
		procPath := fmt.Sprintf("/proc/%d", pid)
		if _, err := i.platform.Stat(procPath); err == nil {
			entries = append(entries, fmt.Sprintf("%d", pid))
		}
	}

	return entries, nil
}

// mountRuntimeForJob mounts the runtime directories if runtime is specified
// DEPRECATED: This function is no longer used. Runtime mounting is handled by JobFilesystem.Setup()
// in isolator.go before chroot, not after. Keeping for reference.
/*
func (i *Isolator) mountRuntimeForJob(jobID string, jobFS interface{}) error {
	runtime := i.platform.Getenv("JOB_RUNTIME")
	if runtime == "" {
		i.logger.Debug("no runtime specified, skipping runtime mount")
		return nil
	}

	i.logger.Info("mounting runtime for job", "runtime", runtime, "jobID", jobID)

	// Get runtime manager path from environment
	runtimeBasePath := i.platform.Getenv("RUNTIME_MANAGER_PATH")
	if runtimeBasePath == "" {
		runtimeBasePath = "/opt/joblet/runtimes"
	}

	// Parse runtime spec and find runtime directory
	parts := strings.Split(runtime, ":")
	if len(parts) != 2 {
		return fmt.Errorf("invalid runtime specification: %s", runtime)
	}

	language := parts[0]
	version := parts[1]
	runtimeDirName := fmt.Sprintf("%s-%s", language, strings.ReplaceAll(version, "+", "-"))
	runtimeDir := fmt.Sprintf("%s/%s/%s", runtimeBasePath, language, runtimeDirName)

	// Check if runtime exists
	if _, err := i.platform.Stat(runtimeDir); err != nil {
		return fmt.Errorf("runtime directory %s not found: %w", runtimeDir, err)
	}

	// Load runtime.yml configuration
	configPath := fmt.Sprintf("%s/runtime.yml", runtimeDir)
	configData, err := i.platform.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read runtime config: %w", err)
	}

	// Parse YAML configuration (simple parser)
	mounts := i.parseRuntimeMounts(string(configData))

	// Mount each directory according to runtime config
	jobRootDir := fmt.Sprintf("/opt/joblet/jobs/%s", jobID)
	for _, mount := range mounts {
		sourcePath := fmt.Sprintf("%s/%s", runtimeDir, mount.Source)
		targetPath := fmt.Sprintf("%s/%s", jobRootDir, strings.TrimPrefix(mount.Target, "/"))

		// Check if source exists
		if _, err := i.platform.Stat(sourcePath); err != nil {
			if i.platform.IsNotExist(err) {
				i.logger.Debug("skipping non-existent runtime source", "path", sourcePath)
				continue
			}
			return fmt.Errorf("failed to stat runtime source %s: %w", sourcePath, err)
		}

		// Create target directory
		if err := i.platform.MkdirAll(targetPath, 0755); err != nil {
			return fmt.Errorf("failed to create runtime target %s: %w", targetPath, err)
		}

		// Bind mount
		if err := i.platform.Mount(sourcePath, targetPath, "", 0x1000, ""); err != nil { // 0x1000 = MS_BIND
			return fmt.Errorf("failed to mount %s to %s: %w", sourcePath, targetPath, err)
		}

		// Remount as read-only if specified
		if mount.ReadOnly {
			if err := i.platform.Mount("", targetPath, "", 0x1000|0x200000|0x1, ""); err != nil { // MS_BIND | MS_REMOUNT | MS_RDONLY
				i.logger.Warn("failed to remount as read-only", "target", targetPath, "error", err)
			}
		}

		i.logger.Debug("mounted runtime path", "source", sourcePath, "target", targetPath)
	}

	i.logger.Info("runtime mounted successfully", "runtime", runtime)
	return nil
}
*/

// RuntimeMount represents a runtime mount configuration
type RuntimeMount struct {
	Source   string
	Target   string
	ReadOnly bool
}

// parseRuntimeMounts parses YAML mounts configuration
// DEPRECATED: Only used by deprecated mountRuntimeForJob function
/*
func (i *Isolator) parseRuntimeMounts(yamlContent string) []RuntimeMount {
	var mounts []RuntimeMount
	lines := strings.Split(yamlContent, "\n")
	var currentMount *RuntimeMount
	inMounts := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "mounts:") {
			inMounts = true
			continue
		}

		if inMounts {
			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "  - ") {
				// New mount entry
				if currentMount != nil {
					mounts = append(mounts, *currentMount)
				}
				currentMount = &RuntimeMount{}
				continue
			} else if currentMount != nil && (strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")) {
				// Mount property
				if strings.Contains(line, "source:") {
					currentMount.Source = strings.TrimSpace(strings.Split(line, ":")[1])
					currentMount.Source = strings.Trim(currentMount.Source, "\"'")
				} else if strings.Contains(line, "target:") {
					currentMount.Target = strings.TrimSpace(strings.Split(line, ":")[1])
					currentMount.Target = strings.Trim(currentMount.Target, "\"'")
				} else if strings.Contains(line, "readonly:") {
					currentMount.ReadOnly = strings.Contains(line, "true")
				}
			} else if !strings.HasPrefix(line, "  ") {
				// End of mounts section
				if currentMount != nil {
					mounts = append(mounts, *currentMount)
					currentMount = nil
				}
				inMounts = false
			}
		}
	}

	// Don't forget the last mount
	if currentMount != nil {
		mounts = append(mounts, *currentMount)
	}

	return mounts
}
*/
