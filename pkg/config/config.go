package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration
type Config struct {
	Version    string           `yaml:"version" json:"version"`
	Server     ServerConfig     `yaml:"server" json:"server"`
	Security   SecurityConfig   `yaml:"security" json:"security"`
	Joblet     JobletConfig     `yaml:"joblet" json:"joblet"`
	Cgroup     CgroupConfig     `yaml:"cgroup" json:"cgroup"`
	Filesystem FilesystemConfig `yaml:"filesystem" json:"filesystem"`
	GRPC       GRPCConfig       `yaml:"grpc" json:"grpc"`
	Logging    LoggingConfig    `yaml:"logging" json:"logging"`
	Network    NetworkConfig    `yaml:"network"`
	Monitoring MonitoringConfig `yaml:"monitoring" json:"monitoring"`
	Buffers    BuffersConfig    `yaml:"buffers" json:"buffers"`
	Volumes    VolumesConfig    `yaml:"volumes" json:"volumes"`
}

type NetworkConfig struct {
	StateDir            string                       `yaml:"state_dir"`
	Enabled             bool                         `yaml:"enabled"`
	DefaultNetwork      string                       `yaml:"default_network"`
	Networks            map[string]NetworkDefinition `yaml:"networks"`
	AllowCustomNetworks bool                         `yaml:"allow_custom_networks"`
	MaxCustomNetworks   int                          `yaml:"max_custom_networks"`
	Storage             NetworkStorageConfig         `yaml:"storage"`
}

type NetworkDefinition struct {
	CIDR       string `yaml:"cidr"`
	BridgeName string `yaml:"bridge_name"`
}

type IPAllocationConfig struct {
	StartOffset int `yaml:"start_offset"`
	EndReserve  int `yaml:"end_reserve"`
}

type NetworkStorageConfig struct {
	Path string `yaml:"path" json:"path"`
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Address       string        `yaml:"address" json:"address"`
	Port          int           `yaml:"port" json:"port"`
	Mode          string        `yaml:"mode" json:"mode"`
	Timeout       time.Duration `yaml:"timeout" json:"timeout"`
	MinTLSVersion string        `yaml:"minTlsVersion" json:"minTlsVersion"`
}

// SecurityConfig holds all certificates as embedded PEM content
type SecurityConfig struct {
	ServerCert string `yaml:"serverCert" json:"serverCert"`
	ServerKey  string `yaml:"serverKey" json:"serverKey"`
	CACert     string `yaml:"caCert" json:"caCert"`
}

// JobletConfig holds joblet-specific configuration
type JobletConfig struct {
	DefaultCPULimit    int32         `yaml:"defaultCpuLimit" json:"defaultCpuLimit"`
	DefaultMemoryLimit int32         `yaml:"defaultMemoryLimit" json:"defaultMemoryLimit"`
	DefaultIOLimit     int32         `yaml:"defaultIoLimit" json:"defaultIoLimit"`
	MaxConcurrentJobs  int           `yaml:"maxConcurrentJobs" json:"maxConcurrentJobs"`
	JobTimeout         time.Duration `yaml:"jobTimeout" json:"jobTimeout"`
	CleanupTimeout     time.Duration `yaml:"cleanupTimeout" json:"cleanupTimeout"`
	ValidateCommands   bool          `yaml:"validateCommands" json:"validateCommands"`

	// Resource validation limits
	MinCPULimit    int32 `yaml:"minCpuLimit" json:"minCpuLimit"`       // Minimum CPU percentage (0 = no limit)
	MaxCPULimit    int32 `yaml:"maxCpuLimit" json:"maxCpuLimit"`       // Maximum CPU percentage
	MinMemoryLimit int32 `yaml:"minMemoryLimit" json:"minMemoryLimit"` // Minimum memory MB (0 = no limit)
	MaxMemoryLimit int32 `yaml:"maxMemoryLimit" json:"maxMemoryLimit"` // Maximum memory MB
	MinIOLimit     int32 `yaml:"minIoLimit" json:"minIoLimit"`         // Minimum IO BPS (0 = no limit)
	MaxIOLimit     int32 `yaml:"maxIoLimit" json:"maxIoLimit"`         // Maximum IO BPS
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
	WorkspaceDir  string   `yaml:"workspaceDir" json:"workspaceDir"`
	AllowedMounts []string `yaml:"allowedMounts" json:"allowedMounts"`
	BlockDevices  bool     `yaml:"blockDevices" json:"blockDevices"`
}

