//go:build linux

// Package linux provides Linux-specific configuration for the job worker system.
// This package contains all configuration parameters needed to run job processes
// with proper isolation using Linux namespaces, cgroups, and network management.
//
// The configuration system supports:
// - Network namespace isolation with configurable paths
// - Cgroup resource management with customizable limits
// - Process lifecycle timeouts and retry mechanisms
// - Security constraints and validation rules
// - Environment-based configuration overrides
package linux

import (
	"fmt"
	"path/filepath"
	"time"

	"job-worker/internal/config"
)

// Config holds Linux platform-specific configuration parameters.
// This structure centralizes all settings needed for job execution on Linux systems,
// including resource limits, namespace paths, timeouts, and security constraints.
//
// The configuration is designed to be:
// - Immutable after creation (use Clone() for modifications)
// - Validatable (Validate() ensures consistency)
// - Serializable (ToMap() for debugging/monitoring)
// - Environment-configurable (LoadFromEnvironment() for overrides)
//
// Thread Safety: Config instances are read-only after creation and can be
// safely shared across goroutines. Use Clone() to create mutable copies.
type Config struct {
	// Network namespace configuration
	// These paths determine where network namespace references are stored
	// for both temporary (isolated jobs) and persistent (network groups) namespaces
	NetnsPath   string // Temporary namespace storage, e.g., "/tmp/shared-netns/"
	VarRunNetns string // Persistent namespace storage, e.g., "/var/run/netns/"

	// Cgroup resource management configuration
	// Base directory where job-specific cgroups will be created
	// Each job gets a subdirectory: {CgroupsBaseDir}/job-{jobID}
	CgroupsBaseDir string // e.g., "/sys/fs/cgroup/job-worker"

	// Process lifecycle timeouts
	// These control how long various operations are allowed to take
	GracefulShutdownTimeout time.Duration // Time to wait for SIGTERM before SIGKILL
	ProcessStartTimeout     time.Duration // Maximum time to wait for process startup
	CleanupTimeout          time.Duration // Maximum time for resource cleanup operations

	// Default resource limits applied when not specified by user
	// These provide sensible defaults while preventing resource exhaustion
	DefaultCPULimitPercent int32 // CPU limit as percentage (100 = 1 core)
	DefaultMemoryLimitMB   int32 // Memory limit in megabytes
	DefaultIOBPS           int32 // I/O bandwidth limit in bytes per second (0 = unlimited)

	// Retry and recovery settings
	// Used for operations that may fail due to timing or system state
	DefaultRetryAttempts int // Number of retry attempts for transient failures
	DefaultCPUWeight     int // Default CPU weight for cgroup scheduling

	// Process argument and environment validation limits
	// These prevent resource exhaustion from malicious or malformed job requests
	MaxJobArgs           int // Maximum number of command-line arguments
	MaxJobArgLength      int // Maximum length of individual arguments
	MaxEnvironmentVars   int // Maximum number of environment variables
	MaxEnvironmentVarLen int // Maximum length of environment variable values

	// System-level operational limits
	// Controls overall system resource usage and prevents overload
	MaxConcurrentJobs int32 // Maximum number of jobs running simultaneously
}

