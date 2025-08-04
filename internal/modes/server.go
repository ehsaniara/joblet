//go:build linux

package modes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"joblet/internal/joblet"
	"joblet/internal/joblet/adapters"
	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/monitoring"
	"joblet/internal/joblet/pubsub"
	"joblet/internal/joblet/server"
	"joblet/internal/modes/isolation"
	"joblet/internal/modes/jobexec"
	"joblet/pkg/buffer"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// RunServer starts and runs the Joblet server with the provided configuration.
// Initializes all required components including storage adapters, volume management,
// network setup, monitoring services, and the gRPC server. Handles graceful shutdown
// when receiving termination signals.
//
// The server supports multiple storage backends (currently memory only) and provides
// comprehensive job execution, resource management, and monitoring capabilities.
//
// Parameters:
//   - cfg: Complete configuration object with all server settings
//
// Returns: Error if server startup or operation fails, nil on successful shutdown
func RunServer(cfg *config.Config) error {
	log := logger.WithField("mode", "server")

	log.Info("starting joblet server",
		"address", cfg.GetServerAddress(),
		"maxJobs", cfg.Joblet.MaxConcurrentJobs)

	// Create adapter factory for new storage system
	adapterFactory := adapters.NewAdapterFactory(log)

	// Create platform instance
	platformInstance := platform.NewPlatform()

	// Create new storage adapters with memory backend

	// Volume storage is filesystem-based - volumes are directories in volumes.base_path
	volumeStoreConfig := &adapters.VolumeStoreConfig{
		Store: &adapters.StoreConfig{
			Backend: "memory",                 // Simple in-memory metadata store
			Memory:  &adapters.MemoryConfig{}, // No limits or eviction
		},
	}
	volumeStoreAdapter, err := adapterFactory.CreateVolumeStoreAdapter(volumeStoreConfig)
	if err != nil {
		return fmt.Errorf("failed to create volume store adapter: %w", err)
	}

	// Create network store adapter - always uses file backend for persistence
	networkStoreConfig := &adapters.NetworkStoreConfig{
		NetworkStore: &adapters.StoreConfig{
			Backend: "file",
			Path:    cfg.Network.Storage.Path,
		},
		AllocationStore: &adapters.StoreConfig{
			Backend: "file", // Always file backend for both network and allocations
			Path:    cfg.Network.Storage.Path,
		},
	}
	networkStoreAdapter, err := adapterFactory.CreateNetworkStoreAdapter(networkStoreConfig)
	if err != nil {
		return fmt.Errorf("failed to create network store adapter: %w", err)
	}

	// Create job store adapter from configuration
	// Jobs storage is always memory-based for single-machine operation
	// PubSub is always memory-based for single-machine operation - simple validation
	if cfg.Buffers.DefaultConfig.PubsubBufferSize <= 0 {
		return fmt.Errorf("pubsub buffer_size must be positive")
	}

	// Validate buffer configuration
	if cfg.Buffers.DefaultConfig.Type == "" {
		return fmt.Errorf("buffer type not configured")
	}
	if cfg.Buffers.DefaultConfig.ChunkSize <= 0 {
		return fmt.Errorf("buffer chunk size not configured or invalid")
	}

	jobStoreConfig := &adapters.JobStoreConfig{
		Store: &adapters.StoreConfig{
			Backend: "memory",                 // Always memory for single-machine operation
			Memory:  &adapters.MemoryConfig{}, // No limits or eviction
		},
		PubSub: &pubsub.PubSubConfig{
			BufferSize: cfg.Buffers.DefaultConfig.PubsubBufferSize,
		},
		BufferManager: &adapters.BufferManagerConfig{
			DefaultBufferConfig: &buffer.BufferConfig{
				Type:                 cfg.Buffers.DefaultConfig.Type,
				InitialCapacity:      int(cfg.Buffers.DefaultConfig.InitialCapacity),
				MaxCapacity:          int(cfg.Buffers.DefaultConfig.MaxCapacity),
				MaxSubscribers:       cfg.Buffers.DefaultConfig.MaxSubscribers,
				SubscriberBufferSize: cfg.Buffers.DefaultConfig.SubscriberBufferSize,
				EnableMetrics:        cfg.Buffers.DefaultConfig.EnableMetrics,
			},
		},
	}
	jobStoreAdapter, err := adapterFactory.CreateJobStoreAdapter(jobStoreConfig)
	if err != nil {
		return fmt.Errorf("failed to create job store adapter: %w", err)
	}

	// Create volume manager using the new adapter
	if cfg.Volumes.BasePath == "" {
		return fmt.Errorf("volumes base path not configured")
	}
	volumeManager := volume.NewManager(volumeStoreAdapter, platformInstance, cfg.Volumes.BasePath)

	// Scan and load existing volumes
	if e := volumeManager.ScanVolumes(); e != nil {
		log.Error("failed to scan existing volumes", "error", e)
		// Continue - don't fail server startup due to volume scan errors
	}

	// Create joblet with configuration using new adapters directly
	jobletInstance := joblet.NewJoblet(jobStoreAdapter, cfg, networkStoreAdapter)
	if jobletInstance == nil {
		return fmt.Errorf("failed to create joblet for current platform")
	}

	// Initialize default networks from configuration
	if e := initializeDefaultNetworks(networkStoreAdapter, cfg, log); e != nil {
		log.Error("failed to initialize default networks", "error", e)
		// Don't fail server startup, just log the error
	}

	// Create and start monitoring service with config
	monitoringService := monitoring.NewServiceFromConfig(&cfg.Monitoring)
	if e := monitoringService.Start(); e != nil {
		return fmt.Errorf("failed to start monitoring service: %w", e)
	}
	log.Info("monitoring service started successfully")

	// Start gRPC server with configuration using new adapters
	grpcServer, err := server.StartGRPCServer(jobStoreAdapter, jobletInstance, cfg, networkStoreAdapter, volumeManager, monitoringService)
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
	log.Info("stopping monitoring service...")
	if err := monitoringService.Stop(); err != nil {
		log.Error("error stopping monitoring service", "error", err)
	}

	// Close new adapters
	log.Info("closing storage adapters...")
	if err := jobStoreAdapter.Close(); err != nil {
		log.Error("error closing job store adapter", "error", err)
	}
	if err := volumeStoreAdapter.Close(); err != nil {
		log.Error("error closing volume store adapter", "error", err)
	}
	if err := networkStoreAdapter.Close(); err != nil {
		log.Error("error closing network store adapter", "error", err)
	}

	grpcServer.GracefulStop()
	log.Info("server stopped gracefully")

	return nil
}

