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
// Supports nested version structure: /opt/joblet/runtimes/<name>/<version>/
func (r *Resolver) ListRuntimes() ([]*RuntimeInfo, error) {
	var runtimes []*RuntimeInfo

	// Check if runtimes directory exists
	if !r.platform.DirExists(r.runtimesPath) {
		return runtimes, nil
	}

	// List runtime name directories (e.g., python-3.11, openjdk-21)
	entries, err := r.platform.ReadDir(r.runtimesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read runtimes directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runtimeNameDir := filepath.Join(r.runtimesPath, entry.Name())

		// Check if this is a version directory structure (has subdirectories with runtime.yml)
		versionEntries, err := r.platform.ReadDir(runtimeNameDir)
		if err != nil {
			r.logger.Debug("failed to read runtime name directory", "path", runtimeNameDir, "error", err)
			continue
		}

		hasVersionDirs := false
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}

			versionPath := filepath.Join(runtimeNameDir, versionEntry.Name())
			configPath := filepath.Join(versionPath, "runtime.yml")

			// Check if this version directory has runtime.yml
			if _, err := r.platform.Stat(configPath); err == nil {
				hasVersionDirs = true

				// Load runtime config
				config, err := r.loadRuntimeConfig(configPath)
				if err != nil {
					r.logger.Debug("skipping version without valid config", "path", versionPath, "error", err)
					continue
				}

				// Get directory size
				size, _ := r.getDirectorySize(versionPath)

				// Get language/type from runtime.yml
				runtimeType := config.Language
				if runtimeType == "" {
					runtimeType = r.extractTypeFromName(entry.Name())
				}

				info := &RuntimeInfo{
					Name:        config.Name,
					Language:    runtimeType,
					Version:     config.Version,
					Description: config.Description,
					Path:        versionPath,
					Size:        size,
					Available:   true,
				}

				runtimes = append(runtimes, info)
			}
		}

		// Fallback: if no version directories found, check for flat structure (backward compatibility)
		if !hasVersionDirs {
			configPath := filepath.Join(runtimeNameDir, "runtime.yml")
			if _, err := r.platform.Stat(configPath); err == nil {
				config, err := r.loadRuntimeConfig(configPath)
				if err != nil {
					r.logger.Debug("skipping runtime without valid config", "path", runtimeNameDir, "error", err)
					continue
				}

				size, _ := r.getDirectorySize(runtimeNameDir)
				runtimeType := config.Language
				if runtimeType == "" {
					runtimeType = r.extractTypeFromName(entry.Name())
				}

				info := &RuntimeInfo{
					Name:        config.Name,
					Language:    runtimeType,
					Version:     config.Version,
					Description: config.Description,
					Path:        runtimeNameDir,
					Size:        size,
					Available:   true,
				}

				runtimes = append(runtimes, info)
			}
		}
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
	runtimeDir, err := r.FindRuntimeDirectory(runtimeSpec)
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

// FindRuntimeDirectory finds the directory for a runtime specification (fully generic)
// Supports nested version structure: /opt/joblet/runtimes/<name>/<version>/
// This is exported for use by the filesystem isolator
func (r *Resolver) FindRuntimeDirectory(spec string) (string, error) {
	// Scan all runtime directories and check their runtime.yml files
	entries, err := r.platform.ReadDir(r.runtimesPath)
	if err != nil {
		return "", fmt.Errorf("failed to read runtimes directory: %w", err)
	}

	// First pass: try exact name match in nested structure
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runtimeNameDir := filepath.Join(r.runtimesPath, entry.Name())

		// Check for version subdirectories
		versionEntries, err := r.platform.ReadDir(runtimeNameDir)
		if err != nil {
			continue
		}

		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}

			versionPath := filepath.Join(runtimeNameDir, versionEntry.Name())
			configPath := filepath.Join(versionPath, "runtime.yml")

			// Load runtime config to check if it matches the spec
			config, err := r.loadRuntimeConfig(configPath)
			if err != nil {
				continue // Skip directories without valid runtime.yml
			}

			// Check if the runtime name matches exactly
			if config.Name == spec {
				return versionPath, nil
			}
		}

		// Also check flat structure (backward compatibility)
		flatConfigPath := filepath.Join(runtimeNameDir, "runtime.yml")
		config, err := r.loadRuntimeConfig(flatConfigPath)
		if err == nil && config.Name == spec {
			return runtimeNameDir, nil
		}
	}

	// Second pass: try parsed spec matching for more complex queries
	parsedSpec := r.parseRuntimeSpec(spec)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runtimeNameDir := filepath.Join(r.runtimesPath, entry.Name())

		// Check for version subdirectories
		versionEntries, err := r.platform.ReadDir(runtimeNameDir)
		if err != nil {
			continue
		}

		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}

			versionPath := filepath.Join(runtimeNameDir, versionEntry.Name())
			configPath := filepath.Join(versionPath, "runtime.yml")

			// Load runtime config to check if it matches the spec
			config, err := r.loadRuntimeConfig(configPath)
			if err != nil {
				continue // Skip directories without valid runtime.yml
			}

			// Check if this runtime matches the requested spec
			if r.runtimeMatches(config, parsedSpec) {
				return versionPath, nil
			}
		}

		// Also check flat structure (backward compatibility)
		flatConfigPath := filepath.Join(runtimeNameDir, "runtime.yml")
		config, err := r.loadRuntimeConfig(flatConfigPath)
		if err == nil && r.runtimeMatches(config, parsedSpec) {
			return runtimeNameDir, nil
		}
	}

	return "", fmt.Errorf("runtime not found for spec %s", spec)
}

// runtimeMatches checks if a runtime config matches a runtime specification
func (r *Resolver) runtimeMatches(config *RuntimeConfig, spec *RuntimeSpec) bool {
	// Match by runtime name (for @ notation specs like python-3.11@1.3.1)
	// The spec.Language field may contain the full runtime name when using @ notation
	if spec.Language != "" && spec.Language != "unknown" {
		// Try exact match against runtime name first
		if config.Name == spec.Language {
			// Name matches, now check version if specified
			if spec.Version != "" && spec.Version != "unknown" && spec.Version != "latest" {
				if config.Version != spec.Version {
					return false
				}
			}
			return true
		}

		// Also try matching against language/type field for backward compatibility
		if config.Language != "" && spec.Language == config.Language {
			// Language matches, check version if specified
			if spec.Version != "" && spec.Version != "unknown" && spec.Version != "latest" {
				if config.Version != spec.Version {
					return false
				}
			}
			return true
		}

		// No match
		return false
	}

	// If no language specified in spec, just match by version
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
	// Handle @ notation for versioned runtimes (e.g., "python-3.11@1.3.1", "python-3.11-ml@1.0.0")
	// This is the npm-style package@version notation for the runtime registry
	if strings.Contains(spec, "@") {
		parts := strings.SplitN(spec, "@", 2)
		if len(parts) == 2 {
			runtimeName := parts[0]
			version := parts[1]

			// The runtime name might be something like "python-3.11-ml"
			// We want to match against config.Name which will be the same
			// Return a spec that will match by name and version
			return &RuntimeSpec{
				Language: runtimeName, // Use full name as language for exact matching
				Version:  version,
				Tags:     nil,
			}
		}
	}

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
