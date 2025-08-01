//go:build linux

package filesystem

import (
	"fmt"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
	InitPath string   // Path to the init binary inside the isolated environment
	Volumes  []string // Volume names to mount
	platform platform.Platform
	config   config.FilesystemConfig
	logger   *logger.Logger
}

// PrepareInitBinary prepare the init binary
func (f *JobFilesystem) PrepareInitBinary(hostBinaryPath string) error {
	log := f.logger.WithField("operation", "prepare-init-binary")

	// Create /sbin directory in the isolated root
	sbinDir := filepath.Join(f.RootDir, "sbin")
	if err := f.platform.MkdirAll(sbinDir, 0755); err != nil {
		return fmt.Errorf("failed to create sbin directory: %w", err)
	}

	// Set the init path that will be used inside the chroot
	f.InitPath = "/sbin/init"

	// Copy the binary to the isolated filesystem
	destPath := filepath.Join(f.RootDir, "sbin", "init")

	// Read the host binary
	data, err := f.platform.ReadFile(hostBinaryPath)
	if err != nil {
		return fmt.Errorf("failed to read host binary: %w", err)
	}

	// Write to the isolated location
	if err := f.platform.WriteFile(destPath, data, 0755); err != nil {
		return fmt.Errorf("failed to write init binary: %w", err)
	}

	log.Debug("init binary prepared in isolated filesystem",
		"hostPath", hostBinaryPath,
		"isolatedPath", destPath,
		"chrootPath", f.InitPath)

	return nil
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

	// check if we're PID 1 (should be in a PID namespace)
	if i.platform.Getpid() != 1 {
		return fmt.Errorf("not in isolated PID namespace - refusing filesystem isolation")
	}

	return nil
}