// RunJobInit runs the joblet in job initialization mode with phase support.
// Called when the joblet binary is executed as PID 1 inside an isolated namespace.
// Supports two-phase execution: upload processing and job execution phases.
// Handles cgroup assignment, resource limits, and proper isolation setup.
//
// Parameters:
//   - cfg: Configuration object with isolation and execution settings
//
// Returns: Error if initialization or phase execution fails
func RunJobInit(cfg *config.Config) error {
	initLogger := logger.WithField("mode", "init")

	// Create platform instance
	platformInstance := platform.NewPlatform()

	// Determine phase
	phase := platformInstance.Getenv("JOB_PHASE")
	jobID := platformInstance.Getenv("JOB_ID")

	if jobID != "" {
		initLogger = initLogger.WithField("jobId", jobID)
	}

	initLogger.Debug("joblet starting in INIT mode",
		"platform", runtime.GOOS,
		"mode", "init",
		"phase", phase,
		"jobId", jobID)

	// Phase-specific handling
	switch phase {
	case "upload":
		return runUploadPhase(cfg, initLogger, platformInstance)
	case "execute":
		return runExecutePhase(cfg, initLogger, platformInstance)
	default:
		// Legacy support - treat as execute phase
		initLogger.Warn("no phase specified, assuming execute phase")
		return runExecutePhase(cfg, initLogger, platformInstance)
	}
}

