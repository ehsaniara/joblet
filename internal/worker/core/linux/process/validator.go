package process

import (
	"fmt"
	"path/filepath"
	"strings"

	"worker/pkg/logger"
	osinterface "worker/pkg/os"
)

// Validator handles validation of process-related operations
type Validator struct {
	osInterface   osinterface.OsInterface
	execInterface osinterface.ExecInterface
	logger        *logger.Logger
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s' (value: %v): %s",
		e.Field, e.Value, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value interface{}, message string) error {
	return ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// LaunchRequest represents a process launch request
type LaunchRequest struct {
	Command        string
	Args           []string
	MaxCPU         int32
	MaxMemory      int32
	MaxIOBPS       int32
	NetworkGroupID string
	JobID          string
	InitPath       string
	Environment    []string
	CgroupPath     string
}

// NewValidator creates a new process validator
func NewValidator(osInterface osinterface.OsInterface, execInterface osinterface.ExecInterface) *Validator {
	return &Validator{
		osInterface:   osInterface,
		execInterface: execInterface,
		logger:        logger.New().WithField("component", "process-validator"),
	}
}

// ValidateLaunchRequest validates a process launch request
func (v *Validator) ValidateLaunchRequest(req *LaunchRequest) error {
	if req == nil {
		return fmt.Errorf("launch request cannot be nil")
	}

	log := v.logger.WithField("jobID", req.JobID)
	log.Debug("validating launch request")

	// Validate command
	if err := v.ValidateCommand(req.Command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Validate arguments
	if err := v.ValidateArguments(req.Args); err != nil {
		return fmt.Errorf("arguments validation failed: %w", err)
	}

	// Validate resource limits
	if err := v.ValidateResourceLimits(req.MaxCPU, req.MaxMemory, req.MaxIOBPS); err != nil {
		return fmt.Errorf("resource limits validation failed: %w", err)
	}

	// Validate job ID
	if err := v.ValidateJobID(req.JobID); err != nil {
		return fmt.Errorf("job ID validation failed: %w", err)
	}

	// Validate network group ID if provided
	if req.NetworkGroupID != "" {
		if err := v.ValidateNetworkGroupID(req.NetworkGroupID); err != nil {
			return fmt.Errorf("network group ID validation failed: %w", err)
		}
	}

	// Validate init path if provided
	if req.InitPath != "" {
		if err := v.ValidateInitPath(req.InitPath); err != nil {
			return fmt.Errorf("init path validation failed: %w", err)
		}
	}

	// Validate cgroup path if provided
	if req.CgroupPath != "" {
		if err := v.ValidateCgroupPath(req.CgroupPath); err != nil {
			return fmt.Errorf("cgroup path validation failed: %w", err)
		}
	}

	log.Debug("launch request validation passed")
	return nil
}

// ValidateCommand validates a command string
func (v *Validator) ValidateCommand(command string) error {
	if command == "" {
		return NewValidationError("command", command, "command cannot be empty")
	}

	// Check for dangerous characters
	if strings.ContainsAny(command, ";&|`$()") {
		return NewValidationError("command", command, "command contains dangerous characters")
	}

	// Check command length
	if len(command) > 1024 {
		return NewValidationError("command", command, "command too long (max 1024 characters)")
	}

	return nil
}

// ValidateArguments validates command arguments
func (v *Validator) ValidateArguments(args []string) error {
	if len(args) > 100 {
		return NewValidationError("args", len(args), "too many arguments (max 100)")
	}

	for i, arg := range args {
		if len(arg) > 1024 {
			return NewValidationError("args", fmt.Sprintf("arg[%d]", i),
				"argument too long (max 1024 characters)")
		}

		// Check for null bytes
		if strings.Contains(arg, "\x00") {
			return NewValidationError("args", fmt.Sprintf("arg[%d]", i),
				"argument contains null bytes")
		}
	}

	return nil
}

// ValidateResourceLimits validates resource limit values
func (v *Validator) ValidateResourceLimits(maxCPU, maxMemory, maxIOBPS int32) error {
	// CPU validation
	if maxCPU < 0 {
		return NewValidationError("maxCPU", maxCPU, "CPU limit cannot be negative")
	}
	if maxCPU > 10000 { // 100 cores max
		return NewValidationError("maxCPU", maxCPU, "CPU limit too high (max 10000%)")
	}

	// Memory validation
	if maxMemory < 0 {
		return NewValidationError("maxMemory", maxMemory, "memory limit cannot be negative")
	}
	if maxMemory > 1024*1024 { // 1TB max
		return NewValidationError("maxMemory", maxMemory, "memory limit too high (max 1TB)")
	}

	// IO validation
	if maxIOBPS < 0 {
		return NewValidationError("maxIOBPS", maxIOBPS, "IO limit cannot be negative")
	}
	if maxIOBPS > 10*1024*1024 { // 10GB/s max
		return NewValidationError("maxIOBPS", maxIOBPS, "IO limit too high (max 10GB/s)")
	}

	return nil
}

// ValidateJobID validates a job ID
func (v *Validator) ValidateJobID(jobID string) error {
	if jobID == "" {
		return NewValidationError("jobID", jobID, "job ID cannot be empty")
	}

	if len(jobID) > 64 {
		return NewValidationError("jobID", jobID, "job ID too long (max 64 characters)")
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	for _, char := range jobID {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return NewValidationError("jobID", jobID,
				"job ID contains invalid characters (only alphanumeric, dash, underscore allowed)")
		}
	}

	return nil
}

// ValidateNetworkGroupID validates a network group ID
func (v *Validator) ValidateNetworkGroupID(groupID string) error {
	if len(groupID) > 64 {
		return NewValidationError("networkGroupID", groupID,
			"network group ID too long (max 64 characters)")
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	for _, char := range groupID {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_') {
			return NewValidationError("networkGroupID", groupID,
				"network group ID contains invalid characters")
		}
	}

	return nil
}

// ValidateInitPath validates the job-init binary path
func (v *Validator) ValidateInitPath(initPath string) error {
	if initPath == "" {
		return NewValidationError("initPath", initPath, "init path cannot be empty")
	}

	// Check if path is absolute
	if !filepath.IsAbs(initPath) {
		return NewValidationError("initPath", initPath, "init path must be absolute")
	}

	// Check if file exists and is executable
	fileInfo, err := v.osInterface.Stat(initPath)
	if err != nil {
		if v.osInterface.IsNotExist(err) {
			return NewValidationError("initPath", initPath, "init binary does not exist")
		}
		return NewValidationError("initPath", initPath,
			fmt.Sprintf("failed to stat init binary: %v", err))
	}

	// Check if it's a regular file
	if !fileInfo.Mode().IsRegular() {
		return NewValidationError("initPath", initPath, "init path is not a regular file")
	}

	// Check if it's executable
	if fileInfo.Mode().Perm()&0111 == 0 {
		return NewValidationError("initPath", initPath, "init binary is not executable")
	}

	return nil
}

// ValidateCgroupPath validates a cgroup path
func (v *Validator) ValidateCgroupPath(cgroupPath string) error {
	if cgroupPath == "" {
		return NewValidationError("cgroupPath", cgroupPath, "cgroup path cannot be empty")
	}

	// Check if path is absolute
	if !filepath.IsAbs(cgroupPath) {
		return NewValidationError("cgroupPath", cgroupPath, "cgroup path must be absolute")
	}

	// Check for path traversal attempts
	cleanPath := filepath.Clean(cgroupPath)
	if cleanPath != cgroupPath {
		return NewValidationError("cgroupPath", cgroupPath,
			"cgroup path contains path traversal attempts")
	}

	// Check if path exists
	if _, err := v.osInterface.Stat(cgroupPath); err != nil {
		if v.osInterface.IsNotExist(err) {
			return NewValidationError("cgroupPath", cgroupPath, "cgroup directory does not exist")
		}
		return NewValidationError("cgroupPath", cgroupPath,
			fmt.Sprintf("failed to stat cgroup directory: %v", err))
	}

	return nil
}

// ResolveCommand resolves a command to its full path
func (v *Validator) ResolveCommand(command string) (string, error) {
	if command == "" {
		return "", fmt.Errorf("command cannot be empty")
	}

	log := v.logger.WithField("command", command)

	// If command is already absolute, validate it exists
	if filepath.IsAbs(command) {
		if _, err := v.osInterface.Stat(command); err != nil {
			log.Error("absolute command path not found", "error", err)
			return "", fmt.Errorf("command %s not found: %w", command, err)
		}
		log.Debug("using absolute command path")
		return command, nil
	}

	// Try to resolve using PATH
	if resolvedPath, err := v.execInterface.LookPath(command); err == nil {
		log.Debug("resolved command via PATH", "resolved", resolvedPath)
		return resolvedPath, nil
	}

	// Try common paths
	commonPaths := []string{
		filepath.Join("/bin", command),
		filepath.Join("/usr/bin", command),
		filepath.Join("/usr/local/bin", command),
		filepath.Join("/sbin", command),
		filepath.Join("/usr/sbin", command),
	}

	log.Debug("checking common command locations", "paths", commonPaths)

	for _, path := range commonPaths {
		if _, err := v.osInterface.Stat(path); err == nil {
			log.Debug("found command in common location", "path", path)
			return path, nil
		}
	}

	log.Error("command not found anywhere", "searchedPaths", commonPaths)
	return "", fmt.Errorf("command %s not found in PATH or common locations", command)
}

// ValidateEnvironment validates environment variables
func (v *Validator) ValidateEnvironment(env []string) error {
	if len(env) > 1000 {
		return NewValidationError("environment", len(env),
			"too many environment variables (max 1000)")
	}

	for i, envVar := range env {
		if len(envVar) > 8192 {
			return NewValidationError("environment", fmt.Sprintf("env[%d]", i),
				"environment variable too long (max 8192 characters)")
		}

		// Check for null bytes
		if strings.Contains(envVar, "\x00") {
			return NewValidationError("environment", fmt.Sprintf("env[%d]", i),
				"environment variable contains null bytes")
		}

		// Check for valid format (KEY=VALUE)
		if !strings.Contains(envVar, "=") {
			return NewValidationError("environment", fmt.Sprintf("env[%d]", i),
				"environment variable missing '=' separator")
		}
	}

	return nil
}

// ValidatePID validates a process ID
func (v *Validator) ValidatePID(pid int32) error {
	if pid <= 0 {
		return NewValidationError("pid", pid, "PID must be positive")
	}

	if pid > 4194304 { // Linux PID_MAX_LIMIT
		return NewValidationError("pid", pid, "PID too large")
	}

	return nil
}

// ValidateSignal validates a signal number
func (v *Validator) ValidateSignal(signal int) error {
	// Valid signal range in Unix systems
	if signal < 1 || signal > 64 {
		return NewValidationError("signal", signal, "invalid signal number (1-64)")
	}

	return nil
}

// SanitizeCommand sanitizes a command string for logging
func (v *Validator) SanitizeCommand(command string) string {
	// Remove potentially sensitive information for logging
	if len(command) > 100 {
		return command[:97] + "..."
	}
	return command
}

// SanitizeArgs sanitizes arguments for logging
func (v *Validator) SanitizeArgs(args []string) []string {
	if len(args) == 0 {
		return args
	}

	sanitized := make([]string, len(args))
	for i, arg := range args {
		if len(arg) > 50 {
			sanitized[i] = arg[:47] + "..."
		} else {
			sanitized[i] = arg
		}
	}

	// Limit number of args shown
	if len(sanitized) > 10 {
		result := make([]string, 11)
		copy(result, sanitized[:10])
		result[10] = fmt.Sprintf("... (%d more)", len(sanitized)-10)
		return result
	}

	return sanitized
}
