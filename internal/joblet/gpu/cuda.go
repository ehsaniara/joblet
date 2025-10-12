package gpu

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// CUDAVersion represents a CUDA version
type CUDAVersion struct {
	Major int    // Major version (e.g., 12 in "12.1")
	Minor int    // Minor version (e.g., 1 in "12.1")
	Path  string // Installation path
}

// String returns the version as a string
func (v CUDAVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// IsCompatible checks if this version is compatible with a required version
// Returns true if this version >= required version
func (v CUDAVersion) IsCompatible(required CUDAVersion) bool {
	if v.Major > required.Major {
		return true
	}
	if v.Major == required.Major && v.Minor >= required.Minor {
		return true
	}
	return false
}

// CUDAInstallation represents a detected CUDA installation
type CUDAInstallation struct {
	Path          string      // Installation path
	Version       CUDAVersion // Detected version
	DriverVersion string      // Driver version (if available)
	IsDefault     bool        // Whether this is the default installation
}

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

// DetectCUDAInstallations finds all CUDA installations with version information
func (c *CUDADetector) DetectCUDAInstallations() ([]CUDAInstallation, error) {
	c.logger.Debug("detecting CUDA installations with version information")

	installations, err := c.findAllCUDAInstallations()
	if err != nil {
		return nil, err
	}

	if len(installations) == 0 {
		return nil, fmt.Errorf("no valid CUDA installations found")
	}

	// Mark the default installation (first valid one or CUDA_HOME)
	c.markDefaultInstallation(installations)

	c.logger.Info("detected CUDA installations", "count", len(installations))
	for _, install := range installations {
		c.logger.Info("found CUDA installation",
			"path", install.Path,
			"version", install.Version.String(),
			"default", install.IsDefault,
			"driverVersion", install.DriverVersion)
	}

	return installations, nil
}

// DetectCUDA finds CUDA installation paths (legacy method for backward compatibility)
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

// findAllCUDAInstallations discovers all CUDA installations and their versions
func (c *CUDADetector) findAllCUDAInstallations() ([]CUDAInstallation, error) {
	var installations []CUDAInstallation

	// Enhanced candidate paths with version-specific searches
	candidatePaths := c.getCandidatePaths()

	for _, path := range candidatePaths {
		if c.isValidCUDAInstallation(path) {
			version, err := c.detectCUDAVersion(path)
			if err != nil {
				c.logger.Warn("failed to detect CUDA version", "path", path, "error", err)
				// Still add it with unknown version
				version = CUDAVersion{Major: 0, Minor: 0, Path: path}
			}

			driverVersion := c.detectDriverVersion()

			installation := CUDAInstallation{
				Path:          path,
				Version:       version,
				DriverVersion: driverVersion,
				IsDefault:     false, // Will be set later
			}

			installations = append(installations, installation)
			c.logger.Debug("found valid CUDA installation",
				"path", path,
				"version", version.String())
		}
	}

	return installations, nil
}

// getCandidatePaths returns all possible CUDA installation paths
func (c *CUDADetector) getCandidatePaths() []string {
	var paths []string

	// Check CUDA_HOME first (highest priority)
	if cudaHome := c.platform.Getenv("CUDA_HOME"); cudaHome != "" {
		paths = append(paths, cudaHome)
	}

	// Common installation paths
	commonPaths := []string{
		"/usr/local/cuda", // Symlink to latest version
		"/opt/cuda",
		"/usr/cuda",
	}
	paths = append(paths, commonPaths...)

	// Version-specific paths (scan for multiple versions)
	versionedPaths := c.findVersionedPaths()
	paths = append(paths, versionedPaths...)

	return paths
}

// findVersionedPaths looks for version-specific CUDA installations
func (c *CUDADetector) findVersionedPaths() []string {
	var paths []string

	// Common version patterns
	basePaths := []string{"/usr/local", "/opt"}

	for _, basePath := range basePaths {
		// Look for cuda-* directories
		entries, err := c.platform.ReadDir(basePath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			// Match patterns like cuda-12.1, cuda-11.8, etc.
			if strings.HasPrefix(name, "cuda-") && c.isVersionedCUDADir(name) {
				paths = append(paths, filepath.Join(basePath, name))
			}
		}
	}

	return paths
}

// isVersionedCUDADir checks if a directory name matches CUDA versioning pattern
func (c *CUDADetector) isVersionedCUDADir(name string) bool {
	// Match patterns: cuda-12.1, cuda-11.8, etc.
	matched, _ := regexp.MatchString(`^cuda-\d+\.\d+$`, name)
	return matched
}

// detectCUDAVersion detects the CUDA version from an installation path
func (c *CUDADetector) detectCUDAVersion(cudaPath string) (CUDAVersion, error) {
	// First, try to extract version from path
	if version, err := c.extractVersionFromPath(cudaPath); err == nil {
		version.Path = cudaPath
		return version, nil
	}

	// Try to get version from nvcc
	if version, err := c.getVersionFromNvcc(cudaPath); err == nil {
		version.Path = cudaPath
		return version, nil
	}

	// Try to get version from version.txt (older installations)
	if version, err := c.getVersionFromFile(cudaPath); err == nil {
		version.Path = cudaPath
		return version, nil
	}

	return CUDAVersion{}, fmt.Errorf("could not detect CUDA version for path: %s", cudaPath)
}

// extractVersionFromPath extracts version from directory name like "/usr/local/cuda-12.1"
func (c *CUDADetector) extractVersionFromPath(path string) (CUDAVersion, error) {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "cuda-") {
		return CUDAVersion{}, fmt.Errorf("path does not contain version: %s", path)
	}

	versionStr := strings.TrimPrefix(base, "cuda-")
	return c.parseVersionString(versionStr)
}