// GRPCConfig holds gRPC-specific configuration
type GRPCConfig struct {
	MaxRecvMsgSize        int32         `yaml:"maxRecvMsgSize" json:"maxRecvMsgSize"`
	MaxSendMsgSize        int32         `yaml:"maxSendMsgSize" json:"maxSendMsgSize"`
	MaxHeaderListSize     int32         `yaml:"maxHeaderListSize" json:"maxHeaderListSize"`
	KeepAliveTime         time.Duration `yaml:"keepAliveTime" json:"keepAliveTime"`
	KeepAliveTimeout      time.Duration `yaml:"keepAliveTimeout" json:"keepAliveTimeout"`
	MaxConcurrentStreams  uint32        `yaml:"maxConcurrentStreams" json:"maxConcurrentStreams"`
	ConnectionTimeout     time.Duration `yaml:"connectionTimeout" json:"connectionTimeout"`
	MaxConnectionIdle     time.Duration `yaml:"maxConnectionIdle" json:"maxConnectionIdle"`
	MaxConnectionAge      time.Duration `yaml:"maxConnectionAge" json:"maxConnectionAge"`
	MaxConnectionAgeGrace time.Duration `yaml:"maxConnectionAgeGrace" json:"maxConnectionAgeGrace"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" json:"level"`
	Format string `yaml:"format" json:"format"`
	Output string `yaml:"output" json:"output"`
}

// MonitoringConfig holds monitoring system configuration
type MonitoringConfig struct {
	Enabled         bool          `yaml:"enabled" json:"enabled"`
	SystemInterval  time.Duration `yaml:"system_interval" json:"system_interval"`
	ProcessInterval time.Duration `yaml:"process_interval" json:"process_interval"`
	CloudDetection  bool          `yaml:"cloud_detection" json:"cloud_detection"`
}

// ClientConfig represents the client-side configuration with multiple nodes
type ClientConfig struct {
	Version string           `yaml:"version"`
	Nodes   map[string]*Node `yaml:"nodes"`
}

// Node represents a single server configuration with embedded certificates
type Node struct {
	Address string `yaml:"address"`
	Cert    string `yaml:"cert"` // Embedded PEM certificate
	Key     string `yaml:"key"`  // Embedded PEM private key
	CA      string `yaml:"ca"`   // Embedded PEM CA certificate
}

// BuffersConfig holds consolidated buffer and pub-sub configuration
type BuffersConfig struct {
	DefaultConfig BufferDefaultConfig `yaml:"default_config" json:"default_config"`
}

// BufferDefaultConfig holds default buffer configuration (consolidated with pub-sub settings)
type BufferDefaultConfig struct {
	Type                 string        `yaml:"type" json:"type"`
	InitialCapacity      int64         `yaml:"initial_capacity" json:"initial_capacity"`
	MaxCapacity          int64         `yaml:"max_capacity" json:"max_capacity"`
	MaxSubscribers       int           `yaml:"max_subscribers" json:"max_subscribers"`
	SubscriberBufferSize int           `yaml:"subscriber_buffer_size" json:"subscriber_buffer_size"`
	PubsubBufferSize     int           `yaml:"pubsub_buffer_size" json:"pubsub_buffer_size"`
	EnableMetrics        bool          `yaml:"enable_metrics" json:"enable_metrics"`
	UploadTimeout        time.Duration `yaml:"upload_timeout" json:"upload_timeout"`
	ChunkSize            int           `yaml:"chunk_size" json:"chunk_size"`
}

// VolumesConfig holds volume management configuration
type VolumesConfig struct {
	BasePath              string `yaml:"base_path" json:"base_path"`
	DefaultDiskQuotaBytes int64  `yaml:"default_disk_quota_bytes" json:"default_disk_quota_bytes"`
}

