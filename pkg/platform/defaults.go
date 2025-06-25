package platform

import (
	"fmt"
	"runtime"
	"syscall"
)

// Default implementations that work on all platforms
// These will be overridden by platform-specific implementations

// DefaultMount provides a default mount implementation (returns error)
func DefaultMount(platformName string, source string, target string, fstype string, flags uintptr, data string) error {
	return fmt.Errorf("mount operation not supported on platform %s (current: %s)", platformName, runtime.GOOS)
}

// DefaultUnmount provides a default unmount implementation (returns error)
func DefaultUnmount(platformName string, target string, flags int) error {
	return fmt.Errorf("unmount operation not supported on platform %s (current: %s)", platformName, runtime.GOOS)
}

// DefaultGetInfo provides default platform information
func DefaultGetInfo(platformName string) *PlatformInfo {
	return &PlatformInfo{
		OS:                    platformName,
		Architecture:          runtime.GOARCH,
		SupportsNamespaces:    false,
		SupportsCgroups:       false,
		SupportsNetworkNS:     false,
		SupportsMountNS:       false,
		SupportsResourceLimit: false,
	}
}

// DefaultValidateRequirements provides default validation (returns error)
func DefaultValidateRequirements(platformName string) error {
	return fmt.Errorf("platform requirements validation not implemented for %s (current: %s)",
		platformName, runtime.GOOS)
}

// DefaultCreateProcessGroup provides default process group creation
func DefaultCreateProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
		// No platform-specific flags
	}
}
