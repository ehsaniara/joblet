//go:build linux

package linux

import (
	"fmt"
	"path/filepath"
	"time"

	"job-worker/internal/config"
)

// Config holds Linux platform-specific configuration
type Config struct {
	// Network paths
	NetnsPath   string // e.g., "/tmp/shared-netns/"
	VarRunNetns string // e.g., "/var/run/netns/"

	// Cgroup configuration
	CgroupsBaseDir string // e.g., "/sys/fs/cgroup/job-worker"

	// Timeouts
	GracefulShutdownTimeout time.Duration
	ProcessStartTimeout     time.Duration
	CleanupTimeout          time.Duration

	// Resource defaults
	DefaultCPULimitPercent int32
	DefaultMemoryLimitMB   int32
	DefaultIOBPS           int32

	// Retry settings
	DefaultRetryAttempts int
	DefaultCPUWeight     int

	// Process limits
	MaxJobArgs           int
	MaxJobArgLength      int
	MaxEnvironmentVars   int
	MaxEnvironmentVarLen int

	// System limits
	MaxConcurrentJobs int32
	MaxJobDuration    time.Duration
}

// Constants for default configuration values
const (
	// Network paths
	DefaultNetnsPath   = "/tmp/shared-netns/"
	DefaultVarRunNetns = "/var/run/netns/"

	// Timeouts
	DefaultGracefulShutdownTimeout = 100 * time.Millisecond
	DefaultProcessStartTimeout     = 10 * time.Second
	DefaultCleanupTimeout          = 30 * time.Second

	// Resource defaults
	DefaultCPULimitPercent = 100 // 100% = 1 core
	DefaultMemoryLimitMB   = 512 // 512 MB
	DefaultIOBPS           = 0   // No limit

	// Retry settings
	DefaultRetryAttempts = 10
	DefaultCPUWeight     = 100

	// Process limits
	DefaultMaxJobArgs           = 100
	DefaultMaxJobArgLength      = 1024
	DefaultMaxEnvironmentVars   = 1000
	DefaultMaxEnvironmentVarLen = 8192

	// System limits
	DefaultMaxConcurrentJobs = 100
	DefaultMaxJobDuration    = 24 * time.Hour
)

// DefaultConfig creates a configuration with default values
func DefaultConfig() *Config {
	return &Config{
		// Network paths
		NetnsPath:   DefaultNetnsPath,
		VarRunNetns: DefaultVarRunNetns,

		// Cgroup configuration
		CgroupsBaseDir: config.CgroupsBaseDir, // Use from main config

		// Timeouts
		GracefulShutdownTimeout: DefaultGracefulShutdownTimeout,
		ProcessStartTimeout:     DefaultProcessStartTimeout,
		CleanupTimeout:          DefaultCleanupTimeout,

		// Resource defaults
		DefaultCPULimitPercent: DefaultCPULimitPercent,
		DefaultMemoryLimitMB:   DefaultMemoryLimitMB,
		DefaultIOBPS:           DefaultIOBPS,

		// Retry settings
		DefaultRetryAttempts: DefaultRetryAttempts,
		DefaultCPUWeight:     DefaultCPUWeight,

		// Process limits
		MaxJobArgs:           DefaultMaxJobArgs,
		MaxJobArgLength:      DefaultMaxJobArgLength,
		MaxEnvironmentVars:   DefaultMaxEnvironmentVars,
		MaxEnvironmentVarLen: DefaultMaxEnvironmentVarLen,

		// System limits
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		MaxJobDuration:    DefaultMaxJobDuration,
	}
}

// BuildCgroupPath builds a cgroup path for a given job ID
func (c *Config) BuildCgroupPath(jobID string) string {
	return filepath.Join(c.CgroupsBaseDir, "job-"+jobID)
}

// BuildNamespacePath builds a namespace path for a given job ID (isolated jobs)
func (c *Config) BuildNamespacePath(jobID string) string {
	return filepath.Join(c.NetnsPath, jobID)
}

