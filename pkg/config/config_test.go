package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig

	// Test Server defaults
	if cfg.Server.Address != "0.0.0.0" {
		t.Errorf("Expected Server Address '0.0.0.0', got '%s'", cfg.Server.Address)
	}
	if cfg.Server.Port != 50051 {
		t.Errorf("Expected Server Port 50051, got %d", cfg.Server.Port)
	}
	if cfg.Server.ServerCertPath != "/opt/worker/certs/server-cert.pem" {
		t.Errorf("Expected Server ServerCertPath '/opt/worker/certs/server-cert.pem', got '%s'", cfg.Server.ServerCertPath)
	}

	// Test Worker defaults
	if cfg.Worker.DefaultCPULimit != 100 {
		t.Errorf("Expected Worker DefaultCPULimit 100, got %d", cfg.Worker.DefaultCPULimit)
	}
	if cfg.Worker.MaxConcurrentJobs != 100 {
		t.Errorf("Expected Worker MaxConcurrentJobs 100, got %d", cfg.Worker.MaxConcurrentJobs)
	}
}

func TestLoadConfig_WithEnvironmentVariables(t *testing.T) {
	// Set environment variables
	originalEnvVars := setTestEnvVars(t, map[string]string{
		"WORKER_SERVER_ADDRESS": "env.example.com",
		"WORKER_SERVER_PORT":    "7777",
		"WORKER_DEFAULT_CPU":    "200",
		"WORKER_DEFAULT_MEMORY": "1024",
		"LOG_LEVEL":             "DEBUG",
		"WORKER_MODE":           "server", // Use valid mode
	})
	defer restoreEnvVars(t, originalEnvVars)

	// Create basic config file
	testConfig := `
server:
  address: "file.example.com"
  port: 6666
  serverCertPath: "/test/server-cert.pem"
  serverKeyPath: "/test/server-key.pem"
  caCertPath: "/test/ca-cert.pem"
worker:
  defaultCpuLimit: 50
  defaultMemoryLimit: 256
cgroup:
  baseDir: "/test/cgroup"
`

	configFile := createTestConfigFile(t, "server-config.yml", testConfig)
	defer os.Remove(configFile)

	cfg, path, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Environment variables should override file config
	if cfg.Server.Address != "env.example.com" {
		t.Errorf("Expected Server Address 'env.example.com', got '%s'", cfg.Server.Address)
	}
	if cfg.Server.Port != 7777 {
		t.Errorf("Expected Server Port 7777, got %d", cfg.Server.Port)
	}
	if cfg.Server.Mode != "server" {
		t.Errorf("Expected Server Mode 'server', got '%s'", cfg.Server.Mode)
	}
	if cfg.Worker.DefaultCPULimit != 200 {
		t.Errorf("Expected Worker DefaultCPULimit 200, got %d", cfg.Worker.DefaultCPULimit)
	}
	if cfg.Worker.DefaultMemoryLimit != 1024 {
		t.Errorf("Expected Worker DefaultMemoryLimit 1024, got %d", cfg.Worker.DefaultMemoryLimit)
	}
	if cfg.Logging.Level != "DEBUG" {
		t.Errorf("Expected Logging Level 'DEBUG', got '%s'", cfg.Logging.Level)
	}

	// Path should be reported
	if path == "" {
		t.Error("Expected config path to be reported")
	}
}

