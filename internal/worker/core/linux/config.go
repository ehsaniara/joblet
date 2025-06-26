//go:build linux

package linux

import (
	"fmt"
	"path/filepath"
	"time"
)

const (
	// CgroupsBaseDir Use the delegated cgroup path for worker user
	CgroupsBaseDir = "/sys/fs/cgroup/worker.slice/worker.service"
)

type Config struct {
	// Cgroup resource management configuration
	CgroupsBaseDir string

	// Process lifecycle timeouts
	GracefulShutdownTimeout time.Duration
	ProcessStartTimeout     time.Duration
	CleanupTimeout          time.Duration

	// Default resource limits
	DefaultCPULimitPercent int32
	DefaultMemoryLimitMB   int32
	DefaultIOBPS           int32

	// Process limits
	MaxJobArgs           int
	MaxJobArgLength      int
	MaxEnvironmentVars   int
	MaxEnvironmentVarLen int

	// System limits
	MaxConcurrentJobs int32

	// Mount point inside namespace
	CgroupNamespaceMount string
}

// DefaultConfigWithCgroupNamespace creates config with mandatory cgroup namespaces
func DefaultConfigWithCgroupNamespace() *Config {
	return &Config{
		CgroupsBaseDir:          CgroupsBaseDir,
		GracefulShutdownTimeout: 100 * time.Millisecond,
		ProcessStartTimeout:     10 * time.Second,
		CleanupTimeout:          30 * time.Second,
		DefaultCPULimitPercent:  100,
		DefaultMemoryLimitMB:    512,
		DefaultIOBPS:            0,
		MaxJobArgs:              100,
		MaxJobArgLength:         1024,
		MaxEnvironmentVars:      1000,
		MaxEnvironmentVarLen:    8192,
		MaxConcurrentJobs:       100,
		CgroupNamespaceMount:    "/sys/fs/cgroup", // Standard mount point
	}
}

// BuildCgroupPath constructs the filesystem path for a job's cgroup
func (c *Config) BuildCgroupPath(jobID string) string {
	return filepath.Join(c.CgroupsBaseDir, "job-"+jobID)
}

// Validate ensures cgroup namespace requirements are met
func (c *Config) Validate() error {
	if c.CgroupsBaseDir == "" {
		return fmt.Errorf("CgroupsBaseDir cannot be empty")
	}

	if c.GracefulShutdownTimeout < 0 {
		return fmt.Errorf("GracefulShutdownTimeout cannot be negative")
	}

	if c.ProcessStartTimeout <= 0 {
		return fmt.Errorf("ProcessStartTimeout must be positive")
	}

	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("MaxConcurrentJobs must be positive")
	}

	if c.CgroupNamespaceMount == "" {
		return fmt.Errorf("CgroupNamespaceMount cannot be empty")
	}

	return nil
}
