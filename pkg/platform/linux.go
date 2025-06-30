//go:build linux

package platform

import (
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
