package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the complete persist configuration
type Config struct {
	Server  ServerConfig  `yaml:"server"`
	IPC     IPCConfig     `yaml:"ipc"`
	Storage StorageConfig `yaml:"storage"`
	// Note: Logging config is now read from the root level (shared with main joblet)
}

// ServerConfig contains gRPC server settings
type ServerConfig struct {
	GRPCAddress    string     `yaml:"grpc_address"` // TCP address (optional, can be empty to disable)
	GRPCSocket     string     `yaml:"grpc_socket"`  // Unix socket for internal IPC (e.g., /opt/joblet/run/persist-grpc.sock)
	MaxConnections int        `yaml:"max_connections"`
	TLS            *TLSConfig `yaml:"tls,omitempty"` // Optional: defaults to inherited security
}

// TLSConfig contains TLS/mTLS settings (TLS is always enabled)
type TLSConfig struct {
	CertFile   string `yaml:"cert_file"`   // Empty = inherit from parent's security section
	KeyFile    string `yaml:"key_file"`    // Empty = inherit from parent's security section
	CAFile     string `yaml:"ca_file"`     // Empty = inherit from parent's security section
	ClientAuth string `yaml:"client_auth"` // "none", "request", "require" (default: "require")
}

// IPCConfig contains Unix socket IPC settings
type IPCConfig struct {
	Socket         string `yaml:"socket"`
	MaxMessageSize int    `yaml:"max_message_size"`
	ReadBuffer     int    `yaml:"read_buffer"`

	// Pipeline settings
	BufferSize          int    `yaml:"buffer_size"`          // Write pipeline buffer size (default: 100000)
	WorkerCount         int    `yaml:"worker_count"`         // Number of write workers (default: 4)
	BatchSize           int    `yaml:"batch_size"`           // Max messages per batch (default: 100)
	BackpressureTimeout int    `yaml:"backpressure_timeout"` // Timeout in seconds before action (default: 5)
	BackpressureMode    string `yaml:"backpressure_mode"`    // "drop" or "block" (default: "block")
}

// StorageConfig contains storage backend settings
type StorageConfig struct {
	Type       string           `yaml:"type"` // "local", "cloudwatch", "s3"
	Local      LocalConfig      `yaml:"local"`
	CloudWatch CloudWatchConfig `yaml:"cloudwatch"`
	S3         S3Config         `yaml:"s3"`
}

// S3Config contains AWS S3 storage settings
// Authentication: Uses AWS default credential chain (IAM roles, environment variables, etc.)
type S3Config struct {
	Region    string `yaml:"region"`     // AWS region (REQUIRED)
	Bucket    string `yaml:"bucket"`     // S3 bucket name (REQUIRED)
	KeyPrefix string `yaml:"key_prefix"` // Object key prefix (default: "jobs/")
	NodeID    string `yaml:"-"`          // Node ID (inherited from server.nodeId, not from YAML)

	// Buffering settings
	FlushInterval  int `yaml:"flush_interval"`  // Flush buffer interval in seconds (default: 30)
	FlushThreshold int `yaml:"flush_threshold"` // Flush when buffer reaches this size in bytes (default: 5MB)
	MaxBufferSize  int `yaml:"max_buffer_size"` // Maximum buffer size before blocking (default: 50MB)

	// S3-specific options
	StorageClass         string `yaml:"storage_class"` // S3 storage class (default: "STANDARD")
	ServerSideEncryption string `yaml:"sse"`           // Server-side encryption: "" (none), "AES256", "aws:kms"
	KMSKeyID             string `yaml:"kms_key_id"`    // KMS key ID if sse="aws:kms"
}

// LocalConfig contains local filesystem storage settings
type LocalConfig struct {
	Logs    LogStorageConfig    `yaml:"logs"`
	Metrics MetricStorageConfig `yaml:"metrics"`
	Events  EventStorageConfig  `yaml:"events"`

	// File handle cache settings
	MaxOpenFiles  int `yaml:"max_open_files"`  // Max open file handles per type (default: 1000)
	FileHandleTTL int `yaml:"file_handle_ttl"` // TTL in seconds for idle file handles (default: 300)
}

// EventStorageConfig contains telematics event storage settings
type EventStorageConfig struct {
	Directory string `yaml:"directory"`
}

// CloudWatchConfig contains AWS CloudWatch storage settings
// Authentication: Uses AWS default credential chain (IAM roles, environment variables, etc.)
type CloudWatchConfig struct {
	Region         string `yaml:"region"`           // AWS region (REQUIRED - must be set by installation script)
	NodeID         string `yaml:"-"`                // Node ID (inherited from server.nodeId, not from YAML)
	LogGroupPrefix string `yaml:"log_group_prefix"` // Prefix for CloudWatch Logs groups (default: /joblet/jobs)

	// Metrics configuration
	MetricNamespace  string            `yaml:"metric_namespace"`  // CloudWatch Metrics namespace (default: Joblet/Jobs)
	MetricDimensions map[string]string `yaml:"metric_dimensions"` // Additional dimensions for metrics

	// Batch settings
	LogBatchSize    int `yaml:"log_batch_size"`    // Max log events per batch (default: 100)
	MetricBatchSize int `yaml:"metric_batch_size"` // Max metric data points per batch (default: 20)

	// Retention settings
	LogRetentionDays int `yaml:"log_retention_days"` // Log retention in days (0 = use default, -1 = never expire, default: 7)
	// Valid values: 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653
	// 0 or not set = default to 7 days, -1 = never expire
}

// LogStorageConfig contains log storage settings
type LogStorageConfig struct {
	Directory string `yaml:"directory"`
}

