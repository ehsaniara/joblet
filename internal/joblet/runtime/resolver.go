package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"

	"gopkg.in/yaml.v3"
)

// Resolver provides basic runtime listing functionality for CLI commands
// This replaces the complex Manager/Resolver that was unused by job execution
type Resolver struct {
	runtimesPath string
	platform     platform.Platform
	logger       *logger.Logger
}

// NewResolver creates a new simple runtime resolver (maintains API compatibility)
func NewResolver(runtimesPath string, platform platform.Platform) *Resolver {
	return &Resolver{
		runtimesPath: runtimesPath,
		platform:     platform,
		logger:       logger.New().WithField("component", "simple-runtime-resolver"),
	}
}

// ListRuntimes lists all available runtimes by scanning directories
func (r *Resolver) ListRuntimes() ([]*RuntimeInfo, error) {
	var runtimes []*RuntimeInfo

	// Check if runtimes directory exists
	if !r.platform.DirExists(r.runtimesPath) {
		return runtimes, nil
	}

	// List runtime directories (flat structure: /opt/joblet/runtimes/openjdk-21)
	entries, err := r.platform.ReadDir(r.runtimesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read runtimes directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runtimePath := filepath.Join(r.runtimesPath, entry.Name())
		configPath := filepath.Join(runtimePath, "runtime.yml")

		// Load runtime config
		config, err := r.loadRuntimeConfig(configPath)
		if err != nil {
			r.logger.Debug("skipping runtime without valid config", "path", runtimePath, "error", err)
			continue
		}

		// Get directory size
		size, _ := r.getDirectorySize(runtimePath)

		// Get language/type from runtime.yml (could be language, framework, or any runtime type)
		runtimeType := config.Language
		if runtimeType == "" {
			// Fallback: use first part of directory name as runtime type identifier
			runtimeType = r.extractTypeFromName(entry.Name())
		}

		info := &RuntimeInfo{
			Name:        config.Name,
			Language:    runtimeType, // Note: "Language" field is kept for API compatibility but represents any runtime type
			Version:     config.Version,
			Description: config.Description,
			Path:        runtimePath,
			Size:        size,
			Available:   true,
		}

		runtimes = append(runtimes, info)
	}

	return runtimes, nil
}

// loadRuntimeConfig loads a runtime configuration from a YAML file
func (r *Resolver) loadRuntimeConfig(configPath string) (*RuntimeConfig, error) {
	data, err := r.platform.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config RuntimeConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// extractTypeFromName extracts runtime type from directory name (generic approach)
func (r *Resolver) extractTypeFromName(runtimeName string) string {
	// Generic approach: extract the first part before the dash as the runtime type
	// This could be a language, framework, system, or any kind of runtime environment
	// The actual type will be read from runtime.yml's "language" field (which is a misnomer - it's really "type")
	parts := strings.Split(runtimeName, "-")
	if len(parts) > 0 {
		return parts[0]
	}

	return runtimeName
}

// getDirectorySize calculates the total size of a directory
func (r *Resolver) getDirectorySize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// ResolveRuntime resolves a runtime spec to config (for runtime info command)
// This is a simplified version that just loads the config directly
func (r *Resolver) ResolveRuntime(runtimeSpec string) (*RuntimeConfig, error) {
	if runtimeSpec == "" {
		return nil, nil
	}

	// Find the runtime directory
	runtimeDir, err := r.findRuntimeDirectory(runtimeSpec)
	if err != nil {
		return nil, fmt.Errorf("runtime not found: %w", err)
	}

	// Load runtime configuration
	configPath := filepath.Join(runtimeDir, "runtime.yml")
	config, err := r.loadRuntimeConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime config: %w", err)
	}

	// Basic validation
	if err := r.validateRuntime(config); err != nil {
		return nil, fmt.Errorf("runtime validation failed: %w", err)
	}

	return config, nil
}

