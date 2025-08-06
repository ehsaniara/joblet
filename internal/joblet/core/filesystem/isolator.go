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

// NewIsolator creates a new filesystem isolator with the given configuration.
// The isolator provides secure filesystem isolation for job execution using
// chroot, bind mounts, and namespace isolation techniques.
// Returns an Isolator instance ready to create isolated job filesystems.
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

// PrepareInitBinary copies the joblet init binary into the isolated filesystem.
// Creates /sbin directory in the chroot environment and copies the host binary
// to /sbin/init with executable permissions. This binary will be executed as
// PID 1 inside the isolated environment to manage the job process.
// Returns error if directory creation or binary copying fails.
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

// CreateJobFilesystem creates a new isolated filesystem environment for a job.
// Sets up the directory structure needed for job execution:
//   - Job root directory under configured base path
//   - Temporary directory with job ID substitution
//   - Work directory for job files and execution
//
// Performs safety validation to ensure running in proper job conte
// Performs safety validation to ensure running in proper job context.
// Returns JobFilesystem instance ready for setup and chroot operations.
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

// validateJobContext ensures the process is running in a safe job environment.
// Performs critical safety checks to prevent filesystem isolation on the host:
//   - Verifies JOB_ID environment variable is set
//   - Confirms process is PID 1 (running in PID namespace)
//
// These checks prevent accidental host filesystem corruption during development.
// Returns error if not running in proper isolated job context.
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

// createEssentialFiles creates basic system files needed in the isolated environment.
// Sets up minimal /etc directory with:
//   - /etc/resolv.conf with DNS configuration (Google DNS servers)
//   - /etc/hosts with localhost mappings
//
// These files enable basic network resolution and hostname lookup within jobs.
// Logs warnings but does not fail job execution if file creation fails.
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