// MetricStorageConfig contains metric storage settings
type MetricStorageConfig struct {
	Directory string `yaml:"directory"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level string `yaml:"level"` // debug, info, warn, error
}

// SecurityConfig contains embedded TLS certificates (inherited from parent)
type SecurityConfig struct {
	ServerCert string `yaml:"serverCert"`
	ServerKey  string `yaml:"serverKey"`
	CACert     string `yaml:"caCert"`
}

// ServerInfo contains server-level configuration inherited from parent
type ServerInfo struct {
	NodeID string `yaml:"nodeId"` // Node identifier for distributed deployments
}

// ParentIPCConfig holds the top-level IPC configuration from joblet
// Used to inherit socket path (single source of truth)
type ParentIPCConfig struct {
	Socket string `yaml:"socket"` // Unix socket path (inherited by persist)
}

// RootConfig wraps the persist config to support nested structure
// and includes shared configurations from parent (joblet)
type RootConfig struct {
	Server   ServerInfo      `yaml:"server"` // Server info (nodeId)
	IPC      ParentIPCConfig `yaml:"ipc"`    // Top-level IPC config (socket path)
	Persist  *Config         `yaml:"persist"`
	Logging  LoggingConfig   `yaml:"logging"`  // Inherited logging config
	Security SecurityConfig  `yaml:"security"` // Inherited TLS certificates
}

// LoadResult contains persist config and inherited parent configurations
type LoadResult struct {
	Config   *Config
	NodeID   string         // Inherited from parent (server.nodeId)
	Logging  LoggingConfig  // Inherited from parent
	Security SecurityConfig // Inherited from parent (TLS certificates)
}

// Load loads configuration from a YAML file
// Supports both standalone persist config and nested config within joblet-config.yml
// Returns both persist config and the shared logging configuration
func Load(path string) (*LoadResult, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Try loading as nested config first (persist section within joblet-config.yml)
	rootCfg := &RootConfig{}
	if err := yaml.Unmarshal(data, rootCfg); err == nil && rootCfg.Persist != nil {
		// Found persist section in joblet-config.yml - inherit parent configs

		// Inherit IPC socket from top-level ipc.socket if not specified in persist.ipc
		// This ensures single source of truth for socket path (Go best practice: avoid duplication)
		if rootCfg.Persist.IPC.Socket == "" && rootCfg.IPC.Socket != "" {
			rootCfg.Persist.IPC.Socket = rootCfg.IPC.Socket
		}

		// Apply defaults for missing storage configurations (backward compatibility)
		defaults := DefaultConfig()
		if rootCfg.Persist.Storage.Local.Events.Directory == "" {
			rootCfg.Persist.Storage.Local.Events = defaults.Storage.Local.Events
		}

		// Validate after inheritance so inherited values are checked
		if err := rootCfg.Persist.Validate(); err != nil {
			return nil, fmt.Errorf("invalid persist configuration: %w", err)
		}

		// Set default ClientAuth if TLS section exists but ClientAuth not specified
		if rootCfg.Persist.Server.TLS != nil && rootCfg.Persist.Server.TLS.ClientAuth == "" {
			rootCfg.Persist.Server.TLS.ClientAuth = "require"
		}
		// If TLS section is nil, it means fully inherited (handled in server code)

		return &LoadResult{
			Config:   rootCfg.Persist,
			NodeID:   rootCfg.Server.NodeID,
			Logging:  rootCfg.Logging,
			Security: rootCfg.Security,
		}, nil
	}

	// Fall back to standalone persist config
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Use default configs for standalone (no inheritance)
	return &LoadResult{
		Config: cfg,
		Logging: LoggingConfig{
			Level: "info",
		},
		Security: SecurityConfig{
			// Standalone mode requires external cert files
			ServerCert: "",
			ServerKey:  "",
			CACert:     "",
		},
	}, nil
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			GRPCAddress:    "",                                  // TCP disabled - using Unix socket
			GRPCSocket:     "/opt/joblet/run/persist-grpc.sock", // Unix socket for gRPC queries
			MaxConnections: 500,
			TLS:            nil, // nil = fully inherited from parent's security section
		},
		IPC: IPCConfig{
			Socket:              "/opt/joblet/run/persist-ipc.sock", // Unix socket for log/metric writes
			MaxMessageSize:      134217728,                          // 128MB - handle large historical data streams
			ReadBuffer:          8388608,                            // 8MB
			BufferSize:          100000,                             // 100k message buffer for high-frequency eBPF events
			WorkerCount:         4,                                  // 4 parallel write workers
			BatchSize:           100,                                // Flush after 100 messages
			BackpressureTimeout: 5,                                  // 5 seconds before backpressure action
			BackpressureMode:    "block",                            // Block instead of drop (prevents data loss)
		},
		Storage: StorageConfig{
			Type: "local",
			Local: LocalConfig{
				Logs: LogStorageConfig{
					Directory: "/opt/joblet/logs",
				},
				Metrics: MetricStorageConfig{
					Directory: "/opt/joblet/metrics",
				},
				Events: EventStorageConfig{
					Directory: "/opt/joblet/events",
				},
				MaxOpenFiles:  1000, // Max 1000 file handles per type
				FileHandleTTL: 300,  // 5 minutes TTL for idle handles
			},
		},
		// Note: Logging config now comes from root level (shared with main joblet)
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.IPC.Socket == "" {
		return fmt.Errorf("ipc.socket is required")
	}

	if c.Storage.Type == "" {
		return fmt.Errorf("storage.type is required")
	}

	return nil
}
