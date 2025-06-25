//go:build darwin

package platform

import (
	"fmt"
	"runtime"
	"syscall"
)

// Darwin mount operations (override default - no-op for development)
func (dp *DarwinPlatform) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	dp.logger.Debug("mount operation not implemented on macOS",
		"source", source, "target", target, "fstype", fstype)
	return nil // No-op for development
}

func (dp *DarwinPlatform) Unmount(target string, flags int) error {
	dp.logger.Debug("unmount operation not implemented on macOS", "target", target)
	return nil // No-op for development
}

// Darwin process group creation (override default - no namespace support)
func (dp *DarwinPlatform) CreateProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
		// No Cloneflags - those are Linux-specific
	}
}

// GetInfo returns Darwin platform information (override default)
func (dp *DarwinPlatform) GetInfo() *PlatformInfo {
	return &PlatformInfo{
		OS:                    "darwin",
		Architecture:          runtime.GOARCH,
		SupportsNamespaces:    false,
		SupportsCgroups:       false,
		SupportsNetworkNS:     false,
		SupportsMountNS:       false,
		SupportsResourceLimit: false,
	}
}

// ValidateRequirements checks if Darwin platform meets basic requirements (override default)
func (dp *DarwinPlatform) ValidateRequirements() error {
	// Basic checks for macOS development environment
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("platform mismatch: expected darwin, got %s", runtime.GOOS)
	}

	dp.logger.Info("Darwin platform validated (development mode)")
	return nil
}