// runUploadPhase handles the upload phase within full isolation.
// Processes file uploads within cgroup resource limits to prevent resource exhaustion.
// Assigns process to cgroup immediately, sets up isolation, and processes uploads
// with memory and I/O constraints enforced by the kernel.
//
// Parameters:
//   - cfg: Configuration with filesystem and buffer settings
//   - logger: Structured logger for the upload phase
//   - platform: Platform abstraction for system operations
//
// Returns: Error if upload processing fails within resource constraints
func runUploadPhase(cfg *config.Config, logger *logger.Logger, platform platform.Platform) error {
	logger.Info("starting upload phase in isolation")

	// Wait for network if needed (for consistency)
	if err := waitForNetworkReady(logger, platform); err != nil {
		return fmt.Errorf("failed to wait for network ready: %w", err)
	}

	// Get cgroup path and assign immediately
	cgroupPath := platform.Getenv("JOB_CGROUP_PATH")
	if cgroupPath == "" {
		return fmt.Errorf("JOB_CGROUP_PATH environment variable is required")
	}

	// Assign to cgroup - THIS IS CRITICAL
	if err := assignToCgroup(cgroupPath, logger, platform); err != nil {
		return fmt.Errorf("failed to assign to cgroup: %w", err)
	}

	// Verify cgroup assignment
	if err := verifyCgroupAssignment(cgroupPath, logger, platform); err != nil {
		return fmt.Errorf("cgroup assignment verification failed: %w", err)
	}

	logger.Info("process assigned to cgroup, starting upload processing")

	// Set up isolation
	if err := isolation.Setup(logger); err != nil {
		return fmt.Errorf("job isolation setup failed: %w", err)
	}

	// Process uploads within resource limits
	return processUploadsInCgroup(cfg, logger, platform)
}

// runExecutePhase handles the execution phase (existing logic refactored).
// Executes the actual job command within full isolation and resource constraints.
// Waits for network setup, assigns to cgroup, verifies resource limits,
// and delegates to the job execution engine.
//
// Parameters:
//   - cfg: Configuration with execution and resource settings
//   - logger: Structured logger for the execution phase
//   - platform: Platform abstraction for system operations
//
// Returns: Error if job execution fails or resource setup encounters issues
func runExecutePhase(cfg *config.Config, logger *logger.Logger, platform platform.Platform) error {
	logger.Debug("starting execution phase in isolation")

	// CRITICAL: Wait for network setup FIRST before any other operations
	if err := waitForNetworkReady(logger, platform); err != nil {
		return fmt.Errorf("failed to wait for network ready: %w", err)
	}

	// Validate required environment
	cgroupPath := platform.Getenv("JOB_CGROUP_PATH")
	if cgroupPath == "" {
		return fmt.Errorf("JOB_CGROUP_PATH environment variable is required")
	}

	// Assign to cgroup immediately
	if err := assignToCgroup(cgroupPath, logger, platform); err != nil {
		return fmt.Errorf("failed to assign to cgroup: %w", err)
	}

	// Verify cgroup assignment
	if err := verifyCgroupAssignment(cgroupPath, logger, platform); err != nil {
		return fmt.Errorf("cgroup assignment verification failed: %w", err)
	}

	limits := map[string]string{
		"maxCPU":    platform.Getenv("JOB_MAX_CPU"),
		"maxMemory": platform.Getenv("JOB_MAX_MEMORY"),
		"maxIOBPS":  platform.Getenv("JOB_MAX_IOBPS"),
	}

	logger.Debug("resource limits applied", "limits", limits)

	// Set up isolation
	if err := isolation.Setup(logger); err != nil {
		return fmt.Errorf("job isolation setup failed: %w", err)
	}

	// Execute the job using the new consolidated approach
	if err := jobexec.Execute(logger); err != nil {
		return fmt.Errorf("job execution failed: %w", err)
	}

	return nil
}