// BuildNetworkGroupPath builds a namespace path for a network group
func (c *Config) BuildNetworkGroupPath(groupID string) string {
	return filepath.Join(c.VarRunNetns, groupID)
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.NetnsPath == "" {
		return fmt.Errorf("NetnsPath cannot be empty")
	}

	if c.VarRunNetns == "" {
		return fmt.Errorf("VarRunNetns cannot be empty")
	}

	if c.CgroupsBaseDir == "" {
		return fmt.Errorf("CgroupsBaseDir cannot be empty")
	}

	if c.GracefulShutdownTimeout < 0 {
		return fmt.Errorf("GracefulShutdownTimeout cannot be negative")
	}

	if c.ProcessStartTimeout <= 0 {
		return fmt.Errorf("ProcessStartTimeout must be positive")
	}

	if c.CleanupTimeout <= 0 {
		return fmt.Errorf("CleanupTimeout must be positive")
	}

	if c.DefaultCPULimitPercent < 0 {
		return fmt.Errorf("DefaultCPULimitPercent cannot be negative")
	}

	if c.DefaultMemoryLimitMB < 0 {
		return fmt.Errorf("DefaultMemoryLimitMB cannot be negative")
	}

	if c.DefaultIOBPS < 0 {
		return fmt.Errorf("DefaultIOBPS cannot be negative")
	}

	if c.MaxJobArgs <= 0 {
		return fmt.Errorf("MaxJobArgs must be positive")
	}

	if c.MaxJobArgLength <= 0 {
		return fmt.Errorf("MaxJobArgLength must be positive")
	}

	if c.MaxEnvironmentVars <= 0 {
		return fmt.Errorf("MaxEnvironmentVars must be positive")
	}

	if c.MaxEnvironmentVarLen <= 0 {
		return fmt.Errorf("MaxEnvironmentVarLen must be positive")
	}

	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("MaxConcurrentJobs must be positive")
	}

	if c.MaxJobDuration <= 0 {
		return fmt.Errorf("MaxJobDuration must be positive")
	}

	return nil
}

// Clone creates a deep copy of the configuration
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}

	return &Config{
		NetnsPath:               c.NetnsPath,
		VarRunNetns:             c.VarRunNetns,
		CgroupsBaseDir:          c.CgroupsBaseDir,
		GracefulShutdownTimeout: c.GracefulShutdownTimeout,
		ProcessStartTimeout:     c.ProcessStartTimeout,
		CleanupTimeout:          c.CleanupTimeout,
		DefaultCPULimitPercent:  c.DefaultCPULimitPercent,
		DefaultMemoryLimitMB:    c.DefaultMemoryLimitMB,
		DefaultIOBPS:            c.DefaultIOBPS,
		DefaultRetryAttempts:    c.DefaultRetryAttempts,
		DefaultCPUWeight:        c.DefaultCPUWeight,
		MaxJobArgs:              c.MaxJobArgs,
		MaxJobArgLength:         c.MaxJobArgLength,
		MaxEnvironmentVars:      c.MaxEnvironmentVars,
		MaxEnvironmentVarLen:    c.MaxEnvironmentVarLen,
		MaxConcurrentJobs:       c.MaxConcurrentJobs,
		MaxJobDuration:          c.MaxJobDuration,
	}
}

// GetResourceDefaults returns the default resource limits
func (c *Config) GetResourceDefaults() (cpu int32, memory int32, ioBPS int32) {
	return c.DefaultCPULimitPercent, c.DefaultMemoryLimitMB, c.DefaultIOBPS
}

// GetTimeouts returns the configured timeouts
func (c *Config) GetTimeouts() (graceful, start, cleanup time.Duration) {
	return c.GracefulShutdownTimeout, c.ProcessStartTimeout, c.CleanupTimeout
}

// GetProcessLimits returns the process-related limits
func (c *Config) GetProcessLimits() (maxArgs, maxArgLen, maxEnvVars, maxEnvVarLen int) {
	return c.MaxJobArgs, c.MaxJobArgLength, c.MaxEnvironmentVars, c.MaxEnvironmentVarLen
}

