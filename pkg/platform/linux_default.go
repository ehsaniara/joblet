//go:build !linux

package platform

import (
	"fmt"
	"syscall"
)

// Default implementations for LinuxPlatform when NOT compiled on Linux
// These provide fallback behavior when LinuxPlatform is used on non-Linux systems

func (lp *LinuxPlatform) Mount(source string, target string, fstype string, flags uintptr, data string) error {
	lp.logger.Warn("attempting Linux mount operation on non-Linux platform",
		"currentOS", "non-linux", "source", source, "target", target)
	return DefaultMount("linux", source, target, fstype, flags, data)
}

func (lp *LinuxPlatform) Unmount(target string, flags int) error {
	lp.logger.Warn("attempting Linux unmount operation on non-Linux platform",
		"currentOS", "non-linux", "target", target)
	return DefaultUnmount("linux", target, flags)
}

func (lp *LinuxPlatform) GetInfo() *Info {
	// Return Linux platform info even when not on Linux
	// This is useful for cross-platform information queries
	return &Info{
		OS:           "linux",
		Architecture: "unknown", // Can't determine target arch
	}
}

func (lp *LinuxPlatform) ValidateRequirements() error {
	lp.logger.Error("cannot validate Linux requirements on non-Linux platform")
	return DefaultValidateRequirements("linux")
}

func (lp *LinuxPlatform) CreateProcessGroup() *syscall.SysProcAttr {
	// Return basic process group without Linux-specific features
	lp.logger.Debug("creating basic process group (Linux-specific features unavailable)")
	return DefaultCreateProcessGroup()
}

func (lp *LinuxPlatform) Chdir(path string) error {
	lp.logger.Warn("attempting Linux chdir operation on non-Linux platform", "path", path)
	return fmt.Errorf("chdir operation not supported on non-Linux platform")
}

func (lp *LinuxPlatform) Chroot(path string) error {
	lp.logger.Warn("attempting Linux chroot operation on non-Linux platform", "path", path)
	return fmt.Errorf("chroot operation not supported on non-Linux platform")
}

func (lp *LinuxPlatform) Mknod(path string, mode uint32, dev int) error {
	lp.logger.Warn("attempting Linux mknod operation on non-Linux platform", "path", path)
	return fmt.Errorf("mknod operation not supported on non-Linux platform")
}

func (lp *LinuxPlatform) Mkfifo(path string, mode uint32) error {
	lp.logger.Warn("attempting Linux mkfifo operation on non-Linux platform", "path", path)
	return fmt.Errorf("mkfifo operation not supported on non-Linux platform")
}

func (lp *LinuxPlatform) Chmod(path string, mode uint32) error {
	lp.logger.Warn("attempting Linux chmod operation on non-Linux platform", "path", path)
	return fmt.Errorf("chmod operation not supported on non-Linux platform")
}

func (lp *LinuxPlatform) Statfs(path string, buf *syscall.Statfs_t) error {
	lp.logger.Warn("attempting Linux statfs operation on non-Linux platform", "path", path)
	return fmt.Errorf("statfs operation not supported on non-Linux platform")
}

func (lp *LinuxPlatform) SetNonblock(fd int, nonblocking bool) error {
	lp.logger.Warn("attempting Linux setnonblock operation on non-Linux platform", "fd", fd)
	return fmt.Errorf("setnonblock operation not supported on non-Linux platform")
}
