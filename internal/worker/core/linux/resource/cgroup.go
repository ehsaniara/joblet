package resource

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"worker/pkg/logger"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

const (
	CleanupTimeout = 5 * time.Second
	// CgroupsBaseDir Use the delegated cgroup path for worker user
	CgroupsBaseDir = "/sys/fs/cgroup/worker.slice/worker.service"
)

//counterfeiter:generate . Resource
type Resource interface {
	Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error
	SetIOLimit(cgroupPath string, ioBPS int) error
	SetCPULimit(cgroupPath string, cpuLimit int) error
	SetMemoryLimit(cgroupPath string, memoryLimitMB int) error
	CleanupCgroup(jobID string)
}

type cgroup struct {
	logger      *logger.Logger
	initialized bool
}

func New() Resource {
	return &cgroup{
		logger: logger.New().WithField("component", "resource-manager"),
	}
}

func (c *cgroup) ensureControllers() error {
	if c.initialized {
		return nil
	}

	log := c.logger.WithField("operation", "ensure-controllers")

	// Check if controllers are enabled in our delegated cgroup
	subtreeControlFile := filepath.Join(CgroupsBaseDir, "cgroup.subtree_control")
	currentBytes, err := os.ReadFile(subtreeControlFile)
	if err != nil {
		log.Warn("could not read subtree_control, controllers may not be enabled", "error", err)
		// Don't fail here - systemd should handle this
		c.initialized = true
		return nil
	}

	current := strings.TrimSpace(string(currentBytes))
	log.Debug("current subtree_control", "controllers", current)

	// Check what controllers are available
	controllersFile := filepath.Join(CgroupsBaseDir, "cgroup.controllers")
	if availableBytes, err := os.ReadFile(controllersFile); err == nil {
		available := strings.Fields(string(availableBytes))
		log.Debug("available controllers", "controllers", available)
	}

	c.initialized = true
	return nil
}

func (c *cgroup) Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error {
	log := c.logger.WithFields(
		"cgroupPath", cgroupJobDir,
		"maxCPU", maxCPU,
		"maxMemory", maxMemory,
		"maxIOBPS", maxIOBPS)

	log.Debug("creating cgroup")

	// Ensure we're working within our delegated subtree
	if !strings.HasPrefix(cgroupJobDir, CgroupsBaseDir) {
		return fmt.Errorf("security violation: cgroup path outside delegated subtree: %s", cgroupJobDir)
	}

	// Ensure controllers are set up
	if err := c.ensureControllers(); err != nil {
		log.Warn("controller setup failed", "error", err)
	}

	// Create the cgroup directory
	if err := os.MkdirAll(cgroupJobDir, 0755); err != nil {
		log.Error("failed to create cgroup directory", "error", err)
		return fmt.Errorf("failed to create cgroup directory: %v", err)
	}

	// Wait a moment for controller files to appear
	time.Sleep(100 * time.Millisecond)

	// Set CPU limit (with better error handling)
	if maxCPU > 0 {
		if err := c.SetCPULimit(cgroupJobDir, int(maxCPU)); err != nil {
			log.Warn("failed to set CPU limit", "error", err)
			// Don't fail the job creation - just log the warning
		}
	}

	// Set memory limit (with better error handling)
	if maxMemory > 0 {
		if err := c.SetMemoryLimit(cgroupJobDir, int(maxMemory)); err != nil {
			log.Warn("failed to set memory limit", "error", err)
			// Don't fail the job creation - just log the warning
		}
	}

	// Set IO limit (with better error handling)
	if maxIOBPS > 0 {
		if err := c.SetIOLimit(cgroupJobDir, int(maxIOBPS)); err != nil {
			log.Warn("failed to set IO limit", "error", err)
			// Don't fail the job creation - just log the warning
		}
	}

	log.Info("cgroup created successfully")
	return nil
}

