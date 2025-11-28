package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/ehsaniara/joblet/internal/joblet/auth"
	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/ipc"
	"github.com/ehsaniara/joblet/persist/internal/server"
	"github.com/ehsaniara/joblet/persist/internal/storage"
	"github.com/ehsaniara/joblet/pkg/logger"
)

var (
	configPath = flag.String("config", "/opt/joblet/config/joblet-config.yml", "Path to configuration file")
	version    = "1.0.0-dev"
	commit     = "unknown"
	buildTime  = "unknown"
)

func main() {
	// Handle version subcommand before flag parsing
	if len(os.Args) > 1 && os.Args[1] == "version" {
		println("persist version", version, "(commit:", commit+", built:", buildTime+")")
		os.Exit(0)
	}

	flag.Parse()

	// Initialize logger with defaults first
	log := logger.New().WithMode("persist")
	log.Info("Starting persist",
		"version", version,
		"commit", commit,
		"buildTime", buildTime)

	// Load configuration (includes shared logging config)
	result, err := config.Load(*configPath)
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	cfg := result.Config

	// Apply logging configuration from config file
	if logLevel, err := logger.ParseLevel(result.Logging.Level); err == nil {
		log.SetLevel(logLevel)
	}

	log.Info("Configuration loaded",
		"socket", cfg.IPC.Socket,
		"grpcAddress", cfg.Server.GRPCAddress,
		"storageType", cfg.Storage.Type,
		"nodeId", result.NodeID,
		"logLevel", result.Logging.Level)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize storage backend with graceful fallback
	backend, err := storage.NewBackend(&cfg.Storage, result.NodeID, log)
	actualStorageType := cfg.Storage.Type

	if err != nil {
		log.Error("Failed to initialize storage backend, falling back to local",
			"error", err,
			"requested_backend", cfg.Storage.Type)

		// Fall back to local backend with default directories
		fallbackConfig := &config.StorageConfig{
			Type: "local",
			Local: config.LocalConfig{
				Logs: config.LogStorageConfig{
					Directory: "/opt/joblet/logs",
					Format:    "jsonl",
					Rotation: config.RotationConfig{
						MaxSizeMB:       100,
						MaxFiles:        10,
						CompressRotated: true,
					},
				},
				Metrics: config.MetricStorageConfig{
					Directory: "/opt/joblet/metrics",
					Format:    "jsonl.gz",
					Rotation: config.RotationConfig{
						MaxSizeMB:       50,
						MaxFiles:        5,
						CompressRotated: true,
					},
				},
			},
		}

		backend, err = storage.NewBackend(fallbackConfig, result.NodeID, log)
		if err != nil {
			log.Error("Failed to create fallback local backend", "error", err)
			os.Exit(1)
		}
		actualStorageType = "local"
		log.Warn("========================================================================")
		log.Warn("[PERSIST] WARNING: Running with LOCAL storage backend (fallback mode)")
		log.Warn("[PERSIST] Logs will be stored on disk at /opt/joblet/logs")
		log.Warn("[PERSIST] CloudWatch Logs integration is DISABLED")
		log.Warn("[PERSIST] Reason: failed to connect to " + cfg.Storage.Type)
		log.Warn("[PERSIST] To fix: Check IAM role and CloudWatch permissions")
		log.Warn("========================================================================")
	}
	defer backend.Close()

	log.Info("Storage backend initialized", "type", actualStorageType)

	// Initialize IPC server
	ipcServer := ipc.NewServer(&cfg.IPC, backend, log)
	if err := ipcServer.Start(ctx); err != nil {
		log.Error("Failed to start IPC server", "error", err)
		os.Exit(1)
	}
	defer ipcServer.Stop()

	log.Info("IPC server started", "socket", cfg.IPC.Socket)

	// Initialize authorization
	// Use no-op authorization for Unix socket (internal IPC without TLS)
	// Trust is established by Unix socket file permissions
	authorization := auth.NewNoOpAuthorization()
	log.Info("Authorization initialized (no-op for Unix socket IPC)")

	// Initialize gRPC server with inherited security config
	grpcServer := server.NewGRPCServer(&cfg.Server, backend, log, authorization, &result.Security)
	if err := grpcServer.Start(ctx); err != nil {
		log.Error("Failed to start gRPC server", "error", err)
		os.Exit(1)
	}
	defer grpcServer.Stop()

	log.Info("gRPC server started", "address", cfg.Server.GRPCAddress, "tlsEnabled", true)

	// Ignore SIGPIPE to prevent crashes when stdout/stderr are closed
	signal.Ignore(syscall.SIGPIPE)

	// Wait for signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("persist is running. Press Ctrl+C to stop.")

	// Block until signal received
	sig := <-sigChan
	log.Info("Received signal, shutting down gracefully...", "signal", sig)

	// Cancel context to trigger shutdown
	cancel()

	log.Info("persist stopped")
}