// GetSystemLimits returns the system-level limits
func (c *Config) GetSystemLimits() (maxJobs int32, maxDuration time.Duration) {
	return c.MaxConcurrentJobs, c.MaxJobDuration
}

// IsValidJobID validates a job ID format
func (c *Config) IsValidJobID(jobID string) bool {
	if jobID == "" {
		return false
	}

	if len(jobID) > 64 {
		return false
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	for _, char := range jobID {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return false
		}
	}

	return true
}

// IsValidNetworkGroupID validates a network group ID format
func (c *Config) IsValidNetworkGroupID(groupID string) bool {
	// Same validation as job ID for now
	return c.IsValidJobID(groupID)
}

// GetNetworkPaths returns the network namespace paths
func (c *Config) GetNetworkPaths() (netns, varRunNetns string) {
	return c.NetnsPath, c.VarRunNetns
}

// SetNetworkPaths sets the network namespace paths
func (c *Config) SetNetworkPaths(netns, varRunNetns string) {
	c.NetnsPath = netns
	c.VarRunNetns = varRunNetns
}

// SetResourceDefaults sets the default resource limits
func (c *Config) SetResourceDefaults(cpu, memory, ioBPS int32) {
	c.DefaultCPULimitPercent = cpu
	c.DefaultMemoryLimitMB = memory
	c.DefaultIOBPS = ioBPS
}

// SetTimeouts sets the timeout values
func (c *Config) SetTimeouts(graceful, start, cleanup time.Duration) {
	c.GracefulShutdownTimeout = graceful
	c.ProcessStartTimeout = start
	c.CleanupTimeout = cleanup
}

// LoadFromEnvironment loads configuration from environment variables
func (c *Config) LoadFromEnvironment() {
	// You could implement environment variable loading here
	// For now, we'll use the default values
	// Example:
	// if netnsPath := os.Getenv("WORKER_NETNS_PATH"); netnsPath != "" {
	//     c.NetnsPath = netnsPath
	// }
}

// ToMap converts the configuration to a map for debugging/logging
func (c *Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"netns_path":                c.NetnsPath,
		"var_run_netns":             c.VarRunNetns,
		"cgroups_base_dir":          c.CgroupsBaseDir,
		"graceful_shutdown_timeout": c.GracefulShutdownTimeout.String(),
		"process_start_timeout":     c.ProcessStartTimeout.String(),
		"cleanup_timeout":           c.CleanupTimeout.String(),
		"default_cpu_limit_percent": c.DefaultCPULimitPercent,
		"default_memory_limit_mb":   c.DefaultMemoryLimitMB,
		"default_io_bps":            c.DefaultIOBPS,
		"default_retry_attempts":    c.DefaultRetryAttempts,
		"default_cpu_weight":        c.DefaultCPUWeight,
		"max_job_args":              c.MaxJobArgs,
		"max_job_arg_length":        c.MaxJobArgLength,
		"max_environment_vars":      c.MaxEnvironmentVars,
		"max_environment_var_len":   c.MaxEnvironmentVarLen,
		"max_concurrent_jobs":       c.MaxConcurrentJobs,
		"max_job_duration":          c.MaxJobDuration.String(),
	}
}

// String returns a string representation of the configuration
func (c *Config) String() string {
	return fmt.Sprintf("Config{NetnsPath: %s, VarRunNetns: %s, CgroupsBaseDir: %s, ...}",
		c.NetnsPath, c.VarRunNetns, c.CgroupsBaseDir)
}

// Constants that were previously scattered in the service file
const (
	// System call numbers
	SysSetnsX86_64 = 308 // x86_64 syscall number for setns

	// Network defaults (re-exported for backward compatibility)
	InternalSubnet  = "172.20.0.0/24"
	InternalGateway = "172.20.0.1"
	InternalIface   = "internal0"
	BaseNetwork     = "172.20.0.0/16"
)

// Global constants that were previously in the service file
var (
	GracefulShutdownTimeout = DefaultGracefulShutdownTimeout
	ProcessStartTimeout     = DefaultProcessStartTimeout
)