// FileUpload represents a file or directory to upload
type FileUpload struct {
	Path        string `json:"path"`
	Content     []byte `json:"content"`
	Mode        uint32 `json:"mode"`
	IsDirectory bool   `json:"isDirectory"`
	Size        int64  `json:"size"`
}

// processUploadsInCgroup processes uploads within cgroup limits.
// Decodes base64-encoded upload data from environment variables,
// creates workspace directory, and processes each file/directory
// within memory and I/O resource constraints enforced by cgroups.
//
// Parameters:
//   - cfg: Configuration with filesystem settings
//   - logger: Structured logger for upload processing
//   - platform: Platform abstraction for environment access
//
// Returns: Error if upload decoding or file processing fails
func processUploadsInCgroup(cfg *config.Config, logger *logger.Logger, platform platform.Platform) error {
	// Get upload data from environment
	uploadsB64 := platform.Getenv("JOB_UPLOADS_DATA")
	if uploadsB64 == "" {
		return fmt.Errorf("no upload data provided")
	}

	// Decode base64
	uploadsJSON, err := base64.StdEncoding.DecodeString(uploadsB64)
	if err != nil {
		return fmt.Errorf("failed to decode upload data: %w", err)
	}

	// Parse uploads
	var uploads []FileUpload
	if e := json.Unmarshal(uploadsJSON, &uploads); e != nil {
		return fmt.Errorf("failed to parse upload data: %w", e)
	}

	logger.Info("processing uploads within cgroup limits", "count", len(uploads))

	// Create workspace directory from configuration
	workspaceDir := cfg.Filesystem.WorkspaceDir
	if workspaceDir == "" {
		return fmt.Errorf("workspace directory not configured")
	}
	if e := os.MkdirAll(workspaceDir, 0755); e != nil {
		return fmt.Errorf("failed to create workspace: %w", e)
	}

	// Process each file - ALL I/O IS NOW SUBJECT TO CGROUP LIMITS
	for _, upload := range uploads {
		if e := processUploadFile(&upload, workspaceDir, cfg, logger); e != nil {
			// Log the error but include context about resource limits
			logger.Error("failed to process upload file",
				"path", upload.Path,
				"size", len(upload.Content),
				"error", e,
				"hint", "possible resource limit exceeded")
			return fmt.Errorf("upload processing failed for %s: %w", upload.Path, e)
		}
	}

	logger.Info("all uploads processed successfully within resource limits")
	return nil
}

// processUploadFile writes a single file within cgroup limits.
// Handles both files and directories, creates necessary parent directories,
// and writes file content in chunks to handle large files efficiently
// within memory constraints.
//
// Parameters:
//   - upload: FileUpload interface{} containing file data and metadata
//   - workspaceDir: Base workspace directory for file creation
//   - cfg: Configuration with chunk size and buffer settings
//   - logger: Structured logger for file processing
//
// Returns: Error if file creation or writing fails
func processUploadFile(upload interface{}, workspaceDir string, cfg *config.Config, logger *logger.Logger) error {
	// Type assertion to access fields
	u := upload.(*FileUpload)

	fullPath := filepath.Join(workspaceDir, u.Path)

	if u.IsDirectory {
		// Create directory
		mode := os.FileMode(u.Mode)
		if mode == 0 {
			mode = 0755
		}
		if err := os.MkdirAll(fullPath, mode); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		logger.Debug("created directory", "path", u.Path)
	} else {
		// Create parent directory
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}

		// Write file - THIS WRITE IS SUBJECT TO MEMORY/IO LIMITS
		mode := os.FileMode(u.Mode)
		if mode == 0 {
			mode = 0644
		}

		// Write in chunks to handle large files better
		if err := writeFileInChunks(fullPath, u.Content, mode, logger, cfg); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		logger.Debug("wrote file within cgroup limits", "path", u.Path, "size", len(u.Content), "mode", mode)
	}

	return nil
}

