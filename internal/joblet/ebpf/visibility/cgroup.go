//go:build linux

package visibility

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

// GetCgroupID returns the cgroup v2 ID for a given cgroup path.
// The path should be relative to /sys/fs/cgroup, e.g., "joblet/job-abc123".
func GetCgroupID(cgroupPath string) (uint64, error) {
	// Build full path
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)

	// Get the cgroup ID using statx
	var stat unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, fullPath, 0, unix.STATX_INO, &stat)
	if err != nil {
		return 0, fmt.Errorf("failed to get cgroup ID for %s: %w", fullPath, err)
	}

	// The cgroup ID is the inode number
	return stat.Ino, nil
}

// GetCgroupIDFromFD returns the cgroup v2 ID from an open file descriptor.
// This is useful when you have a handle to the cgroup directory.
func GetCgroupIDFromFD(fd int) (uint64, error) {
	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return 0, fmt.Errorf("failed to fstat cgroup fd: %w", err)
	}
	return stat.Ino, nil
}

// GetProcessCgroupID returns the cgroup v2 ID for a given process.
func GetProcessCgroupID(pid int) (uint64, error) {
	cgroupPath, err := GetProcessCgroupPath(pid)
	if err != nil {
		return 0, err
	}
	return GetCgroupID(cgroupPath)
}

// GetProcessCgroupPath returns the cgroup v2 path for a given process.
// The returned path is relative to /sys/fs/cgroup.
func GetProcessCgroupPath(pid int) (string, error) {
	// Read /proc/<pid>/cgroup
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", fmt.Errorf("failed to read process cgroup: %w", err)
	}

	// Parse cgroup v2 entry (format: "0::/path")
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" {
			// cgroup v2 entry
			path := strings.TrimSpace(parts[2])
			if path == "/" {
				return "", nil // Root cgroup
			}
			return strings.TrimPrefix(path, "/"), nil
		}
	}

	return "", fmt.Errorf("no cgroup v2 entry found for pid %d", pid)
}

// CgroupIDFromPath is a convenience function that handles full cgroup paths.
// It takes the job's cgroup path (as created by joblet, which is already absolute)
// and returns its cgroup ID.
func CgroupIDFromPath(jobletCgroupPath string) (uint64, error) {
	// The job.CgroupPath is already an absolute path like /sys/fs/cgroup/joblet.slice/...
	// so we use it directly without prepending /sys/fs/cgroup again
	var stat unix.Statx_t
	err := unix.Statx(unix.AT_FDCWD, jobletCgroupPath, 0, unix.STATX_INO, &stat)
	if err != nil {
		return 0, fmt.Errorf("failed to get cgroup ID for %s: %w", jobletCgroupPath, err)
	}
	return stat.Ino, nil
}

// ValidateCgroupPath checks if a cgroup path exists and is a directory.
func ValidateCgroupPath(cgroupPath string) error {
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
	info, err := os.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("cgroup path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cgroup path is not a directory: %s", fullPath)
	}
	return nil
}

// IsCgroupV2 checks if the system is using cgroup v2 (unified hierarchy).
func IsCgroupV2() bool {
	// Check if /sys/fs/cgroup is a cgroup2 mount
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[1] == "/sys/fs/cgroup" {
			return fields[2] == "cgroup2"
		}
	}

	return false
}

// GetCgroupControllers returns the list of controllers enabled for a cgroup.
func GetCgroupControllers(cgroupPath string) ([]string, error) {
	controllersPath := filepath.Join("/sys/fs/cgroup", cgroupPath, "cgroup.controllers")
	data, err := os.ReadFile(controllersPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cgroup controllers: %w", err)
	}

	controllers := strings.Fields(string(data))
	return controllers, nil
}

// ReadCgroupStat reads a specific stat from a cgroup.
func ReadCgroupStat(cgroupPath, statName string) (uint64, error) {
	statPath := filepath.Join("/sys/fs/cgroup", cgroupPath, statName)
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read cgroup stat %s: %w", statName, err)
	}

	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cgroup stat %s: %w", statName, err)
	}

	return value, nil
}
