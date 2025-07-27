package modes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"joblet/internal/joblet"
	"joblet/internal/joblet/server"
	"joblet/internal/joblet/state"
	"joblet/internal/modes/isolation"
	"joblet/internal/modes/jobexec"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func RunServer(cfg *config.Config) error {
	log := logger.WithField("mode", "server")

	log.Info("starting joblet server",
		"address", cfg.GetServerAddress(),
		"maxJobs", cfg.Joblet.MaxConcurrentJobs)

	// Create state store
	store := state.New()

	// Create network store
	networkConfig := &cfg.Network
	if networkConfig.StateDir == "" {
		networkConfig.StateDir = "/var/lib/joblet"
	}
	networkStore := state.NewNetworkStore(networkConfig)
	if err := networkStore.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize network store: %w", err)
	}

	// Create joblet with configuration
	jobletInstance := joblet.NewJoblet(store, cfg, networkStore)
	if jobletInstance == nil {
		return fmt.Errorf("failed to create joblet for current platform")
	}

	// Start gRPC server with configuration
	grpcServer, err := server.StartGRPCServer(store, jobletInstance, cfg, networkStore)
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

// RunJobInit runs the joblet in job initialization mode with phase support
func RunJobInit(cfg *config.Config) error {
	initLogger := logger.WithField("mode", "init")

	// Determine phase
	phase := os.Getenv("JOB_PHASE")
	jobID := os.Getenv("JOB_ID")

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
		return runUploadPhase(cfg, initLogger)
	case "execute":
		return runExecutePhase(cfg, initLogger)
	default:
		// Legacy support - treat as execute phase
		initLogger.Warn("no phase specified, assuming execute phase")
		return runExecutePhase(cfg, initLogger)
	}
}

// runUploadPhase handles the upload phase within full isolation
func runUploadPhase(cfg *config.Config, logger *logger.Logger) error {
	logger.Info("starting upload phase in isolation")

	// Wait for network if needed (for consistency)
	if err := waitForNetworkReady(logger); err != nil {
		return fmt.Errorf("failed to wait for network ready: %w", err)
	}

	// Get cgroup path and assign immediately
	cgroupPath := os.Getenv("JOB_CGROUP_PATH")
	if cgroupPath == "" {
		return fmt.Errorf("JOB_CGROUP_PATH environment variable is required")
	}

	// Assign to cgroup - THIS IS CRITICAL
	if err := assignToCgroup(cgroupPath, logger); err != nil {
		return fmt.Errorf("failed to assign to cgroup: %w", err)
	}

	// Verify cgroup assignment
	if err := verifyCgroupAssignment(cgroupPath, logger); err != nil {
		return fmt.Errorf("cgroup assignment verification failed: %w", err)
	}

	logger.Info("process assigned to cgroup, starting upload processing")

	// Set up isolation
	if err := isolation.Setup(logger); err != nil {
		return fmt.Errorf("job isolation setup failed: %w", err)
	}

	// Process uploads within resource limits
	return processUploadsInCgroup(logger)
}

// runExecutePhase handles the execution phase (existing logic refactored)
func runExecutePhase(cfg *config.Config, logger *logger.Logger) error {
	logger.Info("starting execution phase in isolation")

	// CRITICAL: Wait for network setup FIRST before any other operations
	if err := waitForNetworkReady(logger); err != nil {
		return fmt.Errorf("failed to wait for network ready: %w", err)
	}

	// Validate required environment
	cgroupPath := os.Getenv("JOB_CGROUP_PATH")
	if cgroupPath == "" {
		return fmt.Errorf("JOB_CGROUP_PATH environment variable is required")
	}

	// Assign to cgroup immediately
	if err := assignToCgroup(cgroupPath, logger); err != nil {
		return fmt.Errorf("failed to assign to cgroup: %w", err)
	}

	// Verify cgroup assignment
	if err := verifyCgroupAssignment(cgroupPath, logger); err != nil {
		return fmt.Errorf("cgroup assignment verification failed: %w", err)
	}

	limits := map[string]string{
		"maxCPU":    os.Getenv("JOB_MAX_CPU"),
		"maxMemory": os.Getenv("JOB_MAX_MEMORY"),
		"maxIOBPS":  os.Getenv("JOB_MAX_IOBPS"),
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

// processUploadsInCgroup processes uploads within cgroup limits
func processUploadsInCgroup(logger *logger.Logger) error {
	// Get upload data from environment
	uploadsB64 := os.Getenv("JOB_UPLOADS_DATA")
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

	// Create workspace directory
	workspaceDir := "/work"
	if e := os.MkdirAll(workspaceDir, 0755); e != nil {
		return fmt.Errorf("failed to create workspace: %w", e)
	}

	// Process each file - ALL I/O IS NOW SUBJECT TO CGROUP LIMITS
	for _, upload := range uploads {
		if e := processUploadFile(&upload, workspaceDir, logger); e != nil {
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

// processUploadFile writes a single file within cgroup limits
func processUploadFile(upload interface{}, workspaceDir string, logger *logger.Logger) error {
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
		if err := writeFileInChunks(fullPath, u.Content, mode, logger); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}

		logger.Debug("wrote file within cgroup limits", "path", u.Path, "size", len(u.Content), "mode", mode)
	}

	return nil
}

// writeFileInChunks writes file data in chunks to better handle memory pressure
func writeFileInChunks(path string, content []byte, mode os.FileMode, logger *logger.Logger) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write in 64KB chunks
	chunkSize := 64 * 1024
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

// waitForNetworkReady waits for the parent process to signal that network setup is complete
func waitForNetworkReady(logger *logger.Logger) error {
	networkReadyFD := os.Getenv("NETWORK_READY_FD")
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