func (f *JobFilesystem) createEssentialFiles() error {
	// Create /etc directory
	etcDir := filepath.Join(f.RootDir, "etc")
	if err := f.platform.MkdirAll(etcDir, 0755); err != nil {
		return fmt.Errorf("failed to create /etc directory: %w", err)
	}

	// Create /etc/resolv.conf for DNS resolution
	resolvConf := `# DNS configuration for Joblet container
nameserver 8.8.8.8
nameserver 8.8.4.4
options ndots:0
`
	resolvPath := filepath.Join(etcDir, "resolv.conf")
	if err := f.platform.WriteFile(resolvPath, []byte(resolvConf), 0644); err != nil {
		f.logger.Warn("failed to create resolv.conf", "error", err)
		// Don't fail the job, just warn
	}

	// Create basic /etc/hosts
	hostsContent := `127.0.0.1   localhost
::1         localhost ip6-localhost ip6-loopback
`
	hostsPath := filepath.Join(etcDir, "hosts")
	if err := f.platform.WriteFile(hostsPath, []byte(hostsContent), 0644); err != nil {
		f.logger.Warn("failed to create hosts file", "error", err)
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

	// Create essential files
	if err := f.createEssentialFiles(); err != nil {
		return fmt.Errorf("failed to create essential files: %w", err)
	}

	// Mount allowed read-only directories from host
	if err := f.mountAllowedDirs(); err != nil {
		return fmt.Errorf("failed to mount allowed directories: %w", err)
	}

	// Load volumes from environment if not already set
	if len(f.Volumes) == 0 {
		f.loadVolumesFromEnvironment()
	}

	// Mount volumes BEFORE chroot
	if err := f.mountVolumes(); err != nil {
		return fmt.Errorf("failed to mount volumes: %w", err)
	}

	// If no volumes are mounted, set up limited work directory (1MB)
	if len(f.Volumes) == 0 {
		if err := f.setupLimitedWorkDir(); err != nil {
			log.Warn("failed to setup limited work directory", "error", err)
			// Don't fail - continue with unlimited work dir
		}
	}

	// Mount pipes directory for uploads
	if err := f.mountPipesDirectory(); err != nil {
		// Log warning but don't fail - jobs without uploads should still work
		log.Warn("failed to mount pipes directory", "error", err)
		// Don't return error - continue without upload support
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

// createEssentialDirs creates only the directories that are NOT in allowedMounts
func (f *JobFilesystem) createEssentialDirs() error {
	// Directories that must exist but won't be populated by mounts
	essentialDirs := []string{
		"etc",     // For resolv.conf, hosts, etc.
		"tmp",     // Will be bind mounted to job-specific tmp
		"proc",    // For /proc mount
		"dev",     // For device nodes
		"sys",     // For potential sysfs mount
		"work",    // Working directory
		"var",     // For various runtime needs
		"var/run", // For runtime files
		"var/tmp", // Alternative tmp
		"volumes", // For volume mounts
	}

	// Create essential directories
	for _, dir := range essentialDirs {
		fullPath := filepath.Join(f.RootDir, dir)
		if err := f.platform.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
	}

	// Directories for allowed mounts will be created by mountAllowedDirs
	// This avoids duplication
	return nil
}

// mountAllowedDirs mounts allowed directories from host as read-only
func (f *JobFilesystem) mountAllowedDirs() error {
	// Enhanced to create parent directories automatically
	for _, allowedDir := range f.config.AllowedMounts {
		// Skip if the host directory doesn't exist
		if _, err := f.platform.Stat(allowedDir); f.platform.IsNotExist(err) {
			f.logger.Debug("skipping non-existent allowed directory", "dir", allowedDir)
			continue
		}

		targetPath := filepath.Join(f.RootDir, strings.TrimPrefix(allowedDir, "/"))

		// Create ALL parent directories needed for the mount
		// This replaces the need to pre-create them in createEssentialDirs
		targetDir := filepath.Dir(targetPath)
		if err := f.platform.MkdirAll(targetDir, 0755); err != nil {
			f.logger.Warn("failed to create mount parent directory", "dir", targetDir, "error", err)
			continue
		}

		// For leaf directories, create them too
		if err := f.platform.MkdirAll(targetPath, 0755); err != nil {
			f.logger.Warn("failed to create mount target directory", "target", targetPath, "error", err)
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

	// Unmount any tmpfs mounts before removing directories
	// This handles the limited work directory if it was created
	workLimitedPath := filepath.Join(f.RootDir, "work-limited")
	if err := f.platform.Unmount(workLimitedPath, 0x1); err != nil { // 0x1 = MNT_FORCE
		// Ignore error - mount might not exist
		log.Debug("unmount work-limited failed (may not exist)", "error", err)
	}

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
		// see if we can read host's root filesystem
		if entries, e := f.platform.ReadDir("/"); e == nil && len(entries) > 10 {
			// If we can see many entries in /, we're likely on the host filesystem
			f.logger.Debug("safety check: many root entries visible, may be on host", "entries", len(entries))
		}
	}

	return nil
}

// mountPipesDirectory mounts the host pipes directory into the chroot
// This allows the init process to access upload pipes created by the server
func (f *JobFilesystem) mountPipesDirectory() error {
	// Get job ID from environment
	jobID := f.platform.Getenv("JOB_ID")
	if jobID == "" {
		f.logger.Debug("no JOB_ID set, skipping pipes mount")
		return nil
	}

	// Host pipes directory (where server creates the pipe)
	hostPipesPath := fmt.Sprintf("/opt/joblet/jobs/%s/pipes", jobID)

	// Check if host pipes directory exists
	if _, err := f.platform.Stat(hostPipesPath); err != nil {
		if f.platform.IsNotExist(err) {
			f.logger.Debug("pipes directory doesn't exist yet", "path", hostPipesPath)
			// Create it so mount doesn't fail
			if err := f.platform.MkdirAll(hostPipesPath, 0700); err != nil {
				return fmt.Errorf("failed to create host pipes directory: %w", err)
			}
		} else {
			return fmt.Errorf("failed to stat pipes directory: %w", err)
		}
	}

	// Target path inside chroot (maintaining the same path structure)
	targetPipesPath := filepath.Join(f.RootDir, "opt/joblet/jobs", jobID, "pipes")

	// Create the directory structure in chroot
	targetParentDir := filepath.Dir(targetPipesPath)
	if err := f.platform.MkdirAll(targetParentDir, 0755); err != nil {
		return fmt.Errorf("failed to create pipes parent directory in chroot: %w", err)
	}

	// Create the pipes directory itself
	if err := f.platform.MkdirAll(targetPipesPath, 0700); err != nil {
		return fmt.Errorf("failed to create pipes directory in chroot: %w", err)
	}

	// Bind mount the pipes directory
	flags := uintptr(syscall.MS_BIND)
	if err := f.platform.Mount(hostPipesPath, targetPipesPath, "", flags, ""); err != nil {
		return fmt.Errorf("failed to bind mount pipes directory: %w", err)
	}

	f.logger.Debug("mounted pipes directory",
		"hostPath", hostPipesPath,
		"targetPath", targetPipesPath)

	return nil
}

// mountVolumes mounts all requested volumes into the chroot environment
func (f *JobFilesystem) mountVolumes() error {
	if len(f.Volumes) == 0 {
		f.logger.Debug("no volumes to mount")
		return nil
	}

	log := f.logger.WithField("operation", "mount-volumes")
	log.Debug("mounting volumes", "count", len(f.Volumes), "volumes", f.Volumes)

	for _, volumeName := range f.Volumes {
		if err := f.mountSingleVolume(volumeName); err != nil {
			log.Warn("failed to mount volume", "volume", volumeName, "error", err)
			// Continue with other volumes, don't fail the entire job
			continue
		}
	}

	log.Debug("volume mounting completed")
	return nil
}

// mountSingleVolume mounts a single volume into the job's chroot environment
func (f *JobFilesystem) mountSingleVolume(volumeName string) error {
	log := f.logger.WithField("volume", volumeName)

	// Host volume path - this is where the actual volume data lives
	hostVolumePath := fmt.Sprintf("/opt/joblet/volumes/%s/data", volumeName)

	// Check if host volume directory exists
	if _, err := f.platform.Stat(hostVolumePath); err != nil {
		if f.platform.IsNotExist(err) {
			return fmt.Errorf("volume %s does not exist at %s", volumeName, hostVolumePath)
		}
		return fmt.Errorf("failed to stat volume directory: %w", err)
	}

	// Target path inside chroot - mount volumes under /volumes/{name}
	targetVolumePath := filepath.Join(f.RootDir, "volumes", volumeName)

	// Create the mount point directory
	if err := f.platform.MkdirAll(targetVolumePath, 0755); err != nil {
		return fmt.Errorf("failed to create volume mount point: %w", err)
	}

	// Bind mount the volume (read-write by default)
	flags := uintptr(syscall.MS_BIND)
	if err := f.platform.Mount(hostVolumePath, targetVolumePath, "", flags, ""); err != nil {
		return fmt.Errorf("failed to bind mount volume: %w", err)
	}

	log.Debug("volume mounted successfully",
		"hostPath", hostVolumePath,
		"targetPath", targetVolumePath)

	return nil
}

// SetVolumes sets the list of volumes to mount for this job
func (f *JobFilesystem) SetVolumes(volumes []string) {
	f.Volumes = volumes
	f.logger.Debug("volumes set for job", "volumes", volumes)
}

// loadVolumesFromEnvironment loads volume names from environment variables
func (f *JobFilesystem) loadVolumesFromEnvironment() {
	volumeCountStr := f.platform.Getenv("JOB_VOLUMES_COUNT")
	if volumeCountStr == "" {
		return
	}

	volumeCount := 0
	if count, err := strconv.Atoi(volumeCountStr); err == nil {
		volumeCount = count
	}

	if volumeCount <= 0 {
		return
	}

	volumes := make([]string, 0, volumeCount)
	for i := 0; i < volumeCount; i++ {
		volumeName := f.platform.Getenv(fmt.Sprintf("JOB_VOLUME_%d", i))
		if volumeName != "" {
			volumes = append(volumes, volumeName)
		}
	}

	f.Volumes = volumes
	f.logger.Debug("loaded volumes from environment", "volumes", volumes)
}

// setupLimitedWorkDir sets up a 1MB limited work directory for jobs without volumes
func (f *JobFilesystem) setupLimitedWorkDir() error {
	log := f.logger.WithField("operation", "setup-limited-work")
	log.Debug("setting up limited work directory (1MB) for job without volumes")

	// Create a temporary backing directory for the limited work space
	limitedWorkPath := filepath.Join(f.RootDir, "work-limited")
	if err := f.platform.MkdirAll(limitedWorkPath, 0755); err != nil {
		return fmt.Errorf("failed to create limited work directory: %w", err)
	}

	// Mount tmpfs with 1MB size limit
	sizeOpt := "size=1048576" // 1MB in bytes
	flags := uintptr(0)
	if err := f.platform.Mount("tmpfs", limitedWorkPath, "tmpfs", flags, sizeOpt); err != nil {
		return fmt.Errorf("failed to mount limited tmpfs: %w", err)
	}

	// Now bind mount this limited directory to the actual work directory
	workPath := filepath.Join(f.RootDir, "work")
	if err := f.platform.Mount(limitedWorkPath, workPath, "", syscall.MS_BIND, ""); err != nil {
		// Unmount the tmpfs if bind mount fails
		_ = f.platform.Unmount(limitedWorkPath, 0)
		return fmt.Errorf("failed to bind mount limited work directory: %w", err)
	}

	log.Debug("limited work directory set up successfully", "size", "1MB")
	return nil
}

// Helper function to create device numbers
func makedev(major, minor uint32) uint64 {
	return uint64(major<<8 | minor)
}
