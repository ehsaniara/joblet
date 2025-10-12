//go:build linux

package core

import (
	"fmt"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
	"os"
	"path/filepath"
	"strings"
)

// RuntimeCleanup handles post-installation cleanup of runtimes to ensure
// they are completely isolated and don't expose host OS filesystem to production jobs
type RuntimeCleanup struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewRuntimeCleanup creates a new runtime cleanup processor
func NewRuntimeCleanup(platform platform.Platform) *RuntimeCleanup {
	return &RuntimeCleanup{
		platform: platform,
		logger:   logger.New().WithField("component", "runtime-cleanup"),
	}
}

// CleanupRuntime processes a newly built runtime to create an isolated,
// self-contained runtime directory that doesn't depend on host OS paths
func (rc *RuntimeCleanup) CleanupRuntime(runtimeDir string) error {
	log := rc.logger.WithField("runtimeDir", runtimeDir)
	log.Info("starting runtime cleanup for production isolation")

	// Read the runtime.yml to understand what needs to be extracted
	runtimeYml := filepath.Join(runtimeDir, "runtime.yml")
	config, err := rc.parseRuntimeConfig(runtimeYml)
	if err != nil {
		return fmt.Errorf("failed to parse runtime config: %w", err)
	}

	// Use generic cleanup for all runtimes - no language-specific logic needed
	log.Debug("performing generic runtime cleanup", "name", config.Name)
	return rc.cleanupGenericRuntime(runtimeDir, config)
}

// RuntimeConfig represents the parsed runtime.yml structure
type RuntimeConfig struct {
	Name        string            `yaml:"name"`
	Language    string            `yaml:"language"`
	Version     string            `yaml:"version"`
	Mounts      []RuntimeMount    `yaml:"mounts"`
	Environment map[string]string `yaml:"environment"`
}

// RuntimeMount represents a mount configuration
type RuntimeMount struct {
	Source   string `yaml:"source"`
	Target   string `yaml:"target"`
	ReadOnly bool   `yaml:"readonly"`
}

// parseRuntimeConfig parses the runtime.yml file (simple parser)
func (rc *RuntimeCleanup) parseRuntimeConfig(configPath string) (*RuntimeConfig, error) {
	data, err := rc.platform.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := &RuntimeConfig{
		Mounts:      []RuntimeMount{},
		Environment: make(map[string]string),
	}

	lines := strings.Split(string(data), "\n")
	var currentMount *RuntimeMount
	inMounts := false
	inEnvironment := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse basic fields
		if strings.HasPrefix(line, "name:") {
			config.Name = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.HasPrefix(line, "language:") {
			config.Language = strings.TrimSpace(strings.Split(line, ":")[1])
		} else if strings.HasPrefix(line, "version:") {
			config.Version = strings.Trim(strings.TrimSpace(strings.Split(line, ":")[1]), `"`)
		} else if strings.HasPrefix(line, "mounts:") {
			inMounts = true
			inEnvironment = false
		} else if strings.HasPrefix(line, "environment:") {
			inEnvironment = true
			inMounts = false
		} else if inMounts && strings.HasPrefix(line, "- source:") {
			if currentMount != nil {
				config.Mounts = append(config.Mounts, *currentMount)
			}
			currentMount = &RuntimeMount{}
			currentMount.Source = strings.Trim(strings.TrimSpace(strings.Split(line, ":")[1]), `"`)
		} else if inMounts && currentMount != nil && strings.HasPrefix(line, "target:") {
			currentMount.Target = strings.Trim(strings.TrimSpace(strings.Split(line, ":")[1]), `"`)
		} else if inMounts && currentMount != nil && strings.HasPrefix(line, "readonly:") {
			currentMount.ReadOnly = strings.Contains(line, "true")
		} else if inEnvironment && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
			config.Environment[key] = value
		} else if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "-") {
			// End of current section
			if currentMount != nil && inMounts {
				config.Mounts = append(config.Mounts, *currentMount)
				currentMount = nil
			}
			inMounts = false
			inEnvironment = false
		}
	}

	// Don't forget the last mount
	if currentMount != nil {
		config.Mounts = append(config.Mounts, *currentMount)
	}

	return config, nil
}

// cleanupGenericRuntime creates an isolated runtime structure (generic for all runtimes)
func (rc *RuntimeCleanup) cleanupGenericRuntime(runtimeDir string, config *RuntimeConfig) error {
	log := rc.logger.WithField("runtime", config.Name)
	log.Info("cleaning up runtime for production isolation", "type", "generic")

	// Create isolated directory structure
	isolatedDir := filepath.Join(runtimeDir, "isolated")
	if err := rc.platform.MkdirAll(isolatedDir, 0755); err != nil {
		return fmt.Errorf("failed to create isolated directory: %w", err)
	}

	// Process each mount and copy necessary files to isolated structure
	var newMounts []RuntimeMount
	for _, mount := range config.Mounts {
		newMount, err := rc.processGenericMount(mount, isolatedDir)
		if err != nil {
			log.Warn("failed to process mount", "mount", mount.Source, "error", err)
			continue
		}
		newMounts = append(newMounts, newMount)
	}

	// Generate new runtime.yml with isolated paths
	return rc.generateCleanedRuntimeConfig(runtimeDir, config, newMounts)
}

