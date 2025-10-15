package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the complete joblet-persist configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	IPC        IPCConfig        `yaml:"ipc"`
	Storage    StorageConfig    `yaml:"storage"`
	Writer     WriterConfig     `yaml:"writer"`
	Query      QueryConfig      `yaml:"query"`
	Monitoring MonitoringConfig `yaml:"monitoring"`
	// Note: Logging config is now read from the root level (shared with main joblet)
}

// ServerConfig contains gRPC server settings
type ServerConfig struct {
	GRPCAddress    string     `yaml:"grpc_address"` // TCP address (optional, can be empty to disable)
	GRPCSocket     string     `yaml:"grpc_socket"`  // Unix socket for internal IPC (e.g., /opt/joblet/run/persist-grpc.sock)
	MaxConnections int        `yaml:"max_connections"`
	BasePath       string     `yaml:"base_path"`
	TLS            *TLSConfig `yaml:"tls,omitempty"` // Optional: defaults to inherited security
}

// TLSConfig contains TLS/mTLS settings
// TLS is MANDATORY for persist service (authentication requires it)
type TLSConfig struct {
	// Enabled is removed - TLS is always enabled
	CertFile   string `yaml:"cert_file"`   // Empty = inherit from parent's security section
	KeyFile    string `yaml:"key_file"`    // Empty = inherit from parent's security section
	CAFile     string `yaml:"ca_file"`     // Empty = inherit from parent's security section
	ClientAuth string `yaml:"client_auth"` // "none", "request", "require" (default: "require")
}

// IPCConfig contains Unix socket IPC settings
type IPCConfig struct {
	Socket         string `yaml:"socket"`
	MaxConnections int    `yaml:"max_connections"`
	MaxMessageSize int    `yaml:"max_message_size"`
	ReadBuffer     int    `yaml:"read_buffer"`
	WriteBuffer    int    `yaml:"write_buffer"`
}

// StorageConfig contains storage backend settings
type StorageConfig struct {
	Type        string            `yaml:"type"` // "local", "cloudwatch", "s3"
	BaseDir     string            `yaml:"base_dir"`
	Local       LocalConfig       `yaml:"local"`
	Retention   RetentionConfig   `yaml:"retention"`
	Compression CompressionConfig `yaml:"compression"`
}

// LocalConfig contains local filesystem storage settings
type LocalConfig struct {
	Logs    LogStorageConfig    `yaml:"logs"`
	Metrics MetricStorageConfig `yaml:"metrics"`
}

// LogStorageConfig contains log storage settings
type LogStorageConfig struct {
	Directory string         `yaml:"directory"`
	Format    string         `yaml:"format"` // "jsonl"
	Rotation  RotationConfig `yaml:"rotation"`
}

// MetricStorageConfig contains metric storage settings
type MetricStorageConfig struct {
	Directory string         `yaml:"directory"`
	Format    string         `yaml:"format"` // "jsonl.gz"
	Rotation  RotationConfig `yaml:"rotation"`
}

// RotationConfig contains file rotation settings
type RotationConfig struct {
	MaxSizeMB       int  `yaml:"max_size_mb"`
	MaxFiles        int  `yaml:"max_files"`
	CompressRotated bool `yaml:"compress_rotated"`
}

// RetentionConfig contains data retention policies
type RetentionConfig struct {
	LogsDays            int    `yaml:"logs_days"`
	MetricsDays         int    `yaml:"metrics_days"`
	CleanupSchedule     string `yaml:"cleanup_schedule"`
	ArchiveBeforeDelete bool   `yaml:"archive_before_delete"`
}

// CompressionConfig contains compression settings
type CompressionConfig struct {
	Enabled            bool     `yaml:"enabled"`
	Algorithm          string   `yaml:"algorithm"` // "gzip"
	Level              int      `yaml:"level"`     // 1-9
	CompressExtensions []string `yaml:"compress_extensions"`
}

// WriterConfig contains write pipeline settings
type WriterConfig struct {
	BufferSize    int    `yaml:"buffer_size"`
	BatchSize     int    `yaml:"batch_size"`
	FlushInterval string `yaml:"flush_interval"`
	Workers       int    `yaml:"workers"`
}

// QueryConfig contains query engine settings
type QueryConfig struct {
	Cache  CacheConfig  `yaml:"cache"`
	Stream StreamConfig `yaml:"stream"`
}

// CacheConfig contains query cache settings
type CacheConfig struct {
	Enabled   bool   `yaml:"enabled"`
	MaxSizeMB int    `yaml:"max_size_mb"`
	TTL       string `yaml:"ttl"`
}

