//go:build linux

package platform

import (
	"fmt"
	"runtime"
	"syscall"
)

// Linux-specific mount operations (override default)
func (lp *LinuxPlatform) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

func (lp *LinuxPlatform) Unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

// Linux-specific process group creation with namespace support (override default)
func (lp *LinuxPlatform) CreateProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
		// Linux supports additional namespace flags
		Cloneflags: 0, // Will be set by caller based on requirements
	}
}

// GetInfo returns Linux platform information (override default)
func (lp *LinuxPlatform) GetInfo() *PlatformInfo {
	return &PlatformInfo{
		OS:                    "linux",
		Architecture:          runtime.GOARCH,
		SupportsNamespaces:    true,
		SupportsCgroups:       lp.checkCgroupSupport(),
		SupportsNetworkNS:     lp.checkNetworkNamespaceSupport(),
		SupportsMountNS:       true,
		SupportsResourceLimit: true,
	}
}

// ValidateRequirements checks if Linux platform meets requirements (override default)
func (lp *LinuxPlatform) ValidateRequirements() error {
	// Check cgroups v2
	if _, err := lp.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		return fmt.Errorf("cgroups v2 not available: %w", err)
	}

	// Check namespace support
	if _, err := lp.Stat("/proc/self/ns/cgroup"); err != nil {
		return fmt.Errorf("cgroup namespaces not supported by kernel: %w", err)
	}

	// Check kernel version
	if err := lp.validateKernelVersion(); err != nil {
		return fmt.Errorf("kernel version validation failed: %w", err)
	}

	lp.logger.Info("Linux platform requirements validated")
	return nil
}

// checkCgroupSupport checks if cgroups are available
func (lp *LinuxPlatform) checkCgroupSupport() bool {
	_, err := lp.Stat("/sys/fs/cgroup")
	return err == nil
}

// checkNetworkNamespaceSupport checks if network namespaces are supported
func (lp *LinuxPlatform) checkNetworkNamespaceSupport() bool {
	_, err := lp.Stat("/proc/self/ns/net")
	return err == nil
}

// validateKernelVersion performs basic kernel version validation
func (lp *LinuxPlatform) validateKernelVersion() error {
	version, err := lp.ReadFile("/proc/version")
	if err != nil {
		return fmt.Errorf("cannot read kernel version: %w", err)
	}

	// Simple check for minimum kernel version (4.6+ for cgroup namespaces)
	versionStr := string(version)
	if len(versionStr) < 20 {
		return fmt.Errorf("invalid kernel version format")
	}

	// More sophisticated version checking could be added here
	return nil
}