// DefaultConfig provides default configuration values
var DefaultConfig = Config{
	Version: "3.0",
	Server: ServerConfig{
		Address:       "0.0.0.0",
		Port:          50051,
		Mode:          "server",
		Timeout:       30 * time.Second,
		MinTLSVersion: "1.3",
	},
	Security: SecurityConfig{
		// Will be populated by certificate generation
		ServerCert: "",
		ServerKey:  "",
		CACert:     "",
	},
	Joblet: JobletConfig{
		DefaultCPULimit:    100,
		DefaultMemoryLimit: 512,
		DefaultIOLimit:     0,
		MaxConcurrentJobs:  100,
		JobTimeout:         1 * time.Hour,
		CleanupTimeout:     5 * time.Second,
		ValidateCommands:   true,

		// Resource validation limits (0 = no minimum/maximum)
		MinCPULimit:    0,       // No minimum CPU limit
		MaxCPULimit:    1000,    // 10 cores worth (1000%)
		MinMemoryLimit: 1,       // 1MB minimum - configurable!
		MaxMemoryLimit: 32768,   // 32GB maximum
		MinIOLimit:     0,       // No minimum IO limit
		MaxIOLimit:     1000000, // 1GB/s maximum
	},
	Cgroup: CgroupConfig{
		BaseDir:           "/sys/fs/cgroup/joblet.slice/joblet.service",
		NamespaceMount:    "/sys/fs/cgroup",
		EnableControllers: []string{"cpu", "memory", "io", "pids", "cpuset"},
		CleanupTimeout:    5 * time.Second,
	},
	Filesystem: FilesystemConfig{
		BaseDir:       "/opt/joblet/jobs",
		TmpDir:        "/tmp/job-{JOB_ID}",
		WorkspaceDir:  "/work",
		AllowedMounts: []string{"/usr/bin", "/bin", "/lib", "/lib64"},
		BlockDevices:  false,
	},
	GRPC: GRPCConfig{
		MaxRecvMsgSize:        134217728,          // 128MB for production traffic
		MaxSendMsgSize:        134217728,          // 128MB for production traffic
		MaxHeaderListSize:     16777216,           // 16MB for production traffic
		KeepAliveTime:         10 * time.Second,   // More frequent keepalives
		KeepAliveTimeout:      3 * time.Second,    // Faster timeout detection
		MaxConcurrentStreams:  1000,               // High concurrent streams
		ConnectionTimeout:     10 * time.Second,   // Connection timeout
		MaxConnectionIdle:     300 * time.Second,  // 5min idle
		MaxConnectionAge:      1800 * time.Second, // 30min max age
		MaxConnectionAgeGrace: 30 * time.Second,   // 30s grace period
	},
	Logging: LoggingConfig{
		Level:  "INFO",
		Format: "text",
		Output: "stdout",
	},
	Network: NetworkConfig{
		StateDir:            "/opt/joblet/network",
		Enabled:             true,
		DefaultNetwork:      "bridge",
		AllowCustomNetworks: true,
		MaxCustomNetworks:   50,
		Storage: NetworkStorageConfig{
			Path: "/opt/joblet/network",
		},
		Networks: map[string]NetworkDefinition{
			"bridge": {
				CIDR:       "172.20.0.0/16",
				BridgeName: "joblet0",
			},
		},
	},
	Monitoring: MonitoringConfig{
		Enabled:         true,
		SystemInterval:  10 * time.Second,
		ProcessInterval: 30 * time.Second,
		CloudDetection:  true,
	},
	Buffers: BuffersConfig{
		DefaultConfig: BufferDefaultConfig{
			Type:                 "memory",
			InitialCapacity:      2097152,          // 2MB initial buffer for high-throughput
			MaxCapacity:          0,                // Unlimited for production
			MaxSubscribers:       0,                // Unlimited for production
			SubscriberBufferSize: 1000,             // Large channel buffer for high concurrency
			PubsubBufferSize:     10000,            // Large pub-sub channel buffer (consolidated from pubsub section)
			EnableMetrics:        false,            // Disabled for maximum performance
			UploadTimeout:        10 * time.Minute, // Extended timeout for large uploads
			ChunkSize:            1048576,          // 1MB chunks for optimal streaming
		},
	},
	Volumes: VolumesConfig{
		BasePath:              "/opt/joblet/volumes",
		DefaultDiskQuotaBytes: 1048576, // 1MB default
	},
}

// GetServerAddress returns the complete server address in "host:port" format.
// Combines the configured server address and port into a single string
// suitable for network listeners and client connections.
// Example: "0.0.0.0:50051" or "localhost:8080"
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}

