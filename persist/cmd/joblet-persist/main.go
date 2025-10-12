package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/ehsaniara/joblet/persist/internal/config"
	"github.com/ehsaniara/joblet/persist/internal/ipc"
	"github.com/ehsaniara/joblet/persist/internal/server"
	"github.com/ehsaniara/joblet/persist/internal/storage"
	"github.com/ehsaniara/joblet/persist/pkg/logger"
)

var (
	configPath = flag.String("config", "/opt/joblet/config/joblet-config.yml", "Path to configuration file")
	version    = "1.0.0-dev"
	commit     = "unknown"
	buildTime  = "unknown"
)

func main() {
	flag.Parse()

	// Initialize logger
	log := logger.New().WithMode("persist")
	log.Info("Starting joblet-persist",
		"version", version,
		"commit", commit,
		"buildTime", buildTime)

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	log.Info("Configuration loaded",
		"socket", cfg.IPC.Socket,
		"grpcAddress", cfg.Server.GRPCAddress,
		"storageType", cfg.Storage.Type)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize storage backend
	backend, err := storage.NewBackend(&cfg.Storage, log)
	if err != nil {
		log.Error("Failed to initialize storage backend", "error", err)
		os.Exit(1)
	}
	defer backend.Close()

	log.Info("Storage backend initialized", "type", cfg.Storage.Type)

	// Initialize IPC server
	ipcServer := ipc.NewServer(&cfg.IPC, backend, log)
	if err := ipcServer.Start(ctx); err != nil {
		log.Error("Failed to start IPC server", "error", err)
		os.Exit(1)
	}
	defer ipcServer.Stop()

	log.Info("IPC server started", "socket", cfg.IPC.Socket)

	// Initialize gRPC server
	grpcServer := server.NewGRPCServer(&cfg.Server, backend, log)
	if err := grpcServer.Start(ctx); err != nil {
		log.Error("Failed to start gRPC server", "error", err)
		os.Exit(1)
	}
	defer grpcServer.Stop()

	log.Info("gRPC server started", "address", cfg.Server.GRPCAddress)

	// Wait for signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("joblet-persist is running. Press Ctrl+C to stop.")

	// Block until signal received
	sig := <-sigChan
	log.Info("Received signal, shutting down gracefully...", "signal", sig)

	// Cancel context to trigger shutdown
	cancel()

	log.Info("joblet-persist stopped")
}
