package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"joblet/pkg/logger"
	"joblet/pkg/platform"

	"gopkg.in/yaml.v3"
)

// Resolver handles runtime resolution and configuration loading
type Resolver struct {
	runtimesPath string
	platform     platform.Platform
	logger       *logger.Logger
}

// NewResolver creates a new runtime resolver
func NewResolver(runtimesPath string, platform platform.Platform) *Resolver {
	return &Resolver{
		runtimesPath: runtimesPath,
		platform:     platform,
		logger:       logger.New().WithField("component", "runtime-resolver"),
	}
}

// ResolveRuntime resolves a runtime specification to a runtime configuration
func (r *Resolver) ResolveRuntime(spec string) (*RuntimeConfig, error) {
	if spec == "" {
		return nil, nil // No runtime specified
	}

	parsedSpec := r.parseRuntimeSpec(spec)

	// Find matching runtime directory
	runtimeDir, err := r.findRuntimeDirectory(parsedSpec)
	if err != nil {
		return nil, fmt.Errorf("runtime not found: %w", err)
	}

	// Load runtime configuration
	configPath := filepath.Join(runtimeDir, "runtime.yml")
	config, err := r.loadRuntimeConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load runtime config: %w", err)
	}

	// Validate system compatibility
	if err := r.validateRuntime(config); err != nil {
		return nil, fmt.Errorf("runtime validation failed: %w", err)
	}

	return config, nil
}

// parseRuntimeSpec parses a runtime specification string
// Supports two formats:
// 1. Traditional format: language:version[+tag1][+tag2]... (e.g., "python:3.11", "python:3.11+ml+gpu")
// 2. Runtime name format: language-version-tag1-tag2 (e.g., "python-3.11-ml")
func (r *Resolver) parseRuntimeSpec(spec string) *RuntimeSpec {
	// Traditional format with colon separator
	if strings.Contains(spec, ":") {
		parts := strings.Split(spec, ":")
		if len(parts) != 2 {
			return &RuntimeSpec{
				Language: spec,
				Version:  "latest",
				Tags:     []string{},
			}
		}

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

	// Runtime name format: parse language-version-tags
	return r.parseRuntimeName(spec)
}

// parseRuntimeName parses runtime name format (e.g., "python-3.11-ml")
func (r *Resolver) parseRuntimeName(spec string) *RuntimeSpec {
	parts := strings.Split(spec, "-")
	if len(parts) < 2 {
		// If no hyphens, treat entire string as language with default version
		return &RuntimeSpec{
			Language: spec,
			Version:  "latest",
			Tags:     []string{},
		}
	}

	// First part is language
	language := parts[0]

	// Try to identify version part (usually numeric like "3.11", "17", "1.20")
	versionIdx := -1
	for i := 1; i < len(parts); i++ {
		part := parts[i]
		// Check if this part looks like a version (contains digits and dots)
		if r.isVersionLike(part) {
			versionIdx = i
			break
		}
	}

	var version string
	var tags []string

	if versionIdx >= 0 {
		// Found version part
		version = parts[versionIdx]
		// Everything before version (except language) is part of language
		if versionIdx > 1 {
			language = strings.Join(parts[0:versionIdx], "-")
		}
		// Everything after version is tags
		if versionIdx+1 < len(parts) {
			tags = parts[versionIdx+1:]
		}
	} else {
		// No clear version found, last part is version, rest are tags
		version = parts[len(parts)-1]
		if len(parts) > 2 {
			tags = parts[1 : len(parts)-1]
		}
	}

	return &RuntimeSpec{
		Language: language,
		Version:  version,
		Tags:     tags,
	}
}

// isVersionLike checks if a string looks like a version number
func (r *Resolver) isVersionLike(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Must contain at least one digit
	hasDigit := false
	for _, char := range s {
		if char >= '0' && char <= '9' {
			hasDigit = true
		} else if char != '.' {
			// If it contains anything other than digits and dots, it's probably not a version
			return false
		}
	}

	return hasDigit
}

// findRuntimeDirectory finds the directory for a runtime specification
func (r *Resolver) findRuntimeDirectory(spec *RuntimeSpec) (string, error) {
	// Build potential directory names
	baseName := fmt.Sprintf("%s-%s", spec.Language, spec.Version)

	// Check with tags
	if len(spec.Tags) > 0 {
		fullName := fmt.Sprintf("%s-%s", baseName, strings.Join(spec.Tags, "-"))
		fullPath := filepath.Join(r.runtimesPath, spec.Language, fullName)
		if r.platform.DirExists(fullPath) {
			return fullPath, nil
		}
	}

	// Check without tags
	basePath := filepath.Join(r.runtimesPath, spec.Language, baseName)
	if r.platform.DirExists(basePath) {
		return basePath, nil
	}

	// Try version-only directory (e.g., "openjdk-17" for "java:17")
	versionPath := filepath.Join(r.runtimesPath, spec.Language, fmt.Sprintf("%s-%s", getBaseLanguageName(spec.Language), spec.Version))
	if r.platform.DirExists(versionPath) {
		return versionPath, nil
	}

	return "", fmt.Errorf("runtime directory not found for %s:%s", spec.Language, spec.Version)
}

// getBaseLanguageName returns the base name for common language runtimes
func getBaseLanguageName(language string) string {
	switch language {
	case "java":
		return "openjdk"
	case "node":
		return "node"
	case "python":
		return "python"
	case "go":
		return "go"
	default:
		return language
	}
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

	// Set absolute paths for mounts
	runtimeDir := filepath.Dir(configPath)
	for i := range config.Mounts {
		if !filepath.IsAbs(config.Mounts[i].Source) {
			config.Mounts[i].Source = filepath.Join(runtimeDir, config.Mounts[i].Source)
		}
	}

	return &config, nil
}

// validateRuntime validates runtime compatibility with the system
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

	// Check GPU requirements
	if config.Requirements.GPU {
		// Check for NVIDIA GPU presence
		if !r.checkGPUAvailable() {
			r.logger.Warn("runtime requires GPU but none detected")
		}
	}

	// Validate mount points exist
	for _, mount := range config.Mounts {
		if !r.platform.DirExists(mount.Source) && !r.platform.FileExists(mount.Source) {
			return fmt.Errorf("mount source does not exist: %s", mount.Source)
		}
	}

	return nil
}

