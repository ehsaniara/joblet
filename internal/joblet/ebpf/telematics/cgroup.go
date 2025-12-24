package telematics

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ehsaniara/joblet/pkg/platform"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// CgroupOps provides operations for cgroup ID retrieval and validation.
// This interface allows mocking for testing.
//
//counterfeiter:generate . CgroupOps
type CgroupOps interface {
	// GetCgroupID returns the cgroup v2 ID for a given cgroup path.
	// The path should be relative to /sys/fs/cgroup.
	GetCgroupID(cgroupPath string) (uint64, error)

	// GetCgroupIDFromFD returns the cgroup v2 ID from an open file descriptor.
	GetCgroupIDFromFD(fd int) (uint64, error)

	// GetProcessCgroupID returns the cgroup v2 ID for a given process.
	GetProcessCgroupID(pid int) (uint64, error)

	// GetProcessCgroupPath returns the cgroup v2 path for a given process.
	GetProcessCgroupPath(pid int) (string, error)

	// CgroupIDFromPath returns the cgroup ID for an absolute cgroup path.
	CgroupIDFromPath(jobletCgroupPath string) (uint64, error)

	// ValidateCgroupPath checks if a cgroup path exists and is a directory.
	ValidateCgroupPath(cgroupPath string) error

	// IsCgroupV2 checks if the system is using cgroup v2.
	IsCgroupV2() bool

	// GetCgroupControllers returns the list of controllers enabled for a cgroup.
	GetCgroupControllers(cgroupPath string) ([]string, error)

	// ReadCgroupStat reads a specific stat from a cgroup.
	ReadCgroupStat(cgroupPath, statName string) (uint64, error)
}

// cgroupOps implements CgroupOps using the platform interface.
type cgroupOps struct {
	platform platform.Platform
}

// NewCgroupOps creates a new CgroupOps instance.
func NewCgroupOps(p platform.Platform) CgroupOps {
	return &cgroupOps{platform: p}
}

// GetCgroupID returns the cgroup v2 ID for a given cgroup path.
// The path should be relative to /sys/fs/cgroup, e.g., "joblet/job-abc123".
func (c *cgroupOps) GetCgroupID(cgroupPath string) (uint64, error) {
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
	ino, err := c.platform.Statx(platform.AT_FDCWD, fullPath, 0, platform.STATX_INO)
	if err != nil {
		return 0, fmt.Errorf("failed to get cgroup ID for %s: %w", fullPath, err)
	}
	return ino, nil
}

// GetCgroupIDFromFD returns the cgroup v2 ID from an open file descriptor.
// This is useful when you have a handle to the cgroup directory.
func (c *cgroupOps) GetCgroupIDFromFD(fd int) (uint64, error) {
	ino, err := c.platform.Fstat(fd)
	if err != nil {
		return 0, fmt.Errorf("failed to fstat cgroup fd: %w", err)
	}
	return ino, nil
}

// GetProcessCgroupID returns the cgroup v2 ID for a given process.
func (c *cgroupOps) GetProcessCgroupID(pid int) (uint64, error) {
	cgroupPath, err := c.GetProcessCgroupPath(pid)
	if err != nil {
		return 0, err
	}
	return c.GetCgroupID(cgroupPath)
}

// GetProcessCgroupPath returns the cgroup v2 path for a given process.
// The returned path is relative to /sys/fs/cgroup.
func (c *cgroupOps) GetProcessCgroupPath(pid int) (string, error) {
	data, err := c.platform.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", fmt.Errorf("failed to read process cgroup: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) == 3 && parts[0] == "0" {
			path := strings.TrimSpace(parts[2])
			if path == "/" {
				return "", nil
			}
			return strings.TrimPrefix(path, "/"), nil
		}
	}

	return "", fmt.Errorf("no cgroup v2 entry found for pid %d", pid)
}

// CgroupIDFromPath is a convenience function that handles full cgroup paths.
// It takes the job's cgroup path (as created by joblet, which is already absolute)
// and returns its cgroup ID.
func (c *cgroupOps) CgroupIDFromPath(jobletCgroupPath string) (uint64, error) {
	ino, err := c.platform.Statx(platform.AT_FDCWD, jobletCgroupPath, 0, platform.STATX_INO)
	if err != nil {
		return 0, fmt.Errorf("failed to get cgroup ID for %s: %w", jobletCgroupPath, err)
	}
	return ino, nil
}

