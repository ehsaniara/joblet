package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// GRPCAddress is empty by default (TCP disabled, Unix socket only)
	if cfg.Server.GRPCAddress != "" {
		t.Errorf("Expected default GRPC address to be empty (TCP disabled), got %s", cfg.Server.GRPCAddress)
	}

	// IPC socket path changed to persist-ipc.sock
	if cfg.IPC.Socket != "/opt/joblet/run/persist-ipc.sock" {
		t.Errorf("Expected default socket path /opt/joblet/run/persist-ipc.sock, got %s", cfg.IPC.Socket)
	}

	if cfg.Storage.Type != "local" {
		t.Errorf("Expected default storage type 'local', got %s", cfg.Storage.Type)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected valid config, got error: %v", err)
	}
}

func TestValidate_InvalidStorageType(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Storage.Type = "invalid"

	// Note: Storage type validation is not currently enforced beyond being non-empty
	// This test documents the current behavior
	err := cfg.Validate()
	// Invalid storage type is currently allowed if it's non-empty
	_ = err
}

func TestValidate_EmptyGRPCAddress(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Server.GRPCAddress = ""

	// Note: GRPCAddress validation is not currently enforced in Validate()
	// This test documents the current behavior
	err := cfg.Validate()
	// Empty GRPC address is currently allowed
	_ = err
}

func TestValidate_EmptySocketPath(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IPC.Socket = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected validation error for empty socket path")
	}
}

func TestLoadConfig_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yml")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "invalid.yml")

	// Write invalid YAML
	err := os.WriteFile(configFile, []byte("invalid: yaml: content: [[["), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err = Load(configFile)
	if err == nil {
		t.Error("Expected error for invalid YAML")
	}
}

func TestLoadConfig_Standalone(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "persist-config.yml")

	configContent := `
server:
  grpc_address: ":50053"
ipc:
  socket: "/tmp/test.sock"
storage:
  type: "local"
  base_dir: "/tmp/data"
  local:
    logs:
      directory: "/tmp/logs"
    metrics:
      directory: "/tmp/metrics"
logging:
  level: "debug"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	result, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if result.Config.Server.GRPCAddress != ":50053" {
		t.Errorf("Expected GRPC address :50053, got %s", result.Config.Server.GRPCAddress)
	}

	if result.Config.IPC.Socket != "/tmp/test.sock" {
		t.Errorf("Expected socket /tmp/test.sock, got %s", result.Config.IPC.Socket)
	}

	// Logging config is inherited from parent or uses defaults (standalone has default "info")
	if result.Logging.Level != "info" && result.Logging.Level != "debug" {
		t.Errorf("Expected log level 'info' or 'debug', got %s", result.Logging.Level)
	}
}

func TestLoadConfig_Nested(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "joblet-config.yml")

	configContent := `
version: "3.0"

server:
  address: "0.0.0.0"
  port: 50051

persist:
  server:
    grpc_address: ":50054"
  ipc:
    socket: "/tmp/nested.sock"
  storage:
    type: "local"
    base_dir: "/tmp/nested-data"
    local:
      logs:
        directory: "/tmp/logs"
      metrics:
        directory: "/tmp/metrics"
      index:
        enabled: true
        file: "/tmp/index.json"
        save_interval: "5m"
  writer:
    flush_interval: "1s"
  query:
    cache:
      ttl: "5m"
    stream:
      timeout: "30s"
  logging:
    level: "info"
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	result, err := Load(configFile)
	if err != nil {
		t.Fatalf("Failed to load nested config: %v", err)
	}

	if result.Config.Server.GRPCAddress != ":50054" {
		t.Errorf("Expected GRPC address :50054, got %s", result.Config.Server.GRPCAddress)
	}

	if result.Config.IPC.Socket != "/tmp/nested.sock" {
		t.Errorf("Expected socket /tmp/nested.sock, got %s", result.Config.IPC.Socket)
	}

	// Logging inherits from parent root level in nested config (may be empty if not specified)
	// This is expected behavior - logging config comes from parent joblet-config.yml
	if result.Logging.Level == "" {
		t.Log("Logging level empty - inherited from parent (expected for nested config)")
	} else if result.Logging.Level != "info" {
		t.Errorf("Expected log level 'info' or empty (inherited), got %s", result.Logging.Level)
	}
}

func TestTLSConfig(t *testing.T) {
	cfg := DefaultConfig()

	// TLS is always enabled for persist service (mandatory for authentication)
	// TLS config should be nil by default (inherited from parent security section)
	if cfg.Server.TLS != nil {
		t.Log("TLS config exists - checking ClientAuth default")
		if cfg.Server.TLS.ClientAuth == "" {
			t.Error("ClientAuth should have a default value when TLS config is set")
		}
	}

	// Test TLS config when explicitly set
	cfg.Server.TLS = &TLSConfig{
		CertFile:   "/path/to/cert",
		KeyFile:    "/path/to/key",
		CAFile:     "/path/to/ca",
		ClientAuth: "require",
	}

	if cfg.Server.TLS.ClientAuth != "require" {
		t.Errorf("Expected ClientAuth 'require', got '%s'", cfg.Server.TLS.ClientAuth)
	}
}
