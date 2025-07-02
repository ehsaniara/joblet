package isolation

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"worker/internal/worker/core/filesystem"

	"worker/pkg/config"
	"worker/pkg/logger"
	"worker/pkg/platform"
)

// Isolator provides job isolation functionality
type Isolator struct {
	platform   platform.Platform
	filesystem *filesystem.Isolator
	logger     *logger.Logger
}

// NewIsolator creates a new isolator with the given platform
func NewIsolator(p platform.Platform, logger *logger.Logger) *Isolator {
	// Load configuration for filesystem isolation
	cfg, _, _ := config.LoadConfig() // You might want to pass this instead

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

	// Only PID 1 should setup isolation
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
	jobFS, err := i.filesystem.CreateJobFilesystem(jobID)
	if err != nil {
		return fmt.Errorf("failed to create job filesystem: %w", err)
	}

	// Setup the filesystem isolation (chroot, mounts, etc.)
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

		// In isolated namespace, PID 1 should be our worker binary
		if !strings.Contains(pid1Process, "worker") {
			i.logger.Warn("PID 1 is not worker binary, isolation may be incomplete",
				"actualPid1", pid1Process)
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

	i.logger.Debug("isolation verification",
		"visibleProcesses", pidCount,
		"isolationQuality", assessIsolationQuality(pidCount))

	return nil
}

// readProcDir reads /proc directory entries using platform abstraction
func (i *Isolator) readProcDir() ([]string, error) {
	// Since platform.Platform doesn't have ReadDir, we need to work around this
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

// assessIsolationQuality provides feedback on isolation effectiveness
func assessIsolationQuality(pidCount int) string {
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
