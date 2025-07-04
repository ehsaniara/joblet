package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration
type Config struct {
	CLI        CLIConfig        `yaml:"cli" json:"cli"`
	Server     ServerConfig     `yaml:"server" json:"server"`
	Worker     WorkerConfig     `yaml:"worker" json:"worker"`
	Cgroup     CgroupConfig     `yaml:"cgroup" json:"cgroup"`
	Filesystem FilesystemConfig `yaml:"filesystem" json:"filesystem"`
	GRPC       GRPCConfig       `yaml:"grpc" json:"grpc"`
	Logging    LoggingConfig    `yaml:"logging" json:"logging"`
}

// CLIConfig holds CLI-specific configuration
type CLIConfig struct {
	ServerAddr     string `yaml:"serverAddr" json:"serverAddr"`
	ClientCertPath string `yaml:"clientCertPath" json:"clientCertPath"`
	ClientKeyPath  string `yaml:"clientKeyPath" json:"clientKeyPath"`
	CACertPath     string `yaml:"caCertPath" json:"caCertPath"`
}

// ServerConfig holds server-specific configuration (includes security settings)
type ServerConfig struct {
	Address        string        `yaml:"address" json:"address"`
	Port           int           `yaml:"port" json:"port"`
	Mode           string        `yaml:"mode" json:"mode"`
	Timeout        time.Duration `yaml:"timeout" json:"timeout"`
	ServerCertPath string        `yaml:"serverCertPath" json:"serverCertPath"`
	ServerKeyPath  string        `yaml:"serverKeyPath" json:"serverKeyPath"`
	CACertPath     string        `yaml:"caCertPath" json:"caCertPath"`
	MinTLSVersion  string        `yaml:"minTlsVersion" json:"minTlsVersion"`
}

// WorkerConfig holds worker-specific configuration
type WorkerConfig struct {
	DefaultCPULimit    int32         `yaml:"defaultCpuLimit" json:"defaultCpuLimit"`
	DefaultMemoryLimit int32         `yaml:"defaultMemoryLimit" json:"defaultMemoryLimit"`
	DefaultIOLimit     int32         `yaml:"defaultIoLimit" json:"defaultIoLimit"`
	MaxConcurrentJobs  int           `yaml:"maxConcurrentJobs" json:"maxConcurrentJobs"`
	JobTimeout         time.Duration `yaml:"jobTimeout" json:"jobTimeout"`
	CleanupTimeout     time.Duration `yaml:"cleanupTimeout" json:"cleanupTimeout"`
	ValidateCommands   bool          `yaml:"validateCommands" json:"validateCommands"`
}

// CgroupConfig holds cgroup-related configuration
type CgroupConfig struct {
	BaseDir           string        `yaml:"baseDir" json:"baseDir"`
	NamespaceMount    string        `yaml:"namespaceMount" json:"namespaceMount"`
	EnableControllers []string      `yaml:"enableControllers" json:"enableControllers"`
	CleanupTimeout    time.Duration `yaml:"cleanupTimeout" json:"cleanupTimeout"`
}

// FilesystemConfig holds filesystem configuration
type FilesystemConfig struct {
	BaseDir       string   `yaml:"baseDir" json:"baseDir"`
	TmpDir        string   `yaml:"tmpDir" json:"tmpDir"`
	AllowedMounts []string `yaml:"allowedMounts" json:"allowedMounts"`
	BlockDevices  bool     `yaml:"blockDevices" json:"blockDevices"`
}