// processGenericMount processes a runtime mount by copying files to isolated structure (generic for all runtimes)
func (rc *RuntimeCleanup) processGenericMount(mount RuntimeMount, isolatedDir string) (RuntimeMount, error) {
	sourcePath := "/" + strings.TrimPrefix(mount.Source, "/")

	// Create relative path within isolated directory
	relativePath := strings.TrimPrefix(mount.Source, "/")
	isolatedPath := filepath.Join(isolatedDir, relativePath)

	// Create parent directories
	if err := rc.platform.MkdirAll(filepath.Dir(isolatedPath), 0755); err != nil {
		return RuntimeMount{}, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Copy the source to isolated location
	if err := rc.copyPath(sourcePath, isolatedPath); err != nil {
		return RuntimeMount{}, fmt.Errorf("failed to copy %s to isolated location: %w", sourcePath, err)
	}

	// Return new mount configuration pointing to isolated location
	newMount := RuntimeMount{
		Source:   "isolated/" + relativePath,
		Target:   mount.Target,
		ReadOnly: mount.ReadOnly,
	}

	rc.logger.Debug("processed runtime mount",
		"original", mount.Source,
		"isolated", newMount.Source,
		"target", mount.Target)

	return newMount, nil
}

// copyPath recursively copies a file or directory
func (rc *RuntimeCleanup) copyPath(src, dst string) error {
	srcInfo, err := rc.platform.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() {
		return rc.copyDir(src, dst, srcInfo)
	} else {
		return rc.copyFile(src, dst, srcInfo)
	}
}

// copyDir recursively copies a directory
func (rc *RuntimeCleanup) copyDir(src, dst string, srcInfo os.FileInfo) error {
	if err := rc.platform.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := rc.platform.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if err := rc.copyPath(srcPath, dstPath); err != nil {
			return err
		}
	}

	return nil
}

// copyFile copies a single file
func (rc *RuntimeCleanup) copyFile(src, dst string, srcInfo os.FileInfo) error {
	srcFile, err := rc.platform.ReadFile(src)
	if err != nil {
		return err
	}

	if err := rc.platform.WriteFile(dst, srcFile, srcInfo.Mode()); err != nil {
		return err
	}

	return nil
}

// generateCleanedRuntimeConfig creates a new runtime.yml with isolated paths
func (rc *RuntimeCleanup) generateCleanedRuntimeConfig(runtimeDir string, config *RuntimeConfig, newMounts []RuntimeMount) error {
	cleanedConfigPath := filepath.Join(runtimeDir, "runtime.yml")

	// Backup original config
	originalConfigPath := filepath.Join(runtimeDir, "runtime.yml.original")
	if err := rc.copyPath(cleanedConfigPath, originalConfigPath); err != nil {
		rc.logger.Warn("failed to backup original runtime config", "error", err)
	}

	// Generate new runtime.yml with isolated paths
	var configContent strings.Builder

	configContent.WriteString(fmt.Sprintf("name: %s\n", config.Name))
	configContent.WriteString(fmt.Sprintf("language: %s\n", config.Language))
	configContent.WriteString(fmt.Sprintf("version: \"%s\"\n", config.Version))
	configContent.WriteString("description: \"Isolated runtime for production jobs (processed by cleanup)\"\n")
	configContent.WriteString("\n")

	// Write isolated mounts
	configContent.WriteString("mounts:\n")
	for _, mount := range newMounts {
		configContent.WriteString(fmt.Sprintf("  - source: \"%s\"\n", mount.Source))
		configContent.WriteString(fmt.Sprintf("    target: \"%s\"\n", mount.Target))
		configContent.WriteString(fmt.Sprintf("    readonly: %t\n", mount.ReadOnly))
	}
	configContent.WriteString("\n")

	// Write environment variables (unchanged)
	if len(config.Environment) > 0 {
		configContent.WriteString("environment:\n")
		for key, value := range config.Environment {
			configContent.WriteString(fmt.Sprintf("  %s: \"%s\"\n", key, value))
		}
	}

	// Write the cleaned configuration
	if err := rc.platform.WriteFile(cleanedConfigPath, []byte(configContent.String()), 0644); err != nil {
		return fmt.Errorf("failed to write cleaned runtime config: %w", err)
	}

	rc.logger.Info("runtime cleanup completed successfully",
		"runtime", config.Name,
		"mountsProcessed", len(newMounts),
		"originalConfig", "runtime.yml.original")

	return nil
}
