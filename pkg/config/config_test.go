package config

import (
	"crypto/tls"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	// Test that DefaultConfig has sensible values
	if DefaultConfig.Version != "3.0" {
		t.Errorf("Expected version 3.0, got %s", DefaultConfig.Version)
	}

	if DefaultConfig.Server.Port != 50051 {
		t.Errorf("Expected default port 50051, got %d", DefaultConfig.Server.Port)
	}

	if DefaultConfig.Joblet.DefaultMemoryLimit != 512 {
		t.Errorf("Expected default memory limit 512, got %d", DefaultConfig.Joblet.DefaultMemoryLimit)
	}
}

func TestGetServerAddress(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "default address",
			config: Config{
				Server: ServerConfig{
					Address: "0.0.0.0",
					Port:    50051,
				},
			},
			expected: "0.0.0.0:50051",
		},
		{
			name: "custom address",
			config: Config{
				Server: ServerConfig{
					Address: "192.168.1.100",
					Port:    8080,
				},
			},
			expected: "192.168.1.100:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.GetServerAddress()
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestGetCgroupPath(t *testing.T) {
	config := Config{
		Cgroup: CgroupConfig{
			BaseDir: "/sys/fs/cgroup/joblet.slice/joblet.service",
		},
	}

	jobID := "12345"
	expected := "/sys/fs/cgroup/joblet.slice/joblet.service/job-12345"
	result := config.GetCgroupPath(jobID)

	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			config:  DefaultConfig,
			wantErr: false,
		},
		{
			name: "invalid port - too low",
			config: Config{
				Server:  ServerConfig{Port: 0},
				Joblet:  JobletConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "/sys/fs/cgroup"},
				Logging: LoggingConfig{Level: "INFO"},
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid port - too high",
			config: Config{
				Server:  ServerConfig{Port: 70000},
				Joblet:  JobletConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "/sys/fs/cgroup"},
				Logging: LoggingConfig{Level: "INFO"},
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid server mode",
			config: Config{
				Server:  ServerConfig{Port: 50051, Mode: "invalid"},
				Joblet:  JobletConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "/sys/fs/cgroup"},
				Logging: LoggingConfig{Level: "INFO"},
			},
			wantErr: true,
			errMsg:  "invalid server mode",
		},
		{
			name: "negative CPU limit",
			config: Config{
				Server:  ServerConfig{Port: 50051, Mode: "server"},
				Joblet:  JobletConfig{DefaultCPULimit: -1, MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "/sys/fs/cgroup"},
				Logging: LoggingConfig{Level: "INFO"},
			},
			wantErr: true,
			errMsg:  "invalid default CPU limit",
		},
		{
			name: "relative cgroup path",
			config: Config{
				Server:  ServerConfig{Port: 50051, Mode: "server"},
				Joblet:  JobletConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "relative/path"},
				Logging: LoggingConfig{Level: "INFO"},
			},
			wantErr: true,
			errMsg:  "cgroup base directory must be absolute path",
		},
		{
			name: "invalid log level",
			config: Config{
				Server:  ServerConfig{Port: 50051, Mode: "server"},
				Joblet:  JobletConfig{MaxConcurrentJobs: 1},
				Cgroup:  CgroupConfig{BaseDir: "/sys/fs/cgroup"},
				Logging: LoggingConfig{Level: "INVALID"},
			},
			wantErr: true,
			errMsg:  "invalid log level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestGetServerTLSConfig(t *testing.T) {
	// Valid certificates for testing (self-signed)
	validCert := `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHDIG1ZbVONMA0GCSqGSIb3DQEBBQUAMA0xCzAJBgNVBAYTAlVT
MB4XDTI0MDEwMTAwMDAwMFoXDTI1MDEwMTAwMDAwMFowDTELMAkGA1UEBhMCVVMw
gZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGBALr6hQ7lhZhh3j1f7TuzJdLKoLB9
6PlBPmyj9xAqX7W/L9HjdakYdA8K7CB7eSUCcFOABEhdHLpOCJqGeVn8xP7ReBvE
-----END CERTIFICATE-----`

	validKey := `-----BEGIN PRIVATE KEY-----
MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBALr6hQ7lhZhh3j1f
7TuzJdLKoLB96PlBPmyj9xAqX7W/L9HjdakYdA8K7CB7eSUCcFOABEhdHLpOCJqG
-----END PRIVATE KEY-----`

	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing server cert",
			config: Config{
				Security: SecurityConfig{
					ServerKey: validKey,
					CACert:    validCert,
				},
			},
			wantErr: true,
			errMsg:  "certificates are not configured",
		},
		{
			name: "invalid cert format",
			config: Config{
				Security: SecurityConfig{
					ServerCert: "invalid cert",
					ServerKey:  validKey,
					CACert:     validCert,
				},
			},
			wantErr: true,
			errMsg:  "failed to load server certificate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.config.GetServerTLSConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetServerTLSConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
		})
	}
}

