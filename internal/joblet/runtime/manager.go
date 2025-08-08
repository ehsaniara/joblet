package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// Manager handles runtime mounting and environment setup for jobs
type Manager struct {
	resolver *Resolver
	platform platform.Platform
	logger   *logger.Logger
}

// NewManager creates a new runtime manager
func NewManager(runtimesPath string, platform platform.Platform) *Manager {
	return &Manager{
		resolver: NewResolver(runtimesPath, platform),
		platform: platform,
		logger:   logger.New().WithField("component", "runtime-manager"),
	}
}

// ResolveRuntimeConfig resolves runtime configuration without mounting
func (m *Manager) ResolveRuntimeConfig(runtimeSpec string) (*RuntimeConfig, error) {
	if runtimeSpec == "" {
		return nil, nil // No runtime requested
	}

	log := m.logger.WithField("runtime", runtimeSpec)
	log.Debug("resolving runtime configuration")

	// Resolve runtime configuration
	config, err := m.resolver.ResolveRuntime(runtimeSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve runtime: %w", err)
	}

	log.Info("runtime configuration resolved", "runtime", runtimeSpec)
	return config, nil
}

// MountRuntimeConfig mounts a previously resolved runtime configuration
func (m *Manager) MountRuntimeConfig(jobRootDir string, config *RuntimeConfig) error {
	if config == nil {
		return nil // No runtime to mount
	}

	log := m.logger.WithField("runtime", config.Name)
	log.Debug("mounting runtime configuration")

	// Mount runtime directories
	if err := m.mountRuntime(jobRootDir, config); err != nil {
		return fmt.Errorf("failed to mount runtime: %w", err)
	}

	log.Info("runtime mounted successfully", "runtime", config.Name)
	return nil
}

// SetupRuntime sets up the runtime for a job
func (m *Manager) SetupRuntime(jobRootDir string, runtimeSpec string, volumes []string) (*RuntimeConfig, error) {
	// First resolve configuration
	config, err := m.ResolveRuntimeConfig(runtimeSpec)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, nil // No runtime requested
	}

	log := m.logger.WithField("runtime", runtimeSpec)
	log.Debug("setting up runtime for job")

	// Mount runtime directories
	if err := m.mountRuntime(jobRootDir, config); err != nil {
		return nil, fmt.Errorf("failed to mount runtime: %w", err)
	}

	// Setup package manager volumes if configured
	if config.PackageManager != nil {
		if err := m.setupPackageManagerVolumes(jobRootDir, config.PackageManager, volumes); err != nil {
			return nil, fmt.Errorf("failed to setup package manager volumes: %w", err)
		}
	}

	log.Info("runtime setup complete", "name", config.Name)
	return config, nil
}

// mountRuntime mounts the runtime directories into the job filesystem
func (m *Manager) mountRuntime(jobRootDir string, config *RuntimeConfig) error {
	log := m.logger.WithField("runtime", config.Name)

	for _, mount := range config.Mounts {
		targetPath := filepath.Join(jobRootDir, strings.TrimPrefix(mount.Target, "/"))

		// Create mount point directory
		targetDir := targetPath
		if len(mount.Selective) > 0 {
			// For selective mounts, target is a directory
			targetDir = filepath.Dir(targetPath)
		}

		if err := m.platform.MkdirAll(targetDir, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", targetDir, err)
		}

		// Handle selective file mounting
		if len(mount.Selective) > 0 {
			for _, file := range mount.Selective {
				sourcePath := filepath.Join(mount.Source, file)
				targetFile := filepath.Join(targetPath, file)

				if err := m.mountPath(sourcePath, targetFile, mount.ReadOnly); err != nil {
					return fmt.Errorf("failed to mount %s: %w", file, err)
				}
			}
		} else {
			// Mount entire directory
			if err := m.mountPath(mount.Source, targetPath, mount.ReadOnly); err != nil {
				return fmt.Errorf("failed to mount %s to %s: %w", mount.Source, mount.Target, err)
			}
		}

		log.Debug("mounted runtime path", "source", mount.Source, "target", mount.Target)
	}

	return nil
}

