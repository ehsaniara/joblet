//go:build linux

package linux

import (
	"fmt"
	"path/filepath"
	"time"

	"job-worker/internal/config"
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

	// Cgroup namespace configuration (mandatory)
	CgroupNamespaceMount string // Mount point inside namespace

	// User namespace configuration
	UserNamespaceEnabled bool
	UserNamespaceConfig  *UserNamespaceConfig
}

// UserNamespaceConfig contains user namespace specific settings
type UserNamespaceConfig struct {
	BaseUID          uint32
	BaseGID          uint32
	RangeSize        uint32
	MaxJobs          uint32
	SubUIDFile       string
	SubGIDFile       string
	UseSetuidHelper  bool
	SetuidHelperPath string
}

// DefaultConfigWithUserNamespaces creates config
func DefaultConfigWithUserNamespaces() *Config {
	return &Config{
		CgroupsBaseDir:          config.CgroupsBaseDir,
		GracefulShutdownTimeout: 1 * time.Second,
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
		CgroupNamespaceMount:    "/sys/fs/cgroup",

		// User namespace configuration
		UserNamespaceEnabled: true,
		UserNamespaceConfig: &UserNamespaceConfig{
			BaseUID:          100000,
			BaseGID:          100000,
			RangeSize:        65536,
			MaxJobs:          100,
			SubUIDFile:       "/etc/subuid",
			SubGIDFile:       "/etc/subgid",
			UseSetuidHelper:  false,
			SetuidHelperPath: "/usr/bin/newuidmap",
		},
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

	// User namespace validation
	if c.UserNamespaceEnabled {
		if c.UserNamespaceConfig == nil {
			return fmt.Errorf("UserNamespaceConfig cannot be nil when user namespaces are enabled")
		}

		if c.UserNamespaceConfig.BaseUID < 1000 {
			return fmt.Errorf("BaseUID should be >= 1000 for security")
		}

		if c.UserNamespaceConfig.BaseGID < 1000 {
			return fmt.Errorf("BaseGID should be >= 1000 for security")
		}

		if c.UserNamespaceConfig.RangeSize == 0 {
			return fmt.Errorf("RangeSize cannot be zero")
		}

		if c.UserNamespaceConfig.MaxJobs == 0 {
			return fmt.Errorf("MaxJobs cannot be zero")
		}

		// Check if UID range would overflow
		maxUID := c.UserNamespaceConfig.BaseUID + (c.UserNamespaceConfig.MaxJobs * c.UserNamespaceConfig.RangeSize)
		if maxUID > 4294967295 { // uint32 max
			return fmt.Errorf("UID range would overflow: max UID would be %d", maxUID)
		}
	}

	return nil
}