func TestLoadConfig_CompleteConfigFile(t *testing.T) {
	testConfig := `
cli:
  serverAddr: "cli.example.com:50051"
  clientCertPath: "./test-certs/client-cert.pem"
  clientKeyPath: "./test-certs/client-key.pem"
  caCertPath: "./test-certs/ca-cert.pem"

server:
  address: "server.example.com"
  port: 50052
  mode: "server"
  timeout: "60s"
  serverCertPath: "/opt/test/server-cert.pem"
  serverKeyPath: "/opt/test/server-key.pem"
  caCertPath: "/opt/test/ca-cert.pem"
  minTlsVersion: "1.2"

worker:
  defaultCpuLimit: 150
  defaultMemoryLimit: 768
  defaultIoLimit: 1000000
  maxConcurrentJobs: 50
  jobTimeout: "2h"
  cleanupTimeout: "10s"
  validateCommands: false

cgroup:
  baseDir: "/custom/cgroup/path"
  namespaceMount: "/custom/mount"
  enableControllers: ["cpu", "memory"]
  cleanupTimeout: "3s"

filesystem:
  baseDir: "/custom/jobs"
  tmpDir: "/custom/tmp/job-{JOB_ID}"
  allowedMounts: ["/usr/bin", "/bin"]
  blockDevices: true

grpc:
  maxRecvMsgSize: 1048576
  maxSendMsgSize: 2097152
  maxHeaderListSize: 524288
  keepAliveTime: "45s"
  keepAliveTimeout: "10s"

logging:
  level: "WARN"
  format: "json"
  output: "file"
`

	configFile := createTestConfigFile(t, "server-config.yml", testConfig)
	defer os.Remove(configFile)

	cfg, path, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Test Server config
	if cfg.Server.Address != "server.example.com" {
		t.Errorf("Expected Server Address 'server.example.com', got '%s'", cfg.Server.Address)
	}
	if cfg.Server.Port != 50052 {
		t.Errorf("Expected Server Port 50052, got %d", cfg.Server.Port)
	}
	if cfg.Server.MinTLSVersion != "1.2" {
		t.Errorf("Expected Server MinTLSVersion '1.2', got '%s'", cfg.Server.MinTLSVersion)
	}

	// Test Worker config
	if cfg.Worker.DefaultCPULimit != 150 {
		t.Errorf("Expected Worker DefaultCPULimit 150, got %d", cfg.Worker.DefaultCPULimit)
	}
	if cfg.Worker.JobTimeout != 2*time.Hour {
		t.Errorf("Expected Worker JobTimeout 2h, got %v", cfg.Worker.JobTimeout)
	}
	if cfg.Worker.ValidateCommands != false {
		t.Errorf("Expected Worker ValidateCommands false, got %v", cfg.Worker.ValidateCommands)
	}

	// Test Cgroup config
	if cfg.Cgroup.BaseDir != "/custom/cgroup/path" {
		t.Errorf("Expected Cgroup BaseDir '/custom/cgroup/path', got '%s'", cfg.Cgroup.BaseDir)
	}

	// Test Filesystem config
	if cfg.Filesystem.BlockDevices != true {
		t.Errorf("Expected Filesystem BlockDevices true, got %v", cfg.Filesystem.BlockDevices)
	}

	// Test GRPC config
	if cfg.GRPC.MaxRecvMsgSize != 1048576 {
		t.Errorf("Expected GRPC MaxRecvMsgSize 1048576, got %d", cfg.GRPC.MaxRecvMsgSize)
	}

	// Test Logging config
	if cfg.Logging.Level != "WARN" {
		t.Errorf("Expected Logging Level 'WARN', got '%s'", cfg.Logging.Level)
	}

	// Verify path is reported
	if path == "" {
		t.Error("Expected config path to be reported")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid config",
			config:      DefaultConfig,
			expectError: false,
		},
		{
			name: "invalid port - too low",
			config: Config{
				Server: ServerConfig{Port: 0},
			},
			expectError: true,
			errorMsg:    "invalid server port",
		},
		{
			name: "invalid port - too high",
			config: Config{
				Server: ServerConfig{Port: 70000},
			},
			expectError: true,
			errorMsg:    "invalid server port",
		},
		{
			name: "invalid server mode",
			config: Config{
				Server: ServerConfig{
					Port: 50051,
					Mode: "invalid-mode",
				},
			},
			expectError: true,
			errorMsg:    "invalid server mode",
		},
		{
			name: "negative CPU limit",
			config: Config{
				Server: ServerConfig{Port: 50051, Mode: "server"},
				Worker: WorkerConfig{DefaultCPULimit: -1},
			},
			expectError: true,
			errorMsg:    "invalid default CPU limit",
		},
		{
			name: "negative memory limit",
			config: Config{
				Server: ServerConfig{Port: 50051, Mode: "server"},
				Worker: WorkerConfig{DefaultMemoryLimit: -1},
			},
			expectError: true,
			errorMsg:    "invalid default memory limit",
		},
		{
			name: "zero max concurrent jobs",
			config: Config{
				Server: ServerConfig{Port: 50051, Mode: "server"},
				Worker: WorkerConfig{MaxConcurrentJobs: 0},
			},
			expectError: true,
			errorMsg:    "invalid max concurrent jobs",
		},
		{
			name: "missing server cert path",
			config: Config{
				Server: ServerConfig{
					Port:           50051,
					Mode:           "server",
					ServerCertPath: "", // Missing
					ServerKeyPath:  "/path/to/key",
					CACertPath:     "/path/to/ca",
				},
				Worker:  WorkerConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "/absolute/path"},
				Logging: LoggingConfig{Level: "INFO"},
			},
			expectError: true,
			errorMsg:    "server certificate path required",
		},
		{
			name: "relative cgroup path",
			config: Config{
				Server: ServerConfig{
					Port:           50051,
					Mode:           "server",
					ServerCertPath: "/path/to/cert",
					ServerKeyPath:  "/path/to/key",
					CACertPath:     "/path/to/ca",
				},
				Worker:  WorkerConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "relative/path"}, // Should be absolute
				Logging: LoggingConfig{Level: "INFO"},
			},
			expectError: true,
			errorMsg:    "cgroup base directory must be absolute path",
		},
		{
			name: "invalid log level",
			config: Config{
				Server: ServerConfig{
					Port:           50051,
					Mode:           "server",
					ServerCertPath: "/path/to/cert",
					ServerKeyPath:  "/path/to/key",
					CACertPath:     "/path/to/ca",
				},
				Worker:  WorkerConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "/absolute/path"},
				Logging: LoggingConfig{Level: "INVALID"},
			},
			expectError: true,
			errorMsg:    "invalid log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error for %s, but got none", tt.name)
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected validation error for %s: %v", tt.name, err)
				}
			}
		})
	}
}