// checkGPUAvailable checks if a GPU is available on the system
func (r *Resolver) checkGPUAvailable() bool {
	// Check for NVIDIA GPU devices
	nvidiaDevices := []string{"/dev/nvidia0", "/dev/nvidiactl", "/dev/nvidia-uvm"}
	for _, device := range nvidiaDevices {
		if _, err := os.Stat(device); err == nil {
			return true
		}
	}

	// Check for AMD GPU devices
	amdDevices := []string{"/dev/dri/card0", "/dev/kfd"}
	amdCount := 0
	for _, device := range amdDevices {
		if _, err := os.Stat(device); err == nil {
			amdCount++
		}
	}
	if amdCount > 0 {
		return true
	}

	// Check for nvidia-smi command
	if cmd := r.platform.CommandContext(nil, "nvidia-smi", "--list-gpus"); cmd.Run() == nil {
		return true
	}

	// Check for rocm-smi command (AMD)
	if cmd := r.platform.CommandContext(nil, "rocm-smi", "--showid"); cmd.Run() == nil {
		return true
	}

	return false
}

// ListRuntimes lists all available runtimes
func (r *Resolver) ListRuntimes() ([]*RuntimeInfo, error) {
	var runtimes []*RuntimeInfo

	// Check if runtimes directory exists
	if !r.platform.DirExists(r.runtimesPath) {
		return runtimes, nil
	}

	// List language directories
	languages, err := r.platform.ReadDir(r.runtimesPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read runtimes directory: %w", err)
	}

	for _, langDir := range languages {
		if !langDir.IsDir() {
			continue
		}

		langPath := filepath.Join(r.runtimesPath, langDir.Name())
		versions, err := r.platform.ReadDir(langPath)
		if err != nil {
			r.logger.Warn("failed to read language directory", "path", langPath, "error", err)
			continue
		}

		for _, versionDir := range versions {
			if !versionDir.IsDir() {
				continue
			}

			versionPath := filepath.Join(langPath, versionDir.Name())
			configPath := filepath.Join(versionPath, "runtime.yml")

			// Load runtime config
			config, err := r.loadRuntimeConfig(configPath)
			if err != nil {
				r.logger.Debug("skipping runtime without valid config", "path", versionPath)
				continue
			}

			// Get directory size
			size, _ := r.getDirectorySize(versionPath)

			info := &RuntimeInfo{
				Name:        config.Name,
				Language:    langDir.Name(),
				Version:     config.Version,
				Description: config.Description,
				Path:        versionPath,
				Size:        size,
				Available:   true,
			}

			runtimes = append(runtimes, info)
		}
	}

	return runtimes, nil
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
