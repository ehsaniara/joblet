package gpu

import (
	"fmt"
	"path/filepath"
	"strings"

	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// CUDADetector implements CUDA installation detection
type CUDADetector struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewCUDADetector creates a new CUDA detector
func NewCUDADetector(platform platform.Platform) *CUDADetector {
	return &CUDADetector{
		platform: platform,
		logger:   logger.New().WithField("component", "cuda-detector"),
	}
}

// DetectCUDA finds CUDA installation paths
func (c *CUDADetector) DetectCUDA() ([]string, error) {
	c.logger.Debug("detecting CUDA installations")

	// Common CUDA installation paths (as specified in design doc)
	candidatePaths := []string{
		"/usr/local/cuda",
		"/opt/cuda",
		"/usr/cuda",
		"/usr/local/cuda-12.0", // Versioned paths
		"/usr/local/cuda-11.8",
		"/usr/local/cuda-11.0",
	}

	// Also check CUDA_HOME environment variable
	if cudaHome := c.platform.Getenv("CUDA_HOME"); cudaHome != "" {
		candidatePaths = append([]string{cudaHome}, candidatePaths...)
	}

	var validPaths []string

	for _, path := range candidatePaths {
		if c.isValidCUDAInstallation(path) {
			validPaths = append(validPaths, path)
			c.logger.Debug("found valid CUDA installation", "path", path)
		}
	}

	if len(validPaths) == 0 {
		return nil, fmt.Errorf("no valid CUDA installations found")
	}

	c.logger.Info("detected CUDA installations", "paths", validPaths)
	return validPaths, nil
}

// isValidCUDAInstallation checks if a path contains a valid CUDA installation
func (c *CUDADetector) isValidCUDAInstallation(path string) bool {
	// Check for essential CUDA directories and files
	essentialPaths := []string{
		filepath.Join(path, "lib64"),       // Library directory
		filepath.Join(path, "include"),     // Header files
		filepath.Join(path, "bin", "nvcc"), // CUDA compiler
	}

	for _, essentialPath := range essentialPaths {
		if _, err := c.platform.Stat(essentialPath); err != nil {
			c.logger.Debug("missing essential CUDA path", "path", essentialPath, "cudaRoot", path)
			return false
		}
	}

	// Additional validation: check for libcudart
	libPaths := []string{
		filepath.Join(path, "lib64", "libcudart.so"),
		filepath.Join(path, "lib", "libcudart.so"),
	}

	for _, libPath := range libPaths {
		if _, err := c.platform.Stat(libPath); err == nil {
			return true
		}
	}

	c.logger.Debug("could not find libcudart.so", "cudaRoot", path)
	return false
}

// GetCUDAEnvironment returns environment variables needed for CUDA
func (c *CUDADetector) GetCUDAEnvironment(cudaPath string) map[string]string {
	env := make(map[string]string)

	// Set CUDA_HOME
	env["CUDA_HOME"] = cudaPath

	// Build LD_LIBRARY_PATH
	libPaths := []string{
		filepath.Join(cudaPath, "lib64"),
		filepath.Join(cudaPath, "lib"),
	}

	// Check which lib paths exist and build LD_LIBRARY_PATH
	var existingLibPaths []string
	for _, libPath := range libPaths {
		if _, err := c.platform.Stat(libPath); err == nil {
			existingLibPaths = append(existingLibPaths, libPath)
		}
	}

	if len(existingLibPaths) > 0 {
		env["LD_LIBRARY_PATH"] = strings.Join(existingLibPaths, ":")
	}

	// Set PATH to include CUDA binaries
	binPath := filepath.Join(cudaPath, "bin")
	if _, err := c.platform.Stat(binPath); err == nil {
		env["PATH"] = binPath // Will be prepended to existing PATH by job executor
	}

	// Set CUDA_VISIBLE_DEVICES (will be set by GPU manager based on allocation)
	// env["CUDA_VISIBLE_DEVICES"] = "0,1,2,3" // This will be set dynamically

	c.logger.Debug("generated CUDA environment",
		"cudaPath", cudaPath,
		"environment", env)

	return env
}

// GetCUDAMountPaths returns paths that should be mounted for CUDA support
func (c *CUDADetector) GetCUDAMountPaths(cudaPath string) ([]string, error) {
	mountPaths := []string{}

	// Essential CUDA directories to mount
	candidatePaths := []string{
		filepath.Join(cudaPath, "lib64"),
		filepath.Join(cudaPath, "lib"),
		filepath.Join(cudaPath, "include"),
		filepath.Join(cudaPath, "bin"),
		filepath.Join(cudaPath, "extras"), // For additional tools
	}

	for _, path := range candidatePaths {
		if _, err := c.platform.Stat(path); err == nil {
			mountPaths = append(mountPaths, path)
		}
	}

	if len(mountPaths) == 0 {
		return nil, fmt.Errorf("no valid CUDA paths found for mounting in %s", cudaPath)
	}

	c.logger.Debug("identified CUDA mount paths",
		"cudaRoot", cudaPath,
		"mountPaths", mountPaths)

	return mountPaths, nil
}