func TestConvenienceMethods(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Address: "test.example.com",
			Port:    9999,
		},
		Cgroup: CgroupConfig{
			BaseDir: "/test/cgroup",
		},
	}

	// Test GetServerAddress
	addr := cfg.GetServerAddress()
	expected := "test.example.com:9999"
	if addr != expected {
		t.Errorf("Expected GetServerAddress '%s', got '%s'", expected, addr)
	}

	// Test GetCgroupPath
	jobID := "test-job-123"
	cgroupPath := cfg.GetCgroupPath(jobID)
	expectedPath := "/test/cgroup/job-test-job-123"
	if cgroupPath != expectedPath {
		t.Errorf("Expected GetCgroupPath '%s', got '%s'", expectedPath, cgroupPath)
	}

	// Test GetServerSecurityConfig
	cfg.Server.ServerCertPath = "/path/to/server.pem"
	cfg.Server.ServerKeyPath = "/path/to/server.key"
	cfg.Server.CACertPath = "/path/to/ca.pem"

	serverCert, serverKey, caCert := cfg.GetServerSecurityConfig()
	if serverCert != "/path/to/server.pem" {
		t.Errorf("Expected serverCert '/path/to/server.pem', got '%s'", serverCert)
	}
	if serverKey != "/path/to/server.key" {
		t.Errorf("Expected serverKey '/path/to/server.key', got '%s'", serverKey)
	}
	if caCert != "/path/to/ca.pem" {
		t.Errorf("Expected caCert '/path/to/ca.pem', got '%s'", caCert)
	}
}

// Helper functions

func createTestConfigFile(t *testing.T, filename, content string) string {
	t.Helper()

	// Create directory if it doesn't exist
	dir := filepath.Dir(filename)
	if dir != "." && dir != "" {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			t.Fatalf("Failed to create test directory %s: %v", dir, err)
		}
	}

	tmpFile := filepath.Join(".", filename)
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file %s: %v", tmpFile, err)
	}
	return tmpFile
}

func setTestEnvVars(t *testing.T, envVars map[string]string) map[string]string {
	t.Helper()
	original := make(map[string]string)
	for key, value := range envVars {
		original[key] = os.Getenv(key)
		os.Setenv(key, value)
	}
	return original
}

func restoreEnvVars(t *testing.T, envVars map[string]string) {
	t.Helper()
	for key, value := range envVars {
		if value == "" {
			os.Unsetenv(key)
		} else {
			os.Setenv(key, value)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsAtAnyPosition(s, substr)))
}

func containsAtAnyPosition(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