// ValidateCgroupPath checks if a cgroup path exists and is a directory.
func (c *cgroupOps) ValidateCgroupPath(cgroupPath string) error {
	fullPath := filepath.Join("/sys/fs/cgroup", cgroupPath)
	info, err := c.platform.Stat(fullPath)
	if err != nil {
		return fmt.Errorf("cgroup path does not exist: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cgroup path is not a directory: %s", fullPath)
	}
	return nil
}

// IsCgroupV2 checks if the system is using cgroup v2 (unified hierarchy).
func (c *cgroupOps) IsCgroupV2() bool {
	data, err := c.platform.ReadFile("/proc/mounts")
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
func (c *cgroupOps) GetCgroupControllers(cgroupPath string) ([]string, error) {
	controllersPath := filepath.Join("/sys/fs/cgroup", cgroupPath, "cgroup.controllers")
	data, err := c.platform.ReadFile(controllersPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cgroup controllers: %w", err)
	}

	controllers := strings.Fields(string(data))
	return controllers, nil
}

// ReadCgroupStat reads a specific stat from a cgroup.
func (c *cgroupOps) ReadCgroupStat(cgroupPath, statName string) (uint64, error) {
	statPath := filepath.Join("/sys/fs/cgroup", cgroupPath, statName)
	data, err := c.platform.ReadFile(statPath)
	if err != nil {
		return 0, fmt.Errorf("failed to read cgroup stat %s: %w", statName, err)
	}

	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse cgroup stat %s: %w", statName, err)
	}

	return value, nil
}

// Package-level convenience functions for backward compatibility.
// These use the default platform implementation.

var defaultPlatform platform.Platform

func init() {
	defaultPlatform = platform.NewLinuxPlatform()
}

// GetCgroupID returns the cgroup v2 ID for a given cgroup path.
func GetCgroupID(cgroupPath string) (uint64, error) {
	return NewCgroupOps(defaultPlatform).GetCgroupID(cgroupPath)
}

// GetCgroupIDFromFD returns the cgroup v2 ID from an open file descriptor.
func GetCgroupIDFromFD(fd int) (uint64, error) {
	return NewCgroupOps(defaultPlatform).GetCgroupIDFromFD(fd)
}

// GetProcessCgroupID returns the cgroup v2 ID for a given process.
func GetProcessCgroupID(pid int) (uint64, error) {
	return NewCgroupOps(defaultPlatform).GetProcessCgroupID(pid)
}

// GetProcessCgroupPath returns the cgroup v2 path for a given process.
func GetProcessCgroupPath(pid int) (string, error) {
	return NewCgroupOps(defaultPlatform).GetProcessCgroupPath(pid)
}

// CgroupIDFromPath returns the cgroup ID for an absolute cgroup path.
func CgroupIDFromPath(jobletCgroupPath string) (uint64, error) {
	return NewCgroupOps(defaultPlatform).CgroupIDFromPath(jobletCgroupPath)
}

// ValidateCgroupPath checks if a cgroup path exists and is a directory.
func ValidateCgroupPath(cgroupPath string) error {
	return NewCgroupOps(defaultPlatform).ValidateCgroupPath(cgroupPath)
}

// IsCgroupV2 checks if the system is using cgroup v2.
func IsCgroupV2() bool {
	return NewCgroupOps(defaultPlatform).IsCgroupV2()
}

// GetCgroupControllers returns the list of controllers enabled for a cgroup.
func GetCgroupControllers(cgroupPath string) ([]string, error) {
	return NewCgroupOps(defaultPlatform).GetCgroupControllers(cgroupPath)
}

// ReadCgroupStat reads a specific stat from a cgroup.
func ReadCgroupStat(cgroupPath, statName string) (uint64, error) {
	return NewCgroupOps(defaultPlatform).ReadCgroupStat(cgroupPath, statName)
}