// GetCgroupPath constructs the full cgroup path for a specific job.
// Takes a job ID and combines it with the configured cgroup base directory
// to create a unique cgroup path for resource isolation.
// Returns path in format: "/sys/fs/cgroup/joblet.slice/joblet.service/job-{jobID}"
func (c *Config) GetCgroupPath(jobID string) string {
	return filepath.Join(c.Cgroup.BaseDir, "job-"+jobID)
}

// GetServerTLSConfig creates a server-side TLS configuration from embedded certificates.
// Parses the PEM-encoded server certificate, private key, and CA certificate
// from the security configuration section to create a TLS config that:
//   - Requires client certificate authentication (mTLS)
//   - Uses TLS 1.3 minimum version for security
//   - Validates client certificates against the configured CA
//
// Returns configured tls.Config or error if certificate parsing fails.
func (c *Config) GetServerTLSConfig() (*tls.Config, error) {
	if c.Security.ServerCert == "" || c.Security.ServerKey == "" || c.Security.CACert == "" {
		return nil, fmt.Errorf("server certificates are not configured in security section")
	}

	// Load server certificate and key from embedded PEM
	serverCert, err := tls.X509KeyPair([]byte(c.Security.ServerCert), []byte(c.Security.ServerKey))
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Load CA certificate from embedded PEM
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(c.Security.CACert)); !ok {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS13,
	}

	return tlsConfig, nil
}

// GetClientTLSConfig creates a client-side TLS configuration from node certificates.
// Parses the PEM-encoded client certificate, private key, and CA certificate
// from the node configuration to create a TLS config that:
//   - Presents client certificate for mTLS authentication
//   - Validates server certificate against the configured CA
//   - Uses TLS 1.3 minimum version for security
//   - Sets server name to "joblet" for certificate validation
//
// Returns configured tls.Config or error if certificate parsing fails.
func (n *Node) GetClientTLSConfig() (*tls.Config, error) {
	if n.Cert == "" || n.Key == "" || n.CA == "" {
		return nil, fmt.Errorf("client certificates are not configured for node")
	}

	// Load client certificate and key from embedded PEM
	clientCert, err := tls.X509KeyPair([]byte(n.Cert), []byte(n.Key))
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	// Load CA certificate from embedded PEM
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM([]byte(n.CA)); !ok {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	// Create TLS configuration
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
		ServerName:   "joblet", // Must match server certificate
	}

	return tlsConfig, nil
}

// LoadConfig loads the main server configuration from file and environment variables.
//  1. Path specified in JOBLET_CONFIG_PATH environment variable
//  2. /opt/joblet/config/joblet-config.yml
//  3. ./config/joblet-config.yml
//  4. ./joblet-config.yml
//  5. /etc/joblet/joblet-config.yml
//  5. /etc/joblet/joblet-config.yml
//
// Applies environment variable overrides for server address, mode, and logging.
// Validates the final configuration before returning.
// Returns (config, configPath, error) - configPath indicates source of configuration.
func LoadConfig() (*Config, string, error) {
	config := DefaultConfig

	// Load from config file if it exists
	path, err := loadFromFile(&config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config file: %w", err)
	}

	if val := os.Getenv("JOBLET_SERVER_ADDRESS"); val != "" {
		config.Server.Address = val
	}
	if val := os.Getenv("JOBLET_MODE"); val != "" {
		config.Server.Mode = val
	}

	if val := os.Getenv("JOBLET_LOG_LEVEL"); val != "" {
		config.Logging.Level = val
	}
	if val := os.Getenv("JOBLET_LOG_FORMAT"); val != "" {
		config.Logging.Format = val
	}

	// Validate the configuration
	if e := config.Validate(); e != nil {
		return nil, "", fmt.Errorf("configuration validation failed: %w", e)
	}

	return &config, path, nil
}