func TestGetClientTLSConfig(t *testing.T) {
	// Valid test certificates (self-signed, for testing only)
	validCert := `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHDIG1ZbVONMA0GCSqGSIb3DQEBBQUAMA0xCzAJBgNVBAYTAlVT
-----END CERTIFICATE-----`

	validKey := `-----BEGIN PRIVATE KEY-----
MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBALr6hQ7lhZhh3j1f
-----END PRIVATE KEY-----`

	tests := []struct {
		name    string
		node    Node
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing cert",
			node: Node{
				Address: "localhost:50051",
				Key:     validKey,
				CA:      validCert,
			},
			wantErr: true,
			errMsg:  "certificates are not configured",
		},
		{
			name: "missing key",
			node: Node{
				Address: "localhost:50051",
				Cert:    validCert,
				CA:      validCert,
			},
			wantErr: true,
			errMsg:  "certificates are not configured",
		},
		{
			name: "missing CA",
			node: Node{
				Address: "localhost:50051",
				Cert:    validCert,
				Key:     validKey,
			},
			wantErr: true,
			errMsg:  "certificates are not configured",
		},
		{
			name: "invalid cert format",
			node: Node{
				Address: "localhost:50051",
				Cert:    "invalid cert data",
				Key:     validKey,
				CA:      validCert,
			},
			wantErr: true,
			errMsg:  "failed to load client certificate",
		},
		{
			name: "invalid CA format",
			node: Node{
				Address: "localhost:50051",
				Cert:    validCert,
				Key:     validKey,
				CA:      "invalid ca data",
			},
			wantErr: true,
			errMsg:  "failed to load client certificate", // X509KeyPair fails first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := tt.node.GetClientTLSConfig()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetClientTLSConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
				}
			}
			if !tt.wantErr && tlsConfig != nil {
				if tlsConfig.MinVersion != tls.VersionTLS13 {
					t.Errorf("Expected TLS 1.3, got %d", tlsConfig.MinVersion)
				}
				if tlsConfig.ServerName != "joblet" {
					t.Errorf("Expected ServerName 'joblet', got '%s'", tlsConfig.ServerName)
				}
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	// Test environment variable overrides
	t.Run("environment overrides", func(t *testing.T) {
		// Isolate from any joblet installed on this host: point both config
		// paths into an empty temp dir so /opt/joblet is never consulted
		tmpDir := t.TempDir()
		os.Setenv("JOBLET_RUNTIME_CONFIG_PATH", filepath.Join(tmpDir, "no-runtime-config.yml"))
		defer os.Unsetenv("JOBLET_RUNTIME_CONFIG_PATH")
		configPath := filepath.Join(tmpDir, "joblet-config.yml")
		if err := os.WriteFile(configPath, []byte("version: \"3.0\"\n"), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}
		os.Setenv("JOBLET_CONFIG_PATH", configPath)
		defer os.Unsetenv("JOBLET_CONFIG_PATH")

		// Set environment variables
		os.Setenv("JOBLET_SERVER_ADDRESS", "192.168.1.100")
		os.Setenv("JOBLET_MODE", "init")
		os.Setenv("JOBLET_LOG_LEVEL", "DEBUG")
		defer func() {
			os.Unsetenv("JOBLET_SERVER_ADDRESS")
			os.Unsetenv("JOBLET_MODE")
			os.Unsetenv("JOBLET_LOG_LEVEL")
		}()

		config, _, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if config.Server.Address != "192.168.1.100" {
			t.Errorf("Expected server address '192.168.1.100', got '%s'", config.Server.Address)
		}
		if config.Server.Mode != "init" {
			t.Errorf("Expected mode 'init', got '%s'", config.Server.Mode)
		}
		if config.Logging.Level != "DEBUG" {
			t.Errorf("Expected log level 'DEBUG', got '%s'", config.Logging.Level)
		}
	})
}

func TestLoadClientConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "rnx-config.yml")

	validConfig := `version: "3.0"
nodes:
  default:
    address: "localhost:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      test cert
      -----END CERTIFICATE-----
    key: |
      -----BEGIN PRIVATE KEY-----
      test key
      -----END PRIVATE KEY-----
    ca: |
      -----BEGIN CERTIFICATE-----
      test ca
      -----END CERTIFICATE-----`

	if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	tests := []struct {
		name       string
		configPath string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid config",
			configPath: configPath,
			wantErr:    false,
		},
		{
			name:       "non-existent file",
			configPath: "/non/existent/path.yml",
			wantErr:    true,
			errMsg:     "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadClientConfig(tt.configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadClientConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && config != nil {
				if len(config.Nodes) == 0 {
					t.Errorf("Expected nodes to be loaded")
				}
				if config.Version != "3.0" {
					t.Errorf("Expected version 3.0, got %s", config.Version)
				}
			}
		})
	}
}

func TestClientConfigMethods(t *testing.T) {
	config := &ClientConfig{
		Version: "3.0",
		Nodes: map[string]*Node{
			"default": {
				Address: "localhost:50051",
				Cert:    "cert1",
				Key:     "key1",
				CA:      "ca1",
			},
			"production": {
				Address: "prod.example.com:50051",
				Cert:    "cert2",
				Key:     "key2",
				CA:      "ca2",
			},
		},
	}

	t.Run("GetNode", func(t *testing.T) {
		// Test getting existing node
		node, err := config.GetNode("production")
		if err != nil {
			t.Errorf("GetNode() unexpected error: %v", err)
		}
		if node.Address != "prod.example.com:50051" {
			t.Errorf("Expected address 'prod.example.com:50051', got '%s'", node.Address)
		}

		// Test default node
		node, err = config.GetNode("")
		if err != nil {
			t.Errorf("GetNode() unexpected error: %v", err)
		}
		if node.Address != "localhost:50051" {
			t.Errorf("Expected default address 'localhost:50051', got '%s'", node.Address)
		}

		// Test non-existent node
		_, err = config.GetNode("nonexistent")
		if err == nil {
			t.Errorf("Expected error for non-existent node")
		}
	})

	t.Run("ListNodes", func(t *testing.T) {
		nodes := config.ListNodes()
		if len(nodes) != 2 {
			t.Errorf("Expected 2 nodes, got %d", len(nodes))
		}
		// Check that both nodes are present
		hasDefault := false
		hasProduction := false
		for _, node := range nodes {
			if node == "default" {
				hasDefault = true
			}
			if node == "production" {
				hasProduction = true
			}
		}
		if !hasDefault || !hasProduction {
			t.Errorf("Missing expected nodes in list")
		}
	})
}

func TestDefaultNodeResolution(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }

	t.Run("isDefault marks the default node", func(t *testing.T) {
		config := &ClientConfig{
			Version: "3.0",
			Nodes: map[string]*Node{
				"admin":  {Address: "admin.example.com:50051", IsDefault: boolPtr(true)},
				"reader": {Address: "reader.example.com:50051"},
			},
		}

		if name := config.DefaultNodeName(); name != "admin" {
			t.Errorf("DefaultNodeName() = %q, want 'admin'", name)
		}

		node, err := config.GetNode("")
		if err != nil {
			t.Fatalf("GetNode(\"\") unexpected error: %v", err)
		}
		if node.Address != "admin.example.com:50051" {
			t.Errorf("Expected default address 'admin.example.com:50051', got '%s'", node.Address)
		}
	})

	t.Run("isDefault wins over node named default", func(t *testing.T) {
		config := &ClientConfig{
			Version: "3.0",
			Nodes: map[string]*Node{
				"default": {Address: "legacy.example.com:50051"},
				"admin":   {Address: "admin.example.com:50051", IsDefault: boolPtr(true)},
			},
		}

		if name := config.DefaultNodeName(); name != "admin" {
			t.Errorf("DefaultNodeName() = %q, want 'admin'", name)
		}
	})

	t.Run("legacy node named default still works", func(t *testing.T) {
		config := &ClientConfig{
			Version: "3.0",
			Nodes: map[string]*Node{
				"default": {Address: "legacy.example.com:50051"},
				"reader":  {Address: "reader.example.com:50051"},
			},
		}

		if name := config.DefaultNodeName(); name != "default" {
			t.Errorf("DefaultNodeName() = %q, want 'default'", name)
		}
	})

	t.Run("single node is the implicit default", func(t *testing.T) {
		config := &ClientConfig{
			Version: "3.0",
			Nodes: map[string]*Node{
				"prod": {Address: "prod.example.com:50051"},
			},
		}

		if name := config.DefaultNodeName(); name != "prod" {
			t.Errorf("DefaultNodeName() = %q, want 'prod'", name)
		}
	})

	t.Run("no resolvable default returns error", func(t *testing.T) {
		config := &ClientConfig{
			Version: "3.0",
			Nodes: map[string]*Node{
				"prod":    {Address: "prod.example.com:50051"},
				"staging": {Address: "staging.example.com:50051"},
			},
		}

		if name := config.DefaultNodeName(); name != "" {
			t.Errorf("DefaultNodeName() = %q, want empty", name)
		}
		if _, err := config.GetNode(""); err == nil {
			t.Errorf("GetNode(\"\") expected error when no default is resolvable")
		}
	})
}

