package validation

import (
	"fmt"
	"runtime"
	"strings"

	"worker/pkg/logger"
	"worker/pkg/platform"
)

// PlatformValidator validates platform requirements
type PlatformValidator struct {
	platform platform.Platform
	logger   *logger.Logger
}

// ValidatePlatformRequirements checks platform-specific requirements using platform abstraction
func (pv *PlatformValidator) ValidatePlatformRequirements() error {
	switch runtime.GOOS {
	case "linux":
		return pv.validateLinux()
	case "darwin":
		return pv.validateDarwin()
	default:
		return fmt.Errorf("unsupported platform: %s (cgroup namespaces required)", runtime.GOOS)
	}
}

// validateLinux validates Linux-specific requirements using platform abstraction
func (pv *PlatformValidator) validateLinux() error {
	// Check cgroups v2 using platform abstraction
	cgroupPath := "/sys/fs/cgroup/worker.slice/worker.service"
	if _, err := pv.platform.Stat(cgroupPath); pv.platform.IsNotExist(err) {
		return fmt.Errorf("cgroups not available at %s", cgroupPath)
	}

	// Check cgroup namespace support using platform abstraction
	if _, err := pv.platform.Stat("/proc/self/ns/cgroup"); pv.platform.IsNotExist(err) {
		return fmt.Errorf("cgroup namespaces not supported by kernel (required)")
	}

	// Check kernel version using platform abstraction
	if err := pv.validateKernelVersion(); err != nil {
		return fmt.Errorf("kernel version validation failed: %w", err)
	}

	pv.logger.Info("Linux requirements validated",
		"cgroupsPath", cgroupPath,
		"cgroupNamespace", true)
	return nil
}

// validateDarwin validates macOS-specific requirements
func (pv *PlatformValidator) validateDarwin() error {
	pv.logger.Warn("macOS detected - limited functionality available")
	pv.logger.Info("macOS validation completed (development mode)")
	return nil // Allow macOS for development, but with warnings
}

// validateKernelVersion performs basic kernel version validation using platform abstraction
func (pv *PlatformValidator) validateKernelVersion() error {
	version, err := pv.platform.ReadFile("/proc/version")
	if err != nil {
		return fmt.Errorf("cannot read kernel version: %w", err)
	}

	versionStr := string(version)
	if strings.Contains(versionStr, "Linux version 4.") {
		// Check if it's 4.6 or higher (required for cgroup namespaces)
		oldVersions := []string{"4.0.", "4.1.", "4.2.", "4.3.", "4.4.", "4.5."}
		for _, oldVer := range oldVersions {
			if strings.Contains(versionStr, oldVer) {
				return fmt.Errorf("kernel too old for cgroup namespaces (need 4.6+)")
			}
		}
	}

	// Extract kernel version for logging
	versionParts := strings.Fields(versionStr)
	if len(versionParts) >= 3 {
		pv.logger.Info("kernel version validated", "version", versionParts[2])
	}

	return nil
}