// Configuration constants with carefully chosen default values.
// These defaults are based on:
// - Common Linux system capabilities
// - Security best practices
// - Performance considerations
// - Resource availability on typical systems
const (
	// Network namespace paths
	// Temporary namespaces use /tmp for automatic cleanup on reboot
	// Persistent namespaces use /var/run for system-managed lifecycle
	DefaultNetnsPath   = "/tmp/shared-netns/"
	DefaultVarRunNetns = "/var/run/netns/"

	// Process lifecycle timeouts
	// Short graceful timeout prevents hanging during shutdown
	// Longer startup timeout accommodates complex initialization
	// Extended cleanup timeout handles complex resource dependencies
	DefaultGracefulShutdownTimeout = 100 * time.Millisecond // Quick shutdown for responsiveness
	DefaultProcessStartTimeout     = 10 * time.Second       // Allow time for process initialization
	DefaultCleanupTimeout          = 30 * time.Second       // Comprehensive resource cleanup

	// Resource defaults - conservative but usable
	// 100% CPU = 1 full core, reasonable for most jobs
	// 512MB memory = sufficient for typical tasks without waste
	// 0 I/O limit = unlimited, can be restricted per job if needed
	DefaultCPULimitPercent = 100 // 1 CPU core equivalent
	DefaultMemoryLimitMB   = 512 // 512 MB - conservative default
	DefaultIOBPS           = 0   // No I/O limit by default

	// Retry and scheduling parameters
	// 10 retries handle most transient system issues
	// CPU weight 100 provides balanced scheduling
	DefaultRetryAttempts = 10
	DefaultCPUWeight     = 100

	// Process security limits - prevent abuse while allowing legitimate use
	// These limits are generous enough for real workloads but prevent DoS attacks
	DefaultMaxJobArgs           = 100  // Reasonable argument count
	DefaultMaxJobArgLength      = 1024 // 1KB per argument
	DefaultMaxEnvironmentVars   = 1000 // Generous environment space
	DefaultMaxEnvironmentVarLen = 8192 // 8KB per environment variable

	// System capacity limits
	// 100 concurrent jobs balanced between capability and resource protection
	DefaultMaxConcurrentJobs = 100
)

// DefaultConfig creates a new configuration with sensible default values.
// This factory function ensures all configuration fields are properly initialized
// with values suitable for production use while maintaining security.
//
// The returned configuration:
// - Uses secure default resource limits
// - Configures standard Linux filesystem paths
// - Sets conservative timeouts
// - Enables reasonable concurrency
//
// Returns: A fully configured Config instance ready for production use
func DefaultConfig() *Config {
	return &Config{
		// Network namespace paths - use Linux standard locations
		NetnsPath:   DefaultNetnsPath,   // Temporary namespace references
		VarRunNetns: DefaultVarRunNetns, // Persistent namespace references

		// Cgroup configuration - integrates with main config system
		CgroupsBaseDir: config.CgroupsBaseDir, // Shared with main application config

		// Timeout configuration - balanced for responsiveness and reliability
		GracefulShutdownTimeout: DefaultGracefulShutdownTimeout, // Quick shutdown
		ProcessStartTimeout:     DefaultProcessStartTimeout,     // Reasonable startup time
		CleanupTimeout:          DefaultCleanupTimeout,          // Thorough cleanup

		// Resource defaults - conservative but functional
		DefaultCPULimitPercent: DefaultCPULimitPercent, // 1 CPU core
		DefaultMemoryLimitMB:   DefaultMemoryLimitMB,   // 512 MB
		DefaultIOBPS:           DefaultIOBPS,           // Unlimited I/O

		// Retry and scheduling settings
		DefaultRetryAttempts: DefaultRetryAttempts, // Handle transient failures
		DefaultCPUWeight:     DefaultCPUWeight,     // Balanced CPU scheduling

		// Process validation limits - security-focused
		MaxJobArgs:           DefaultMaxJobArgs,           // Prevent argument flooding
		MaxJobArgLength:      DefaultMaxJobArgLength,      // Limit argument size
		MaxEnvironmentVars:   DefaultMaxEnvironmentVars,   // Prevent environment abuse
		MaxEnvironmentVarLen: DefaultMaxEnvironmentVarLen, // Limit variable size

		// System capacity management
		MaxConcurrentJobs: DefaultMaxConcurrentJobs, // Prevent resource exhaustion
	}
}