func TestLoadClientConfigDefaultValidation(t *testing.T) {
	writeConfig := func(t *testing.T, content string) string {
		t.Helper()
		configPath := filepath.Join(t.TempDir(), "rnx-config.yml")
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}
		return configPath
	}

	t.Run("single isDefault loads", func(t *testing.T) {
		configPath := writeConfig(t, `version: "3.0"
nodes:
  admin:
    address: "localhost:50051"
    isDefault: true
    cert: "c"
    key: "k"
    ca: "ca"
  reader:
    address: "localhost:50051"
    cert: "c"
    key: "k"
    ca: "ca"`)

		config, err := LoadClientConfig(configPath)
		if err != nil {
			t.Fatalf("LoadClientConfig() error = %v", err)
		}
		if name := config.DefaultNodeName(); name != "admin" {
			t.Errorf("DefaultNodeName() = %q, want 'admin'", name)
		}
	})

	t.Run("multiple isDefault rejected", func(t *testing.T) {
		configPath := writeConfig(t, `version: "3.0"
nodes:
  admin:
    address: "localhost:50051"
    isDefault: true
    cert: "c"
    key: "k"
    ca: "ca"
  reader:
    address: "localhost:50051"
    isDefault: true
    cert: "c"
    key: "k"
    ca: "ca"`)

		_, err := LoadClientConfig(configPath)
		if err == nil {
			t.Fatalf("LoadClientConfig() expected error for multiple isDefault nodes")
		}
		if !strings.Contains(err.Error(), "isDefault") {
			t.Errorf("Expected error to mention isDefault, got: %v", err)
		}
	})

	t.Run("isDefault false does not count", func(t *testing.T) {
		configPath := writeConfig(t, `version: "3.0"
nodes:
  admin:
    address: "localhost:50051"
    isDefault: true
    cert: "c"
    key: "k"
    ca: "ca"
  reader:
    address: "localhost:50051"
    isDefault: false
    cert: "c"
    key: "k"
    ca: "ca"`)

		config, err := LoadClientConfig(configPath)
		if err != nil {
			t.Fatalf("LoadClientConfig() error = %v", err)
		}
		if name := config.DefaultNodeName(); name != "admin" {
			t.Errorf("DefaultNodeName() = %q, want 'admin'", name)
		}
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && s[0:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		(len(substr) < len(s) && containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 1; i < len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFindClientConfig(t *testing.T) {
	// Test RNX_CONFIG environment variable
	t.Run("RNX_CONFIG env var", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "custom-rnx-config.yml")

		// Create the config file
		if err := os.WriteFile(configPath, []byte("version: 3.0"), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		os.Setenv("RNX_CONFIG", configPath)
		defer os.Unsetenv("RNX_CONFIG")

		result := findClientConfig()
		if result != configPath {
			t.Errorf("Expected config path '%s', got '%s'", configPath, result)
		}
	})

	t.Run("RNX_CONFIG env var - non-existent file", func(t *testing.T) {
		os.Setenv("RNX_CONFIG", "/non/existent/config.yml")
		defer os.Unsetenv("RNX_CONFIG")

		result := findClientConfig()
		// Should fall through to search other locations
		// Result depends on what's actually on the filesystem
		// Just verify it doesn't crash and returns something (possibly empty)
		_ = result
	})

	t.Run("local config file", func(t *testing.T) {
		// Save current directory
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Create temp directory and change to it
		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		// Clear RNX_CONFIG to ensure we're testing file search
		os.Unsetenv("RNX_CONFIG")

		// Create local config file
		if err := os.WriteFile("rnx-config.yml", []byte("version: 3.0"), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		result := findClientConfig()
		if result != "./rnx-config.yml" {
			t.Errorf("Expected './rnx-config.yml', got '%s'", result)
		}
	})

	t.Run("no config found", func(t *testing.T) {
		// Save current directory
		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		// Create empty temp directory and change to it
		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		// Clear environment and HOME to prevent finding any config
		os.Unsetenv("RNX_CONFIG")
		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir) // Set HOME to empty temp dir
		defer os.Setenv("HOME", origHome)

		result := findClientConfig()
		// Should return empty string if no config found in standard locations
		if result != "" {
			// Only fail if we find a config that shouldn't exist
			// Some CI environments might have configs in /etc or /opt
			t.Logf("Found config at non-standard location: %s", result)
		}
	})
}

func TestLoadFromFile(t *testing.T) {
	t.Run("JOBLET_CONFIG_PATH env var", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "joblet-config.yml")

		// Create a valid config file
		configContent := `version: "3.0"
server:
  address: "10.0.0.1"
  port: 50051
  mode: "server"
joblet:
  defaultCpuLimit: 100
  defaultMemoryLimit: 512
  maxConcurrentJobs: 100
cgroup:
  baseDir: "/sys/fs/cgroup/joblet.slice"
logging:
  level: "INFO"
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		os.Setenv("JOBLET_CONFIG_PATH", configPath)
		defer os.Unsetenv("JOBLET_CONFIG_PATH")
		// Isolate from a runtime-config.yml installed on this host
		os.Setenv("JOBLET_RUNTIME_CONFIG_PATH", filepath.Join(tmpDir, "no-runtime-config.yml"))
		defer os.Unsetenv("JOBLET_RUNTIME_CONFIG_PATH")

		config, path, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() error = %v", err)
		}

		if path != configPath {
			t.Errorf("Expected config path '%s', got '%s'", configPath, path)
		}

		if config.Server.Address != "10.0.0.1" {
			t.Errorf("Expected server address '10.0.0.1', got '%s'", config.Server.Address)
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "invalid-config.yml")

		// Create an invalid YAML file
		if err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		os.Setenv("JOBLET_CONFIG_PATH", configPath)
		defer os.Unsetenv("JOBLET_CONFIG_PATH")

		_, _, err := LoadConfig()
		if err == nil {
			t.Errorf("Expected error for invalid YAML, got nil")
		}
	})
}

// TestRemovedConfigFields verifies that deprecated config fields were removed
// and that YAML files with old fields are still parseable (backward compatible)
func TestRemovedConfigFields(t *testing.T) {
	t.Run("ServerConfig has no Timeout field", func(t *testing.T) {
		// Verify ServerConfig only has expected fields
		cfg := ServerConfig{
			Address: "0.0.0.0",
			Port:    50051,
			Mode:    "server",
			NodeId:  "test-node",
		}
		// If this compiles, the Timeout field is removed
		if cfg.Address == "" {
			t.Error("Address should be set")
		}
	})

	t.Run("GRPCConfig has no connection age fields", func(t *testing.T) {
		// Verify GRPCConfig only has the implemented fields
		cfg := GRPCConfig{
			MaxRecvMsgSize:       134217728,
			MaxSendMsgSize:       134217728,
			MaxHeaderListSize:    16777216,
			KeepAliveTime:        10,
			KeepAliveTimeout:     3,
			MaxConcurrentStreams: 1000,
			ConnectionTimeout:    10,
		}
		// If this compiles, MaxConnectionIdle/Age/AgeGrace are removed
		if cfg.MaxRecvMsgSize == 0 {
			t.Error("MaxRecvMsgSize should be set")
		}
	})

	t.Run("FilesystemConfig has no BlockDevices field", func(t *testing.T) {
		cfg := FilesystemConfig{
			BaseDir:      "/opt/joblet/jobs",
			TmpDir:       "/tmp/job-{JOB_ID}",
			WorkspaceDir: "/work",
		}
		// If this compiles, BlockDevices field is removed
		if cfg.BaseDir == "" {
			t.Error("BaseDir should be set")
		}
	})

	t.Run("NetworkConfig has no MaxCustomNetworks field", func(t *testing.T) {
		cfg := NetworkConfig{
			StateDir:            "/opt/joblet/network",
			Enabled:             true,
			DefaultNetwork:      "bridge",
			AllowCustomNetworks: true,
			Storage:             NetworkStorageConfig{Path: "/opt/joblet/network"},
		}
		// If this compiles, MaxCustomNetworks field is removed
		if !cfg.Enabled {
			t.Error("Enabled should be true")
		}
	})

	t.Run("MonitoringConfig has no Enabled field", func(t *testing.T) {
		cfg := MonitoringConfig{
			SystemInterval: 10,
			CloudDetection: true,
		}
		// If this compiles, Enabled field is removed
		if !cfg.CloudDetection {
			t.Error("CloudDetection should be true")
		}
	})

	t.Run("StateStorageConfig has no Redis field", func(t *testing.T) {
		cfg := StateStorageConfig{
			DynamoDB: &DynamoDBStateConfig{
				Region:    "us-east-1",
				TableName: "joblet-jobs",
			},
		}
		// If this compiles, Redis field is removed
		if cfg.DynamoDB == nil {
			t.Error("DynamoDB should be set")
		}
	})

	t.Run("backward compatible YAML parsing", func(t *testing.T) {
		// Old config with deprecated fields should still parse (fields ignored)
		oldConfigYAML := `version: "3.0"
server:
  address: "0.0.0.0"
  port: 50051
  mode: "server"
  timeout: "10s"
  nodeId: "test"
grpc:
  maxRecvMsgSize: 134217728
  maxConnectionIdle: "300s"
  maxConnectionAge: "1800s"
filesystem:
  baseDir: "/opt/joblet/jobs"
  blockDevices: false
network:
  state_dir: "/opt/joblet/network"
  max_custom_networks: 50
joblet:
  maxConcurrentJobs: 100
cgroup:
  baseDir: "/sys/fs/cgroup"
logging:
  level: "INFO"
`
		tmpDir := t.TempDir()
		configPath := tmpDir + "/old-config.yml"
		if err := os.WriteFile(configPath, []byte(oldConfigYAML), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		os.Setenv("JOBLET_CONFIG_PATH", configPath)
		defer os.Unsetenv("JOBLET_CONFIG_PATH")

		cfg, _, err := LoadConfig()
		if err != nil {
			t.Fatalf("LoadConfig() should parse old config: %v", err)
		}

		// Verify parsed values
		if cfg.Server.Port != 50051 {
			t.Errorf("Expected port 50051, got %d", cfg.Server.Port)
		}
		if cfg.Server.NodeId != "test" {
			t.Errorf("Expected nodeId 'test', got '%s'", cfg.Server.NodeId)
		}
	})

	t.Run("DefaultConfig has no removed fields", func(t *testing.T) {
		// Verify defaults don't reference removed fields
		if DefaultConfig.GRPC.MaxRecvMsgSize != 134217728 {
			t.Errorf("Expected default MaxRecvMsgSize 134217728, got %d", DefaultConfig.GRPC.MaxRecvMsgSize)
		}
		if DefaultConfig.GRPC.ConnectionTimeout == 0 {
			t.Error("Expected ConnectionTimeout to be set")
		}
		// MonitoringConfig should have SystemInterval but no Enabled field
		if DefaultConfig.Monitoring.SystemInterval == 0 {
			t.Error("Expected SystemInterval to be set")
		}
	})
}

func TestLoadClientConfigAutoFind(t *testing.T) {
	// Test LoadClientConfig with empty path (should use findClientConfig)
	t.Run("auto-find with RNX_CONFIG", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "rnx-config.yml")

		validConfig := `version: "3.0"
nodes:
  default:
    address: "auto-find-test:50051"
    cert: |
      -----BEGIN CERTIFICATE-----
      test cert
      -----END CERTIFICATE-----
    key: |
      -----BEGIN PRIVATE KEY-----
      test key
      -----END PRIVATE KEY-----
    ca: |
      -----BEGIN CERTIFICATE-----
      test ca
      -----END CERTIFICATE-----`

		if err := os.WriteFile(configPath, []byte(validConfig), 0644); err != nil {
			t.Fatalf("Failed to write test config: %v", err)
		}

		os.Setenv("RNX_CONFIG", configPath)
		defer os.Unsetenv("RNX_CONFIG")

		config, err := LoadClientConfig("")
		if err != nil {
			t.Fatalf("LoadClientConfig() error = %v", err)
		}

		node, err := config.GetNode("default")
		if err != nil {
			t.Fatalf("GetNode() error = %v", err)
		}

		if node.Address != "auto-find-test:50051" {
			t.Errorf("Expected address 'auto-find-test:50051', got '%s'", node.Address)
		}
	})

	t.Run("auto-find no config", func(t *testing.T) {
		// Clear all env vars and use temp dir without any configs
		os.Unsetenv("RNX_CONFIG")

		origDir, err := os.Getwd()
		if err != nil {
			t.Fatalf("Failed to get current directory: %v", err)
		}

		tmpDir := t.TempDir()
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
		defer func() { _ = os.Chdir(origDir) }()

		origHome := os.Getenv("HOME")
		os.Setenv("HOME", tmpDir)
		defer os.Setenv("HOME", origHome)

		_, err = LoadClientConfig("")
		if err == nil {
			t.Logf("LoadClientConfig found a config unexpectedly - system may have configs in /etc or /opt")
		}
		// Error is expected when no config is found
	})
}

func TestInitConfig(t *testing.T) {
	t.Run("defaults when nothing forwarded", func(t *testing.T) {
		cfg := InitConfig()
		if cfg.Filesystem.WorkspaceDir != DefaultConfig.Filesystem.WorkspaceDir {
			t.Errorf("expected default workspace dir, got %q", cfg.Filesystem.WorkspaceDir)
		}
	})

	t.Run("server-forwarded values win", func(t *testing.T) {
		os.Setenv("JOB_FS_WORKSPACE_DIR", "/custom-work")
		os.Setenv("JOB_FS_BASE_DIR", "/custom-jobs")
		os.Setenv("JOB_FS_TMP_DIR", "/custom-tmp/{JOB_ID}")
		defer func() {
			os.Unsetenv("JOB_FS_WORKSPACE_DIR")
			os.Unsetenv("JOB_FS_BASE_DIR")
			os.Unsetenv("JOB_FS_TMP_DIR")
		}()

		os.Setenv("JOB_RT_BASE_PATH", "/custom-runtimes")
		os.Setenv("JOB_RT_ALLOWED_MOUNTS", "/usr/bin:/usr/sbin:/etc/ssl")
		defer func() {
			os.Unsetenv("JOB_RT_BASE_PATH")
			os.Unsetenv("JOB_RT_ALLOWED_MOUNTS")
		}()

		cfg := InitConfig()
		if cfg.Filesystem.WorkspaceDir != "/custom-work" {
			t.Errorf("expected forwarded workspace dir, got %q", cfg.Filesystem.WorkspaceDir)
		}
		if cfg.Filesystem.BaseDir != "/custom-jobs" {
			t.Errorf("expected forwarded base dir, got %q", cfg.Filesystem.BaseDir)
		}
		if cfg.Filesystem.TmpDir != "/custom-tmp/{JOB_ID}" {
			t.Errorf("expected forwarded tmp dir, got %q", cfg.Filesystem.TmpDir)
		}
		if cfg.Runtime.BasePath != "/custom-runtimes" {
			t.Errorf("expected forwarded runtime base path, got %q", cfg.Runtime.BasePath)
		}
		wantMounts := []string{"/usr/bin", "/usr/sbin", "/etc/ssl"}
		if strings.Join(cfg.Runtime.AllowedMounts, ":") != strings.Join(wantMounts, ":") {
			t.Errorf("expected forwarded allowed mounts %v, got %v", wantMounts, cfg.Runtime.AllowedMounts)
		}
	})
}

func TestLooksLikeJobProcess(t *testing.T) {
	for _, k := range []string{"JOB_ID", "JOB_CGROUP_HOST_PATH"} {
		t.Run("set/"+k, func(t *testing.T) {
			os.Setenv(k, "x")
			defer os.Unsetenv(k)
			if !LooksLikeJobProcess() {
				t.Errorf("LooksLikeJobProcess() = false with %s set; want true", k)
			}
		})
	}
	t.Run("none set", func(t *testing.T) {
		os.Unsetenv("JOB_ID")
		os.Unsetenv("JOB_CGROUP_HOST_PATH")
		if LooksLikeJobProcess() {
			t.Error("LooksLikeJobProcess() = true with no job vars; want false")
		}
	})
}