// StreamConfig contains streaming settings
type StreamConfig struct {
	BufferSize int    `yaml:"buffer_size"`
	Timeout    string `yaml:"timeout"`
}

// MonitoringConfig contains monitoring settings
type MonitoringConfig struct {
	Prometheus PrometheusConfig `yaml:"prometheus"`
	Health     HealthConfig     `yaml:"health"`
}

// PrometheusConfig contains Prometheus metrics settings
type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

// HealthConfig contains health check settings
type HealthConfig struct {
	Port int    `yaml:"port"`
	Path string `yaml:"path"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string        `yaml:"level"`  // debug, info, warn, error
	Format string        `yaml:"format"` // json, text
	Output string        `yaml:"output"` // stdout, file, syslog
	File   LogFileConfig `yaml:"file"`
}

// LogFileConfig contains log file settings
type LogFileConfig struct {
	Path       string `yaml:"path"`
	MaxSize    string `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	Compress   bool   `yaml:"compress"`
}

// SecurityConfig contains embedded TLS certificates (inherited from parent)
type SecurityConfig struct {
	ServerCert string `yaml:"serverCert"`
	ServerKey  string `yaml:"serverKey"`
	CACert     string `yaml:"caCert"`
}

// RootConfig wraps the persist config to support nested structure
// and includes shared configurations from parent (joblet)
type RootConfig struct {
	Persist  *Config        `yaml:"persist"`
	Logging  LoggingConfig  `yaml:"logging"`  // Inherited logging config
	Security SecurityConfig `yaml:"security"` // Inherited TLS certificates
}

// LoadResult contains persist config and inherited parent configurations
type LoadResult struct {
	Config   *Config
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
			Level:  "info",
			Format: "text",
			Output: "stdout",
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
			BasePath:       "/opt/joblet",
			TLS:            nil, // nil = fully inherited from parent's security section
		},
		IPC: IPCConfig{
			Socket:         "/opt/joblet/run/persist-ipc.sock", // Unix socket for log/metric writes
			MaxConnections: 10,
			MaxMessageSize: 134217728, // 128MB - handle large historical data streams
			ReadBuffer:     8388608,   // 8MB
			WriteBuffer:    8388608,   // 8MB
		},
		Storage: StorageConfig{
			Type:    "local",
			BaseDir: "/opt/joblet",
			Local: LocalConfig{
				Logs: LogStorageConfig{
					Directory: "/opt/joblet/logs",
					Format:    "jsonl",
					Rotation: RotationConfig{
						MaxSizeMB:       100,
						MaxFiles:        10,
						CompressRotated: true,
					},
				},
				Metrics: MetricStorageConfig{
					Directory: "/opt/joblet/metrics",
					Format:    "jsonl.gz",
					Rotation: RotationConfig{
						MaxSizeMB:       50,
						MaxFiles:        5,
						CompressRotated: true,
					},
				},
			},
			Retention: RetentionConfig{
				LogsDays:            7,
				MetricsDays:         30,
				CleanupSchedule:     "0 2 * * *",
				ArchiveBeforeDelete: false,
			},
			Compression: CompressionConfig{
				Enabled:            true,
				Algorithm:          "gzip",
				Level:              6,
				CompressExtensions: []string{".log", ".jsonl"},
			},
		},
		Writer: WriterConfig{
			BufferSize:    10485760, // 10MB
			BatchSize:     100,
			FlushInterval: "1s",
			Workers:       4,
		},
		Query: QueryConfig{
			Cache: CacheConfig{
				Enabled:   true,
				MaxSizeMB: 500,
				TTL:       "5m",
			},
			Stream: StreamConfig{
				BufferSize: 1000,
				Timeout:    "30s",
			},
		},
		Monitoring: MonitoringConfig{
			Prometheus: PrometheusConfig{
				Enabled: true,
				Port:    9092,
				Path:    "/metrics",
			},
			Health: HealthConfig{
				Port: 9093,
				Path: "/health",
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

	if c.Storage.Type == "local" {
		if c.Storage.BaseDir == "" {
			return fmt.Errorf("storage.base_dir is required for local storage")
		}
	}

	// Validate durations
	if _, err := time.ParseDuration(c.Writer.FlushInterval); err != nil {
		return fmt.Errorf("invalid writer.flush_interval: %w", err)
	}

	if c.Query.Cache.Enabled {
		if _, err := time.ParseDuration(c.Query.Cache.TTL); err != nil {
			return fmt.Errorf("invalid query.cache.ttl: %w", err)
		}
	}

	if _, err := time.ParseDuration(c.Query.Stream.Timeout); err != nil {
		return fmt.Errorf("invalid query.stream.timeout: %w", err)
	}

	return nil
}

// ParseDuration is a helper to parse duration from config
func ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}