// BuildCgroupPath constructs the filesystem path for a job's cgroup.
// Each job gets its own cgroup directory for resource isolation and management.
//
// Path Structure:
//
//	{CgroupsBaseDir}/job-{jobID}/
//	├── cgroup.procs      ← Process membership
//	├── cpu.max           ← CPU limits
//	├── memory.max        ← Memory limits
//	└── io.max            ← I/O limits
//
// Parameters:
//
//	jobID: Unique identifier for the job (must be valid filesystem name)
//
// Returns: Absolute path to the job's cgroup directory
//
// Example:
//
//	config.BuildCgroupPath("12345")
//	// Returns: "/sys/fs/cgroup/job-worker/job-12345"
func (c *Config) BuildCgroupPath(jobID string) string {
	// Prefix job ID to avoid conflicts with system cgroups
	// and make job cgroups easily identifiable
	return filepath.Join(c.CgroupsBaseDir, "job-"+jobID)
}

// BuildNamespacePath constructs the filesystem path for an isolated job's namespace.
// Isolated jobs (those without network groups) get temporary namespace references
// that are automatically cleaned up when the job completes.
//
// Namespace Lifecycle for Isolated Jobs:
//  1. Job starts → namespace created
//  2. Symlink created: {NetnsPath}/{jobID} → /proc/{pid}/ns/net
//  3. Job completes → symlink becomes invalid
//  4. Cleanup removes symlink
//
// Parameters:
//
//	jobID: Unique identifier for the job
//
// Returns: Absolute path to the namespace symlink
//
// Example:
//
//	config.BuildNamespacePath("12345")
//	// Returns: "/tmp/shared-netns/12345"
func (c *Config) BuildNamespacePath(jobID string) string {
	return filepath.Join(c.NetnsPath, jobID)
}

// BuildNetworkGroupPath constructs the filesystem path for a network group's namespace.
// Network groups use persistent namespace references that survive individual job
// lifecycle and allow multiple jobs to share the same network environment.
//
// Namespace Lifecycle for Network Groups:
//  1. First job in group starts → namespace created
//  2. Bind mount created: {VarRunNetns}/{groupID} ← /proc/{pid}/ns/net
//  3. Additional jobs join existing namespace
//  4. Last job completes → bind mount removed
//
// Parameters:
//
//	groupID: Unique identifier for the network group
//
// Returns: Absolute path to the persistent namespace bind mount
//
// Example:
//
//	config.BuildNetworkGroupPath("web-services")
//	// Returns: "/var/run/netns/web-services"
func (c *Config) BuildNetworkGroupPath(groupID string) string {
	return filepath.Join(c.VarRunNetns, groupID)
}