// mountPath performs a bind mount of a single path
func (m *Manager) mountPath(source, target string, readOnly bool) error {
	// Check if source exists
	sourceInfo, err := m.platform.Stat(source)
	if err != nil {
		return fmt.Errorf("source path does not exist: %w", err)
	}

	// Create target based on source type
	if sourceInfo.IsDir() {
		if e := m.platform.MkdirAll(target, 0755); e != nil {
			return fmt.Errorf("failed to create target directory: %w", e)
		}
	} else {
		// For files, ensure parent directory exists
		parentDir := filepath.Dir(target)
		if err := m.platform.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
		// Create empty file for mount point
		file, err := m.platform.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to create target file: %w", err)
		}
		file.Close()
	}

	// Prepare mount flags
	flags := syscall.MS_BIND
	if readOnly {
		flags |= syscall.MS_RDONLY
	}

	// Perform bind mount
	if err := m.platform.Mount(source, target, "", uintptr(flags), ""); err != nil {
		return fmt.Errorf("bind mount failed: %w", err)
	}

	// For read-only mounts, remount to ensure read-only flag is applied
	if readOnly {
		if err := m.platform.Mount("", target, "", uintptr(syscall.MS_REMOUNT|syscall.MS_BIND|syscall.MS_RDONLY), ""); err != nil {
			m.logger.Warn("failed to remount as read-only", "target", target, "error", err)
		}
	}

	return nil
}

// setupPackageManagerVolumes sets up volumes for package manager caches
func (m *Manager) setupPackageManagerVolumes(jobRootDir string, pmConfig *PackageManagerConfig, volumes []string) error {
	// Check if cache volume is requested and available
	if pmConfig.CacheVolume != "" {
		cacheVolumeFound := false
		for _, vol := range volumes {
			if vol == pmConfig.CacheVolume {
				cacheVolumeFound = true
				break
			}
		}

		if cacheVolumeFound {
			// The volume will be mounted by the volume manager
			// We just need to create symlinks or environment variables
			m.logger.Debug("package manager cache volume configured", "volume", pmConfig.CacheVolume)
		}
	}

	// Setup user packages volume if configured
	if pmConfig.UserPackagesVolume != "" {
		userVolumeFound := false
		for _, vol := range volumes {
			if vol == pmConfig.UserPackagesVolume {
				userVolumeFound = true
				break
			}
		}

		if userVolumeFound {
			m.logger.Debug("user packages volume configured", "volume", pmConfig.UserPackagesVolume)
		}
	}

	return nil
}

// GetEnvironmentVariables returns the environment variables for a runtime
func (m *Manager) GetEnvironmentVariables(config *RuntimeConfig) map[string]string {
	if config == nil {
		return nil
	}

	env := make(map[string]string)

	// Copy runtime environment variables
	for key, value := range config.Environment {
		env[key] = value
	}

	// Handle PATH_PREPEND specially
	if pathPrepend, ok := env["PATH_PREPEND"]; ok {
		// This will be handled by the execution engine to prepend to existing PATH
		delete(env, "PATH_PREPEND")
		env["RUNTIME_PATH_PREPEND"] = pathPrepend
	}

	return env
}

// CleanupRuntime unmounts runtime directories
func (m *Manager) CleanupRuntime(jobRootDir string, config *RuntimeConfig) error {
	if config == nil {
		return nil
	}

	log := m.logger.WithField("runtime", config.Name)
	log.Debug("cleaning up runtime mounts")

	var unmountErrors []error

	// Unmount in reverse order
	for i := len(config.Mounts) - 1; i >= 0; i-- {
		mount := config.Mounts[i]
		targetPath := filepath.Join(jobRootDir, strings.TrimPrefix(mount.Target, "/"))

		if len(mount.Selective) > 0 {
			// Unmount selective files
			for _, file := range mount.Selective {
				targetFile := filepath.Join(targetPath, file)
				if err := syscall.Unmount(targetFile, 0); err != nil {
					log.Debug("failed to unmount file", "path", targetFile, "error", err)
					unmountErrors = append(unmountErrors, fmt.Errorf("unmount %s: %w", targetFile, err))
				}
			}
		} else {
			// Unmount directory
			if err := syscall.Unmount(targetPath, 0); err != nil {
				log.Debug("failed to unmount directory", "path", targetPath, "error", err)
				unmountErrors = append(unmountErrors, fmt.Errorf("unmount %s: %w", targetPath, err))
			}
		}
	}

	// Log summary of unmount errors but don't fail cleanup
	if len(unmountErrors) > 0 {
		log.Warn("encountered unmount errors during cleanup", "errorCount", len(unmountErrors))
	}

	return nil
}
