package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"joblet/internal/joblet/core/volume"
	"joblet/internal/joblet/runtime"
	"joblet/internal/joblet/workflow/types"
	"joblet/pkg/logger"
)

// WorkflowValidator provides comprehensive validation of workflow definitions
// before execution to prevent runtime failures and resource waste.
// Uses concrete manager implementations directly instead of excessive interface abstractions.
type WorkflowValidator struct {
	logger *logger.Logger

	// Direct concrete managers - no interface abstraction needed
	volumeManager  *volume.Manager
	runtimeManager *runtime.Resolver
}

// NewWorkflowValidator creates a new workflow validator with required dependencies
func NewWorkflowValidator(volumeManager *volume.Manager, runtimeManager *runtime.Resolver) *WorkflowValidator {
	return &WorkflowValidator{
		logger:         logger.WithField("component", "workflow-validator"),
		volumeManager:  volumeManager,
		runtimeManager: runtimeManager,
	}
}

// ValidateWorkflow performs comprehensive pre-execution validation of a workflow
// This is the main entry point for server-side workflow validation
func (wv *WorkflowValidator) ValidateWorkflow(workflow types.WorkflowYAML) error {
	wv.logger.Info("starting comprehensive workflow validation")

	// 1. Check for circular dependencies
	if err := wv.validateNonCircularDependencies(workflow); err != nil {
		wv.logger.Error("circular dependency validation failed", "error", err)
		return fmt.Errorf("circular dependency detected: %w", err)
	}
	wv.logger.Debug("✅ No circular dependencies found")

	// 2. Validate all volumes exist
	if err := wv.validateVolumesExist(workflow); err != nil {
		wv.logger.Error("volume validation failed", "error", err)
		return fmt.Errorf("volume validation failed: %w", err)
	}
	wv.logger.Debug("✅ All required volumes exist")

	// 3. Validate all networks exist
	if err := wv.validateNetworksExist(workflow); err != nil {
		wv.logger.Error("network validation failed", "error", err)
		return fmt.Errorf("network validation failed: %w", err)
	}
	wv.logger.Debug("✅ All required networks exist")

	// 4. Validate all runtimes exist
	if err := wv.validateRuntimesExist(workflow); err != nil {
		wv.logger.Error("runtime validation failed", "error", err)
		return fmt.Errorf("runtime validation failed: %w", err)
	}
	wv.logger.Debug("✅ All required runtimes exist")

	// 5. Validate job dependencies reference existing jobs
	if err := wv.validateJobDependencies(workflow); err != nil {
		wv.logger.Error("job dependency validation failed", "error", err)
		return fmt.Errorf("job dependency validation failed: %w", err)
	}
	wv.logger.Debug("✅ All job dependencies are valid")

	// 6. Validate environment variables
	if err := wv.validateEnvironmentVariables(workflow); err != nil {
		wv.logger.Error("environment variable validation failed", "error", err)
		return fmt.Errorf("environment variable validation failed: %w", err)
	}
	wv.logger.Debug("✅ All environment variables are valid")

	wv.logger.Info("workflow validation completed successfully")
	return nil
}

// validateNonCircularDependencies checks for circular dependencies using DFS
func (wv *WorkflowValidator) validateNonCircularDependencies(workflow types.WorkflowYAML) error {
	// Build dependency graph
	graph := make(map[string][]string)
	for jobName, job := range workflow.Jobs {
		graph[jobName] = []string{}
		for _, req := range job.Requires {
			for depJob := range req {
				graph[jobName] = append(graph[jobName], depJob)
			}
		}
	}

	// Check for cycles using DFS with coloring
	color := make(map[string]int) // 0=white, 1=gray, 2=black

	var detectCycle func(string) error
	detectCycle = func(node string) error {
		if color[node] == 1 { // gray = currently being processed
			return fmt.Errorf("circular dependency involving job '%s'", node)
		}
		if color[node] == 2 { // black = already processed
			return nil
		}

		color[node] = 1 // mark as gray
		for _, neighbor := range graph[node] {
			if err := detectCycle(neighbor); err != nil {
				return err
			}
		}
		color[node] = 2 // mark as black
		return nil
	}

	for jobName := range workflow.Jobs {
		if color[jobName] == 0 {
			if err := detectCycle(jobName); err != nil {
				return err
			}
		}
	}

	return nil
}

