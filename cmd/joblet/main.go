//go:build linux

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ehsaniara/joblet/internal/modes"
	"github.com/ehsaniara/joblet/pkg/config"
	"github.com/ehsaniara/joblet/pkg/logger"
)

func main() {
	// Determine mode before touching any config: the init process runs inside
	// the job filesystem where no server config exists (and must not - it
	// embeds private keys). Init gets everything it needs from JOB_* env vars
	// and built-in defaults, without any config file I/O.
	var cfg *config.Config
	var path string
	if os.Getenv("JOBLET_MODE") == "init" {
		cfg = config.InitConfig()
		cfg.Server.Mode = "init"
		path = config.BuiltInDefaultsPath
	} else {
		var err error
		cfg, path, err = config.LoadConfig()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}

		// Fail fast: a server without a config file is a broken installation.
		// Running on built-in defaults would silently use wrong addresses, no
		// certificates, and wrong paths.
		if cfg.Server.Mode == "server" && strings.HasPrefix(path, config.BuiltInDefaultsPath) {
			log.Fatalf("No configuration file found - refusing to start server with built-in defaults.\n" +
				"This indicates a broken installation. Expected config at one of:\n" +
				"  $JOBLET_CONFIG_PATH, /opt/joblet/config/joblet-config.yml, ./config/joblet-config.yml, ./joblet-config.yml, /etc/joblet/joblet-config.yml\n" +
				"Reinstall the joblet package or run: JOBLET_SERVER_ADDRESS='<ip>' /usr/local/bin/certs_gen_embedded.sh")
		}
	}

	// Initialize logging with configuration
	initializeLogging(cfg)

	// Set the global logger mode based on the configuration
	logger.SetGlobalMode(cfg.Server.Mode)

	// Create context logger with mode
	mainLogger := logger.WithField("component", "main")

	// Only log config loading in trace mode
	if cfg.Logging.Level == "TRACE" {
		mainLogger.Debug("Configuration loaded", "path", path)
	}

	// Only log startup details in trace mode
	if cfg.Logging.Level == "TRACE" {
		mainLogger.Debug("joblet starting with configuration",
			"mode", cfg.Server.Mode,
			"address", cfg.GetServerAddress(),
			"logLevel", cfg.Logging.Level)
	}

	// Run based on configured mode
	var runErr error
	switch cfg.Server.Mode {
	case "server":
		runErr = modes.RunServer(cfg)
	case "init":
		runErr = modes.RunJobInit(cfg)
	default:
		runErr = fmt.Errorf("unknown mode: %s (check JOBLET_MODE or config file)", cfg.Server.Mode)
	}

	if runErr != nil {
		mainLogger.Error("joblet failed", "error", runErr)
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
