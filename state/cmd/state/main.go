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

	// Create storage backend
	storageConfig := convertToStorageConfig(&cfg.State)
	backend, err := storage.NewBackend(storageConfig)
	if err != nil {
		log.Fatal("failed to create storage backend", "error", err)
	}
	defer backend.Close()

	// Health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := backend.HealthCheck(ctx); err != nil {
		cancel()
		log.Fatal("backend health check failed", "error", err)
	}
	cancel()

	log.Info("[STATE] Storage backend initialized successfully", "backend", cfg.State.Backend)

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

	// Convert Redis config (future)
	if stateConfig.Storage.Redis != nil {
		storageConfig.Redis = &storage.RedisConfig{
			Endpoint: stateConfig.Storage.Redis.Endpoint,
			Password: stateConfig.Storage.Redis.Password,
			DB:       stateConfig.Storage.Redis.DB,
			TTLDays:  stateConfig.Storage.Redis.TTLDays,
		}
	}

	return storageConfig
}
