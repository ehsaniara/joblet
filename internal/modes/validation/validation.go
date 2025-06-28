package validation

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"worker/pkg/logger"
)

// ValidatePlatformRequirements checks platform-specific requirements
func ValidatePlatformRequirements(logger *logger.Logger) error {
	switch runtime.GOOS {
	case "linux":
		return validateLinux(logger)
	case "darwin":
		return validateDarwin(logger)
	default:
		return fmt.Errorf("unsupported platform: %s (cgroup namespaces required)", runtime.GOOS)
	}
}

// validateLinux validates Linux-specific requirements
func validateLinux(logger *logger.Logger) error {
	// Check cgroups v2
	cgroupPath := "/sys/fs/cgroup/worker.slice/worker.service"
	if _, err := os.Stat(cgroupPath); os.IsNotExist(err) {
		return fmt.Errorf("cgroups not available at %s", cgroupPath)
	}

	// Check cgroup namespace support
	if _, err := os.Stat("/proc/self/ns/cgroup"); os.IsNotExist(err) {
		return fmt.Errorf("cgroup namespaces not supported by kernel (required)")
	}

	// Check kernel version
	if err := validateKernelVersion(); err != nil {
		return fmt.Errorf("kernel version validation failed: %w", err)
	}

	logger.Debug("Linux requirements validated",
		"cgroupsPath", cgroupPath,
		"cgroupNamespace", true)
	return nil
}

// validateDarwin validates macOS-specific requirements
func validateDarwin(logger *logger.Logger) error {
	return fmt.Errorf("macOS not supported when cgroup namespaces are required")
}

// validateKernelVersion performs basic kernel version validation
func validateKernelVersion() error {
	version, err := os.ReadFile("/proc/version")
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

	return nil
}