// SetIOLimit sets IO limits for a cgroup
func (c *cgroup) SetIOLimit(cgroupPath string, ioBPS int) error {
	log := c.logger.WithFields("cgroupPath", cgroupPath, "ioBPS", ioBPS)

	// Check if io.max exists to confirm cgroup v2
	ioMaxPath := filepath.Join(cgroupPath, "io.max")
	if _, err := os.Stat(ioMaxPath); os.IsNotExist(err) {
		log.Debug("io.max not found, IO limiting not available")
		return fmt.Errorf("io.max not found, cgroup v2 IO limiting not available")
	}

	// Check current device format by reading io.max
	if currentConfig, err := os.ReadFile(ioMaxPath); err == nil {
		log.Debug("current io.max content", "content", string(currentConfig))
	}

	// Try different formats with valid device identification
	formats := []string{
		// Device with just rbps (more likely to work)
		fmt.Sprintf("8:0 rbps=%d", ioBPS),
		// Device with just wbps
		fmt.Sprintf("8:0 wbps=%d", ioBPS),
		// With "max" device syntax
		fmt.Sprintf("max rbps=%d", ioBPS),
		// With riops and wiops, operations per second instead of bytes
		fmt.Sprintf("8:0 riops=1000 wiops=1000"),
	}

	var lastErr error
	for _, format := range formats {
		log.Debug("trying IO limit format", "format", format)

		if e := os.WriteFile(ioMaxPath, []byte(format), 0644); e != nil {
			log.Debug("IO limit format failed", "format", format, "error", e)
			lastErr = e
		} else {
			log.Info("successfully set IO limit", "format", format)
			return nil
		}
	}

	log.Debug("all IO limit formats failed", "lastError", lastErr, "triedFormats", len(formats))
	return fmt.Errorf("all IO limit formats failed, last error: %w", lastErr)
}

// SetCPULimit sets CPU limits for the cgroup
func (c *cgroup) SetCPULimit(cgroupPath string, cpuLimit int) error {
	log := c.logger.WithFields("cgroupPath", cgroupPath, "cpuLimit", cpuLimit)

	// CPU controller files
	cpuMaxPath := filepath.Join(cgroupPath, "cpu.max")
	cpuWeightPath := filepath.Join(cgroupPath, "cpu.weight")

	// Try cpu.max (cgroup v2)
	if _, err := os.Stat(cpuMaxPath); err == nil {
		// Format: $MAX $PERIOD
		// Convert percentage to microseconds: 100% = 100000/100000, 50% = 50000/100000
		quota := (cpuLimit * 100000) / 100
		limit := fmt.Sprintf("%d 100000", quota)

		if e := os.WriteFile(cpuMaxPath, []byte(limit), 0644); e != nil {
			log.Error("failed to write to cpu.max", "limit", limit, "error", e)
			return fmt.Errorf("failed to write to cpu.max: %w", e)
		}
		log.Info("set CPU limit with cpu.max", "limit", limit)
		return nil
	}

	// Try cpu.weight as fallback (cgroup v2 alternative)
	if _, err := os.Stat(cpuWeightPath); err == nil {
		// Convert CPU limit to weight (1-10000)
		// Default weight is 100, so scale accordingly
		weight := 100 // Default
		if cpuLimit > 0 {
			// Scale from typical CPU limit (e.g. 100 = 1 core) to weight range
			weight = int(100 * (float64(cpuLimit) / 100.0))
			if weight < 1 {
				weight = 1
			} else if weight > 10000 {
				weight = 10000
			}
		}

		if e := os.WriteFile(cpuWeightPath, []byte(fmt.Sprintf("%d", weight)), 0644); e != nil {
			log.Error("failed to write to cpu.weight", "weight", weight, "error", e)
			return fmt.Errorf("failed to write to cpu.weight: %w", e)
		}

		log.Info("set CPU weight", "weight", weight)
		return nil
	}

	log.Debug("neither cpu.max nor cpu.weight found")
	return fmt.Errorf("neither cpu.max nor cpu.weight found")
}

// SetMemoryLimit sets memory limits for the cgroup
func (c *cgroup) SetMemoryLimit(cgroupPath string, memoryLimitMB int) error {
	log := c.logger.WithFields("cgroupPath", cgroupPath, "memoryLimitMB", memoryLimitMB)

	// Convert MB to bytes
	memoryLimitBytes := int64(memoryLimitMB) * 1024 * 1024

	// Cgroup v2
	memoryMaxPath := filepath.Join(cgroupPath, "memory.max")
	memoryHighPath := filepath.Join(cgroupPath, "memory.high")

	var setMax, setHigh bool

	// Set memory.max hard limit
	if _, err := os.Stat(memoryMaxPath); err == nil {
		if e := os.WriteFile(memoryMaxPath, []byte(fmt.Sprintf("%d", memoryLimitBytes)), 0644); e != nil {
			log.Warn("failed to write to memory.max", "memoryLimitBytes", memoryLimitBytes, "error", e)
		} else {
			setMax = true
			log.Info("set memory.max limit", "memoryLimitBytes", memoryLimitBytes)
		}
	}

	// Set memory.high soft limit (90% of hard limit)
	if _, err := os.Stat(memoryHighPath); err == nil {
		softLimit := int64(float64(memoryLimitBytes) * 0.9)
		if e := os.WriteFile(memoryHighPath, []byte(fmt.Sprintf("%d", softLimit)), 0644); e != nil {
			log.Warn("failed to write to memory.high", "softLimit", softLimit, "error", e)
		} else {
			setHigh = true
			log.Info("set memory.high limit", "softLimit", softLimit)
		}
	}

	if !setMax && !setHigh {
		log.Debug("neither memory.max nor memory.high found")
		return fmt.Errorf("neither memory.max nor memory.high found")
	}

	return nil
}

