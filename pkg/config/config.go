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
	Worker     WorkerConfig     `yaml:"worker" json:"worker"`
	Cgroup     CgroupConfig     `yaml:"cgroup" json:"cgroup"`
	Filesystem FilesystemConfig `yaml:"filesystem" json:"filesystem"`
	GRPC       GRPCConfig       `yaml:"grpc" json:"grpc"`
	Logging    LoggingConfig    `yaml:"logging" json:"logging"`
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

// GetServerAddress Server-specific convenience methods
func (c *Config) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.Server.Address, c.Server.Port)
}

func (c *Config) GetCgroupPath(jobID string) string {
	return filepath.Join(c.Cgroup.BaseDir, "job-"+jobID)
}

// GetServerTLSConfig returns TLS configuration from embedded certificates
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

// GetClientTLSConfig returns client TLS configuration from node certificates
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
		ServerName:   "worker", // Must match server certificate
	}

	return tlsConfig, nil
}

// LoadConfig loads configuration for server/daemon
func LoadConfig() (*Config, string, error) {
	config := DefaultConfig

	// Load from config file if it exists
	path, err := loadFromFile(&config)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config file: %w", err)
	}

	if val := os.Getenv("WORKER_SERVER_ADDRESS"); val != "" {
		config.Server.Address = val
	}
	if val := os.Getenv("WORKER_MODE"); val != "" {
		config.Server.Mode = val
	}

	if val := os.Getenv("WORKER_LOG_LEVEL"); val != "" {
		config.Logging.Level = val
	}
	if val := os.Getenv("WORKER_LOG_FORMAT"); val != "" {
		config.Logging.Format = val
	}

	// Validate the configuration
	if e := config.Validate(); e != nil {
		return nil, "", fmt.Errorf("configuration validation failed: %w", e)
	}

	return &config, path, nil
}

// loadFromFile loads configuration from YAML file
func loadFromFile(config *Config) (string, error) {
	configPaths := []string{
		os.Getenv("WORKER_CONFIG_PATH"),
		"/opt/worker/config/server-config.yml",
		"./config/server-config.yml",
		"./server-config.yml",
		"/etc/worker/server-config.yml",
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

// LoadClientConfig loads client configuration from file
func LoadClientConfig(configPath string) (*ClientConfig, error) {
	if configPath == "" {
		// Look for client-config.yml in common locations
		configPath = findClientConfig()
		if configPath == "" {
			return nil, fmt.Errorf("client configuration file not found. Please create client-config.yml or specify path with --config")
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

// GetNode returns the configuration for a specific node
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

// ListNodes returns all available node names
func (c *ClientConfig) ListNodes() []string {
	var nodes []string
	for name := range c.Nodes {
		nodes = append(nodes, name)
	}
	return nodes
}

// findClientConfig looks for client-config.yml in common locations
func findClientConfig() string {
	locations := []string{
		"./client-config.yml",
		"./config/client-config.yml",
		filepath.Join(os.Getenv("HOME"), ".worker", "client-config.yml"),
		"/etc/worker/client-config.yml",
		"/opt/worker/config/client-config.yml",
	}

	for _, path := range locations {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
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
