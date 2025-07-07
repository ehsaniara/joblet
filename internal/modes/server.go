package modes

import (
	"context"
	"fmt"
	"joblet/internal/modes/isolation"
	"joblet/internal/modes/jobexec"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"joblet/internal/joblet"
	"joblet/internal/joblet/server"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
	"joblet/pkg/logger"
)

func RunServer(cfg *config.Config) error {
	log := logger.WithField("mode", "server")

	log.Info("starting joblet server",
		"address", cfg.GetServerAddress(),
		"maxJobs", cfg.Joblet.MaxConcurrentJobs)

	// Create state store
	store := state.New()

	// Create joblet with configuration
	jobletInstance := joblet.NewJoblet(store, cfg)
	if jobletInstance == nil {
		return fmt.Errorf("failed to create joblet for current platform")
	}

	// Start gRPC server with configuration
	grpcServer, err := server.StartGRPCServer(store, jobletInstance, cfg)
	if err != nil {
		return fmt.Errorf("failed to start gRPC server: %w", err)
	}

	// Setup graceful shutdown
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info("server started successfully", "address", cfg.GetServerAddress())

	// Wait for shutdown signal
	<-sigChan
	log.Info("received shutdown signal, stopping server...")

	// Graceful shutdown
	grpcServer.GracefulStop()
	log.Info("server stopped gracefully")

	return nil
}

// RunJobInit runs the joblet in job initialization mode
func RunJobInit(cfg *config.Config) error {
	initLogger := logger.WithField("mode", "init")

	jobID := os.Getenv("JOB_ID")
	if jobID != "" {
		initLogger = initLogger.WithField("jobId", jobID)
	}

	initLogger.Debug("joblet starting in INIT mode",
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
	jobConfig, err := jobexec.LoadConfigFromEnv(initLogger)
	if err != nil {
		return fmt.Errorf("failed to load job config: %w", err)
	}

	// Set up isolation
	if err := isolation.Setup(initLogger); err != nil {
		return fmt.Errorf("job isolation setup failed: %w", err)
	}

	// Execute the job
	if err := jobexec.Execute(jobConfig, initLogger); err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	// Handle completion
	jobexec.HandleCompletion(initLogger)
	return nil
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

	logger.Debug("resource limits applied", "limits", limits)
}