// loadFromFile loads configuration from the first available YAML file.
// Searches common configuration locations and parses the first found file.
// Updates the provided config struct with values from the file.
// Returns the path of the loaded file or "built-in defaults" if no file found.
// Does not return error if no file is found - uses defaults instead.
func loadFromFile(config *Config) (string, error) {
	configPaths := []string{
		os.Getenv("JOBLET_CONFIG_PATH"),
		"/opt/joblet/config/joblet-config.yml",
		"./config/joblet-config.yml",
		"./joblet-config.yml",
		"/etc/joblet/joblet-config.yml",
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

// Validate performs comprehensive validation of the configuration.
// Checks all configuration sections for:
//   - Valid port ranges (1-65535)
//   - Valid server modes ("server" or "init")
//   - Non-negative resource limits
//   - Absolute paths for cgroup directories
//   - Valid logging levels
//
// Returns error describing the first validation failure found.
// Does not validate certificates as they may be populated later.
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Server.Mode != "server" && c.Server.Mode != "init" {
		return fmt.Errorf("invalid server mode: %s", c.Server.Mode)
	}

	if c.Joblet.DefaultCPULimit < 0 {
		return fmt.Errorf("invalid default CPU limit: %d", c.Joblet.DefaultCPULimit)
	}

	if c.Joblet.DefaultMemoryLimit < 0 {
		return fmt.Errorf("invalid default memory limit: %d", c.Joblet.DefaultMemoryLimit)
	}

	if c.Joblet.MaxConcurrentJobs < 0 {
		return fmt.Errorf("invalid max concurrent jobs: %d", c.Joblet.MaxConcurrentJobs)
	}

	// Note: We don't validate certificates here as they might be populated later
	// Certificate validation happens in GetServerTLSConfig()

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

// LoadClientConfig loads RNX client configuration from the specified file.
//
//  1. Path from RNX_CONFIG environment variable
//
//  2. ./rnx-config.yml
//
//  3. ./config/rnx-config.yml
//
//  4. ~/.rnx/rnx-config.yml
//
//  5. /etc/joblet/rnx-config.yml
//
//  6. /opt/joblet/config/rnx-config.yml
//
//  6. /opt/joblet/config/rnx-config.yml
//
// Parses YAML configuration and validates that at least one node is configured.
// Returns ClientConfig with node definitions for connecting to Joblet servers.
func LoadClientConfig(configPath string) (*ClientConfig, error) {
	if configPath == "" {
		// Look for rnx-config.yml in common locations
		configPath = findClientConfig()
		if configPath == "" {
			return nil, fmt.Errorf("client configuration file not found. Please create rnx-config.yml or specify path with --config")
		}
	}

	// Check if file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("client configuration file not found: %s", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read client config file %s: %w", configPath, err)
	}

	var config ClientConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse client config: %w", err)
	}

	// Validate that we have nodes
	if len(config.Nodes) == 0 {
		return nil, fmt.Errorf("no nodes configured in %s", configPath)
	}

	return &config, nil
}

// GetNode retrieves the configuration for a named Joblet server node.
// If nodeName is empty, defaults to "default" node.
// Returns the Node configuration containing server address and certificates,
// or error if the specified node name is not found in the configuration.
// Used by RNX client to select which Joblet server to connect to.
func (c *ClientConfig) GetNode(nodeName string) (*Node, error) {
	if nodeName == "" {
		nodeName = "default"
	}

	node, exists := c.Nodes[nodeName]
	if !exists {
		return nil, fmt.Errorf("node '%s' not found in configuration", nodeName)
	}

	return node, nil
}

// ListNodes returns a slice of all configured node names.
// Provides a list of available Joblet servers that the client can connect to.
// Used by RNX client for node discovery and selection.
// Returns empty slice if no nodes are configured.
func (c *ClientConfig) ListNodes() []string {
	var nodes []string
	for name := range c.Nodes {
		nodes = append(nodes, name)
	}
	return nodes
}

// findClientConfig searches for RNX client configuration file in standard locations.
// First checks RNX_CONFIG environment variable, then searches common paths.
// Returns the path of the first found configuration file.
// Returns empty string if no configuration file is found.
// Used internally by LoadClientConfig when no specific path is provided.
func findClientConfig() string {
	// First check RNX_CONFIG environment variable
	if envPath := os.Getenv("RNX_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	locations := []string{
		"./rnx-config.yml",
		"./config/rnx-config.yml",
		filepath.Join(os.Getenv("HOME"), ".rnx", "rnx-config.yml"),
		"/etc/joblet/rnx-config.yml",
		"/opt/joblet/config/rnx-config.yml",
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