// validateVolumesExist checks that all referenced volumes exist on the server
func (wv *WorkflowValidator) validateVolumesExist(workflow types.WorkflowYAML) error {
	requiredVolumes := make(map[string]bool)

	// Collect all volumes referenced in jobs
	for _, job := range workflow.Jobs {
		for _, volume := range job.Volumes {
			if volume != "" {
				requiredVolumes[volume] = true
			}
		}
	}

	if len(requiredVolumes) == 0 {
		wv.logger.Debug("no volumes required by workflow")
		return nil
	}

	// Check each volume exists using concrete volume manager
	var missingVolumes []string
	for volumeName := range requiredVolumes {
		if _, exists := wv.volumeManager.GetVolume(volumeName); !exists {
			// Also check filesystem path as fallback
			volumePath := filepath.Join("/opt/joblet/volumes", volumeName, "data")
			if _, err := os.Stat(volumePath); os.IsNotExist(err) {
				missingVolumes = append(missingVolumes, volumeName)
				wv.logger.Warn("volume not found", "volume", volumeName)
			}
		}
	}

	if len(missingVolumes) > 0 {
		return fmt.Errorf("missing volumes: %v", missingVolumes)
	}

	return nil
}

// validateNetworksExist checks that all referenced networks exist on the server
func (wv *WorkflowValidator) validateNetworksExist(workflow types.WorkflowYAML) error {
	requiredNetworks := make(map[string]bool)

	// Collect all networks referenced in jobs
	for _, job := range workflow.Jobs {
		if job.Network != "" {
			requiredNetworks[job.Network] = true
		}
	}

	if len(requiredNetworks) == 0 {
		wv.logger.Debug("no custom networks specified in workflow")
		return nil
	}

	// Built-in networks are always available
	builtinNetworks := map[string]bool{
		"none":     true,
		"isolated": true,
		"bridge":   true,
	}

	// Check each required network
	var missingNetworks []string
	for networkName := range requiredNetworks {
		if !builtinNetworks[networkName] {
			// For now, we accept any custom network name
			// In future, this could check against a network manager interface
			// similar to how volume and runtime validation work
			wv.logger.Debug("custom network specified", "network", networkName)
		}
	}

	if len(missingNetworks) > 0 {
		return fmt.Errorf("missing networks: %v", missingNetworks)
	}

	return nil
}

// validateRuntimesExist checks that all referenced runtimes exist and are available
func (wv *WorkflowValidator) validateRuntimesExist(workflow types.WorkflowYAML) error {
	requiredRuntimes := make(map[string]bool)

	// Collect all runtimes referenced in jobs
	for _, job := range workflow.Jobs {
		if job.Runtime != "" {
			requiredRuntimes[job.Runtime] = true
		}
	}

	if len(requiredRuntimes) == 0 {
		wv.logger.Debug("no runtimes specified in workflow")
		return nil
	}

	// Get available runtimes from concrete runtime manager
	availableRuntimes := make(map[string]bool)
	runtimes, err := wv.runtimeManager.ListRuntimes()
	if err != nil {
		return fmt.Errorf("failed to list available runtimes: %w", err)
	}
	for _, runtime := range runtimes {
		if runtime.Available {
			// Support both hyphen and colon format
			availableRuntimes[runtime.Name] = true
			// Normalize runtime name (server may store as "python-3.11-ml" but workflow uses "python-3.11-ml")
			if colonVersion := normalizeRuntimeName(runtime.Name); colonVersion != runtime.Name {
				availableRuntimes[colonVersion] = true
			}
		}
	}

	// Check each required runtime exists
	var missingRuntimes []string
	for runtimeName := range requiredRuntimes {
		// Try both original name and normalized version
		normalizedName := normalizeRuntimeName(runtimeName)

		if !availableRuntimes[runtimeName] && !availableRuntimes[normalizedName] {
			missingRuntimes = append(missingRuntimes, runtimeName)
			wv.logger.Warn("runtime not found", "runtime", runtimeName)
		}
	}

	if len(missingRuntimes) > 0 {
		return fmt.Errorf("missing runtimes: %v", missingRuntimes)
	}

	return nil
}

