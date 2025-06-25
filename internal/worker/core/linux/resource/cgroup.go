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
	CgroupsBaseDir = "/sys/fs/cgroup"
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
	logger *logger.Logger
}

func New() Resource {
	return &cgroup{
		logger: logger.New(),
	}
}

func (c *cgroup) Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error {

	log := c.logger.WithFields(
		"cgroupPath", cgroupJobDir,
		"maxCPU", maxCPU,
		"maxMemory", maxMemory,
		"maxIOBPS", maxIOBPS)

	log.Debug("creating cgroup")

	if err := os.MkdirAll(cgroupJobDir, 0755); err != nil {

		log.Error("failed to create cgroup directory", "error", err)
		return fmt.Errorf("failed to create cgroup directory: %v", err)
	}

	// using cpu.max for cgroup v2
	if err := c.SetCPULimit(cgroupJobDir, int(maxCPU)); err != nil {

		log.Error("failed to set CPU limit", "error", err)
		return err
	}

	if err := c.SetMemoryLimit(cgroupJobDir, int(maxMemory)); err != nil {

		log.Error("failed to set memory limit", "error", err)
		return err
	}

	if maxIOBPS > 0 {

		err := c.SetIOLimit(cgroupJobDir, int(maxIOBPS))
		if err != nil {

			log.Error("failed to set IO limit", "error", err)
			return err
		}
	}

	log.Info("cgroup created successfully")

	return nil
}

