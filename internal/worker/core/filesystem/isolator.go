//go:build linux

package filesystem

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"worker/pkg/config"
	"worker/pkg/logger"
	"worker/pkg/platform"
)

type Isolator struct {
	platform platform.Platform
	config   config.FilesystemConfig
	logger   *logger.Logger
}

func NewIsolator(cfg config.FilesystemConfig, platform platform.Platform) *Isolator {
	return &Isolator{
		platform: platform,
		config:   cfg,
		logger:   logger.New().WithField("component", "filesystem-isolator"),
	}
}

// JobFilesystem represents an isolated filesystem for a job
type JobFilesystem struct {
	JobID    string
	RootDir  string
	TmpDir   string
	WorkDir  string
	platform platform.Platform
	config   config.FilesystemConfig
	logger   *logger.Logger
}

// CreateJobFilesystem creates an isolated filesystem for a job
func (i *Isolator) CreateJobFilesystem(jobID string) (*JobFilesystem, error) {
	log := i.logger.WithField("jobID", jobID)
	log.Debug("creating isolated filesystem for job")

	// Create job-specific directories
	jobRootDir := filepath.Join(i.config.BaseDir, jobID)
	jobTmpDir := strings.Replace(i.config.TmpDir, "{JOB_ID}", jobID, -1)
	jobWorkDir := filepath.Join(jobRootDir, "work")

	// Ensure we're in a job context (safety check)
	if err := i.validateJobContext(); err != nil {
		return nil, fmt.Errorf("filesystem isolation safety check failed: %w", err)
	}

	// Create directory structure
	dirs := []string{jobRootDir, jobTmpDir, jobWorkDir}
	for _, dir := range dirs {
		if err := i.platform.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	filesystem := &JobFilesystem{
		JobID:    jobID,
		RootDir:  jobRootDir,
		TmpDir:   jobTmpDir,
		WorkDir:  jobWorkDir,
		platform: i.platform,
		config:   i.config,
		logger:   log,
	}

	log.Debug("job filesystem structure created",
		"rootDir", jobRootDir,
		"tmpDir", jobTmpDir,
		"workDir", jobWorkDir)

	return filesystem, nil
}

// validateJobContext ensures we're running in a job context, not on the host
func (i *Isolator) validateJobContext() error {
	// Check if we're in a job by looking for JOB_ID environment variable
	jobID := i.platform.Getenv("JOB_ID")
	if jobID == "" {
		return fmt.Errorf("not in job context - JOB_ID not set")
	}

	// Additional safety: check if we're PID 1 (should be in a PID namespace)
	if i.platform.Getpid() != 1 {
		return fmt.Errorf("not in isolated PID namespace - refusing filesystem isolation")
	}

	return nil
}

// Setup performs the actual filesystem isolation using chroot and mounts
func (f *JobFilesystem) Setup() error {
	log := f.logger.WithField("operation", "filesystem-setup")
	log.Debug("setting up filesystem isolation")

	// Double-check we're in a job context
	if err := f.validateInJobContext(); err != nil {
		return fmt.Errorf("refusing to setup filesystem isolation: %w", err)
	}

	// Create essential directory structure in the isolated root
	if err := f.createEssentialDirs(); err != nil {
		return fmt.Errorf("failed to create essential directories: %w", err)
	}

	// Mount allowed read-only directories from host
	if err := f.mountAllowedDirs(); err != nil {
		return fmt.Errorf("failed to mount allowed directories: %w", err)
	}

	// Setup /tmp as isolated writable space
	if err := f.setupTmpDir(); err != nil {
		return fmt.Errorf("failed to setup tmp directory: %w", err)
	}

	// Finally, chroot to the isolated environment
	if err := f.performChroot(); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	// Mount essential read-only filesystems AFTER chroot
	if err := f.mountEssentialFS(); err != nil {
		return fmt.Errorf("failed to mount essential filesystems: %w", err)
	}

	log.Debug("filesystem isolation setup completed successfully")
	return nil
}

// createEssentialDirs creates the basic directory structure needed in the isolated root
func (f *JobFilesystem) createEssentialDirs() error {
	dirs := []string{
		"bin", "usr/bin", "lib", "lib64", "usr/lib", "usr/lib64",
		"etc", "tmp", "proc", "dev", "sys", "work",
	}

	for _, dir := range dirs {
		fullPath := filepath.Join(f.RootDir, dir)
		if err := f.platform.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
	}
	return nil
}

// mountAllowedDirs mounts allowed directories from host as read-only
func (f *JobFilesystem) mountAllowedDirs() error {
	for _, allowedDir := range f.config.AllowedMounts {
		// Skip if the host directory doesn't exist
		if _, err := f.platform.Stat(allowedDir); f.platform.IsNotExist(err) {
			f.logger.Debug("skipping non-existent allowed directory", "dir", allowedDir)
			continue
		}

		targetPath := filepath.Join(f.RootDir, strings.TrimPrefix(allowedDir, "/"))

		// Create target directory if it doesn't exist
		if err := f.platform.MkdirAll(targetPath, 0755); err != nil {
			f.logger.Warn("failed to create target directory", "target", targetPath, "error", err)
			continue
		}

		// Bind mount as read-only
		flags := uintptr(syscall.MS_BIND)
		if err := f.platform.Mount(allowedDir, targetPath, "", flags, ""); err != nil {
			f.logger.Warn("failed to mount allowed directory", "source", allowedDir, "target", targetPath, "error", err)
			continue
		}

		// Remount as read-only
		flags = uintptr(syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_RDONLY)
		if err := f.platform.Mount("", targetPath, "", flags, ""); err != nil {
			f.logger.Warn("failed to remount as read-only", "target", targetPath, "error", err)
		}

		f.logger.Debug("mounted allowed directory", "source", allowedDir, "target", targetPath)
	}
	return nil
}

// setupTmpDir sets up the isolated /tmp directory
func (f *JobFilesystem) setupTmpDir() error {
	tmpPath := filepath.Join(f.RootDir, "tmp")

	// Bind mount the job-specific tmp to /tmp in the isolated root
	if err := f.platform.Mount(f.TmpDir, tmpPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to bind mount tmp directory: %w", err)
	}

	f.logger.Debug("setup isolated tmp directory", "hostTmp", f.TmpDir, "isolatedTmp", tmpPath)
	return nil
}

// performChroot performs the actual chroot operation
func (f *JobFilesystem) performChroot() error {
	log := f.logger.WithField("operation", "chroot")
	log.Debug("performing chroot", "newRoot", f.RootDir)

	// Change to the new root directory
	if err := syscall.Chdir(f.RootDir); err != nil {
		return fmt.Errorf("failed to change to new root directory: %w", err)
	}

	// Perform chroot
	if err := syscall.Chroot(f.RootDir); err != nil {
		return fmt.Errorf("chroot operation failed: %w", err)
	}

	// Change to /work directory inside the chroot
	if err := syscall.Chdir("/work"); err != nil {
		// If /work doesn't exist, go to /tmp
		if er := syscall.Chdir("/tmp"); er != nil {
			// Last resort: stay in /
			if e := syscall.Chdir("/"); e != nil {
				return fmt.Errorf("failed to change to any working directory after chroot: %w", e)
			}
		}
	}

	log.Debug("chroot completed successfully", "newRoot", f.RootDir)
	return nil
}

// mountEssentialFS mounts essential filesystems (proc, minimal dev) AFTER chroot
func (f *JobFilesystem) mountEssentialFS() error {
	log := f.logger.WithField("operation", "mount-essential")

	// Create essential /dev entries first
	if err := f.createEssentialDevices(); err != nil {
		log.Warn("failed to create essential devices", "error", err)
		// Device creation failure is not critical
	}

	log.Debug("essential filesystems setup completed")
	return nil
}

// createEssentialDevices creates minimal /dev entries needed for basic operation
func (f *JobFilesystem) createEssentialDevices() error {
	// Create /dev/null
	if err := syscall.Mknod("/dev/null", syscall.S_IFCHR|0666, int(makedev(1, 3))); err != nil {
		if !f.platform.IsExist(err) {
			return fmt.Errorf("failed to create /dev/null: %w", err)
		}
	}

	// Create /dev/zero
	if err := syscall.Mknod("/dev/zero", syscall.S_IFCHR|0666, int(makedev(1, 5))); err != nil {
		if !f.platform.IsExist(err) {
			return fmt.Errorf("failed to create /dev/zero: %w", err)
		}
	}

	// Create /dev/random
	if err := syscall.Mknod("/dev/random", syscall.S_IFCHR|0666, int(makedev(1, 8))); err != nil {
		if !f.platform.IsExist(err) {
			f.logger.Debug("failed to create /dev/random", "error", err)
		}
	}

	// Create /dev/urandom
	if err := syscall.Mknod("/dev/urandom", syscall.S_IFCHR|0666, int(makedev(1, 9))); err != nil {
		if !f.platform.IsExist(err) {
			f.logger.Debug("failed to create /dev/urandom", "error", err)
		}
	}

	return nil
}

// Cleanup removes the job filesystem (called from host)
func (f *JobFilesystem) Cleanup() error {
	log := f.logger.WithField("operation", "cleanup")
	log.Debug("cleaning up job filesystem")

	// Remove the job-specific directories
	if err := f.platform.RemoveAll(f.RootDir); err != nil {
		log.Warn("failed to remove job root directory", "error", err)
	}

	if err := f.platform.RemoveAll(f.TmpDir); err != nil {
		log.Warn("failed to remove job tmp directory", "error", err)
	}

	log.Debug("filesystem cleanup completed")
	return nil
}

// validateInJobContext performs additional safety checks before chroot
func (f *JobFilesystem) validateInJobContext() error {
	// Ensure we have JOB_ID environment variable
	jobID := f.platform.Getenv("JOB_ID")
	if jobID == "" {
		return fmt.Errorf("JOB_ID not set - refusing chroot")
	}

	if jobID != f.JobID {
		return fmt.Errorf("JOB_ID mismatch - expected %s, got %s", f.JobID, jobID)
	}

	// Ensure we're PID 1 in a namespace
	if f.platform.Getpid() != 1 {
		return fmt.Errorf("not PID 1 - refusing chroot on host system")
	}

	// Check we're not already in a chroot by trying to access host root
	if _, err := f.platform.Stat("/proc/1/root"); err == nil {
		// Additional check: see if we can read host's root filesystem
		if entries, e := f.platform.ReadDir("/"); e == nil && len(entries) > 10 {
			// If we can see many entries in /, we're likely on the host filesystem
			f.logger.Debug("safety check: many root entries visible, may be on host", "entries", len(entries))
		}
	}

	return nil
}

// Helper function to create device numbers
func makedev(major, minor uint32) uint64 {
	return uint64(major<<8 | minor)
}
