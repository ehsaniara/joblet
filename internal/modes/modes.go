package modes

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
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

// RunServer runs the worker in server mode (gRPC server)
func RunServer() error {
	ctx := context.Background()

	appLogger := setupLogger()
	appLogger.Debug("worker starting in SERVER mode",
		"version", "1.0.0",
		"platform", runtime.GOOS,
		"mode", "server")

	// Platform validation
	if err := validation.ValidatePlatformRequirements(appLogger); err != nil {
		return fmt.Errorf("platform requirements not met: %w", err)
	}

	// Initialize store and worker
	store := state.New()
	worker := worker.NewWorker(store)

	// Start gRPC server
	grpcServer, err := server.StartGRPCServer(store, worker)
	if err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	appLogger.Info("gRPC server started successfully")

	// Set up graceful shutdown
	return handleServerShutdown(ctx, grpcServer, appLogger)
}

// RunJobInit runs the worker in job initialization mode
func RunJobInit() error {
	initLogger := setupLogger()

	jobID := os.Getenv("JOB_ID")
	if jobID != "" {
		initLogger = initLogger.WithField("jobId", jobID)
	}

	initLogger.Debug("worker starting in INIT mode",
		"platform", runtime.GOOS,
		"mode", "init",
		"jobId", jobID)

	// Validate required environment
	cgroupPath := os.Getenv("JOB_CGROUP_PATH")
	if cgroupPath == "" {
		return fmt.Errorf("JOB_CGROUP_PATH environment variable is required")
	}

	// Assign to cgroup immediately
	if err := assignToCgroup(cgroupPath, initLogger); err != nil {
		return fmt.Errorf("failed to assign to cgroup: %w", err)
	}

	// Verify cgroup assignment
	if err := verifyCgroupAssignment(cgroupPath, initLogger); err != nil {
		return fmt.Errorf("cgroup assignment verification failed: %w", err)
	}

	// Log resource limits for transparency
	logResourceLimits(initLogger)

	// Load job configuration
	config, err := jobexec.LoadConfigFromEnv(initLogger)
	if err != nil {
		return fmt.Errorf("failed to load job config: %w", err)
	}

	// Set up isolation
	if err := isolation.Setup(initLogger); err != nil {
		return fmt.Errorf("job isolation setup failed: %w", err)
	}

	// Execute the job
	if err := jobexec.Execute(config, initLogger); err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	// Handle completion
	jobexec.HandleCompletion(initLogger)
	return nil
}

// setupLogger configures the logger based on environment variables
func setupLogger() *logger.Logger {
	appLogger := logger.New()

	if logLevel := os.Getenv("LOG_LEVEL"); logLevel != "" {
		if level, err := logger.ParseLevel(logLevel); err == nil {
			appLogger.SetLevel(level)
		}
	}

	return appLogger
}

// handleServerShutdown sets up graceful shutdown for the server
func handleServerShutdown(ctx context.Context, grpcServer interface{}, logger *logger.Logger) error {
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create a context for shutdown timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("received shutdown signal", "signal", sig)

	// Graceful shutdown
	done := make(chan struct{})
	go func() {
		defer close(done)

		// Type assertion to access GracefulStop method
		if s, ok := grpcServer.(interface{ GracefulStop() }); ok {
			s.GracefulStop()
			logger.Info("gRPC server stopped gracefully")
		} else {
			logger.Warn("unable to gracefully stop server")
		}
	}()

	// Wait for graceful shutdown or timeout
	select {
	case <-done:
		logger.Info("server shutdown completed")
		return nil
	case <-shutdownCtx.Done():
		logger.Warn("shutdown timeout exceeded, forcing stop")
		if s, ok := grpcServer.(interface{ Stop() }); ok {
			s.Stop()
		}
		return shutdownCtx.Err()
	}
}

// assignToCgroup assigns the current process to the specified cgroup
func assignToCgroup(cgroupPath string, logger *logger.Logger) error {
	if cgroupPath == "" {
		return fmt.Errorf("cgroup path cannot be empty")
	}

	pid := os.Getpid()
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	pidBytes := []byte(fmt.Sprintf("%d", pid))

	logger.Debug("assigning process to cgroup",
		"pid", pid,
		"cgroupPath", cgroupPath,
		"procsFile", procsFile)

	if err := os.WriteFile(procsFile, pidBytes, 0644); err != nil {
		return fmt.Errorf("failed to write PID %d to %s: %w", pid, procsFile, err)
	}

	logger.Debug("process assigned to cgroup successfully")
	return nil
}

// verifyCgroupAssignment verifies that the current process is in a cgroup namespace
func verifyCgroupAssignment(expectedCgroupPath string, logger *logger.Logger) error {
	const cgroupFile = "/proc/self/cgroup"

	// Read /proc/self/cgroup
	cgroupData, err := os.ReadFile(cgroupFile)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", cgroupFile, err)
	}

	cgroupContent := strings.TrimSpace(string(cgroupData))
	pid := os.Getpid()

	logger.Debug("cgroup assignment verification",
		"pid", pid,
		"cgroupView", cgroupContent,
		"expectedHostPath", expectedCgroupPath)

	// In cgroup namespace, expect "0::/" format
	if !strings.HasPrefix(cgroupContent, "0::") {
		return fmt.Errorf("process not in cgroup namespace (expected format '0::/...', got: %q)", cgroupContent)
	}

	logger.Debug("cgroup assignment verified successfully", "pid", pid)
	return nil
}

// logResourceLimits logs the applied resource limits for transparency
func logResourceLimits(logger *logger.Logger) {
	limits := map[string]string{
		"maxCPU":    os.Getenv("JOB_MAX_CPU"),
		"maxMemory": os.Getenv("JOB_MAX_MEMORY"),
		"maxIOBPS":  os.Getenv("JOB_MAX_IOBPS"),
	}

	logger.Info("resource limits applied", "limits", limits)
}
