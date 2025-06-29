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

	// Default resource limits
	DefaultCPULimitPercent int32
	DefaultMemoryLimitMB   int32
	DefaultIOBPS           int32

	// Mount point inside namespace
	CgroupNamespaceMount string
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

	if c.CgroupNamespaceMount == "" {
		return fmt.Errorf("CgroupNamespaceMount cannot be empty")
	}

	return nil
}