// CleanupCgroup deletes a cgroup after removing job processes
func (c *cgroup) CleanupCgroup(jobID string) {
	cleanupLogger := c.logger.WithField("jobId", jobID)
	cleanupLogger.Debug("starting cgroup cleanup")

	// Cleanup in a separate goroutine
	go func() {
		// Timeout for the cleanup operation
		ctx, cancel := context.WithTimeout(context.Background(), CleanupTimeout)
		defer cancel()

		done := make(chan bool)
		go func() {
			cleanupJobCgroup(jobID, cleanupLogger)
			done <- true
		}()

		// Wait for cleanup or timeout
		select {
		case <-done:
			cleanupLogger.Info("cgroup cleanup completed")
		case <-ctx.Done():
			cleanupLogger.Warn("cgroup cleanup timed out")
		}
	}()
}

// cleanupJobCgroup clean process first SIGTERM and SIGKILL then remove the cgroupPath items
func cleanupJobCgroup(jobID string, logger *logger.Logger) {
	// Use the delegated cgroup path
	cgroupPath := filepath.Join(CgroupsBaseDir, "job-"+jobID)
	cleanupLogger := logger.WithField("cgroupPath", cgroupPath)

	// Security check: ensure we're only cleaning up within our delegated subtree
	if !strings.HasPrefix(cgroupPath, CgroupsBaseDir+"/job-") {
		cleanupLogger.Error("security violation: attempted to clean up non-job cgroup", "path", cgroupPath)
		return
	}

	// Check if the cgroup exists
	if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
		cleanupLogger.Debug("cgroup directory does not exist, skipping cleanup")
		return
	}

	// Try to kill any processes still in the cgroup
	procsPath := filepath.Join(cgroupPath, "cgroup.procs")
	if procsData, err := os.ReadFile(procsPath); err == nil {
		pids := strings.Split(string(procsData), "\n")
		activePids := []string{}

		for _, pidStr := range pids {
			if pidStr == "" {
				continue
			}
			activePids = append(activePids, pidStr)

			if pid, e1 := strconv.Atoi(pidStr); e1 == nil {
				cleanupLogger.Debug("terminating process in cgroup", "pid", pid)

				// Try to terminate the process
				proc, e2 := os.FindProcess(pid)
				if e2 == nil {
					// Try SIGTERM first
					proc.Signal(syscall.SIGTERM)

					// Wait a moment
					time.Sleep(100 * time.Millisecond)

					// Then SIGKILL if needed
					proc.Signal(syscall.SIGKILL)
				}
			}
		}

		if len(activePids) > 0 {
			cleanupLogger.Info("terminated processes in cgroup", "pids", activePids)
		}
	}

	cgroupPathRemoveAll(cgroupPath, cleanupLogger)
}

func cgroupPathRemoveAll(cgroupPath string, logger *logger.Logger) {
	if err := os.RemoveAll(cgroupPath); err != nil {
		logger.Warn("failed to remove cgroup directory", "error", err)

		files, _ := os.ReadDir(cgroupPath)
		removedFiles := []string{}

		for _, file := range files {
			// Skip directories and read-only files like cgroup.events
			if file.IsDir() || strings.HasPrefix(file.Name(), "cgroup.") {
				continue
			}

			// Remove each file one by one
			filePath := filepath.Join(cgroupPath, file.Name())
			if e := os.Remove(filePath); e == nil {
				removedFiles = append(removedFiles, file.Name())
			}
		}

		if len(removedFiles) > 0 {
			logger.Debug("manually removed cgroup files", "files", removedFiles)
		}

		// Try to remove the directory again
		if e := os.Remove(cgroupPath); e != nil {
			logger.Info("could not remove cgroup directory completely, will be cleaned up later", "error", e)
		} else {
			logger.Debug("successfully removed cgroup directory on retry")
		}
	} else {
		logger.Debug("successfully removed cgroup directory")
	}
}