// Setup performs complete filesystem isolation for the job environment.
//
//  1. Validates running in job context (safety check)
//
//  2. Creates essential directory structure
//
//  3. Creates basic system files (/etc/resolv.conf, /etc/hosts)
//
//  4. Mounts allowed read-only directories from host
//
//  5. Loads and mounts job volumes
//
//  6. Sets up limited work directory (1MB) if no volumes
//
//  7. Mounts upload pipes directory
//
//  8. Sets up isolated /tmp directory
//
//  9. Performs chroot to isolated environment
//
//  10. Mounts essential filesystems (/proc, /dev)
//
//  10. Mounts essential filesystems (/proc, /dev)
//
// Returns error if any step fails - job cannot proceed without proper isolation.
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

	// If no volumes are mounted, try to set up limited work directory (1MB)
	if len(f.Volumes) == 0 {
		if err := f.setupLimitedWorkDir(); err != nil {
			log.Warn("failed to setup limited work directory, using unlimited work dir", "error", err)
			// Ensure work directory is still accessible
			workPath := filepath.Join(f.RootDir, "work")
			if _, statErr := f.platform.Stat(workPath); statErr != nil {
				// Work directory might have been corrupted, recreate it
				if mkdirErr := f.platform.MkdirAll(workPath, 0755); mkdirErr != nil {
					log.Error("failed to recreate work directory", "error", mkdirErr)
				} else {
					log.Debug("recreated work directory after mount failure")
				}
			}
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

// createEssentialDirs creates the basic directory structure in the isolated root.
// Creates directories that won't be populated by bind mounts:
//   - /etc (for configuration files)
//   - /tmp (will be bind mounted to job-specific tmp)
//   - /proc, /dev, /sys (for essential filesystems)
//   - /work (working directory for job execution)
//   - /var, /var/run, /var/tmp (runtime directories)
//   - /volumes (mount point for persistent volumes)
//
// Directories for allowed mounts are created dynamically during mount operations.
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

// mountAllowedDirs bind mounts essential host directories into the isolated environment.
// Creates read-only mounts for system directories like /bin, /usr/bin, /lib, etc.
// that are needed for job execution but should not be writable.
// Automatically creates parent directories and handles missing host directories gracefully.
// Each mount is first bound, then remounted as read-only for security.
// Continues with remaining mounts if individual mounts fail.
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

// setupTmpDir creates an isolated temporary directory for the job.
// Bind mounts the job-specific temporary directory (from host) to /tmp
// in the isolated environment, providing writable temporary space that
// is automatically cleaned up when the job completes.
// Each job gets its own isolated /tmp to prevent interference.
func (f *JobFilesystem) setupTmpDir() error {
	tmpPath := filepath.Join(f.RootDir, "tmp")

	// Bind mount the job-specific tmp to /tmp in the isolated root
	if err := f.platform.Mount(f.TmpDir, tmpPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("failed to bind mount tmp directory: %w", err)
	}

	f.logger.Debug("setup isolated tmp directory", "hostTmp", f.TmpDir, "isolatedTmp", tmpPath)
	return nil
}

// performChroot executes the chroot system call to isolate the filesystem.
// Changes to the prepared isolated root directory, performs chroot operation,
// then changes working directory to the configured workspace (/work by default).
// After chroot, the process can only access files within the isolated environment.
// Falls back to /tmp or / if workspace directory is not accessible.
// This is the critical isolation step that restricts filesystem access.
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

	// Change to workspace directory inside the chroot
	workspaceDir := f.config.WorkspaceDir
	if workspaceDir == "" {
		workspaceDir = "/work" // fallback to default
	}
	if err := syscall.Chdir(workspaceDir); err != nil {
		// If workspace doesn't exist, go to /tmp
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

// mountEssentialFS sets up essential device nodes after chroot isolation.
// Creates minimal /dev entries needed for basic program operation:
//   - /dev/null (null device)

//   - /dev/zero (zero device)
//   - /dev/random, /dev/urandom (entropy devices)
//
// These device nodes are required by most programs and provide secure
// access to kernel functionality within the isolated environment.
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

// createEssentialDevices creates character device nodes in /dev directory.
// Uses mknod system call to create device files with proper major/minor numbers:
//   - /dev/null (1,3) - discards all writes, returns EOF on reads
//   - /dev/zero (1,5) - returns null bytes on reads
//   - /dev/random (1,8) - cryptographically secure random bytes
//   - /dev/urandom (1,9) - pseudorandom bytes (faster than /dev/random)
//
// Ignores errors if devices already exist, logs warnings for creation failures.
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

// Cleanup removes all job filesystem resources after job completion.
// Called from the host system (not within chroot) to clean up:
//   - Unmounts any tmpfs mounts (limited work directories)
//   - Removes job root directory and all contents
//   - Removes job-specific temporary directory
//
// Handles cleanup failures gracefully with warnings rather than errors
// to prevent cleanup issues from affecting other jobs.
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

// validateInJobContext performs final safety validation before chroot execution.
// Critical safety checks to prevent host system corruption:
//   - Confirms JOB_ID environment variable matches expected job ID
//   - Verifies process is PID 1 (isolated PID namespace)
//   - Checks we're not already in a chroot environment
//
// These validations are the last line of defense against accidental
// host system isolation during development or misconfiguration.
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

// mountPipesDirectory enables file upload functionality within the isolated environment.
// Bind mounts the host pipes directory (where server creates upload pipes) into
// the chroot at the same path structure. This allows the init process to access
// named pipes for file uploads while maintaining path consistency.
// Creates directory structure as needed and handles missing pipes gracefully.
// File uploads won't work if this mount fails, but job can still execute.
func (f *JobFilesystem) mountPipesDirectory() error {
	// Get job ID from environment
	jobID := f.platform.Getenv("JOB_ID")
	if jobID == "" {
		f.logger.Debug("no JOB_ID set, skipping pipes mount")
		return nil
	}

	// Host pipes directory (where server creates the pipe)
	hostPipesPath := fmt.Sprintf("%s/%s/pipes", f.config.BaseDir, jobID)

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

// mountVolumes attaches persistent storage volumes to the job environment.
// Iterates through all volumes assigned to the job and bind mounts each one
// from the host volume storage location to /volumes/{name} in the chroot.
// Volumes provide persistent, writable storage that survives job restarts.
// Continues mounting remaining volumes if individual volume mounts fail,
// ensuring partial volume failures don't prevent job execution.
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

// mountSingleVolume performs the mount operation for one specific volume.
// Validates the volume exists on the host, creates the mount point directory
// in the chroot (/volumes/{volumeName}), and bind mounts the volume data.
// Volumes are mounted read-write by default to allow job data persistence.
// Returns error if volume doesn't exist or mount operation fails.
func (f *JobFilesystem) mountSingleVolume(volumeName string) error {
	log := f.logger.WithField("volume", volumeName)

	// Host volume path - this is where the actual volume data lives
	hostVolumePath := fmt.Sprintf("%s/%s/data", f.getVolumesBasePath(), volumeName)

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

// SetVolumes configures which persistent volumes should be mounted for this job.
// Takes a slice of volume names that should be available at /volumes/{name}
// within the job environment. Called by the job execution system before
// filesystem setup to configure volume access.
// Volume names must correspond to existing volumes in the volume store.
func (f *JobFilesystem) SetVolumes(volumes []string) {
	f.Volumes = volumes
	f.logger.Debug("volumes set for job", "volumes", volumes)
}

// loadVolumesFromEnvironment reads volume configuration from environment variables.
// Parses JOB_VOLUMES_COUNT and JOB_VOLUME_{index} environment variables
// to determine which volumes should be mounted for the job.
// Used as fallback when volumes aren't explicitly set via SetVolumes.
// Environment variables are set by the job execution system.
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

// setupLimitedWorkDir creates a size-limited work directory for jobs without volumes.
// Mounts a tmpfs filesystem with configured size limit (default 1MB) to provide
// writable workspace while preventing runaway disk usage.
// The limited workspace is then bind mounted to /work in the chroot.
// Used for jobs that don't have persistent volumes but need some writable space.
// Falls back to unlimited work directory if tmpfs mount fails.
func (f *JobFilesystem) setupLimitedWorkDir() error {
	log := f.logger.WithField("operation", "setup-limited-work")
	log.Debug("setting up limited work directory (1MB) for job without volumes")

	// Create a temporary backing directory for the limited work space
	limitedWorkPath := filepath.Join(f.RootDir, "work-limited")
	if err := f.platform.MkdirAll(limitedWorkPath, 0755); err != nil {
		return fmt.Errorf("failed to create limited work directory: %w", err)
	}

	// Mount tmpfs with configured size limit
	sizeOpt := fmt.Sprintf("size=%d", f.getDefaultDiskQuotaBytes())
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

// makedev creates a device number from major and minor numbers.
// Combines major and minor device numbers into the format expected by mknod.
// Used for creating character device nodes like /dev/null, /dev/zero.
// Linux device number format: major number in high bits, minor in low bits.
func makedev(major, minor uint32) uint64 {
	return uint64(major<<8 | minor)
}

// getVolumesBasePath returns the host directory where volume data is stored.
// Checks JOBLET_VOLUMES_BASE_PATH environment variable first,
// then falls back to default /opt/joblet/volumes location.
// Used to locate volume data directories on the host for bind mounting.
func (f *JobFilesystem) getVolumesBasePath() string {
	if volumesBasePath := f.platform.Getenv("JOBLET_VOLUMES_BASE_PATH"); volumesBasePath != "" {
		return volumesBasePath
	}
	return "/opt/joblet/volumes"
}

// getDefaultDiskQuotaBytes returns the size limit for job work directories.
// Checks JOBLET_DEFAULT_DISK_QUOTA_BYTES environment variable first,
// then falls back to 1MB (1048576 bytes) default.
// Used to limit tmpfs size for jobs without persistent volumes
// to prevent excessive memory usage.
func (f *JobFilesystem) getDefaultDiskQuotaBytes() int64 {
	if diskQuotaStr := f.platform.Getenv("JOBLET_DEFAULT_DISK_QUOTA_BYTES"); diskQuotaStr != "" {
		if quota, err := strconv.ParseInt(diskQuotaStr, 10, 64); err == nil && quota > 0 {
			return quota
		}
	}
	return 1048576 // 1MB default
}
