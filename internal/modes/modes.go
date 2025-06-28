package modes

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"worker/internal/modes/isolation"
	"worker/internal/modes/jobexec"
	"worker/internal/modes/validation"
	"worker/internal/worker"
	"worker/internal/worker/server"
	"worker/internal/worker/state"
	"worker/pkg/logger"
)

// ExecutionMode determines which stage of the worker should run
type ExecutionMode string

const (
	ServerMode ExecutionMode = "server"
	InitMode   ExecutionMode = "init"
)

// RunServer runs the worker in server mode (gRPC server)
func RunServer() {
	appLogger := logger.New()

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		if level, err := logger.ParseLevel(logLevel); err == nil {
			appLogger.SetLevel(level)
			appLogger.Debug("log level set from environment", "level", level)
		}
	}

	appLogger.Debug("worker starting in SERVER mode",
		"version", "1.0.0",
		"platform", runtime.GOOS,
		"mode", "server")

	// Platform-specific initialization
	if err := validation.ValidatePlatformRequirements(appLogger); err != nil {
		appLogger.Fatal("platform requirements not met", "error", err)
	}

	// Initialize store and worker
	s := state.New()
	w := worker.NewWorker(s)
	appLogger.Debug("worker and store initialized", "platform", runtime.GOOS)

	// Start gRPC server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	setupGracefulShutdown(ctx, cancel, appLogger)

	// Start server
	grpcServer, err := server.StartGRPCServer(s, w)
	if err != nil {
		appLogger.Fatal("failed to start server", "error", err)
	}

	appLogger.Info("gRPC server started successfully")

	// Wait for shutdown signal
	<-ctx.Done()

	// Graceful shutdown
	gracefulShutdown(grpcServer, appLogger)
}

// RunJobInit runs the worker in job initialization mode
func RunJobInit() {
	initLogger := logger.New()

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		if level, err := logger.ParseLevel(logLevel); err == nil {
			initLogger.SetLevel(level)
		}
	}

	jobID := os.Getenv("JOB_ID")
	if jobID != "" {
		initLogger = initLogger.WithField("jobId", jobID)
	}

	initLogger.Debug("worker starting in INIT mode",
		"platform", runtime.GOOS,
		"mode", "init",
		"jobId", jobID)

	// Load job configuration from environment
	config, err := jobexec.LoadConfigFromEnv(initLogger)
	if err != nil {
		initLogger.Error("failed to load job config", "error", err)
		os.Exit(1)
	}

	// Set up isolation based on platform
	if err := isolation.Setup(initLogger); err != nil {
		initLogger.Error("job isolation setup failed", "error", err)
		os.Exit(1)
	}

	// Execute the job
	if err := jobexec.Execute(config, initLogger); err != nil {
		initLogger.Error("job execution failed", "error", err)
		os.Exit(1)
	}

	// Platform-specific completion handling
	jobexec.HandleCompletion(initLogger)
}

// setupGracefulShutdown sets up signal handling for graceful shutdown
func setupGracefulShutdown(ctx context.Context, cancel context.CancelFunc, logger *logger.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	go func() {
		sig := <-sigChan
		logger.Debug("received shutdown signal, initiating graceful shutdown", "signal", sig)
		cancel()
	}()
}

// gracefulShutdown handles graceful shutdown of the gRPC server
func gracefulShutdown(grpcServer interface{}, logger *logger.Logger) {
	logger.Debug("starting graceful shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	serverStopped := make(chan struct{})
	go func() {
		// Type assertion to access GracefulStop method
		if server, ok := grpcServer.(interface{ GracefulStop() }); ok {
			server.GracefulStop()
		}
		close(serverStopped)
	}()

	select {
	case <-serverStopped:
		logger.Debug("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		logger.Warn("gRPC server shutdown timeout, forcing stop")
		if server, ok := grpcServer.(interface{ Stop() }); ok {
			server.Stop()
		}
	}

	logger.Debug("server gracefully stopped")
}