// Validate performs comprehensive validation of the configuration.
// This method ensures all configuration values are valid, consistent,
// and suitable for safe operation.
//
// Validation Checks:
// - Path fields are non-empty (required for filesystem operations)
// - Timeouts are positive (negative timeouts would cause immediate failure)
// - Resource limits are non-negative (negative limits are meaningless)
// - Process limits are positive (zero limits would reject all jobs)
// - System limits are positive (zero would prevent any job execution)
//
// Returns: nil if configuration is valid, descriptive error otherwise
//
// Example:
//
//	config := DefaultConfig()
//	config.MaxConcurrentJobs = -1 // Invalid!
//	if err := config.Validate(); err != nil {
//	    log.Fatal("Invalid configuration: %v", err)
//	}
func (c *Config) Validate() error {
	// Validate required filesystem paths
	// These are essential for namespace and cgroup operations
	if c.NetnsPath == "" {
		return fmt.Errorf("NetnsPath cannot be empty - required for isolated job namespaces")
	}
	if c.VarRunNetns == "" {
		return fmt.Errorf("VarRunNetns cannot be empty - required for network group namespaces")
	}
	if c.CgroupsBaseDir == "" {
		return fmt.Errorf("CgroupsBaseDir cannot be empty - required for resource management")
	}

	// Validate timeout configurations
	// Negative timeouts would cause immediate failures
	if c.GracefulShutdownTimeout < 0 {
		return fmt.Errorf("GracefulShutdownTimeout cannot be negative")
	}
	if c.ProcessStartTimeout <= 0 {
		return fmt.Errorf("ProcessStartTimeout must be positive - needed for process startup")
	}
	if c.CleanupTimeout <= 0 {
		return fmt.Errorf("CleanupTimeout must be positive - needed for resource cleanup")
	}

	// Validate resource limit defaults
	// Negative values are meaningless for resource limits
	if c.DefaultCPULimitPercent < 0 {
		return fmt.Errorf("DefaultCPULimitPercent cannot be negative")
	}
	if c.DefaultMemoryLimitMB < 0 {
		return fmt.Errorf("DefaultMemoryLimitMB cannot be negative")
	}
	if c.DefaultIOBPS < 0 {
		return fmt.Errorf("DefaultIOBPS cannot be negative")
	}

	// Validate process limits - these prevent abuse
	// Zero limits would make the system unusable
	if c.MaxJobArgs <= 0 {
		return fmt.Errorf("MaxJobArgs must be positive - needed to allow job arguments")
	}
	if c.MaxJobArgLength <= 0 {
		return fmt.Errorf("MaxJobArgLength must be positive - needed to allow argument content")
	}
	if c.MaxEnvironmentVars <= 0 {
		return fmt.Errorf("MaxEnvironmentVars must be positive - needed for job environment")
	}
	if c.MaxEnvironmentVarLen <= 0 {
		return fmt.Errorf("MaxEnvironmentVarLen must be positive - needed for environment content")
	}

	// Validate system capacity limits
	// Zero would prevent any job execution
	if c.MaxConcurrentJobs <= 0 {
		return fmt.Errorf("MaxConcurrentJobs must be positive - needed to allow job execution")
	}

	return nil
}

// Clone creates a deep copy of the configuration.
// This method enables safe modification of configuration without affecting
// the original instance, supporting immutable configuration patterns.
//
// Use Cases:
// - Creating environment-specific configurations
// - Testing with modified parameters
// - Runtime configuration adjustments
// - Template-based configuration generation
//
// Returns: A new Config instance with identical values, or nil if source is nil
//
// Example:
//
//	baseConfig := DefaultConfig()
//	testConfig := baseConfig.Clone()
//	testConfig.MaxConcurrentJobs = 10 // Modify copy only
//
//	// baseConfig.MaxConcurrentJobs still equals 100
//	// testConfig.MaxConcurrentJobs equals 10
func (c *Config) Clone() *Config {
	// Handle nil receiver gracefully
	if c == nil {
		return nil
	}

	// Create new instance with copied values
	// All fields are value types, so shallow copy is sufficient
	return &Config{
		// Network configuration
		NetnsPath:   c.NetnsPath,
		VarRunNetns: c.VarRunNetns,

		// Cgroup configuration
		CgroupsBaseDir: c.CgroupsBaseDir,

		// Timeout configuration
		GracefulShutdownTimeout: c.GracefulShutdownTimeout,
		ProcessStartTimeout:     c.ProcessStartTimeout,
		CleanupTimeout:          c.CleanupTimeout,

		// Resource defaults
		DefaultCPULimitPercent: c.DefaultCPULimitPercent,
		DefaultMemoryLimitMB:   c.DefaultMemoryLimitMB,
		DefaultIOBPS:           c.DefaultIOBPS,

		// Retry settings
		DefaultRetryAttempts: c.DefaultRetryAttempts,
		DefaultCPUWeight:     c.DefaultCPUWeight,

		// Process limits
		MaxJobArgs:           c.MaxJobArgs,
		MaxJobArgLength:      c.MaxJobArgLength,
		MaxEnvironmentVars:   c.MaxEnvironmentVars,
		MaxEnvironmentVarLen: c.MaxEnvironmentVarLen,

		// System limits
		MaxConcurrentJobs: c.MaxConcurrentJobs,
	}
}