// GRPCConfig holds gRPC-specific configuration
type GRPCConfig struct {
	MaxRecvMsgSize    int32         `yaml:"maxRecvMsgSize" json:"maxRecvMsgSize"`
	MaxSendMsgSize    int32         `yaml:"maxSendMsgSize" json:"maxSendMsgSize"`
	MaxHeaderListSize int32         `yaml:"maxHeaderListSize" json:"maxHeaderListSize"`
	KeepAliveTime     time.Duration `yaml:"keepAliveTime" json:"keepAliveTime"`
	KeepAliveTimeout  time.Duration `yaml:"keepAliveTimeout" json:"keepAliveTimeout"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

// DefaultConfig provides default configuration values
var DefaultConfig = Config{
	CLI: CLIConfig{
		ServerAddr:     "localhost:50051",
		ClientCertPath: "./certs/client-cert.pem",
		ClientKeyPath:  "./certs/client-key.pem",
		CACertPath:     "./certs/ca-cert.pem",
	},
	Server: ServerConfig{
		Address:        "0.0.0.0",
		Port:           50051,
		Mode:           "server",
		Timeout:        30 * time.Second,
		ServerCertPath: "/opt/worker/certs/server-cert.pem",
		ServerKeyPath:  "/opt/worker/certs/server-key.pem",
		CACertPath:     "/opt/worker/certs/ca-cert.pem",
		MinTLSVersion:  "1.3",
	},
	Worker: WorkerConfig{
		DefaultCPULimit:    100,
		DefaultMemoryLimit: 512,
		DefaultIOLimit:     0,
		MaxConcurrentJobs:  100,
		JobTimeout:         1 * time.Hour,
		CleanupTimeout:     5 * time.Second,
		ValidateCommands:   true,
	},
	Cgroup: CgroupConfig{
		BaseDir:           "/sys/fs/cgroup/worker.slice/worker.service",
		NamespaceMount:    "/sys/fs/cgroup",
		EnableControllers: []string{"cpu", "memory", "io", "pids"},
		CleanupTimeout:    5 * time.Second,
	},
	Filesystem: FilesystemConfig{
		BaseDir:       "/opt/worker/jobs",
		TmpDir:        "/tmp/job-{JOB_ID}",
		AllowedMounts: []string{"/usr/bin", "/bin", "/lib", "/lib64"},
		BlockDevices:  false,
	},
	GRPC: GRPCConfig{
		MaxRecvMsgSize:    512 * 1024,
		MaxSendMsgSize:    4 * 1024 * 1024,
		MaxHeaderListSize: 1 * 1024 * 1024,
		KeepAliveTime:     30 * time.Second,
		KeepAliveTimeout:  5 * time.Second,
	},
	Logging: LoggingConfig{
		Level:  "INFO",
		Format: "text",
		Output: "stdout",
	},
}

// LoadConfig loads configuration for server/daemon (full config with environment variables)
func LoadConfig() (*Config, string, error) {
	config := DefaultConfig

	// Load from config file if it exists
	path, err := loadFromFile(&config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config file: %w", err)
	}

	// Override with environment variables (server only)
	if e := loadFromEnv(&config); e != nil {
		return nil, "", fmt.Errorf("failed to load environment variables: %w", e)
	}

	// Validate the configuration
	if e := config.Validate(); e != nil {
		return nil, "", fmt.Errorf("configuration validation failed: %w", e)
	}

	return &config, path, nil
}

// LoadCLIConfig loads configuration for CLI (no environment variables)
func LoadCLIConfig() *CLIConfig {
	config := DefaultConfig.CLI // Start with CLI defaults

	// Try to load from config file (CLI paths)
	if fileConfig, err := loadCLIFromFile(); err == nil {
		// Override defaults with file values if they exist
		if fileConfig.CLI.ServerAddr != "" {
			config.ServerAddr = fileConfig.CLI.ServerAddr
		} else if fileConfig.Server.Address != "" && fileConfig.Server.Port != 0 {
			// Fallback to server section if CLI section doesn't exist
			config.ServerAddr = fmt.Sprintf("%s:%d", fileConfig.Server.Address, fileConfig.Server.Port)
		}

		if fileConfig.CLI.ClientCertPath != "" {
			config.ClientCertPath = fileConfig.CLI.ClientCertPath
		}

		if fileConfig.CLI.ClientKeyPath != "" {
			config.ClientKeyPath = fileConfig.CLI.ClientKeyPath
		}

		if fileConfig.CLI.CACertPath != "" {
			config.CACertPath = fileConfig.CLI.CACertPath
		} else if fileConfig.Server.CACertPath != "" {
			// Fallback to server CA cert if CLI doesn't specify one
			config.CACertPath = fileConfig.Server.CACertPath
		}
	}

	return &config
}

// loadFromFile loads configuration from YAML file (server paths)
func loadFromFile(config *Config) (string, error) {
	configPaths := []string{
		os.Getenv("WORKER_CONFIG_PATH"), // Custom path from environment
		"/opt/worker/config/config.yml", // Primary production path
		"./config/config.yml",           // Development - relative to project root
		"./config.yml",                  // Development - current directory
		"/etc/worker/config.yml",        // System-wide alternative
		"/opt/worker/config.yaml",       // Fallback for old naming
		"./config/config.yaml",          // Fallback for old naming in development
	}

	for _, path := range configPaths {
		if path == "" {
			continue
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return "", fmt.Errorf("failed to parse config file %s: %w", path, err)
		}

		return path, nil
	}

	return "built-in defaults (no config file found)", nil
}

// loadCLIFromFile loads configuration from YAML file (CLI paths)
func loadCLIFromFile() (*Config, error) {
	configPaths := []string{
		"./config/config.yml",  // Primary location (relative to current directory)
		"./config.yml",         // Alternative location
		"./config/config.yaml", // YAML extension variant
		"./config.yaml",        // YAML extension variant in current dir
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			continue
		}

		data, err := os.ReadFile(path)
		if err != nil {
			continue // Try next path
		}

		var config Config
		if err := yaml.Unmarshal(data, &config); err != nil {
			continue // Try next path
		}

		return &config, nil
	}

	return &Config{}, os.ErrNotExist
}

// ClientConfig for pkg/client (to avoid import cycles)
type ClientConfig struct {
	ServerAddr     string
	ClientCertPath string
	ClientKeyPath  string
	CACertPath     string
}

// GetServerAddress Server-specific convenience methods
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}

func (c *Config) GetCgroupPath(jobID string) string {
	return filepath.Join(c.Cgroup.BaseDir, "job-"+jobID)
}

func (c *Config) GetServerSecurityConfig() (serverCert, serverKey, caCert string) {
	return c.Server.ServerCertPath, c.Server.ServerKeyPath, c.Server.CACertPath
}

// Environment variable loading (server only)
func loadFromEnv(config *Config) error {
	// Server config
	if val := os.Getenv("WORKER_SERVER_ADDRESS"); val != "" {
		config.Server.Address = val
	}
	if val := os.Getenv("WORKER_SERVER_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.Server.Port = port
		}
	}
	if val := os.Getenv("WORKER_MODE"); val != "" {
		config.Server.Mode = val
	}
	if val := os.Getenv("WORKER_SERVER_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.Server.Timeout = timeout
		}
	}

	// Server security config
	if val := os.Getenv("WORKER_SERVER_CERT_PATH"); val != "" {
		config.Server.ServerCertPath = val
	}
	if val := os.Getenv("WORKER_SERVER_KEY_PATH"); val != "" {
		config.Server.ServerKeyPath = val
	}
	if val := os.Getenv("WORKER_CA_CERT_PATH"); val != "" {
		config.Server.CACertPath = val
	}
	if val := os.Getenv("WORKER_MIN_TLS_VERSION"); val != "" {
		config.Server.MinTLSVersion = val
	}

	// Worker config
	if val := os.Getenv("WORKER_DEFAULT_CPU"); val != "" {
		if cpu, err := strconv.ParseInt(val, 10, 32); err == nil {
			config.Worker.DefaultCPULimit = int32(cpu)
		}
	}
	if val := os.Getenv("WORKER_DEFAULT_MEMORY"); val != "" {
		if memory, err := strconv.ParseInt(val, 10, 32); err == nil {
			config.Worker.DefaultMemoryLimit = int32(memory)
		}
	}
	if val := os.Getenv("WORKER_DEFAULT_IO"); val != "" {
		if io, err := strconv.ParseInt(val, 10, 32); err == nil {
			config.Worker.DefaultIOLimit = int32(io)
		}
	}
	if val := os.Getenv("WORKER_MAX_CONCURRENT_JOBS"); val != "" {
		if jobs, err := strconv.Atoi(val); err == nil {
			config.Worker.MaxConcurrentJobs = jobs
		}
	}
	if val := os.Getenv("WORKER_JOB_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.Worker.JobTimeout = timeout
		}
	}
	if val := os.Getenv("WORKER_CLEANUP_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.Worker.CleanupTimeout = timeout
		}
	}
	if val := os.Getenv("WORKER_VALIDATE_COMMANDS"); val != "" {
		config.Worker.ValidateCommands = val == "true" || val == "1"
	}

	// Cgroup config
	if val := os.Getenv("WORKER_CGROUP_BASE_DIR"); val != "" {
		config.Cgroup.BaseDir = val
	}
	if val := os.Getenv("WORKER_CGROUP_NAMESPACE_MOUNT"); val != "" {
		config.Cgroup.NamespaceMount = val
	}
	if val := os.Getenv("WORKER_CGROUP_CONTROLLERS"); val != "" {
		config.Cgroup.EnableControllers = strings.Split(val, ",")
	}
	if val := os.Getenv("WORKER_CGROUP_CLEANUP_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.Cgroup.CleanupTimeout = timeout
		}
	}

	// GRPC config
	if val := os.Getenv("WORKER_GRPC_MAX_RECV_MSG_SIZE"); val != "" {
		if size, err := strconv.ParseInt(val, 10, 32); err == nil {
			config.GRPC.MaxRecvMsgSize = int32(size)
		}
	}
	if val := os.Getenv("WORKER_GRPC_MAX_SEND_MSG_SIZE"); val != "" {
		if size, err := strconv.ParseInt(val, 10, 32); err == nil {
			config.GRPC.MaxSendMsgSize = int32(size)
		}
	}
	if val := os.Getenv("WORKER_GRPC_MAX_HEADER_LIST_SIZE"); val != "" {
		if size, err := strconv.ParseInt(val, 10, 32); err == nil {
			config.GRPC.MaxHeaderListSize = int32(size)
		}
	}
	if val := os.Getenv("WORKER_GRPC_KEEPALIVE_TIME"); val != "" {
		if keepAlive, err := time.ParseDuration(val); err == nil {
			config.GRPC.KeepAliveTime = keepAlive
		}
	}
	if val := os.Getenv("WORKER_GRPC_KEEPALIVE_TIMEOUT"); val != "" {
		if timeout, err := time.ParseDuration(val); err == nil {
			config.GRPC.KeepAliveTimeout = timeout
		}
	}

	// Logging config
	if val := os.Getenv("LOG_LEVEL"); val != "" {
		config.Logging.Level = val
	}
	if val := os.Getenv("LOG_FORMAT"); val != "" {
		config.Logging.Format = val
	}
	if val := os.Getenv("LOG_OUTPUT"); val != "" {
		config.Logging.Output = val
	}

	return nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.Mode != "server" && c.Server.Mode != "init" {
		return fmt.Errorf("invalid server mode: %s", c.Server.Mode)
	}

	if c.Worker.DefaultCPULimit < 0 {
		return fmt.Errorf("invalid default CPU limit: %d", c.Worker.DefaultCPULimit)
	}

	if c.Worker.DefaultMemoryLimit < 0 {
		return fmt.Errorf("invalid default memory limit: %d", c.Worker.DefaultMemoryLimit)
	}

	if c.Worker.MaxConcurrentJobs < 1 {
		return fmt.Errorf("invalid max concurrent jobs: %d", c.Worker.MaxConcurrentJobs)
	}

	// Validate certificate paths for server
	if c.Server.ServerCertPath == "" {
		return fmt.Errorf("server certificate path required")
	}
	if c.Server.ServerKeyPath == "" {
		return fmt.Errorf("server key path required")
	}
	if c.Server.CACertPath == "" {
		return fmt.Errorf("CA certificate path required")
	}

	// Validate cgroup base directory
	if !filepath.IsAbs(c.Cgroup.BaseDir) {
		return fmt.Errorf("cgroup base directory must be absolute path: %s", c.Cgroup.BaseDir)
	}

	// Validate logging level
	validLevels := map[string]bool{
		"DEBUG": true, "INFO": true, "WARN": true, "ERROR": true,
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s", c.Logging.Level)
	}

	return nil
}

// Validate validates CLI-specific configuration
func (c *CLIConfig) Validate() error {
	// Check if certificate files exist (soft validation)
	certPaths := map[string]string{
		"client certificate": c.ClientCertPath,
		"client key":         c.ClientKeyPath,
		"CA certificate":     c.CACertPath,
	}

	for _, path := range certPaths {
		if path != "" {
			if _, err := os.Stat(path); os.IsNotExist(err) {
				// Don't fail validation, just warn that certs don't exist
				continue
			}
		}
	}

	return nil
}

func (c *Config) ToYAML() ([]byte, error) {
	return yaml.Marshal(c)
}

func (c *Config) SaveToFile(path string) error {
	data, err := c.ToYAML()
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