// writeFileInChunks writes file data in chunks to better handle memory pressure.
// Uses configurable chunk size to write large files without exceeding memory limits.
// Performs periodic syncing to ensure data persistence and handles write failures
// that may indicate resource limit violations.
//
// Parameters:
//   - path: Full path where the file should be written
//   - content: Complete file content as byte slice
//   - mode: File permissions to set on the created file
//   - logger: Structured logger for write operations
//   - cfg: Configuration containing chunk size settings
//
// Returns: Error if file creation, writing, or syncing fails
func writeFileInChunks(path string, content []byte, mode os.FileMode, logger *logger.Logger, cfg *config.Config) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get chunk size from configuration
	chunkSize := cfg.Buffers.DefaultConfig.ChunkSize
	if chunkSize <= 0 {
		return fmt.Errorf("invalid chunk size in configuration: %d", chunkSize)
	}
	for offset := 0; offset < len(content); offset += chunkSize {
		end := offset + chunkSize
		if end > len(content) {
			end = len(content)
		}

		chunk := content[offset:end]
		if _, err := file.Write(chunk); err != nil {
			// This error likely means we hit a resource limit
			return fmt.Errorf("write failed at offset %d: %w", offset, err)
		}

		// Sync periodically to ensure data is written
		if offset%(chunkSize*16) == 0 && offset > 0 {
			if e := file.Sync(); e != nil {
				logger.Warn("failed to sync file during write", "error", e, "offset", offset)
			}
		}
	}

	if e := file.Sync(); e != nil {
		logger.Warn("failed to final sync file", "error", e, "path", path)
		return nil // Data was written, so we don't fail
	}
	return nil
}

// waitForNetworkReady waits for the parent process to signal that network setup is complete.
// Uses a file descriptor passed from the parent process to synchronize network configuration.
// Blocks until the parent writes to the pipe, indicating network namespaces and
// interfaces are properly configured.
//
// Parameters:
//   - logger: Structured logger for network synchronization
//   - platform: Platform abstraction for environment variable access
//
// Returns: Error if network synchronization fails or times out
func waitForNetworkReady(logger *logger.Logger, platform platform.Platform) error {
	networkReadyFD := platform.Getenv("NETWORK_READY_FD")
	if networkReadyFD == "" {
		logger.Debug("NETWORK_READY_FD not set, skipping network wait")
		return nil
	}

	logger.Debug("waiting for network setup", "fd", networkReadyFD)

	fd, err := strconv.Atoi(networkReadyFD)
	if err != nil {
		return fmt.Errorf("invalid network ready FD: %w", err)
	}

	// Open the file descriptor passed from parent
	file := os.NewFile(uintptr(fd), "network-ready")
	if file == nil {
		return fmt.Errorf("failed to open network ready FD %d", fd)
	}
	defer file.Close()

	// Note: Pipes don't support deadlines, so we skip setting one
	// The parent process signals immediately after network setup, so this is safe

	// Read one byte - this blocks until network is ready
	buf := make([]byte, 1)
	logger.Debug("blocking on network ready signal...")

	n, err := file.Read(buf)
	if err != nil {
		return fmt.Errorf("failed to read network ready signal: %w", err)
	}

	if n != 1 {
		return fmt.Errorf("unexpected read size from network ready FD: %d", n)
	}

	logger.Debug("network setup signal received, proceeding with initialization")
	return nil
}