// SetIOLimit sets IO limits for a cgroup
func (c *cgroup) SetIOLimit(cgroupPath string, ioBPS int) error {

	log := c.logger.WithFields("cgroupPath", cgroupPath, "ioBPS", ioBPS)

	// check if io.max exists to confirm cgroup v2
	ioMaxPath := filepath.Join(cgroupPath, "io.max")
	if _, err := os.Stat(ioMaxPath); os.IsNotExist(err) {
		log.Error("io.max not found, cgroup v2 IO limiting not available")
		return fmt.Errorf("io.max not found, cgroup v2 IO limiting not available")
	}

	// check current device format by reading io.max
	currentConfig, err := os.ReadFile(ioMaxPath)
	if err != nil {
		log.Warn("couldn't read current io.max configuration", "error", err)
	} else {
		log.Debug("current io.max content", "content", string(currentConfig))
	}

	// trying different formats with valid device identification
	formats := []string{
		// device with just rbps (more likely to work)
		fmt.Sprintf("8:0 rbps=%d", ioBPS),

		// device with just wbps
		fmt.Sprintf("8:0 wbps=%d", ioBPS),

		// with "max" device syntax
		fmt.Sprintf("max rbps=%d", ioBPS),

		// with riops and wiops , operations per second instead of bytes
		fmt.Sprintf("8:0 riops=1000 wiops=1000"),

		// absolute device path
		fmt.Sprintf("/dev/sda rbps=%d", ioBPS),
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

	log.Error("all IO limit formats failed", "lastError", lastErr, "triedFormats", len(formats))

	return fmt.Errorf("all IO limit formats failed, last error: %w", lastErr)
}

// SetCPULimit sets CPU limits for the cgroup
func (c *cgroup) SetCPULimit(cgroupPath string, cpuLimit int) error {

	log := c.logger.WithFields("cgroupPath", cgroupPath, "cpuLimit", cpuLimit)

	// CPU controller files
	cpuMaxPath := filepath.Join(cgroupPath, "cpu.max")
	cpuWeightPath := filepath.Join(cgroupPath, "cpu.weight")

	// try cpu.max (cgroup v2)
	if _, err := os.Stat(cpuMaxPath); err == nil {

		// format: $MAX $PERIOD
		limit := fmt.Sprintf("%d 100000", cpuLimit*1000)
		if e := os.WriteFile(cpuMaxPath, []byte(limit), 0644); e != nil {
			log.Error("failed to write to cpu.max", "limit", limit, "error", e)
			return fmt.Errorf("failed to write to cpu.max: %w", e)
		}
		log.Info("set CPU limit with cpu.max", "limit", limit)
		return nil
	}

	// try cpu.weight as fallback (cgroup v2 alternative)
	if _, err := os.Stat(cpuWeightPath); err == nil {

		// convert CPU limit to weight (1-10000)

		// higher value = more CPU
		weight := 100 // Default
		if cpuLimit > 0 {
			// from typical CPU limit (e.g. 100 = 1 core) to weight range
			weight = int(10000 * (float64(cpuLimit) / 100.0))
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

	log.Error("neither cpu.max nor cpu.weight found")
	return fmt.Errorf("neither cpu.max nor cpu.weight found")
}

// SetMemoryLimit sets memory limits for the cgroup
func (c *cgroup) SetMemoryLimit(cgroupPath string, memoryLimitMB int) error {
	log := c.logger.WithFields("cgroupPath", cgroupPath, "memoryLimitMB", memoryLimitMB)

	// convert MB to bytes
	memoryLimitBytes := int64(memoryLimitMB) * 1024 * 1024

	// cgroup v2
	memoryMaxPath := filepath.Join(cgroupPath, "memory.max")
	memoryHighPath := filepath.Join(cgroupPath, "memory.high")

	var setMax, setHigh bool

	// set memory.max hard limit
	if _, err := os.Stat(memoryMaxPath); err == nil {
		if e := os.WriteFile(memoryMaxPath, []byte(fmt.Sprintf("%d", memoryLimitBytes)), 0644); e != nil {
			log.Warn("failed to write to memory.max", "memoryLimitBytes", memoryLimitBytes, "error", e)
		} else {
			setMax = true
			log.Info("set memory.max limit", "memoryLimitBytes", memoryLimitBytes)
		}
	}

	// set memory.high soft limit
	if _, err := os.Stat(memoryHighPath); err == nil {
		softLimit := int64(float64(memoryLimitBytes) * 0.9) // 90% of hard limit
		if e := os.WriteFile(memoryHighPath, []byte(fmt.Sprintf("%d", softLimit)), 0644); e != nil {
			log.Warn("failed to write to memory.high", "softLimit", softLimit, "error", e)
		} else {
			setHigh = true
			log.Info("set memory.high limit", "softLimit", softLimit)
		}
	}

	if !setMax && !setHigh {

		log.Error("neither memory.max nor memory.high found")

		return fmt.Errorf("neither memory.max nor memory.high found")
	}

	return nil
}

// CleanupCgroup deletes a cgroup after removing job processes
func (c *cgroup) CleanupCgroup(jobID string) {
	cleanupLogger := c.logger.WithField("jobId", jobID)
	cleanupLogger.Debug("starting cgroup cleanup")

	// cleanup in a separate goroutine
	go func() {
		// timeout for the cleanup operation
		ctx, cancel := context.WithTimeout(context.Background(), CleanupTimeout)
		defer cancel()

		done := make(chan bool)
		go func() {
			cleanupJobCgroup(jobID, cleanupLogger)
			done <- true
		}()

		// wait for cleanup or timeout
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

	cgroupPath := filepath.Join(CgroupsBaseDir, "job-"+jobID)
	cleanupLogger := logger.WithField("cgroupPath", cgroupPath)

	// check if the cgroup exists
	if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
		cleanupLogger.Debug("cgroup directory does not exist, skipping cleanup")
		return
	}

	// trying to kill any processes still in the cgroup
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

				// trying to terminate the process
				proc, e2 := os.FindProcess(pid)
				if e2 == nil {
					// trying SIGTERM first
					proc.Signal(syscall.SIGTERM)

					// wait a moment
					time.Sleep(100 * time.Millisecond)

					// then SIGKILL if needed
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

			// skip directories and read-only files like cgroup.events
			if file.IsDir() || strings.HasPrefix(file.Name(), "cgroup.") {
				continue
			}

			// to remove each file one by one
			filePath := filepath.Join(cgroupPath, file.Name())
			if e := os.Remove(filePath); e == nil {
				removedFiles = append(removedFiles, file.Name())
			}
		}

		if len(removedFiles) > 0 {
			logger.Debug("manually removed cgroup files", "files", removedFiles)
		}

		// try to remove the directory again
		if e := os.Remove(cgroupPath); e != nil {
			logger.Info("could not remove cgroup directory completely, will be cleaned up later", "error", e)
		} else {
			logger.Debug("successfully removed cgroup directory on retry")
		}

	} else {
		logger.Debug("successfully removed cgroup directory")
	}
}