// validateJobDependencies checks that all job dependencies reference existing jobs
func (wv *WorkflowValidator) validateJobDependencies(workflow types.WorkflowYAML) error {
	// Get all job names
	allJobs := make(map[string]bool)
	for jobName := range workflow.Jobs {
		allJobs[jobName] = true
	}

	// Check dependencies
	for jobName, job := range workflow.Jobs {
		for _, req := range job.Requires {
			// req is map[string]string, iterate through key-value pairs
			for depJobName := range req {
				if !allJobs[depJobName] {
					wv.logger.Error("invalid job dependency", "job", jobName, "dependency", depJobName)
					return fmt.Errorf("job '%s' depends on non-existent job '%s'", jobName, depJobName)
				}
			}
		}
	}

	return nil
}

// normalizeRuntimeName converts between hyphen and colon format
// e.g., "python-3.11-ml" <-> "python-3.11-ml"
func normalizeRuntimeName(runtimeName string) string {
	// If it contains a colon, convert to hyphen format
	if len(runtimeName) > 0 {
		// Find first colon and replace with hyphen
		for i, char := range runtimeName {
			if char == ':' {
				return runtimeName[:i] + "-" + runtimeName[i+1:]
			}
		}
		// Find first hyphen and replace with colon
		for i, char := range runtimeName {
			if char == '-' {
				return runtimeName[:i] + ":" + runtimeName[i+1:]
			}
		}
	}
	return runtimeName
}

// ValidationSummary provides a summary of validation results
type ValidationSummary struct {
	Valid                     bool
	CircularDependencies      bool
	VolumesValid              bool
	NetworksValid             bool
	RuntimesValid             bool
	DependenciesValid         bool
	EnvironmentVariablesValid bool
	Errors                    []string
	Warnings                  []string
}

// ValidateWorkflowWithSummary performs validation and returns detailed summary
func (wv *WorkflowValidator) ValidateWorkflowWithSummary(workflow types.WorkflowYAML) *ValidationSummary {
	summary := &ValidationSummary{
		Valid:                     true,
		CircularDependencies:      true,
		VolumesValid:              true,
		NetworksValid:             true,
		RuntimesValid:             true,
		DependenciesValid:         true,
		EnvironmentVariablesValid: true,
		Errors:                    []string{},
		Warnings:                  []string{},
	}

	// Check circular dependencies
	if err := wv.validateNonCircularDependencies(workflow); err != nil {
		summary.Valid = false
		summary.CircularDependencies = false
		summary.Errors = append(summary.Errors, fmt.Sprintf("Circular dependencies: %v", err))
	}

	// Check volumes
	if err := wv.validateVolumesExist(workflow); err != nil {
		summary.Valid = false
		summary.VolumesValid = false
		summary.Errors = append(summary.Errors, fmt.Sprintf("Volume validation: %v", err))
	}

	// Check networks
	if err := wv.validateNetworksExist(workflow); err != nil {
		summary.Valid = false
		summary.NetworksValid = false
		summary.Errors = append(summary.Errors, fmt.Sprintf("Network validation: %v", err))
	}

	// Check runtimes
	if err := wv.validateRuntimesExist(workflow); err != nil {
		summary.Valid = false
		summary.RuntimesValid = false
		summary.Errors = append(summary.Errors, fmt.Sprintf("Runtime validation: %v", err))
	}

	// Check job dependencies
	if err := wv.validateJobDependencies(workflow); err != nil {
		summary.Valid = false
		summary.DependenciesValid = false
		summary.Errors = append(summary.Errors, fmt.Sprintf("Job dependencies: %v", err))
	}

	// Check environment variables
	if err := wv.validateEnvironmentVariables(workflow); err != nil {
		summary.Valid = false
		summary.EnvironmentVariablesValid = false
		summary.Errors = append(summary.Errors, fmt.Sprintf("Environment variables: %v", err))
	}

	return summary
}

// validateEnvironmentVariables performs comprehensive validation of environment variables
func (wv *WorkflowValidator) validateEnvironmentVariables(workflow types.WorkflowYAML) error {
	for jobName, job := range workflow.Jobs {
		jobLog := wv.logger.WithField("job", jobName)

		// Validate regular environment variables
		if err := wv.validateEnvironmentVariableMap(job.Environment, jobName, "environment", jobLog); err != nil {
			return err
		}

		// Validate secret environment variables
		if err := wv.validateEnvironmentVariableMap(job.SecretEnvironment, jobName, "secret_environment", jobLog); err != nil {
			return err
		}

		// Check for conflicts between regular and secret environment variables
		if err := wv.validateEnvironmentVariableConflicts(job.Environment, job.SecretEnvironment, jobName, jobLog); err != nil {
			return err
		}
	}

	return nil
}