// assignToCgroup assigns the current process to the specified cgroup.
// Converts namespace cgroup path to host cgroup path and writes the process PID
// to the cgroup.procs file. Uses the "proc" subgroup to satisfy cgroup v2
// "no internal processes" constraint.
//
// Parameters:
//   - cgroupPath: Cgroup path as seen from within the namespace
//   - logger: Structured logger for cgroup operations
//   - platform: Platform abstraction for environment access
//
// Returns: Error if cgroup assignment fails or cgroup doesn't exist
func assignToCgroup(cgroupPath string, logger *logger.Logger, platform platform.Platform) error {
	if cgroupPath == "" {
		return fmt.Errorf("cgroup path cannot be empty")
	}

	// The cgroupPath from environment is the namespace view (/sys/fs/cgroup)
	// But we need to write to the HOST view of the cgroup
	// Convert from namespace path to host path using JOB_CGROUP_HOST_PATH
	hostCgroupPath := platform.Getenv("JOB_CGROUP_HOST_PATH")
	if hostCgroupPath == "" {
		// Fallback: try to construct it
		jobID := platform.Getenv("JOB_ID")
		if jobID == "" {
			return fmt.Errorf("cannot determine cgroup path: JOB_CGROUP_HOST_PATH and JOB_ID not set")
		}
		hostCgroupPath = fmt.Sprintf("/sys/fs/cgroup/joblet.slice/joblet.service/job-%s", jobID)
	}

	// Use the process subgroup to satisfy "no internal processes" rule
	hostCgroupPath = filepath.Join(hostCgroupPath, "proc")

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

// verifyCgroupAssignment verifies that the current process is in a cgroup namespace.
// Reads /proc/self/cgroup to confirm the process is not in the root cgroup
// and optionally verifies the cgroup contains the expected job ID.
// Provides early detection of cgroup assignment failures.
//
// Parameters:
//   - expectedCgroupPath: Expected cgroup path for verification
//   - logger: Structured logger for verification process
//   - platform: Platform abstraction for environment access
//
// Returns: Error if process is still in root cgroup or verification fails
func verifyCgroupAssignment(expectedCgroupPath string, logger *logger.Logger, platform platform.Platform) error {
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
	jobID := platform.Getenv("JOB_ID")
	if jobID != "" && !strings.Contains(cgroupContent, jobID) {
		logger.Warn("cgroup content doesn't contain job ID, but assignment may still be correct",
			"jobID", jobID, "cgroupContent", cgroupContent)
	}

	logger.Debug("cgroup assignment verified successfully", "pid", pid)
	return nil
}

// initializeDefaultNetworks creates default networks from configuration.
// Reads network definitions from configuration and creates them in the network store
// if they don't already exist. Skips initialization if network management is disabled.
// Ensures required networks are available for job execution.
//
// Parameters:
//   - networkStore: Network storage adapter for network management
//   - cfg: Configuration containing network definitions
//   - log: Structured logger for network initialization
//
// Returns: Error if network creation fails, nil if all networks created successfully
func initializeDefaultNetworks(networkStore adapters.NetworkStoreAdapter, cfg *config.Config, log *logger.Logger) error {
	log.Info("initializing default networks from configuration")

	if !cfg.Network.Enabled {
		log.Debug("network management disabled, skipping network initialization")
		return nil
	}

	// Create each network defined in configuration
	for name, networkDef := range cfg.Network.Networks {
		log.Debug("creating network from configuration", "name", name, "cidr", networkDef.CIDR)

		networkConfig := &adapters.NetworkConfig{
			Name:       name,
			Type:       "bridge", // Default type for configured networks
			CIDR:       networkDef.CIDR,
			BridgeName: networkDef.BridgeName,
		}

		// Check if network already exists
		existing, exists := networkStore.GetNetwork(name)
		if exists {
			log.Debug("network already exists", "name", name, "existingCIDR", existing.CIDR)
			continue
		}

		// Create the network
		if err := networkStore.CreateNetwork(networkConfig); err != nil {
			log.Error("failed to create network", "name", name, "error", err)
			return fmt.Errorf("failed to create network %s: %w", name, err)
		}

		log.Info("created default network", "name", name, "cidr", networkDef.CIDR, "bridge", networkDef.BridgeName)
	}

	log.Info("default network initialization completed", "count", len(cfg.Network.Networks))
	return nil
}