// GetResourceDefaults returns the default resource limits as individual values.
// This convenience method provides easy access to resource defaults for
// job creation and validation logic.
//
// Returns:
//
//	cpu:    Default CPU limit in percentage (100 = 1 core)
//	memory: Default memory limit in megabytes
//	ioBPS:  Default I/O limit in bytes per second (0 = unlimited)
//
// Example:
//
//	cpu, memory, io := config.GetResourceDefaults()
//	fmt.Printf("Defaults: %d%% CPU, %d MB memory, %d B/s I/O", cpu, memory, io)
func (c *Config) GetResourceDefaults() (cpu int32, memory int32, ioBPS int32) {
	return c.DefaultCPULimitPercent, c.DefaultMemoryLimitMB, c.DefaultIOBPS
}

// GetTimeouts returns the configured timeout values.
// This convenience method provides structured access to all timeout settings
// for process lifecycle management.
//
// Returns:
//
//	graceful: Time to wait for graceful shutdown before force kill
//	start:    Maximum time to wait for process startup
//	cleanup:  Maximum time for resource cleanup operations
//
// Example:
//
//	graceful, start, cleanup := config.GetTimeouts()
//	fmt.Printf("Timeouts: %v graceful, %v start, %v cleanup", graceful, start, cleanup)
func (c *Config) GetTimeouts() (graceful, start, cleanup time.Duration) {
	return c.GracefulShutdownTimeout, c.ProcessStartTimeout, c.CleanupTimeout
}

// GetProcessLimits returns the process-related validation limits.
// These limits prevent abuse through oversized arguments or environments.
//
// Returns:
//
//	maxArgs:      Maximum number of command-line arguments
//	maxArgLen:    Maximum length of individual arguments
//	maxEnvVars:   Maximum number of environment variables
//	maxEnvVarLen: Maximum length of environment variable values
//
// Example:
//
//	maxArgs, maxArgLen, maxEnvs, maxEnvLen := config.GetProcessLimits()
//	if len(args) > maxArgs {
//	    return fmt.Errorf("too many arguments: %d > %d", len(args), maxArgs)
//	}
func (c *Config) GetProcessLimits() (maxArgs, maxArgLen, maxEnvVars, maxEnvVarLen int) {
	return c.MaxJobArgs, c.MaxJobArgLength, c.MaxEnvironmentVars, c.MaxEnvironmentVarLen
}

// GetSystemLimits returns the system-level operational limits.
// These limits control overall system resource usage.
//
// Returns:
//
//	maxJobs: Maximum number of jobs that can run concurrently
//
// Example:
//
//	maxJobs := config.GetSystemLimits()
//	if currentJobs >= maxJobs {
//	    return fmt.Errorf("system at capacity: %d jobs running", currentJobs)
//	}
func (c *Config) GetSystemLimits() (maxJobs int32) {
	return c.MaxConcurrentJobs
}

// IsValidJobID validates a job ID format according to security requirements.
// Job IDs are used in filesystem paths and must be safe for file system operations.
//
// Validation Rules:
// - Non-empty (required for identification)
// - Length ≤ 64 characters (prevents path length issues)
// - Alphanumeric characters, dash, underscore only (filesystem-safe)
// - No path traversal sequences (security)
//
// Parameters:
//
//	jobID: The job identifier to validate
//
// Returns: true if jobID is valid and safe to use, false otherwise
//
// Example:
//
//	if !config.IsValidJobID("job-123_test") {
//	    return fmt.Errorf("invalid job ID format")
//	}
//	// jobID is safe to use in filesystem operations
func (c *Config) IsValidJobID(jobID string) bool {
	// Reject empty IDs
	if jobID == "" {
		return false
	}

	// Prevent excessively long IDs that could cause path issues
	if len(jobID) > 64 {
		return false
	}

	// Validate each character for filesystem safety
	for _, char := range jobID {
		// Allow alphanumeric characters
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') {
			continue
		}
		// Allow safe punctuation
		if char == '-' || char == '_' {
			continue
		}
		// Reject any other characters (including path separators)
		return false
	}

	return true
}

