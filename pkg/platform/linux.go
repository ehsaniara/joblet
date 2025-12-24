package platform

import (
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
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

// Chdir changes the current working directory
func (lp *LinuxPlatform) Chdir(path string) error {
	return syscall.Chdir(path)
}

// Chroot changes the root directory
func (lp *LinuxPlatform) Chroot(path string) error {
	return syscall.Chroot(path)
}

// Mknod creates a device node
func (lp *LinuxPlatform) Mknod(path string, mode uint32, dev int) error {
	return syscall.Mknod(path, mode, dev)
}

// Mkfifo creates a named pipe (FIFO)
func (lp *LinuxPlatform) Mkfifo(path string, mode uint32) error {
	return unix.Mkfifo(path, mode)
}

// Chmod changes file permissions
func (lp *LinuxPlatform) Chmod(path string, mode uint32) error {
	return syscall.Chmod(path, mode)
}

// Statfs returns filesystem statistics
func (lp *LinuxPlatform) Statfs(path string, buf *syscall.Statfs_t) error {
	return syscall.Statfs(path, buf)
}

// SetNonblock sets or clears the O_NONBLOCK flag on a file descriptor
func (lp *LinuxPlatform) SetNonblock(fd int, nonblocking bool) error {
	return syscall.SetNonblock(fd, nonblocking)
}

// Statx returns the inode number for a path using the statx syscall.
// This is used for cgroup ID retrieval.
func (lp *LinuxPlatform) Statx(dirfd int, path string, flags int, mask int) (ino uint64, err error) {
	var stat unix.Statx_t
	err = unix.Statx(dirfd, path, flags, mask, &stat)
	if err != nil {
		return 0, err
	}
	return stat.Ino, nil
}

// Fstat returns the inode number for a file descriptor.
// This is used for cgroup ID retrieval.
func (lp *LinuxPlatform) Fstat(fd int) (ino uint64, err error) {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return 0, err
	}
	return stat.Ino, nil
}
