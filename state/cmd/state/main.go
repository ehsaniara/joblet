package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/state/internal/ipc"
	"github.com/ehsaniara/joblet/state/internal/storage"
	"gopkg.in/yaml.v3"
)

const (
	defaultSocketPath = "/opt/joblet/run/state-ipc.sock"
	defaultConfigPath = "/opt/joblet/config/joblet-config.yml"
)

func main() {
	log := logger.WithField("component", "state")

	log.Info("[STATE] Starting state service...")

	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		log.Fatal("failed to load configuration", "error", err)
	}

	// Validate configuration
	if err := validateConfig(cfg); err != nil {
		log.Fatal("invalid configuration", "error", err)
	}

	log.Info("[STATE] Configuration loaded",
		"backend", cfg.State.Backend,
		"socket", cfg.State.Socket)

	// Create storage backend with graceful fallback
	storageConfig := convertToStorageConfig(&cfg.State)
	backend, err := storage.NewBackend(storageConfig)
	actualBackend := cfg.State.Backend

	if err != nil {
		log.Error("failed to create storage backend, falling back to memory", "error", err, "requested_backend", cfg.State.Backend)
		// Fall back to memory backend
		storageConfig.Backend = "memory"
		backend, err = storage.NewBackend(storageConfig)
		if err != nil {
			log.Fatal("failed to create fallback memory backend", "error", err)
		}
		actualBackend = "memory"
		log.Warn("========================================================================")
		log.Warn("[STATE] WARNING: Running with IN-MEMORY backend (fallback mode)")
		log.Warn("[STATE] Job state will NOT persist across restarts!")
		log.Warn("[STATE] Reason: failed to connect to " + cfg.State.Backend)
		log.Warn("[STATE] To fix: Check VPC Endpoint, IAM role, and DynamoDB table")
		log.Warn("========================================================================")
	} else {
		// Health check only if primary backend succeeded
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := backend.HealthCheck(ctx); err != nil {
			cancel()
			log.Error("backend health check failed, falling back to memory", "error", err, "requested_backend", cfg.State.Backend)
			backend.Close()
			// Fall back to memory backend
			storageConfig.Backend = "memory"
			backend, err = storage.NewBackend(storageConfig)
			if err != nil {
				log.Fatal("failed to create fallback memory backend", "error", err)
			}
			actualBackend = "memory"
			log.Warn("========================================================================")
			log.Warn("[STATE] WARNING: Running with IN-MEMORY backend (fallback mode)")
			log.Warn("[STATE] Job state will NOT persist across restarts!")
			log.Warn("[STATE] Reason: health check failed for " + cfg.State.Backend)
			log.Warn("[STATE] To fix: Check VPC Endpoint, IAM role, and DynamoDB table")
			log.Warn("========================================================================")
		}
		cancel()
	}
	defer backend.Close()

	log.Info("[STATE] Storage backend initialized successfully", "backend", actualBackend)

	// Create IPC server
	socketPath := cfg.State.Socket
	if socketPath == "" {
		socketPath = defaultSocketPath
	}

	server := ipc.NewServer(socketPath, backend)

	// Start IPC server
	if err := server.Start(); err != nil {
		log.Fatal("failed to start IPC server", "error", err)
	}
	defer server.Stop()

	log.Info("[STATE] IPC server started successfully", "socket", socketPath)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("[STATE] state service is ready")

	// Block until signal received
	sig := <-sigChan
	log.Info("[STATE] Received shutdown signal, stopping service...", "signal", sig)

	// Graceful shutdown
	if err := server.Stop(); err != nil {
		log.Error("error stopping IPC server", "error", err)
	}

	if err := backend.Close(); err != nil {
		log.Error("error closing backend", "error", err)
	}

	log.Info("[STATE] state service stopped gracefully")
}

func loadConfig() (*config.Config, error) {
	// Try config paths in order
	configPaths := []string{
		os.Getenv("JOBLET_CONFIG_PATH"),
		defaultConfigPath,
		"./config/joblet-config.yml",
		"./joblet-config.yml",
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
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		var cfg config.Config
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
		}

		return &cfg, nil
	}

	return nil, fmt.Errorf("no configuration file found")
}

func validateConfig(cfg *config.Config) error {
	if cfg.State.Backend == "" {
		return fmt.Errorf("state backend is not configured")
	}

	if cfg.State.Backend == "dynamodb" {
		if cfg.State.Storage.DynamoDB == nil {
			return fmt.Errorf("dynamodb configuration is required when backend is 'dynamodb'")
		}
		if cfg.State.Storage.DynamoDB.TableName == "" {
			return fmt.Errorf("dynamodb table_name is required")
		}
	}

	return nil
}

func convertToStorageConfig(stateConfig *config.StateConfig) *storage.Config {
	storageConfig := &storage.Config{
		Backend: stateConfig.Backend,
	}

	// Convert DynamoDB config
	if stateConfig.Storage.DynamoDB != nil {
		storageConfig.DynamoDB = &storage.DynamoDBConfig{
			Region:        stateConfig.Storage.DynamoDB.Region,
			TableName:     stateConfig.Storage.DynamoDB.TableName,
			TTLEnabled:    stateConfig.Storage.DynamoDB.TTLEnabled,
			TTLAttribute:  stateConfig.Storage.DynamoDB.TTLAttribute,
			TTLDays:       stateConfig.Storage.DynamoDB.TTLDays,
			ReadCapacity:  stateConfig.Storage.DynamoDB.ReadCapacity,
			WriteCapacity: stateConfig.Storage.DynamoDB.WriteCapacity,
			BatchSize:     stateConfig.Storage.DynamoDB.BatchSize,
			BatchInterval: stateConfig.Storage.DynamoDB.BatchInterval,
		}
	}

	return storageConfig
}