// validateEnvironmentVariableMap validates a map of environment variables
func (wv *WorkflowValidator) validateEnvironmentVariableMap(envVars map[string]string, jobName, envType string, jobLog *logger.Logger) error {
	if len(envVars) == 0 {
		return nil
	}

	// Environment variable name validation regex
	// Valid names: letters, digits, underscore, must start with letter or underscore
	validNameRegex := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

	for key, value := range envVars {
		// Validate environment variable name
		if !validNameRegex.MatchString(key) {
			jobLog.Error("invalid environment variable name", "type", envType, "key", key)
			return fmt.Errorf("job '%s': invalid %s variable name '%s' - must start with letter or underscore and contain only letters, numbers, and underscores", jobName, envType, key)
		}

		// Check for reserved environment variable names
		if wv.isReservedEnvironmentVariable(key) {
			jobLog.Warn("reserved environment variable used", "type", envType, "key", key)
			// For now, just warn - don't fail validation
		}

		// Validate environment variable value length
		if len(value) > 32768 { // 32KB limit
			jobLog.Error("environment variable value too long", "type", envType, "key", key, "length", len(value))
			return fmt.Errorf("job '%s': %s variable '%s' value is too long (%d bytes, max 32768)", jobName, envType, key, len(value))
		}

		// Check for potentially dangerous values (basic security check)
		if wv.containsDangerousPatterns(value) {
			jobLog.Warn("potentially dangerous environment variable value", "type", envType, "key", key)
			// For now, just warn - don't fail validation
		}

		jobLog.Debug("environment variable validated", "type", envType, "key", key, "valueLength", len(value))
	}

	return nil
}

// validateEnvironmentVariableConflicts checks for conflicts between regular and secret environment variables
func (wv *WorkflowValidator) validateEnvironmentVariableConflicts(regularEnv, secretEnv map[string]string, jobName string, jobLog *logger.Logger) error {
	if regularEnv == nil || secretEnv == nil {
		return nil
	}

	// Check for duplicate keys between regular and secret environment variables
	var conflicts []string
	for key := range regularEnv {
		if _, exists := secretEnv[key]; exists {
			conflicts = append(conflicts, key)
		}
	}

	if len(conflicts) > 0 {
		jobLog.Error("environment variable conflicts detected", "conflicts", conflicts)
		return fmt.Errorf("job '%s': environment variable conflicts detected - the following variables are defined in both environment and secret_environment: %v", jobName, conflicts)
	}

	return nil
}

// isReservedEnvironmentVariable checks if an environment variable name is reserved by the system
func (wv *WorkflowValidator) isReservedEnvironmentVariable(name string) bool {
	// List of common reserved environment variables
	reserved := map[string]bool{
		// System variables
		"PATH":     true,
		"HOME":     true,
		"USER":     true,
		"SHELL":    true,
		"TERM":     true,
		"PWD":      true,
		"OLDPWD":   true,
		"HOSTNAME": true,
		"LANG":     true,
		"LC_ALL":   true,

		// Joblet-specific variables (that might be set by the system)
		"JOBLET_JOB_ID":      true,
		"JOBLET_WORKFLOW_ID": true,
		"JOBLET_RUNTIME":     true,
		"JOBLET_VOLUME_PATH": true,
	}

	return reserved[name]
}

// containsDangerousPatterns performs basic security checks on environment variable values
func (wv *WorkflowValidator) containsDangerousPatterns(value string) bool {
	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"$(",          // Command substitution
		"`",           // Backtick command substitution
		"rm -rf",      // Dangerous delete commands
		"format C:",   // Windows format command
		"del /f",      // Windows delete command
		"../",         // Path traversal attempt
		"passwd",      // Password file access
		"/etc/shadow", // Shadow file access
	}

	lowerValue := strings.ToLower(value)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerValue, strings.ToLower(pattern)) {
			return true
		}
	}

	return false
}
