package jobexec

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// JobConfig represents job configuration with upload support
type JobConfig struct {
	JobID      string
	Command    string
	Args       []string
	CgroupPath string
	Uploads    []UploadInfo
}

// UploadInfo represents file upload information
type UploadInfo struct {
	Path        string
	Content     []byte
	Mode        uint32
	IsDirectory bool
}

// JobExecutor handles job execution using platform abstraction
type JobExecutor struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewJobExecutor creates a new job executor with the given platform
func NewJobExecutor(p platform.Platform, logger *logger.Logger) *JobExecutor {
	return &JobExecutor{
		platform: p,
		logger:   logger.WithField("component", "jobexec"),
	}
}

// LoadConfigFromEnv loads job configuration including uploads from environment variables
func LoadConfigFromEnv(logger *logger.Logger) (*JobConfig, error) {
	p := platform.NewPlatform()
	executor := NewJobExecutor(p, logger)
	return executor.LoadConfigFromEnv()
}

// LoadConfigFromEnv loads job configuration including embedded uploads
func (je *JobExecutor) LoadConfigFromEnv() (*JobConfig, error) {
	jobID := je.platform.Getenv("JOB_ID")
	command := je.platform.Getenv("JOB_COMMAND")
	cgroupPath := je.platform.Getenv("JOB_CGROUP_PATH")
	argsCountStr := je.platform.Getenv("JOB_ARGS_COUNT")

	if jobID == "" || command == "" {
		return nil, fmt.Errorf("missing required environment variables (JOB_ID=%s, JOB_COMMAND=%s)",
			jobID, command)
	}

	var args []string
	if argsCountStr != "" {
		argsCount, err := strconv.Atoi(argsCountStr)
		if err != nil {
			return nil, fmt.Errorf("invalid JOB_ARGS_COUNT: %v", err)
		}

		args = make([]string, argsCount)
		for i := 0; i < argsCount; i++ {
			argKey := fmt.Sprintf("JOB_ARG_%d", i)
			args[i] = je.platform.Getenv(argKey)
		}
	}

	// Load embedded uploads if present
	uploads, err := je.loadUploadsFromEnv()
	if err != nil {
		return nil, fmt.Errorf("failed to load uploads: %w", err)
	}

	je.logger.Debug("loaded job configuration with uploads",
		"jobId", jobID,
		"command", command,
		"argsCount", len(args),
		"uploadsCount", len(uploads),
		"cgroupPath", cgroupPath)

	return &JobConfig{
		JobID:      jobID,
		Command:    command,
		Args:       args,
		CgroupPath: cgroupPath,
		Uploads:    uploads,
	}, nil
}

// loadUploadsFromEnv deserializes uploads from environment variables
func (je *JobExecutor) loadUploadsFromEnv() ([]UploadInfo, error) {
	uploadsData := je.platform.Getenv("JOB_UPLOADS")
	if uploadsData == "" {
		return nil, nil // No uploads
	}

	// Decode base64 encoded JSON
	jsonData, err := base64.StdEncoding.DecodeString(uploadsData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode uploads data: %w", err)
	}

	// Parse JSON upload data
	var uploadData []struct {
		Path        string `json:"path"`
		Content     string `json:"content"` // Base64 encoded
		Mode        uint32 `json:"mode"`
		IsDirectory bool   `json:"isDirectory"`
	}

	if err := json.Unmarshal(jsonData, &uploadData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal upload data: %w", err)
	}

	// Convert to UploadInfo with decoded content
	var uploads []UploadInfo
	for _, data := range uploadData {
		content, err := base64.StdEncoding.DecodeString(data.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to decode file content for %s: %w", data.Path, err)
		}

		uploads = append(uploads, UploadInfo{
			Path:        data.Path,
			Content:     content,
			Mode:        data.Mode,
			IsDirectory: data.IsDirectory,
		})
	}

	je.logger.Debug("deserialized uploads from environment",
		"uploadCount", len(uploads))

	return uploads, nil
}

// Execute executes the job with upload processing
func Execute(config *JobConfig, logger *logger.Logger) error {
	p := platform.NewPlatform()
	executor := NewJobExecutor(p, logger)
	return executor.ExecuteWithUploads(config)
}

// ExecuteWithUploads executes the job with file upload processing
func (je *JobExecutor) ExecuteWithUploads(config *JobConfig) error {
	log := je.logger.WithField("jobID", config.JobID)

	// Process uploads FIRST (inside cgroups and isolation)
	if len(config.Uploads) > 0 {
		log.Info("processing file uploads", "fileCount", len(config.Uploads))

		if err := je.processUploads(config.Uploads); err != nil {
			return fmt.Errorf("file upload processing failed: %w", err)
		}

		log.Info("file upload completed", "filesUploaded", len(config.Uploads))
	}

	// Now execute the actual job command
	log.Debug("executing job command", "command", config.Command, "args", config.Args)

	// Use the existing execution logic but convert back to original JobConfig
	originalConfig := &JobConfig{
		JobID:      config.JobID,
		Command:    config.Command,
		Args:       config.Args,
		CgroupPath: config.CgroupPath,
		Uploads:    nil, // Already processed
	}

	return je.Execute(originalConfig)
}