// findRuntimeDirectory finds the directory for a runtime specification (fully generic)
func (r *Resolver) findRuntimeDirectory(spec string) (string, error) {
	// Scan all runtime directories and check their runtime.yml files
	entries, err := r.platform.ReadDir(r.runtimesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read runtimes directory: %w", err)
	}

	// First pass: try exact name match
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runtimePath := filepath.Join(r.runtimesPath, entry.Name())
		configPath := filepath.Join(runtimePath, "runtime.yml")

		// Load runtime config to check if it matches the spec
		config, err := r.loadRuntimeConfig(configPath)
		if err != nil {
			continue // Skip directories without valid runtime.yml
		}

		// Check if the runtime name matches exactly
		if config.Name == spec {
			return runtimePath, nil
		}
	}

	// Second pass: try parsed spec matching for more complex queries
	parsedSpec := r.parseRuntimeSpec(spec)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runtimePath := filepath.Join(r.runtimesPath, entry.Name())
		configPath := filepath.Join(runtimePath, "runtime.yml")

		// Load runtime config to check if it matches the spec
		config, err := r.loadRuntimeConfig(configPath)
		if err != nil {
			continue // Skip directories without valid runtime.yml
		}

		// Check if this runtime matches the requested spec
		if r.runtimeMatches(config, parsedSpec) {
			return runtimePath, nil
		}
	}

	return "", fmt.Errorf("runtime not found for spec %s", spec)
}

// runtimeMatches checks if a runtime config matches a runtime specification
func (r *Resolver) runtimeMatches(config *RuntimeConfig, spec *RuntimeSpec) bool {
	// Match by runtime type/category (if specified in spec)
	// Note: "Language" field is a misnomer - it represents any runtime type (language, framework, system, etc.)
	if spec.Language != "" && config.Language != "" {
		if spec.Language != config.Language {
			return false
		}
	}

	// Match by version (if specified in spec)
	if spec.Version != "" && spec.Version != "unknown" && spec.Version != "latest" {
		if config.Version != spec.Version {
			return false
		}
	}

	// For now, we don't match by tags since runtime.yml doesn't have a tags field
	// This could be extended in the future if needed

	return true
}

// parseRuntimeSpec parses a runtime specification string
func (r *Resolver) parseRuntimeSpec(spec string) *RuntimeSpec {
	// Handle colon-based specs (e.g., "python:3.11+ml+gpu")
	if strings.Contains(spec, ":") {
		parts := strings.Split(spec, ":")
		if len(parts) == 2 {
			language := parts[0]
			versionAndTags := strings.Split(parts[1], "+")
			version := versionAndTags[0]
			var tags []string
			if len(versionAndTags) > 1 {
				tags = versionAndTags[1:]
			}
			return &RuntimeSpec{
				Language: language,
				Version:  version,
				Tags:     tags,
			}
		}
	}

	// Handle dash-based specs with plus-separated tags (e.g., "python-3.11-ml+gpu")
	if strings.Contains(spec, "+") {
		parts := strings.Split(spec, "+")
		mainSpec := parts[0]
		tags := parts[1:]

		// Parse the main spec (e.g., "python-3.11-ml")
		dashParts := strings.Split(mainSpec, "-")
		if len(dashParts) >= 3 {
			language := dashParts[0]
			version := dashParts[1]
			// Add remaining dash parts as additional tags
			additionalTags := dashParts[2:]
			allTags := append(additionalTags, tags...)
			return &RuntimeSpec{
				Language: language,
				Version:  version,
				Tags:     allTags,
			}
		}
	}

	// Handle simple dash-based specs without plus (e.g., "python-3.11")
	dashParts := strings.Split(spec, "-")
	if len(dashParts) == 2 {
		return &RuntimeSpec{
			Language: dashParts[0],
			Version:  dashParts[1],
			Tags:     nil,
		}
	}

	// Treat as runtime name directly
	return &RuntimeSpec{
		Language: r.extractTypeFromName(spec),
		Version:  "unknown",
		Tags:     []string{},
	}
}

// validateRuntime performs basic runtime validation
func (r *Resolver) validateRuntime(config *RuntimeConfig) error {
	// Check architecture compatibility
	if len(config.Requirements.Architectures) > 0 {
		currentArch := runtime.GOARCH
		archCompatible := false
		for _, arch := range config.Requirements.Architectures {
			if arch == currentArch || arch == "x86_64" && currentArch == "amd64" {
				archCompatible = true
				break
			}
		}
		if !archCompatible {
			return fmt.Errorf("runtime requires architecture %v, but system is %s",
				config.Requirements.Architectures, currentArch)
		}
	}

	return nil
}

// IsGPUEnabled checks if a runtime has GPU support based on its name and tags
func (r *Resolver) IsGPUEnabled(runtimeName string) bool {
	// Check for common GPU indicators in runtime name
	lowerName := strings.ToLower(runtimeName)

	// Direct GPU indicators
	if strings.Contains(lowerName, "gpu") ||
		strings.Contains(lowerName, "cuda") ||
		strings.Contains(lowerName, "-ml") ||
		strings.Contains(lowerName, "tensorflow") ||
		strings.Contains(lowerName, "pytorch") ||
		strings.Contains(lowerName, "nvidia") {
		return true
	}

	// Check by parsing runtime spec and looking for GPU tags
	spec := r.parseRuntimeSpec(runtimeName)
	return r.specHasGPUSupport(spec)
}

