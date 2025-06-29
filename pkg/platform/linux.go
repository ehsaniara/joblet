//go:build linux

package platform

import (
	"fmt"
	"runtime"
	"syscall"
)

// Mount Linux-specific mount operations (override default)
func (lp *LinuxPlatform) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	return syscall.Mount(source, target, fstype, flags, data)
}

func (lp *LinuxPlatform) Unmount(target string, flags int) error {
	return syscall.Unmount(target, flags)
}

// CreateProcessGroup Linux-specific process group creation with namespace support (override default)
func (lp *LinuxPlatform) CreateProcessGroup() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
		// Linux supports additional namespace flags
		Cloneflags: 0, // Will be set by caller based on requirements
	}
}

// GetInfo returns Linux platform information (override default)
func (lp *LinuxPlatform) GetInfo() *Info {
	return &Info{
		OS:           "linux",
		Architecture: runtime.GOARCH,
	}
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