// getVersionFromNvcc gets version by running nvcc --version
func (c *CUDADetector) getVersionFromNvcc(cudaPath string) (CUDAVersion, error) {
	nvccPath := filepath.Join(cudaPath, "bin", "nvcc")

	// Check if nvcc exists
	if _, err := c.platform.Stat(nvccPath); err != nil {
		return CUDAVersion{}, fmt.Errorf("nvcc not found at %s", nvccPath)
	}

	// Execute nvcc --version
	cmd := c.platform.CreateCommand(nvccPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return CUDAVersion{}, fmt.Errorf("failed to run nvcc --version: %w", err)
	}

	// Parse output to extract version
	return c.parseNvccOutput(string(output))
}

// getVersionFromFile reads version from version.txt or similar files
func (c *CUDADetector) getVersionFromFile(cudaPath string) (CUDAVersion, error) {
	versionFiles := []string{
		filepath.Join(cudaPath, "version.txt"),
		filepath.Join(cudaPath, "VERSION"),
		filepath.Join(cudaPath, "include", "cuda_runtime_api.h"), // Last resort - parse header
	}

	for _, versionFile := range versionFiles {
		if content, err := c.platform.ReadFile(versionFile); err == nil {
			if version, err := c.parseVersionFromContent(string(content)); err == nil {
				return version, nil
			}
		}
	}

	return CUDAVersion{}, fmt.Errorf("no version file found")
}

// parseNvccOutput parses nvcc --version output to extract version
func (c *CUDADetector) parseNvccOutput(output string) (CUDAVersion, error) {
	// Look for pattern like "V12.1.105" or "release 12.1"
	patterns := []string{
		`V(\d+)\.(\d+)\.\d+`,
		`release (\d+)\.(\d+)`,
		`cuda_(\d+)\.(\d+)`,
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(output)
		if len(matches) >= 3 {
			major, _ := strconv.Atoi(matches[1])
			minor, _ := strconv.Atoi(matches[2])
			return CUDAVersion{Major: major, Minor: minor}, nil
		}
	}

	return CUDAVersion{}, fmt.Errorf("could not parse CUDA version from nvcc output")
}

// parseVersionFromContent parses version from file content
func (c *CUDADetector) parseVersionFromContent(content string) (CUDAVersion, error) {
	// Common patterns in version files
	patterns := []string{
		`CUDA Version (\d+)\.(\d+)`,
		`(\d+)\.(\d+)\.(\d+)`,
		`#define CUDART_VERSION (\d+)(\d{3})`, // From cuda_runtime_api.h
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) >= 3 {
			major, _ := strconv.Atoi(matches[1])
			minor, _ := strconv.Atoi(matches[2])
			return CUDAVersion{Major: major, Minor: minor}, nil
		}
	}

	return CUDAVersion{}, fmt.Errorf("could not parse version from content")
}