// processUploads handles file upload processing inside the isolated environment
func (je *JobExecutor) processUploads(uploads []UploadInfo) error {
	log := je.logger.WithField("operation", "upload-processing")

	// Determine workspace directory (we're inside chroot, so paths are relative to /)
	workspaceDir := "/work"

	// Ensure workspace exists
	if err := je.platform.MkdirAll(workspaceDir, 0755); err != nil {
		return fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Process each upload
	for i, upload := range uploads {
		log.Debug("processing upload", "file", i+1, "path", upload.Path, "size", len(upload.Content))

		fullPath := filepath.Join(workspaceDir, upload.Path)

		if upload.IsDirectory {
			// Create directory
			mode := os.FileMode(upload.Mode)
			if mode == 0 {
				mode = 0755 // Default directory permissions
			}

			if err := je.platform.MkdirAll(fullPath, mode); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", upload.Path, err)
			}

			log.Debug("created directory", "path", upload.Path, "mode", fmt.Sprintf("%o", mode))
		} else {
			// Ensure parent directory exists
			parentDir := filepath.Dir(fullPath)
			if err := je.platform.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for %s: %w", upload.Path, err)
			}

			// Write file
			mode := os.FileMode(upload.Mode)
			if mode == 0 {
				mode = 0644 // Default file permissions
			}

			if err := je.platform.WriteFile(fullPath, upload.Content, mode); err != nil {
				return fmt.Errorf("failed to write file %s: %w", upload.Path, err)
			}

			log.Debug("wrote file", "path", upload.Path, "size", len(upload.Content), "mode", fmt.Sprintf("%o", mode))
		}
	}

	return nil
}

// Execute executes the job based on platform using platform abstraction
func (je *JobExecutor) Execute(config *JobConfig) error {
	switch runtime.GOOS {
	case "linux":
		return je.executeLinux(config)
	case "darwin":
		return je.executeDarwin(config)
	default:
		return fmt.Errorf("unsupported platform for job execution: %s", runtime.GOOS)
	}
}

// executeLinux executes job on Linux using platform abstraction
func (je *JobExecutor) executeLinux(config *JobConfig) error {
	je.logger.Debug("executing job on Linux", "command", config.Command, "args", config.Args)

	// Resolve command path using platform abstraction
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("command resolution failed: %w", err)
	}

	// Prepare arguments and environment using platform abstraction
	execArgs := append([]string{config.Command}, config.Args...)
	envVars := je.platform.Environ()

	je.logger.Debug("executing command with platform exec",
		"commandPath", commandPath, "args", execArgs)

	// Use platform abstraction for exec
	if err := je.platform.Exec(commandPath, execArgs, envVars); err != nil {
		return fmt.Errorf("platform exec failed: %w", err)
	}

	// This line should never be reached on successful exec
	return nil
}

// executeDarwin executes job on macOS using platform abstraction
func (je *JobExecutor) executeDarwin(config *JobConfig) error {
	je.logger.Info("executing job on macOS", "command", config.Command, "args", config.Args)

	// Resolve command path using platform abstraction
	commandPath, err := je.resolveCommandPath(config.Command)
	if err != nil {
		return fmt.Errorf("command resolution failed: %w", err)
	}

	// Use platform abstraction to create and run command
	cmd := je.platform.CreateCommand(commandPath, config.Args...)
	cmd.SetEnv(je.platform.Environ())

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("command start failed: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command execution failed: %w", err)
	}

	je.logger.Info("command completed successfully on macOS")
	return nil
}

// resolveCommandPath resolves a command to its full path using platform abstraction
func (je *JobExecutor) resolveCommandPath(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	// If already absolute path, verify it exists using platform abstraction
	if filepath.IsAbs(command) {
		if _, err := je.platform.Stat(command); err != nil {
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		return command, nil
	}

	// Try to find in PATH using platform abstraction
	if resolvedPath, err := je.platform.LookPath(command); err == nil {
		je.logger.Debug("resolved command via PATH", "command", command, "resolved", resolvedPath)
		return resolvedPath, nil
	}

	// Check common paths using platform abstraction
	commonPaths := je.getCommonPaths(command)

	for _, path := range commonPaths {
		if _, err := je.platform.Stat(path); err == nil {
			je.logger.Debug("found command in common location", "command", command, "path", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// getCommonPaths returns platform-specific common command paths
func (je *JobExecutor) getCommonPaths(command string) []string {
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	// Add platform-specific paths
	switch runtime.GOOS {
	case "darwin":
		commonPaths = append(commonPaths,
			filepath.Join("/opt/homebrew/bin", command),
			filepath.Join("/usr/local/Cellar", command))
	case "linux":
		commonPaths = append(commonPaths,
			filepath.Join("/usr/local/sbin", command))
	}

	return commonPaths
}

// HandleCompletion handles platform-specific completion logic
func HandleCompletion(logger *logger.Logger) {
	p := platform.NewPlatform()
	executor := NewJobExecutor(p, logger)
	executor.HandleCompletion()
}

// HandleCompletion handles platform-specific completion logic using platform abstraction
func (je *JobExecutor) HandleCompletion() {
	switch runtime.GOOS {
	case "linux":
		// On Linux: This should never be reached since Execute calls platform.Exec
		je.logger.Error("unexpected return from Execute - exec should have replaced process")
		je.platform.Exit(1)
	case "darwin":
		// On macOS: This is expected since we use command execution instead of exec
		je.logger.Info("job execution completed successfully on macOS")
	default:
		je.logger.Info("job execution completed", "platform", runtime.GOOS)
	}
}
