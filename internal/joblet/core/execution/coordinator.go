package execution

import (
	"context"
	"fmt"
	"os"

	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// ExecutionCoordinator coordinates different execution services.
// Replaces monolithic ExecutionEngine with focused coordinator that orchestrates
// environment setup, networking, process management, and isolation for job execution.
type ExecutionCoordinator struct {
	environmentManager EnvironmentManager
	networkManager     NetworkManager
	processManager     ProcessManager
	isolationManager   IsolationManager
	logger             *logger.Logger
}

// NewExecutionCoordinator creates a new execution coordinator.
// Initializes coordinator with environment, network, process, and isolation managers
// for comprehensive job execution orchestration.
func NewExecutionCoordinator(
	envManager EnvironmentManager,
	netManager NetworkManager,
	procManager ProcessManager,
	isolManager IsolationManager,
	logger *logger.Logger,
) *ExecutionCoordinator {
	return &ExecutionCoordinator{
		environmentManager: envManager,
		networkManager:     netManager,
		processManager:     procManager,
		isolationManager:   isolManager,
		logger:             logger.WithField("component", "execution-coordinator"),
	}
}

// StartJob implements JobExecutor interface.
// Main execution entry point: creates isolation, prepares workspace, sets up networking,
// builds environment, and launches process with unified init system for logging.
func (ec *ExecutionCoordinator) StartJob(ctx context.Context, opts *StartProcessOptions) (platform.Command, error) {
	log := ec.logger.WithField("jobID", opts.Job.Uuid)
	log.Debug("coordinating job start", "hasUploads", len(opts.Uploads) > 0, "jobType", opts.Job.GetType())

	// 1. Create isolation environment based on job type
	var err error
	if opts.Job.IsRuntimeBuild() {
		log.Info("creating builder environment for runtime build job")
		_, err = ec.isolationManager.CreateBuilderEnvironment(opts.Job.Uuid)
	} else {
		_, err = ec.isolationManager.CreateIsolatedEnvironment(opts.Job.Uuid)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create isolated environment: %w", err)
	}

	// 2. Setup workspace and process uploads
	workspaceDir, err := ec.environmentManager.PrepareWorkspace(opts.Job.Uuid, opts.Uploads)
	if err != nil {
		if destroyErr := ec.isolationManager.DestroyIsolatedEnvironment(opts.Job.Uuid); destroyErr != nil {
			log.Warn("failed to destroy isolated environment during cleanup", "error", destroyErr)
		}
		return nil, fmt.Errorf("failed to prepare workspace: %w", err)
	}

	// 3. Setup networking
	var networkAlloc *NetworkAllocation
	log.Debug("checking job network configuration", "network", opts.Job.Network, "isEmpty", opts.Job.Network == "")
	if opts.Job.Network != "" {
		log.Info("setting up networking for job", "network", opts.Job.Network)
		networkAlloc, err = ec.networkManager.SetupNetworking(ctx, opts.Job.Uuid, opts.Job.Network)
		if err != nil {
			ec.cleanup(opts.Job.Uuid, workspaceDir)
			return nil, fmt.Errorf("failed to setup networking: %w", err)
		}
		log.Info("networking setup completed", "allocation", networkAlloc != nil)
	} else {
		log.Info("no networking configured for job")
	}

	// 4. Build environment
	environment := ec.environmentManager.BuildEnvironment(opts.Job, "execute")

	// 5. Always use joblet binary as init for unified pub/sub logging
	// The joblet binary runs in init mode, sets up runtime environment, then exec's to the actual command
	// This ensures all jobs (runtime and default) use the same logging mechanism
	initPath := "/opt/joblet/joblet"
	log.Debug("using joblet binary as init for namespace isolation and unified logging", "initPath", initPath)

	// 6. Create network ready file for coordination if networking is enabled
	var networkReadyFile string
	if networkAlloc != nil && networkAlloc.Network != "none" {
		// Create a signal file for network coordination
		networkReadyFile = fmt.Sprintf("/tmp/joblet-network-ready-%s", opts.Job.Uuid)

		// Add NETWORK_READY_FILE to environment
		environment = append(environment, fmt.Sprintf("NETWORK_READY_FILE=%s", networkReadyFile))
		log.Debug("created network ready file path", "file", networkReadyFile)
	}

	// 7. Launch process
	launchConfig := &LaunchConfig{
		InitPath:    initPath, // Use resolved absolute path
		JobID:       opts.Job.Uuid,
		JobType:     opts.Job.Type, // Pass job type for isolation configuration
		Command:     opts.Job.Command,
		Args:        opts.Job.Args,
		Environment: environment,
		// Additional config will be set based on isolation context
	}

	result, err := ec.processManager.LaunchProcess(ctx, launchConfig)
	if err != nil {
		ec.cleanup(opts.Job.Uuid, workspaceDir)
		if networkAlloc != nil {
			if cleanupErr := ec.networkManager.CleanupNetworking(ctx, opts.Job.Uuid); cleanupErr != nil {
				log.Warn("failed to cleanup networking during process launch failure", "error", cleanupErr)
			}
		}
		return nil, fmt.Errorf("failed to launch process: %w", err)
	}

	// Phase 2: Configure network namespace now that we have the PID
	if networkAlloc != nil && networkAlloc.Network != "none" {
		log.Info("configuring network namespace with process PID", "pid", result.PID)
		if err := ec.networkManager.ConfigureNetworkNamespace(ctx, opts.Job.Uuid, result.PID); err != nil {
			log.Error("failed to configure network namespace", "error", err)
			// Continue execution - network issues shouldn't kill the job entirely
		} else {
			log.Info("network namespace configured successfully")
		}

		// Signal network ready to job process by creating the signal file
		if networkReadyFile != "" {
			log.Debug("signaling network ready to job process", "file", networkReadyFile)
			if err := os.WriteFile(networkReadyFile, []byte("ready"), 0644); err != nil {
				log.Error("failed to create network ready signal file", "error", err)
			} else {
				log.Debug("network ready signal file created successfully")
			}
		}
	}

	log.Info("job started successfully", "pid", result.PID)
	return result.Command, nil
}

// StopJob implements JobExecutor interface
func (ec *ExecutionCoordinator) StopJob(ctx context.Context, jobID string) error {
	log := ec.logger.WithField("jobID", jobID)
	log.Debug("coordinating job stop")

	var errs []error

	// Cleanup in reverse order
	if err := ec.networkManager.CleanupNetworking(ctx, jobID); err != nil {
		errs = append(errs, fmt.Errorf("network cleanup failed: %w", err))
	}

	if err := ec.environmentManager.CleanupWorkspace(jobID); err != nil {
		errs = append(errs, fmt.Errorf("workspace cleanup failed: %w", err))
	}

	if err := ec.isolationManager.DestroyIsolatedEnvironment(jobID); err != nil {
		errs = append(errs, fmt.Errorf("isolation cleanup failed: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}

	log.Info("job stopped successfully")
	return nil
}

// cleanup performs cleanup operations
func (ec *ExecutionCoordinator) cleanup(jobID, workspaceDir string) {
	if err := ec.environmentManager.CleanupWorkspace(jobID); err != nil {
		ec.logger.Warn("workspace cleanup failed during error recovery", "jobID", jobID, "error", err)
	}

	if err := ec.isolationManager.DestroyIsolatedEnvironment(jobID); err != nil {
		ec.logger.Warn("isolation cleanup failed during error recovery", "jobID", jobID, "error", err)
	}
}