// parseVersionString parses a version string like "12.1"
func (c *CUDADetector) parseVersionString(versionStr string) (CUDAVersion, error) {
	parts := strings.Split(versionStr, ".")
	if len(parts) < 2 {
		return CUDAVersion{}, fmt.Errorf("invalid version format: %s", versionStr)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return CUDAVersion{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return CUDAVersion{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	return CUDAVersion{Major: major, Minor: minor}, nil
}

// detectDriverVersion detects the NVIDIA driver version
func (c *CUDADetector) detectDriverVersion() string {
	// Try nvidia-smi first
	cmd := c.platform.CreateCommand("nvidia-smi", "--query-gpu=driver_version", "--format=csv,noheader,nounits")
	if output, err := cmd.CombinedOutput(); err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 && strings.TrimSpace(lines[0]) != "" {
			return strings.TrimSpace(lines[0])
		}
	}

	// Try reading from /proc/driver/nvidia/version
	if content, err := c.platform.ReadFile("/proc/driver/nvidia/version"); err == nil {
		re := regexp.MustCompile(`NVRM version: NVIDIA UNIX x86_64 Kernel Module\s+(\d+\.\d+\.\d+)`)
		matches := re.FindStringSubmatch(string(content))
		if len(matches) >= 2 {
			return matches[1]
		}
	}

	return "unknown"
}

// markDefaultInstallation marks the default CUDA installation
func (c *CUDADetector) markDefaultInstallation(installations []CUDAInstallation) {
	if len(installations) == 0 {
		return
	}

	// Priority: CUDA_HOME > /usr/local/cuda > highest version
	cudaHome := c.platform.Getenv("CUDA_HOME")

	for i, install := range installations {
		if cudaHome != "" && install.Path == cudaHome {
			installations[i].IsDefault = true
			return
		}
	}

	// Check for /usr/local/cuda (common symlink to latest)
	for i, install := range installations {
		if install.Path == "/usr/local/cuda" {
			installations[i].IsDefault = true
			return
		}
	}

	// Default to highest version
	maxVersionIdx := 0
	for i, install := range installations {
		if install.Version.Major > installations[maxVersionIdx].Version.Major ||
			(install.Version.Major == installations[maxVersionIdx].Version.Major &&
				install.Version.Minor > installations[maxVersionIdx].Version.Minor) {
			maxVersionIdx = i
		}
	}

	installations[maxVersionIdx].IsDefault = true
}

// FindCompatibleCUDA finds CUDA installations compatible with the required version
func (c *CUDADetector) FindCompatibleCUDA(requiredVersion CUDAVersion) ([]CUDAInstallation, error) {
	installations, err := c.DetectCUDAInstallations()
	if err != nil {
		return nil, err
	}

	var compatible []CUDAInstallation
	for _, install := range installations {
		if install.Version.IsCompatible(requiredVersion) {
			compatible = append(compatible, install)
		}
	}

	if len(compatible) == 0 {
		return nil, fmt.Errorf("no CUDA installations compatible with version %s found", requiredVersion.String())
	}

	return compatible, nil
}

// GetBestCUDA returns the best CUDA installation for a given requirement
func (c *CUDADetector) GetBestCUDA(requiredVersion CUDAVersion) (CUDAInstallation, error) {
	compatible, err := c.FindCompatibleCUDA(requiredVersion)
	if err != nil {
		return CUDAInstallation{}, err
	}

	// Prefer default installation if compatible
	for _, install := range compatible {
		if install.IsDefault {
			return install, nil
		}
	}

	// Otherwise, return the closest compatible version
	best := compatible[0]
	for _, install := range compatible {
		// Prefer newer versions but closest to requirement
		if install.Version.Major < best.Version.Major ||
			(install.Version.Major == best.Version.Major && install.Version.Minor < best.Version.Minor) {
			best = install
		}
	}

	return best, nil
}
