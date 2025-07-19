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

	limits := map[string]string{
		"maxCPU":    os.Getenv("JOB_MAX_CPU"),
		"maxMemory": os.Getenv("JOB_MAX_MEMORY"),
		"maxIOBPS":  os.Getenv("JOB_MAX_IOBPS"),
	}

	logger.Debug("resource limits applied", "limits", limits)

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

	// The cgroupPath from environment is the namespace view (/sys/fs/cgroup)
	// But we need to write to the HOST view of the cgroup
	// Convert from namespace path to host path using JOB_CGROUP_HOST_PATH
	hostCgroupPath := os.Getenv("JOB_CGROUP_HOST_PATH")
	if hostCgroupPath == "" {
		// Fallback: try to construct it
		jobID := os.Getenv("JOB_ID")
		if jobID == "" {
			return fmt.Errorf("cannot determine cgroup path: JOB_CGROUP_HOST_PATH and JOB_ID not set")
		}
		hostCgroupPath = fmt.Sprintf("/sys/fs/cgroup/joblet.slice/joblet.service/job-%s", jobID)
	}

	pid := os.Getpid()
	procsFile := filepath.Join(hostCgroupPath, "cgroup.procs")
	pidBytes := []byte(fmt.Sprintf("%d", pid))

	// Verify the host cgroup directory exists
	if _, err := os.Stat(hostCgroupPath); err != nil {
		return fmt.Errorf("host cgroup directory does not exist: %s: %w", hostCgroupPath, err)
	}

	// Verify the cgroup.procs file exists
	if _, err := os.Stat(procsFile); err != nil {
		return fmt.Errorf("cgroup.procs file does not exist: %s: %w", procsFile, err)
	}

	// Write our PID to the cgroup
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

	// In cgroup namespace, we expect something like "0::/job-1" or similar
	// The key is that it should NOT be "0::/" (root cgroup)
	if cgroupContent == "0::/" {
		return fmt.Errorf("process still in root cgroup after assignment attempt")
	}

	// Extract job ID from expected path and verify it's in our cgroup view
	jobID := os.Getenv("JOB_ID")
	if jobID != "" && !strings.Contains(cgroupContent, jobID) {
		logger.Warn("cgroup content doesn't contain job ID, but assignment may still be correct",
			"jobID", jobID, "cgroupContent", cgroupContent)
	}

	logger.Debug("cgroup assignment verified successfully", "pid", pid)
	return nil
}