// IsValidNetworkGroupID validates a network group ID format.
// Network group IDs follow the same rules as job IDs since they're also
// used in filesystem paths and must be equally safe.
//
// Parameters:
//
//	groupID: The network group identifier to validate
//
// Returns: true if groupID is valid and safe to use, false otherwise
//
// Example:
//
//	if !config.IsValidNetworkGroupID("web-services_v2") {
//	    return fmt.Errorf("invalid network group ID format")
//	}
func (c *Config) IsValidNetworkGroupID(groupID string) bool {
	// Use same validation as job IDs for consistency and safety
	return c.IsValidJobID(groupID)
}

// GetNetworkPaths returns the configured network namespace paths.
// This method provides access to both temporary and persistent namespace
// storage locations for network management operations.
//
// Returns:
//
//	netns:       Path for temporary namespace references (isolated jobs)
//	varRunNetns: Path for persistent namespace references (network groups)
//
// Example:
//
//	tempPath, persistentPath := config.GetNetworkPaths()
//	fmt.Printf("Isolated jobs: %s, Network groups: %s", tempPath, persistentPath)
func (c *Config) GetNetworkPaths() (netns, varRunNetns string) {
	return c.NetnsPath, c.VarRunNetns
}

// SetNetworkPaths updates the network namespace path configuration.
// This method allows runtime modification of namespace storage locations.
//
// Parameters:
//
//	netns:       New path for temporary namespace references
//	varRunNetns: New path for persistent namespace references
//
// Note: Changes affect future operations only. Existing namespaces
// remain at their original locations.
//
// Example:
//
//	config.SetNetworkPaths("/custom/netns", "/custom/persistent")
func (c *Config) SetNetworkPaths(netns, varRunNetns string) {
	c.NetnsPath = netns
	c.VarRunNetns = varRunNetns
}

// SetResourceDefaults updates the default resource limit configuration.
// This method allows runtime modification of resource defaults applied
// to jobs that don't specify explicit limits.
//
// Parameters:
//
//	cpu:    New default CPU limit in percentage (100 = 1 core)
//	memory: New default memory limit in megabytes
//	ioBPS:  New default I/O limit in bytes per second (0 = unlimited)
//
// Example:
//
//	// Set more generous defaults for a development environment
//	config.SetResourceDefaults(200, 1024, 0) // 2 cores, 1GB, unlimited I/O
func (c *Config) SetResourceDefaults(cpu, memory, ioBPS int32) {
	c.DefaultCPULimitPercent = cpu
	c.DefaultMemoryLimitMB = memory
	c.DefaultIOBPS = ioBPS
}

// SetTimeouts updates the timeout configuration.
// This method allows runtime modification of process lifecycle timeouts.
//
// Parameters:
//
//	graceful: New graceful shutdown timeout
//	start:    New process startup timeout
//	cleanup:  New resource cleanup timeout
//
// Example:
//
//	// Set longer timeouts for slow environments
//	config.SetTimeouts(5*time.Second, 30*time.Second, 60*time.Second)
func (c *Config) SetTimeouts(graceful, start, cleanup time.Duration) {
	c.GracefulShutdownTimeout = graceful
	c.ProcessStartTimeout = start
	c.CleanupTimeout = cleanup
}