// specHasGPUSupport checks if a runtime specification indicates GPU support
func (r *Resolver) specHasGPUSupport(spec *RuntimeSpec) bool {
	if spec == nil {
		return false
	}

	// Check tags for GPU indicators
	for _, tag := range spec.Tags {
		lowerTag := strings.ToLower(tag)
		if lowerTag == "gpu" ||
			lowerTag == "cuda" ||
			lowerTag == "ml" ||
			lowerTag == "ai" ||
			lowerTag == "tensorflow" ||
			lowerTag == "pytorch" ||
			lowerTag == "nvidia" {
			return true
		}
	}

	return false
}

// GetCUDARequirements returns the CUDA version requirements for a runtime
func (r *Resolver) GetCUDARequirements(runtimeName string) (string, error) {
	// Try to load runtime config first to get precise requirements
	if config, err := r.getRuntimeConfig(runtimeName); err == nil && config != nil {
		// Check if config has CUDA requirements (this would need to be added to RuntimeConfig)
		if config.Environment != nil {
			// Look for CUDA version in environment variables
			if cudaVersion, exists := config.Environment["CUDA_VERSION"]; exists {
				return cudaVersion, nil
			}
			if cudaVersion, exists := config.Environment["CUDA_HOME"]; exists {
				// Extract version from CUDA_HOME path if possible
				return r.extractVersionFromPath(cudaVersion), nil
			}
		}
	}

	// Fallback to inference from runtime name/spec
	return r.inferCUDAVersion(runtimeName), nil
}

// getRuntimeConfig loads runtime configuration for a given runtime name
func (r *Resolver) getRuntimeConfig(runtimeName string) (*RuntimeConfig, error) {
	// Try to find runtime directory
	entries, err := r.platform.ReadDir(r.runtimesPath)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if this runtime matches the name
		if strings.Contains(entry.Name(), runtimeName) || runtimeName == entry.Name() {
			configPath := filepath.Join(r.runtimesPath, entry.Name(), "runtime.yml")
			return r.loadRuntimeConfig(configPath)
		}
	}

	return nil, fmt.Errorf("runtime config not found for %s", runtimeName)
}

// inferCUDAVersion attempts to infer CUDA version from runtime name
func (r *Resolver) inferCUDAVersion(runtimeName string) string {
	lowerName := strings.ToLower(runtimeName)

	// Common CUDA version patterns in runtime names
	cudaVersionMap := map[string]string{
		"cuda12":     "12.0",
		"cuda12.0":   "12.0",
		"cuda12.1":   "12.1",
		"cuda12.2":   "12.2",
		"cuda11":     "11.8",
		"cuda11.8":   "11.8",
		"cuda11.7":   "11.7",
		"cuda11.6":   "11.6",
		"pytorch":    "11.8", // PyTorch typically uses CUDA 11.8
		"tensorflow": "11.8", // TensorFlow typically uses CUDA 11.8
	}

	for pattern, version := range cudaVersionMap {
		if strings.Contains(lowerName, pattern) {
			return version
		}
	}

	// Default to a commonly compatible version
	return "11.8"
}

// extractVersionFromPath extracts version from a path like "/usr/local/cuda-11.8"
func (r *Resolver) extractVersionFromPath(path string) string {
	// Look for version pattern in path
	parts := strings.Split(path, "-")
	for _, part := range parts {
		// Check if this part looks like a version (contains dot and numbers)
		if strings.Contains(part, ".") && r.isVersionString(part) {
			return part
		}
	}

	return "unknown"
}

// isVersionString checks if a string looks like a version number
func (r *Resolver) isVersionString(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) < 2 {
		return false
	}

	// Check if all parts are numeric
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return false
			}
		}
	}

	return true
}

// ListGPUEnabledRuntimes returns all runtimes that support GPU
func (r *Resolver) ListGPUEnabledRuntimes() ([]*RuntimeInfo, error) {
	allRuntimes, err := r.ListRuntimes()
	if err != nil {
		return nil, err
	}

	var gpuRuntimes []*RuntimeInfo
	for _, runtime := range allRuntimes {
		if r.IsGPUEnabled(runtime.Name) {
			gpuRuntimes = append(gpuRuntimes, runtime)
		}
	}

	return gpuRuntimes, nil
}
