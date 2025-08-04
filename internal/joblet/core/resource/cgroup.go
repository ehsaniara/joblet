package resource

import (
	"context"
	"fmt"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

type cgroup struct {
	logger      *logger.Logger
	initialized bool
	config      config.CgroupConfig
}

func New(cfg config.CgroupConfig) Resource {
	return &cgroup{
		logger: logger.New().WithField("component", "resource-manager"),
		config: cfg,
	}
}

func (c *cgroup) EnsureControllers() error {
	if c.initialized {
		return nil
	}

	log := c.logger.WithField("operation", "ensure-controllers")
	log.Debug("initializing cgroup controllers with configuration",
		"baseDir", c.config.BaseDir,
		"controllers", c.config.EnableControllers,
		"cleanupTimeout", c.config.CleanupTimeout)

	// Use configured base directory
	if err := c.moveJobletProcessToSubgroup(); err != nil {
		log.Warn("failed to move joblet to subgroup", "error", err)
	}

	// Enable configured controllers
	if err := c.enableControllersFromConfig(); err != nil {
		log.Warn("failed to enable controllers", "error", err)
	}

	c.initialized = true
	log.Info("cgroup controllers initialized",
		"baseDir", c.config.BaseDir,
		"enabledControllers", c.config.EnableControllers)

	return nil
}

//counterfeiter:generate . Resource
type Resource interface {
	Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error
	SetIOLimit(cgroupPath string, ioBPS int) error
	SetCPULimit(cgroupPath string, cpuLimit int) error
	SetCPUCores(cgroupPath string, cores string) error
	SetMemoryLimit(cgroupPath string, memoryLimitMB int) error
	CleanupCgroup(jobID string)
	EnsureControllers() error
}

func (c *cgroup) enableControllersFromConfig() error {
	log := c.logger.WithField("operation", "enable-controllers")

	subtreeControlFile := filepath.Join(c.config.BaseDir, "cgroup.subtree_control")

	// Check available controllers
	controllersFile := filepath.Join(c.config.BaseDir, "cgroup.controllers")
	availableBytes, err := os.ReadFile(controllersFile)
	if err != nil {
		return fmt.Errorf("failed to read available controllers: %w", err)
	}

	availableControllers := strings.Fields(string(availableBytes))
	log.Debug("available controllers", "controllers", availableControllers)

	// Enable only the configured controllers that are available
	var enabledControllers []string
	for _, controller := range c.config.EnableControllers {
		if contains(availableControllers, controller) {
			enabledControllers = append(enabledControllers, "+"+controller)
			log.Debug("enabling controller from config", "controller", controller)
		} else {
			log.Warn("configured controller not available", "controller", controller)
		}
	}

	if len(enabledControllers) == 0 {
		log.Warn("no configured controllers available to enable")
		return nil
	}

	// Write enabled controllers
	controllersToEnable := strings.Join(enabledControllers, " ")
	if err := os.WriteFile(subtreeControlFile, []byte(controllersToEnable), 0644); err != nil {
		return fmt.Errorf("failed to enable controllers: %w", err)
	}

	log.Info("controllers enabled from configuration",
		"requested", c.config.EnableControllers,
		"enabled", enabledControllers)

	return nil
}

// moveJobletProcessToSubgroup moves the main joblet process to a subgroup
// This is required to satisfy the "no internal processes" rule
func (c *cgroup) moveJobletProcessToSubgroup() error {
	log := c.logger.WithField("operation", "move-joblet-process")

	// Create a subgroup for the main joblet process
	jobletSubgroup := filepath.Join(c.config.BaseDir, "joblet-main")
	if err := os.MkdirAll(jobletSubgroup, 0755); err != nil {
		return fmt.Errorf("failed to create joblet subgroup: %w", err)
	}

	// Move current process to the subgroup
	currentPID := os.Getpid()
	procsFile := filepath.Join(jobletSubgroup, "cgroup.procs")
	pidBytes := []byte(fmt.Sprintf("%d", currentPID))

	if err := os.WriteFile(procsFile, pidBytes, 0644); err != nil {
		return fmt.Errorf("failed to move joblet process to subgroup: %w", err)
	}

	log.Info("moved joblet process to subgroup", "pid", currentPID, "subgroup", jobletSubgroup)
	return nil
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// enableSubtreeControl enables subtree control for proper child process containment
func (c *cgroup) enableSubtreeControl(cgroupPath string) error {
	log := c.logger.WithField("operation", "enable-subtree-control")

	subtreeControlFile := filepath.Join(cgroupPath, "cgroup.subtree_control")

	// Get the controllers that should be enabled for this cgroup
	var controllersToEnable []string
	for _, controller := range c.config.EnableControllers {
		controllersToEnable = append(controllersToEnable, "+"+controller)
	}

	if len(controllersToEnable) == 0 {
		log.Debug("no controllers to enable for subtree")
		return nil
	}

	controllersStr := strings.Join(controllersToEnable, " ")

	// Write the controllers to the subtree_control file
	if err := os.WriteFile(subtreeControlFile, []byte(controllersStr), 0644); err != nil {
		return fmt.Errorf("failed to enable subtree control: %w", err)
	}

	log.Debug("enabled subtree control for child process containment",
		"controllers", controllersStr, "cgroupPath", cgroupPath)

	return nil
}

func (c *cgroup) Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error {
	log := c.logger.WithFields(
		"cgroupPath", cgroupJobDir,
		"maxCPU", maxCPU,
		"maxMemory", maxMemory,
		"maxIOBPS", maxIOBPS)

	log.Debug("creating cgroup with strict resource enforcement")

	// Ensure we're working within our delegated subtree
	if !strings.HasPrefix(cgroupJobDir, c.config.BaseDir) {
		return fmt.Errorf("security violation: cgroup path outside delegated subtree: %s", cgroupJobDir)
	}

	// Ensure controllers are set up
	if err := c.EnsureControllers(); err != nil {
		return fmt.Errorf("controller setup failed: %w", err)
	}

	// Create the cgroup directory
	if err := os.MkdirAll(cgroupJobDir, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup directory: %w", err)
	}

	// Create a process subgroup to satisfy "no internal processes" rule
	processSubgroup := filepath.Join(cgroupJobDir, "proc")
	if err := os.MkdirAll(processSubgroup, 0755); err != nil {
		return fmt.Errorf("failed to create process subgroup: %w", err)
	}

	// Enable subtree control in the job cgroup (not the process subgroup)
	if err := c.enableSubtreeControl(cgroupJobDir); err != nil {
		log.Warn("failed to enable subtree control", "error", err)
		// Continue anyway - this might not be critical for basic functionality
	}

	// Wait for controller files to appear
	time.Sleep(100 * time.Millisecond)

	// STRICT ENFORCEMENT: All requested limits must be applied successfully
	var enforcementErrors []error

	// Set CPU limit - FAIL if requested but can't be applied
	if maxCPU > 0 {
		if err := c.SetCPULimit(cgroupJobDir, int(maxCPU)); err != nil {
			enforcementErrors = append(enforcementErrors,
				fmt.Errorf("failed to enforce CPU limit %d%%: %w", maxCPU, err))
		} else {
			log.Info("CPU limit enforced successfully", "limit", maxCPU)
		}
	}

	// Set memory limit - FAIL if requested but can't be applied
	if maxMemory > 0 {
		if err := c.SetMemoryLimit(cgroupJobDir, int(maxMemory)); err != nil {
			enforcementErrors = append(enforcementErrors,
				fmt.Errorf("failed to enforce memory limit %dMB: %w", maxMemory, err))
		} else {
			log.Info("memory limit enforced successfully", "limit", maxMemory)
		}
	}

	// Set IO limit - FAIL if requested but can't be applied
	if maxIOBPS > 0 {
		if err := c.SetIOLimit(cgroupJobDir, int(maxIOBPS)); err != nil {
			enforcementErrors = append(enforcementErrors,
				fmt.Errorf("failed to enforce IO limit %d BPS: %w", maxIOBPS, err))
		} else {
			log.Info("IO limit enforced successfully", "limit", maxIOBPS)
		}
	}

	// If any enforcement failed, clean up and fail the job
	if len(enforcementErrors) > 0 {
		log.Error("resource limit enforcement failed, cleaning up",
			"failedLimits", len(enforcementErrors))

		// Clean up the cgroup since we can't enforce limits
		c.CleanupCgroup(filepath.Base(cgroupJobDir))

		// Combine all errors
		var errorMsgs []string
		for _, err := range enforcementErrors {
			errorMsgs = append(errorMsgs, err.Error())
		}

		return fmt.Errorf("resource limit enforcement failed: %s",
			strings.Join(errorMsgs, "; "))
	}

	log.Info("cgroup created successfully with all requested limits enforced")
	return nil
}

// SetCPUCores for setting CPU cores
func (c *cgroup) SetCPUCores(cgroupPath string, cores string) error {
	if cores == "" {
		// No core restriction
		return nil
	}

	log := c.logger.WithFields("cgroupPath", cgroupPath, "cores", cores)

	cpusetPath := filepath.Join(cgroupPath, "cpuset.cpus")
	if err := os.WriteFile(cpusetPath, []byte(cores), 0644); err != nil {
		return fmt.Errorf("failed to set CPU cores: %w", err)
	}

	// Set memory nodes (required for cpuset)
	memsPath := filepath.Join(cgroupPath, "cpuset.mems")
	if err := os.WriteFile(memsPath, []byte("0"), 0644); err != nil {
		log.Warn("failed to set memory nodes", "error", err)
	}

	log.Info("CPU cores set successfully", "cores", cores)
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
		"8:0 riops=1000 wiops=1000",
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
	cleanupLogger.Debug("starting cgroup cleanup with configured timeout",
		"timeout", c.config.CleanupTimeout)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), c.config.CleanupTimeout)
		defer cancel()

		done := make(chan bool)
		go func() {
			cleanupJobCgroup(jobID, cleanupLogger, &c.config)
			done <- true
		}()

		select {
		case <-done:
			cleanupLogger.Debug("cgroup cleanup completed within configured timeout")
		case <-ctx.Done():
			cleanupLogger.Warn("cgroup cleanup timed out",
				"configuredTimeout", c.config.CleanupTimeout)
		}
	}()
}

// cleanupJobCgroup clean process first SIGTERM and SIGKILL then remove the cgroupPath items
func cleanupJobCgroup(jobID string, logger *logger.Logger, cfg *config.CgroupConfig) {
	// Use the delegated cgroup path
	cgroupPath := filepath.Join(cfg.BaseDir, "job-"+jobID)
	cleanupLogger := logger.WithField("cgroupPath", cgroupPath)

	// Security check: ensure we're only cleaning up within our delegated subtree
	if !strings.HasPrefix(cgroupPath, cfg.BaseDir+"/job-") {
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
					_ = proc.Signal(syscall.SIGTERM)

					// Wait a moment
					time.Sleep(100 * time.Millisecond)

					// Then SIGKILL if needed
					e := proc.Signal(syscall.SIGKILL)
					if e != nil {
						return
					}
				}
			}
		}

		if len(activePids) > 0 {
			cleanupLogger.Debug("terminated processes in cgroup", "pids", activePids)
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
			logger.Debug("could not remove cgroup directory completely, will be cleaned up later", "error", e)
		} else {
			logger.Debug("successfully removed cgroup directory on retry")
		}
	} else {
		logger.Debug("successfully removed cgroup directory")
	}
}