// LoadFromEnvironment loads configuration overrides from environment variables.
// This method enables deployment-specific configuration without code changes.
//
// Environment Variables:
//
//	WORKER_NETNS_PATH              → NetnsPath
//	WORKER_VAR_RUN_NETNS          → VarRunNetns
//	WORKER_CGROUPS_BASE_DIR       → CgroupsBaseDir
//	WORKER_DEFAULT_CPU_PERCENT    → DefaultCPULimitPercent
//	WORKER_DEFAULT_MEMORY_MB      → DefaultMemoryLimitMB
//	WORKER_DEFAULT_IO_BPS         → DefaultIOBPS
//	WORKER_MAX_CONCURRENT_JOBS    → MaxConcurrentJobs
//
// Note: Invalid environment values are ignored, preserving current config.
//
// Example:
//
//	// Set environment: WORKER_MAX_CONCURRENT_JOBS=50
//	config.LoadFromEnvironment()
//	// config.MaxConcurrentJobs now equals 50
func (c *Config) LoadFromEnvironment() {
	// TODO: Implement environment variable loading
	// This is a placeholder for future implementation
	//
	// Example implementation:
	// if netnsPath := os.Getenv("WORKER_NETNS_PATH"); netnsPath != "" {
	//     c.NetnsPath = netnsPath
	// }
	// if maxJobsStr := os.Getenv("WORKER_MAX_CONCURRENT_JOBS"); maxJobsStr != "" {
	//     if maxJobs, err := strconv.ParseInt(maxJobsStr, 10, 32); err == nil {
	//         c.MaxConcurrentJobs = int32(maxJobs)
	//     }
	// }
}

// ToMap converts the configuration to a map for debugging and monitoring.
// This method provides a structured representation suitable for logging,
// metrics, and administrative interfaces.
//
// Returns: Map with string keys and interface{} values containing all config fields
//
// Example:
//
//	configMap := config.ToMap()
//	logger.Info("Current configuration", "config", configMap)
//
//	// Or for monitoring systems:
//	for key, value := range configMap {
//	    metrics.Record("config."+key, value)
//	}
func (c *Config) ToMap() map[string]interface{} {
	return map[string]interface{}{
		// Network configuration
		"netns_path":       c.NetnsPath,
		"var_run_netns":    c.VarRunNetns,
		"cgroups_base_dir": c.CgroupsBaseDir,

		// Timeout configuration (as human-readable strings)
		"graceful_shutdown_timeout": c.GracefulShutdownTimeout.String(),
		"process_start_timeout":     c.ProcessStartTimeout.String(),
		"cleanup_timeout":           c.CleanupTimeout.String(),

		// Resource defaults
		"default_cpu_limit_percent": c.DefaultCPULimitPercent,
		"default_memory_limit_mb":   c.DefaultMemoryLimitMB,
		"default_io_bps":            c.DefaultIOBPS,

		// Retry and scheduling
		"default_retry_attempts": c.DefaultRetryAttempts,
		"default_cpu_weight":     c.DefaultCPUWeight,

		// Process limits
		"max_job_args":            c.MaxJobArgs,
		"max_job_arg_length":      c.MaxJobArgLength,
		"max_environment_vars":    c.MaxEnvironmentVars,
		"max_environment_var_len": c.MaxEnvironmentVarLen,

		// System limits
		"max_concurrent_jobs": c.MaxConcurrentJobs,
	}
}

// String returns a concise string representation of the configuration.
// This method provides a human-readable summary suitable for logging
// and debugging output.
//
// Returns: Brief description highlighting key configuration paths
//
// Example:
//
//	fmt.Printf("Worker config: %s", config.String())
//	// Output: Config{NetnsPath: /tmp/shared-netns/, VarRunNetns: /var/run/netns/, CgroupsBaseDir: /sys/fs/cgroup/job-worker, ...}
func (c *Config) String() string {
	return fmt.Sprintf("Config{NetnsPath: %s, VarRunNetns: %s, CgroupsBaseDir: %s, ...}",
		c.NetnsPath, c.VarRunNetns, c.CgroupsBaseDir)
}

// Legacy timeout variables for backward compatibility.
// These global variables are deprecated in favor of the Config struct
// but maintained for existing code compatibility.
//
// Deprecated: Use Config.GracefulShutdownTimeout and Config.ProcessStartTimeout instead
var (
	GracefulShutdownTimeout = DefaultGracefulShutdownTimeout
)
