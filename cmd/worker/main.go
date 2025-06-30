package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"worker/internal/modes"
	"worker/pkg/config"
	"worker/pkg/logger"
)

func main() {
	// Load configuration first
	cfg, path, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logging with configuration
	initializeLogging(cfg)

	// Create context logger
	mainLogger := logger.WithField("component", "main")

	mainLogger.Debug("Configuration loaded", "path", path)

	mainLogger.Debug("worker starting with configuration",
		"mode", cfg.Server.Mode,
		"address", cfg.GetServerAddress(),
		"logLevel", cfg.Logging.Level)

	// Run based on configured mode
	var runErr error
	switch cfg.Server.Mode {
	case "server":
		runErr = modes.RunServer(cfg)
	case "init":
		runErr = modes.RunJobInit(cfg)
	default:
		runErr = fmt.Errorf("unknown mode: %s (check WORKER_MODE or config file)", cfg.Server.Mode)
	}

	if runErr != nil {
		mainLogger.Error("worker failed", "error", runErr)
		os.Exit(1)
	}
}

func initializeLogging(cfg *config.Config) {
	// Parse and set log level
	if level, err := logger.ParseLevel(cfg.Logging.Level); err == nil {
		logger.SetLevel(level)
	} else {
		log.Printf("Invalid log level '%s', using INFO", cfg.Logging.Level)
		logger.SetLevel(logger.INFO)
	}

	// Configure output if needed (for file logging)
	if cfg.Logging.Output != "stdout" && cfg.Logging.Output != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(cfg.Logging.Output)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			log.Printf("Failed to setup log file, using stdout: %v", err)
		}
	}
}
