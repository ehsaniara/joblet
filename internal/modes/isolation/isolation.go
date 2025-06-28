package isolation

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"worker/pkg/logger"
)

// Setup sets up platform-specific job isolation
func Setup(logger *logger.Logger) error {
	switch runtime.GOOS {
	case "linux":
		return setupLinux(logger)
	case "darwin":
		return setupDarwin(logger)
	default:
		return fmt.Errorf("unsupported platform for job isolation: %s", runtime.GOOS)
	}
}

// setupLinux sets up Linux-specific isolation using native Go syscalls
func setupLinux(logger *logger.Logger) error {
	pid := os.Getpid()
	logger.Debug("setting up Linux isolation", "pid", pid, "approach", "native-go")

	// Only PID 1 should setup isolation
	if pid != 1 {
		logger.Debug("not PID 1, skipping isolation setup", "pid", pid)
		return nil
	}

	// Make mounts private
	if err := makePrivate(logger); err != nil {
		logger.Warn("could not make mounts private", "error", err)
		// Continue - not always required
	}

	// Remount /proc
	if err := remountProc(logger); err != nil {
		logger.Error("failed to remount /proc", "error", err)
		return fmt.Errorf("proc remount failed: %w", err)
	}

	// Verify isolation
	if err := verifyIsolation(logger); err != nil {
		logger.Warn("isolation verification failed", "error", err)
		// Continue - isolation might still be partial
	}

	logger.Debug("Linux isolation setup completed successfully")
	return nil
}

// setupDarwin sets up macOS-specific isolation (minimal)
func setupDarwin(logger *logger.Logger) error {
	logger.Debug("macOS isolation setup (minimal - no namespaces available)")
	// macOS doesn't have Linux namespaces, so this is mostly a no-op
	return nil
}

// makePrivate makes mounts private using native Go syscalls
func makePrivate(logger *logger.Logger) error {
	logger.Debug("making mounts private using native Go syscalls")

	err := syscall.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, "")
	if err != nil {
		return fmt.Errorf("native mount syscall failed: %w", err)
	}

	logger.Debug("mounts made private using native Go")
	return nil
}

// remountProc remounts /proc using native Go syscalls
func remountProc(logger *logger.Logger) error {
	logger.Debug("remounting /proc using native Go syscalls")

	// Lazy unmount existing /proc
	if err := syscall.Unmount("/proc", syscall.MNT_DETACH); err != nil {
		logger.Debug("existing /proc unmount", "error", err)
		// Continue
	}

	// Mount new proc
	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		logger.Error("native proc mount failed", "error", err)
		return fmt.Errorf("native proc mount syscall failed: %w", err)
	}

	logger.Debug("/proc successfully remounted using native Go")
	return nil
}

// verifyIsolation checks that isolation worked
func verifyIsolation(logger *logger.Logger) error {
	logger.Debug("verifying isolation effectiveness")

	// Check PID 1 in our namespace
	if comm, err := os.ReadFile("/proc/1/comm"); err == nil {
		pid1Process := strings.TrimSpace(string(comm))
		logger.Debug("PID 1 in namespace", "process", pid1Process)

		// In isolated namespace, PID 1 should be our worker binary
		if !strings.Contains(pid1Process, "worker") {
			logger.Warn("PID 1 is not worker binary, isolation may be incomplete",
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

	logger.Debug("isolation verification",
		"visibleProcesses", pidCount,
		"isolationQuality", assessIsolationQuality(pidCount))

	return nil
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
