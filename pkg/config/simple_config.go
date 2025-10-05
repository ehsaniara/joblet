package config

import (
	"fmt"
	"time"
)

// SimpleConfig replaces the over-engineered nested configuration
// Flattens 28+ config structs into a single, manageable structure
type SimpleConfig struct {
	// Server settings
	ServerAddress string `yaml:"server_address"`
	ServerPort    int    `yaml:"server_port"`

	// Security (simplified)
	TLSEnabled bool   `yaml:"tls_enabled"`
	CertFile   string `yaml:"cert_file"`
	KeyFile    string `yaml:"key_file"`

	// Job execution limits
	MaxCPUPercent     int32 `yaml:"max_cpu_percent"`
	MaxMemoryMB       int32 `yaml:"max_memory_mb"`
	MaxIOBPS          int32 `yaml:"max_io_bps"`
	MaxConcurrentJobs int   `yaml:"max_concurrent_jobs"`

	// Storage paths
	WorkspaceDir   string `yaml:"workspace_dir"`
	CgroupBaseDir  string `yaml:"cgroup_base_dir"`
	VolumesDir     string `yaml:"volumes_dir"`
	NetworkStorage string `yaml:"network_storage"`
	LogDir         string `yaml:"log_dir"`

	// Network settings (flattened)
	NetworkEnabled      bool   `yaml:"network_enabled"`
	DefaultNetwork      string `yaml:"default_network"`
	AllowCustomNetworks bool   `yaml:"allow_custom_networks"`

	// Timeouts (simplified)
	JobTimeout     time.Duration `yaml:"job_timeout"`
	CleanupTimeout time.Duration `yaml:"cleanup_timeout"`

	// Logging (simplified)
	LogLevel       string `yaml:"log_level"`
	LogFormat      string `yaml:"log_format"`
	LogPersistence bool   `yaml:"log_persistence"`

	// Buffer settings (flattened)
	BufferSize     int   `yaml:"buffer_size"`
	MaxMemoryUsage int64 `yaml:"max_memory_usage"`
}

// GetServerAddress returns the full server address
func (c *SimpleConfig) GetServerAddress() string {
	if c.ServerPort == 0 {
		c.ServerPort = 8080
	}
	if c.ServerAddress == "" {
		c.ServerAddress = "localhost"
	}
	return fmt.Sprintf("%s:%d", c.ServerAddress, c.ServerPort)
}

// GetDefaults returns a config with sensible defaults
func GetDefaults() *SimpleConfig {
	return &SimpleConfig{
		ServerAddress:       "localhost",
		ServerPort:          8080,
		TLSEnabled:          false,
		MaxCPUPercent:       100,
		MaxMemoryMB:         1024,
		MaxIOBPS:            1000000,
		MaxConcurrentJobs:   10,
		WorkspaceDir:        "/tmp/joblet/workspaces",
		CgroupBaseDir:       "/sys/fs/cgroup/joblet",
		VolumesDir:          "/tmp/joblet/volumes",
		NetworkStorage:      "/tmp/joblet/networks",
		LogDir:              "/tmp/joblet/logs",
		NetworkEnabled:      true,
		DefaultNetwork:      "bridge",
		AllowCustomNetworks: true,
		JobTimeout:          30 * time.Minute,
		CleanupTimeout:      5 * time.Minute,
		LogLevel:            "info",
		LogFormat:           "structured",
		LogPersistence:      true,
		BufferSize:          10000,
		MaxMemoryUsage:      1024 * 1024 * 1024, // 1GB
	}
}
