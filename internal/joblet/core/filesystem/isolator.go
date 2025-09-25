//go:build linux

package filesystem

import (
	"fmt"
	"joblet/pkg/config"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"gopkg.in/yaml.v3"
)

type Isolator struct {
	platform platform.Platform
	config   *config.Config
	logger   *logger.Logger
}

// NewIsolator creates a new filesystem isolator with the given configuration.
// The isolator provides secure filesystem isolation for job execution using
// chroot, bind mounts, and namespace isolation techniques.
// Returns an Isolator instance ready to create isolated job filesystems.
func NewIsolator(cfg *config.Config, platform platform.Platform) *Isolator {
	return &Isolator{
		platform: platform,
		config:   cfg,
		logger:   logger.New().WithField("component", "filesystem-isolator"),
	}
}

// JobFilesystem represents an isolated filesystem for a job
type JobFilesystem struct {
	JobID         string
	RootDir       string
	TmpDir        string
	WorkDir       string
	InitPath      string      // Path to the init binary inside the isolated environment
	Volumes       []string    // Volume names to mount
	Runtime       string      // Runtime specification
	RuntimePath   string      // Path to runtime base directory
	RuntimeConfig interface{} // Runtime configuration data
	IsBuilder     bool        // True for runtime build jobs requiring full host filesystem access
	platform      platform.Platform
	config        *config.Config
	logger        *logger.Logger
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
	jobRootDir := filepath.Join(i.config.Filesystem.BaseDir, jobID)
	jobTmpDir := strings.Replace(i.config.Filesystem.TmpDir, "{JOB_ID}", jobID, -1)
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

// CreateBuilderFilesystem creates filesystem structure for runtime build jobs.
// Builder jobs require access to the full host OS environment (minus /opt/joblet)
// to provide compilation tools and package managers for building runtime environments.
//
// Sets up the directory structure needed for runtime builds:
//   - Job root directory under configured base path (same as regular jobs)
//   - Temporary directory with job ID substitution
//   - Work directory for build files and execution
//
// The key difference from regular jobs is that builder jobs will mount the entire
// host filesystem (excluding /opt/joblet to prevent recursion) instead of minimal binaries.
//
// Returns JobFilesystem instance configured for builder chroot operations.
func (i *Isolator) CreateBuilderFilesystem(jobID string) (*JobFilesystem, error) {
	log := i.logger.WithField("jobID", jobID)
	log.Debug("creating builder filesystem for runtime build job")

	// Create job-specific directories (same structure as regular jobs)
	jobRootDir := filepath.Join(i.config.Filesystem.BaseDir, jobID)
	jobTmpDir := strings.Replace(i.config.Filesystem.TmpDir, "{JOB_ID}", jobID, -1)
	jobWorkDir := filepath.Join(jobRootDir, "work")

	// Ensure we're in a job context (safety check)
	if err := i.validateJobContext(); err != nil {
		return nil, fmt.Errorf("builder filesystem safety check failed: %w", err)
	}

	// Create directory structure
	dirs := []string{jobRootDir, jobTmpDir, jobWorkDir}
	for _, dir := range dirs {
		if err := i.platform.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	filesystem := &JobFilesystem{
		JobID:     jobID,
		RootDir:   jobRootDir,
		TmpDir:    jobTmpDir,
		WorkDir:   jobWorkDir,
		platform:  i.platform,
		config:    i.config,
		logger:    log,
		IsBuilder: true, // Mark as builder filesystem
	}

	log.Debug("builder filesystem structure created",
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

// createBuilderEssentialFiles creates network configuration files in builder chroot environment.
// This runs AFTER chroot, so paths are relative to the chrooted root.
// Sets up DNS resolution needed for package downloads during runtime installation.
//
// NOTE: Builder jobs use bridge networking (same as production jobs) but with enhanced DNS for reliability.
func (f *JobFilesystem) createBuilderEssentialFiles() error {
	f.logger.Debug("creating DNS configuration for builder chroot")

	// Try to use the dedicated chroot resolv.conf if available, otherwise create one
	chrootResolvPath := "/opt/joblet/chroot-resolv.conf"
	if content, err := f.platform.ReadFile(chrootResolvPath); err == nil {
		// Use the dedicated chroot resolv.conf from host
		if err := f.platform.WriteFile("/etc/resolv.conf", content, 0644); err != nil {
			return fmt.Errorf("failed to copy chroot resolv.conf: %w", err)
		}
		f.logger.Debug("using host-provided DNS configuration for builder")
	} else {
		// Fallback: Create /etc/resolv.conf with multiple public DNS servers for reliability
		resolvConf := `# DNS configuration for Joblet builder chroot
# Enhanced DNS for reliable package downloads during runtime installation
nameserver 8.8.8.8
nameserver 8.8.4.4
nameserver 1.1.1.1
nameserver 1.0.0.1
nameserver 208.67.222.222
nameserver 208.67.220.220
search .
options ndots:0 timeout:3 attempts:3 rotate
`
		if err := f.platform.WriteFile("/etc/resolv.conf", []byte(resolvConf), 0644); err != nil {
			return fmt.Errorf("failed to create resolv.conf for builder: %w", err)
		}
		f.logger.Debug("created enhanced DNS configuration for builder job")
	}

	// Create basic /etc/hosts (we're already chrooted)
	hostsContent := `127.0.0.1   localhost
::1         localhost ip6-localhost ip6-loopback
fe00::0     ip6-localnet
ff00::0     ip6-mcastprefix
ff02::1     ip6-allnodes
ff02::2     ip6-allrouters
`
	if err := f.platform.WriteFile("/etc/hosts", []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to create hosts file in builder: %w", err)
	}
	f.logger.Debug("created /etc/hosts file")

	f.logger.Debug("completed DNS configuration for builder chroot")
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
	log.Debug("JobFilesystem.Setup() called", "jobID", f.JobID, "currentVolumes", f.Volumes)

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

	// Mount allowed read-only directories from host FIRST (default minimal chroot)
	if err := f.mountAllowedDirs(); err != nil {
		return fmt.Errorf("failed to mount allowed directories: %w", err)
	}

	// Load runtime information from environment
	f.loadRuntimeFromEnvironment()

	// Mount runtime AFTER allowed directories to overlay runtime-specific files
	// This allows runtime files to override/extend the default minimal chroot
	f.logger.Debug("about to mount runtime", "runtime", f.Runtime)
	if err := f.mountRuntime(); err != nil {
		return fmt.Errorf("failed to mount runtime: %w", err)
	}
	f.logger.Debug("finished mounting runtime", "runtime", f.Runtime)

	// Load volumes from environment if not already set
	f.logger.Debug("checking volume setup", "jobID", f.JobID, "currentVolumes", f.Volumes, "volumeCount", len(f.Volumes))
	if len(f.Volumes) == 0 {
		f.logger.Debug("loading volumes from environment", "jobID", f.JobID)
		f.loadVolumesFromEnvironment()
	} else {
		f.logger.Debug("volumes already set, skipping environment load", "jobID", f.JobID, "volumes", f.Volumes)
	}

	// Mount volumes BEFORE chroot
	f.logger.Debug("about to mount volumes", "jobID", f.JobID, "volumes", f.Volumes, "volumeCount", len(f.Volumes))
	if err := f.mountVolumes(); err != nil {
		return fmt.Errorf("failed to mount volumes: %w", err)
	}
	f.logger.Debug("volume mounting completed", "jobID", f.JobID)

	// If no volumes are mounted, try to set up limited work directory (1MB)
	// BUT skip if work directory already contains uploaded files
	workPath := filepath.Join(f.RootDir, "work")
	workDirHasFiles := false
	if files, err := f.platform.ReadDir(workPath); err == nil && len(files) > 0 {
		workDirHasFiles = true
		log.Debug("work directory contains uploaded files, skipping tmpfs mount", "fileCount", len(files))
	}

	if len(f.Volumes) == 0 && !workDirHasFiles {
		if err := f.setupLimitedWorkDir(); err != nil {
			log.Warn("failed to setup limited work directory, using unlimited work dir", "error", err)
			// Ensure work directory is still accessible
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

// SetupBuilder sets up filesystem isolation for runtime build jobs.
// Builder jobs require access to the full host OS environment to provide
// compilation tools and package managers. This method:
//
//  1. Validates running in job context (safety check)
//  2. Creates basic directory structure
//  3. Mounts entire host filesystem excluding /opt/joblet to prevent recursion
//  4. Bind mounts /opt/joblet/runtimes as read-write for runtime installation
//  5. Sets up isolated /tmp directory
//  6. Performs chroot to builder environment
//  7. Mounts essential filesystems (/proc, /dev)
//
// Returns error if any step fails - build job cannot proceed without proper isolation.
func (f *JobFilesystem) SetupBuilder() error {
	log := f.logger.WithField("operation", "builder-setup")
	log.Debug("setting up builder filesystem isolation")

	// Double-check we're in a job context
	if err := f.validateInJobContext(); err != nil {
		return fmt.Errorf("refusing to setup builder isolation: %w", err)
	}

	// Create basic directory structure
	if err := f.createBuilderDirs(); err != nil {
		return fmt.Errorf("failed to create builder directories: %w", err)
	}

	// Mount host filesystem excluding /opt/joblet
	if err := f.mountHostFilesystem(); err != nil {
		return fmt.Errorf("failed to mount host filesystem: %w", err)
	}

	// Bind mount runtimes directory as read-write
	if err := f.mountRuntimesDirectory(); err != nil {
		return fmt.Errorf("failed to mount runtimes directory: %w", err)
	}

	// Setup /tmp as isolated writable space
	if err := f.setupTmpDir(); err != nil {
		return fmt.Errorf("failed to setup tmp directory: %w", err)
	}

	// Finally, chroot to the builder environment
	if err := f.performChroot(); err != nil {
		return fmt.Errorf("chroot failed: %w", err)
	}

	// Create essential files (DNS configuration) in builder environment
	// Only for builder jobs - regular jobs don't need internet access
	if err := f.createBuilderEssentialFiles(); err != nil {
		log.Warn("failed to create essential files in builder", "error", err)
		// Continue - don't fail the job for DNS issues
	}

	// Mount essential read-only filesystems AFTER chroot
	if err := f.mountEssentialFS(); err != nil {
		return fmt.Errorf("failed to mount essential filesystems: %w", err)
	}

	log.Debug("builder filesystem isolation setup completed successfully")
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
	for _, allowedDir := range f.config.Filesystem.AllowedMounts {
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

		// Create target based on source type (file or directory)
		sourceInfo, err := f.platform.Stat(allowedDir)
		if err != nil {
			f.logger.Warn("failed to stat allowed source", "source", allowedDir, "error", err)
			continue
		}

		if sourceInfo.IsDir() {
			// Source is directory - create target directory
			if err := f.platform.MkdirAll(targetPath, 0755); err != nil {
				f.logger.Warn("failed to create mount target directory", "target", targetPath, "error", err)
				continue
			}
		} else {
			// Source is file - create empty target file (parent already created above)
			if err := f.platform.WriteFile(targetPath, []byte{}, 0644); err != nil {
				f.logger.Warn("failed to create mount target file", "target", targetPath, "error", err)
				continue
			}
		}

		// Bind mount as read-only
		flags := uintptr(syscall.MS_BIND)
		if err := f.platform.Mount(allowedDir, targetPath, "", flags, ""); err != nil {
			// Only log mount failures in debug mode - /etc/hosts missing is common
			if strings.Contains(err.Error(), "/etc/hosts") {
				// Skip logging for /etc/hosts - this is expected when container networking isn't set up yet
			} else {
				f.logger.Debug("failed to mount allowed directory", "source", allowedDir, "target", targetPath, "error", err)
			}
			continue
		}

		// Remount as read-only
		flags = uintptr(syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_RDONLY)
		if err := f.platform.Mount("", targetPath, "", flags, ""); err != nil {
			f.logger.Debug("failed to remount as read-only", "target", targetPath, "error", err)
		}

		// Mounted allowed directory
	}
	return nil
}

// setupTmpDir creates an isolated temporary directory for the job.
// Bind mounts the job-specific temporary directory (from host) to /tmp
// in the isolated environment, providing writable temporary space that
// is automatically cleaned up when the job completes.
// Each job gets its own isolated /tmp to prevent interference.
func (f *JobFilesystem) setupTmpDir() error {
	// Create the job-specific tmp directory on the host
	if err := f.platform.MkdirAll(f.TmpDir, 0755); err != nil {
		return fmt.Errorf("failed to create job tmp directory: %w", err)
	}

	tmpPath := filepath.Join(f.RootDir, "tmp")

	// Create the tmp mount point in the isolated root
	if err := f.platform.MkdirAll(tmpPath, 0755); err != nil {
		return fmt.Errorf("failed to create tmp mount point: %w", err)
	}

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
	workspaceDir := f.config.Filesystem.WorkspaceDir
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
	// Ensure /dev directory exists
	if err := f.platform.MkdirAll("/dev", 0755); err != nil {
		if !f.platform.IsExist(err) {
			return fmt.Errorf("failed to create /dev directory: %w", err)
		}
	}

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

// CreateGPUDeviceNodes creates GPU device nodes in the isolated environment
// Creates device nodes as specified in the design document:
//   - /dev/nvidia0, /dev/nvidia1, etc. (char 195:N) - specific GPU devices
//   - /dev/nvidiactl (char 195:255) - NVIDIA control device
//   - /dev/nvidia-uvm (char 237:0) - Unified Virtual Memory device
func (f *JobFilesystem) CreateGPUDeviceNodes(gpuIndices []int) error {
	log := f.logger.WithField("gpuIndices", gpuIndices)
	log.Debug("creating GPU device nodes")

	// Ensure /dev directory exists
	if err := f.platform.MkdirAll("/dev", 0755); err != nil {
		if !f.platform.IsExist(err) {
			return fmt.Errorf("failed to create /dev directory: %w", err)
		}
	}

	// Create common NVIDIA devices needed by all GPU jobs
	commonDevices := []struct {
		path  string
		major int
		minor int
	}{
		{"/dev/nvidiactl", 195, 255}, // NVIDIA control device
		{"/dev/nvidia-uvm", 237, 0},  // Unified Virtual Memory
	}

	for _, device := range commonDevices {
		if err := syscall.Mknod(device.path, syscall.S_IFCHR|0666, int(makedev(uint32(device.major), uint32(device.minor)))); err != nil {
			if !f.platform.IsExist(err) {
				log.Warn("failed to create common GPU device", "device", device.path, "error", err)
			} else {
				log.Debug("common GPU device already exists", "device", device.path)
			}
		} else {
			log.Debug("created common GPU device", "device", device.path)
		}
	}

	// Create specific GPU device nodes for allocated GPUs
	for _, gpuIndex := range gpuIndices {
		devicePath := fmt.Sprintf("/dev/nvidia%d", gpuIndex)

		if err := syscall.Mknod(devicePath, syscall.S_IFCHR|0666, int(makedev(195, uint32(gpuIndex)))); err != nil {
			if !f.platform.IsExist(err) {
				log.Warn("failed to create GPU device node", "device", devicePath, "gpuIndex", gpuIndex, "error", err)
			} else {
				log.Debug("GPU device node already exists", "device", devicePath, "gpuIndex", gpuIndex)
			}
		} else {
			log.Debug("created GPU device node", "device", devicePath, "gpuIndex", gpuIndex)
		}
	}

	log.Info("GPU device nodes setup completed", "gpuCount", len(gpuIndices))
	return nil
}

// MountCUDALibraries mounts CUDA libraries and binaries into the isolated environment
// Mounts CUDA installation directories as read-only bind mounts according to the design:
//   - /usr/local/cuda/lib64 → {chroot}/usr/local/cuda/lib64
//   - /usr/local/cuda/bin → {chroot}/usr/local/cuda/bin
//   - /usr/local/cuda/include → {chroot}/usr/local/cuda/include
func (f *JobFilesystem) MountCUDALibraries(cudaPath string, mountPaths []string) error {
	log := f.logger.WithFields("cudaPath", cudaPath, "mountPaths", mountPaths)
	log.Debug("mounting CUDA libraries")

	for _, hostPath := range mountPaths {
		// Skip if the host path doesn't exist
		if _, err := f.platform.Stat(hostPath); f.platform.IsNotExist(err) {
			log.Debug("skipping non-existent CUDA path", "path", hostPath)
			continue
		}

		// Determine target path in chroot - preserve the full path structure
		targetPath := filepath.Join(f.RootDir, strings.TrimPrefix(hostPath, "/"))

		// Create parent directories
		targetDir := filepath.Dir(targetPath)
		if err := f.platform.MkdirAll(targetDir, 0755); err != nil {
			log.Warn("failed to create CUDA mount parent directory", "dir", targetDir, "error", err)
			continue
		}

		// Create target directory/file based on source type
		sourceInfo, err := f.platform.Stat(hostPath)
		if err != nil {
			log.Warn("failed to stat CUDA source", "source", hostPath, "error", err)
			continue
		}

		if sourceInfo.IsDir() {
			// Source is directory - create target directory
			if err := f.platform.MkdirAll(targetPath, 0755); err != nil {
				log.Warn("failed to create CUDA target directory", "target", targetPath, "error", err)
				continue
			}
		} else {
			// Source is file - create empty target file
			if err := f.platform.WriteFile(targetPath, []byte{}, 0644); err != nil {
				log.Warn("failed to create CUDA target file", "target", targetPath, "error", err)
				continue
			}
		}

		// Bind mount as read-only
		flags := uintptr(syscall.MS_BIND)
		if err := f.platform.Mount(hostPath, targetPath, "", flags, ""); err != nil {
			log.Warn("failed to mount CUDA path", "source", hostPath, "target", targetPath, "error", err)
			continue
		}

		// Remount as read-only
		flags = uintptr(syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_RDONLY)
		if err := f.platform.Mount("", targetPath, "", flags, ""); err != nil {
			log.Debug("failed to remount CUDA path as read-only", "target", targetPath, "error", err)
		}

		log.Debug("mounted CUDA path", "source", hostPath, "target", targetPath)
	}

	log.Info("CUDA library mounting completed", "cudaPath", cudaPath, "mountedPaths", len(mountPaths))
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
	hostPipesPath := fmt.Sprintf("%s/%s/pipes", f.config.Filesystem.BaseDir, jobID)

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

	// Target path inside chroot - mount pipes at /pipes instead of /opt/joblet/...
	// since /opt is read-only in runtime mounts
	targetPipesPath := filepath.Join(f.RootDir, "pipes", jobID)

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
	// Check if volumes need mounting
	if len(f.Volumes) == 0 {
		// No volumes to mount
		return nil
	}

	// Proceeding to mount volumes
	log := f.logger.WithField("operation", "mount-volumes")
	log.Debug("mounting volumes", "count", len(f.Volumes))

	for _, volumeName := range f.Volumes {
		if err := f.mountSingleVolume(volumeName); err != nil {
			log.Warn("failed to mount volume", "volume", volumeName, "error", err)
			// Continue with other volumes, don't fail the entire job
			continue
		}
	}

	if len(f.Volumes) > 0 {
		log.Debug("volume mounting completed", "count", len(f.Volumes))
	}
	return nil
}

// mountSingleVolume performs the mount operation for one specific volume.
// Validates the volume exists on the host, creates the mount point directory
// in the chroot (/volumes/{volumeName}), and bind mounts the volume data.
// Volumes are mounted read-write by default to allow job data persistence.
// Returns error if volume doesn't exist or mount operation fails.
func (f *JobFilesystem) mountSingleVolume(volumeName string) error {
	log := f.logger.WithField("volume", volumeName)
	// Mounting volume

	// Host volume path - this is where the actual volume data lives
	hostVolumePath := fmt.Sprintf("%s/%s/data", f.getVolumesBasePath(), volumeName)
	// Check volume path

	// Check if host volume directory exists
	// Verify volume exists
	_, err := f.platform.Stat(hostVolumePath)
	// Volume existence check completed
	if err != nil {
		if f.platform.IsNotExist(err) {
			log.Warn("volume does not exist", "hostVolumePath", hostVolumePath, "volumeName", volumeName)
			return fmt.Errorf("volume %s does not exist at %s", volumeName, hostVolumePath)
		}
		log.Error("failed to stat volume directory", "error", err, "hostVolumePath", hostVolumePath)
		return fmt.Errorf("failed to stat volume directory: %w", err)
	}
	// Volume exists, mounting

	// Target path inside chroot - mount volumes under /volumes/{name}
	targetVolumePath := filepath.Join(f.RootDir, "volumes", volumeName)
	log.Debug("creating target volume path", "targetVolumePath", targetVolumePath)

	// Create the mount point directory
	if err := f.platform.MkdirAll(targetVolumePath, 0755); err != nil {
		log.Error("failed to create volume mount point", "error", err, "targetVolumePath", targetVolumePath)
		return fmt.Errorf("failed to create volume mount point: %w", err)
	}
	log.Debug("target volume path created successfully", "targetVolumePath", targetVolumePath)

	// Bind mount the volume (read-write by default)
	flags := uintptr(syscall.MS_BIND)
	log.Debug("performing bind mount", "hostPath", hostVolumePath, "targetPath", targetVolumePath, "flags", flags)
	if err := f.platform.Mount(hostVolumePath, targetVolumePath, "", flags, ""); err != nil {
		log.Error("failed to bind mount volume", "error", err, "hostPath", hostVolumePath, "targetPath", targetVolumePath)
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
	f.logger.Debug("checking for volume environment variables", "JOB_VOLUMES_COUNT", volumeCountStr, "jobID", f.JobID)
	if volumeCountStr == "" {
		f.logger.Debug("no volume environment variables found - volumes will not be mounted", "jobID", f.JobID)
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
		envVar := fmt.Sprintf("JOB_VOLUME_%d", i)
		volumeName := f.platform.Getenv(envVar)
		f.logger.Debug("checking volume environment variable", "envVar", envVar, "volumeName", volumeName, "jobID", f.JobID)
		if volumeName != "" {
			volumes = append(volumes, volumeName)
		}
	}

	f.Volumes = volumes
	f.logger.Debug("loaded volumes from environment", "volumes", volumes, "volumeCount", len(volumes), "jobID", f.JobID)
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

// loadRuntimeFromEnvironment reads runtime configuration from environment variables
func (f *JobFilesystem) loadRuntimeFromEnvironment() {
	f.Runtime = f.platform.Getenv("JOB_RUNTIME")
	f.RuntimePath = f.platform.Getenv("JOB_RUNTIME_PATH")
	f.logger.Debug("attempting to load runtime from environment", "JOB_RUNTIME", f.Runtime, "JOB_RUNTIME_PATH", f.RuntimePath)
	if f.Runtime != "" {
		f.logger.Debug("loaded runtime from environment", "runtime", f.Runtime, "path", f.RuntimePath)
	} else {
		f.logger.Debug("no runtime specified in environment")
	}
}

// mountRuntime mounts the runtime directories if runtime is specified
func (f *JobFilesystem) mountRuntime() error {
	if f.Runtime == "" {
		f.logger.Debug("no runtime specified, skipping runtime mount")
		return nil
	}

	log := f.logger.WithField("runtime", f.Runtime)
	log.Debug("mounting runtime for job")

	// Check if we have a runtime manager available through environment
	runtimeManagerPath := f.platform.Getenv("RUNTIME_MANAGER_PATH")
	if runtimeManagerPath == "" {
		// Try default runtime path
		runtimeManagerPath = f.config.Runtime.BasePath
	}

	// Create runtime manager to resolve and mount runtime
	if err := f.mountRuntimeWithManager(runtimeManagerPath); err != nil {
		return fmt.Errorf("failed to mount runtime %s: %w", f.Runtime, err)
	}

	log.Debug("runtime mounted", "runtime", f.Runtime, "jobID", f.JobID)
	return nil
}

// mountRuntimeWithManager uses the runtime manager to mount runtime
func (f *JobFilesystem) mountRuntimeWithManager(runtimeBasePath string) error {
	// Import the runtime manager types here
	type RuntimeMount struct {
		Source    string   `yaml:"source"`
		Target    string   `yaml:"target"`
		ReadOnly  bool     `yaml:"readonly"`
		Selective []string `yaml:"selective"`
	}

	type RuntimeConfig struct {
		Name        string            `yaml:"name"`
		Mounts      []RuntimeMount    `yaml:"mounts"`
		Environment map[string]string `yaml:"environment"`
	}

	// Parse runtime spec and find runtime directory
	// Runtime name directly maps to directory name (e.g., "openjdk-21" -> "/opt/joblet/runtimes/openjdk-21")
	runtimeDirName := f.Runtime
	runtimeDir := filepath.Join(runtimeBasePath, runtimeDirName)

	// Load runtime.yml file
	configPath := filepath.Join(runtimeDir, "runtime.yml")
	configData, err := f.platform.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read runtime config: %w", err)
	}

	// Parse YAML configuration using standard YAML library
	var config RuntimeConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse runtime config: %w", err)
	}

	// If no mounts were parsed, this is an error - runtime.yml is malformed
	if len(config.Mounts) == 0 {
		f.logger.Warn("no mounts found in runtime.yml, falling back to simple mount", "runtimeDir", runtimeDir)
		// Fall back to mounting the entire runtime dir
		targetPath := f.RootDir
		flags := uintptr(syscall.MS_BIND)
		if err := f.platform.Mount(runtimeDir, targetPath, "", flags, ""); err != nil {
			return fmt.Errorf("failed to mount runtime dir %s to %s: %w", runtimeDir, targetPath, err)
		}
		f.logger.Debug("mounted runtime path", "target", targetPath)
		return nil
	}

	// Mount each directory according to runtime config
	f.logger.Debug("mounting runtime paths", "numMounts", len(config.Mounts), "rootDir", f.RootDir, "parsedMounts", config.Mounts)

	// Phase 1: Create all mount point directories first (before any mounting)
	// This prevents read-only mount issues where parent dirs can't be modified after being mounted
	for _, mount := range config.Mounts {
		sourcePath := filepath.Join(runtimeDir, mount.Source)
		targetPath := filepath.Join(f.RootDir, strings.TrimPrefix(mount.Target, "/"))

		f.logger.Debug("preparing mount point", "source", sourcePath, "target", targetPath, "mount", mount)

		// Check if source exists
		sourceInfo, err := f.platform.Stat(sourcePath)
		if err != nil {
			if f.platform.IsNotExist(err) {
				f.logger.Debug("skipping non-existent runtime source", "path", sourcePath)
				continue
			}
			return fmt.Errorf("failed to stat runtime source %s: %w", sourcePath, err)
		}

		// Skip empty directories to avoid mount issues
		if sourceInfo.IsDir() {
			entries, err := os.ReadDir(sourcePath)
			if err != nil {
				return fmt.Errorf("failed to read runtime source directory %s: %w", sourcePath, err)
			}
			if len(entries) == 0 {
				f.logger.Debug("skipping empty runtime source directory", "path", sourcePath)
				continue
			}
		}

		// Create target based on source type (file or directory)
		// sourceInfo already obtained above

		if sourceInfo.IsDir() {
			// Source is directory - check if target exists, if not try to create it
			if _, statErr := f.platform.Stat(targetPath); statErr != nil {
				// Target doesn't exist, try to create it
				if err := f.platform.MkdirAll(targetPath, 0755); err != nil {
					f.logger.Warn("failed to create runtime target directory, will skip mount", "target", targetPath, "error", err)
					continue // Skip this mount instead of failing
				}
				f.logger.Debug("created target directory for runtime mount", "target", targetPath)
			} else {
				f.logger.Debug("target directory already exists, will bind mount over it", "target", targetPath)
			}
		} else {
			// Source is file - create parent directory and touch target file
			parentDir := filepath.Dir(targetPath)
			if err := f.platform.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create parent directory for runtime target %s: %w", parentDir, err)
			}

			// Create empty target file
			if err := f.platform.WriteFile(targetPath, []byte{}, 0755); err != nil {
				return fmt.Errorf("failed to create runtime target file %s: %w", targetPath, err)
			}
			f.logger.Debug("created target file for runtime mount", "target", targetPath)
		}
	}

	// Phase 2: Perform all mounts after directories are created
	for _, mount := range config.Mounts {
		sourcePath := filepath.Join(runtimeDir, mount.Source)
		targetPath := filepath.Join(f.RootDir, strings.TrimPrefix(mount.Target, "/"))

		// Skip if source doesn't exist or is empty (already checked in phase 1)
		sourceInfo, err := f.platform.Stat(sourcePath)
		if err != nil {
			if f.platform.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("failed to stat runtime source %s: %w", sourcePath, err)
		}

		// Skip empty directories (same logic as phase 1)
		if sourceInfo.IsDir() {
			entries, err := os.ReadDir(sourcePath)
			if err != nil {
				return fmt.Errorf("failed to read runtime source directory %s: %w", sourcePath, err)
			}
			if len(entries) == 0 {
				continue
			}
		}

		// Check if target directory exists before mounting
		if _, statErr := f.platform.Stat(targetPath); statErr != nil {
			f.logger.Warn("skipping runtime mount because target directory doesn't exist", "target", targetPath)
			continue
		}

		// Bind mount
		flags := uintptr(syscall.MS_BIND)
		if err := f.platform.Mount(sourcePath, targetPath, "", flags, ""); err != nil {
			return fmt.Errorf("failed to mount %s to %s: %w", sourcePath, targetPath, err)
		}

		// Remount as read-only if specified
		if mount.ReadOnly {
			flags = uintptr(syscall.MS_BIND | syscall.MS_REMOUNT | syscall.MS_RDONLY)
			if err := f.platform.Mount("", targetPath, "", flags, ""); err != nil {
				f.logger.Debug("failed to remount as read-only", "target", targetPath, "error", err)
			}
		}

		f.logger.Debug("mounted runtime path", "target", targetPath, "readonly", mount.ReadOnly)
	}

	return nil
}

// createBuilderDirs creates the basic directory structure for builder chroot.
// Creates essential directories and mount points for the full host filesystem.
// Adapts to different Linux distributions by detecting existing directories.
func (f *JobFilesystem) createBuilderDirs() error {
	log := f.logger.WithField("operation", "create-builder-dirs")

	// Core directories present on all Linux distributions (FHS standard)
	coreBuilderDirs := []string{
		"bin", "sbin", "lib", "usr", "etc", "var", "home", "root",
		"proc", "dev", "sys", "tmp", "work", "opt",
	}

	// Additional directories that may exist on modern distributions
	optionalBuilderDirs := []string{
		"lib64", // 64-bit library directory (RHEL, Ubuntu x86_64, Amazon Linux)
		"lib32", // 32-bit compatibility libraries (some Ubuntu, RHEL)
		"run",   // Runtime data directory (systemd-based distributions)
		"srv",   // Service data directory (FHS standard)
		"media", // Removable media mount points
		"mnt",   // Temporary mount points
		"boot",  // Boot loader files (rarely needed for builds)
	}

	// Create core directories (always needed)
	for _, dir := range coreBuilderDirs {
		dirPath := filepath.Join(f.RootDir, dir)
		if err := f.platform.MkdirAll(dirPath, 0755); err != nil {
			log.Warn("failed to create core builder directory", "dir", dir, "error", err)
			// Continue with other directories
		}
	}

	// Create optional directories only if they exist on the host system
	for _, dir := range optionalBuilderDirs {
		hostPath := "/" + dir
		if _, err := f.platform.Stat(hostPath); err == nil {
			// Directory exists on host, create it in builder chroot
			dirPath := filepath.Join(f.RootDir, dir)
			if err := f.platform.MkdirAll(dirPath, 0755); err != nil {
				log.Warn("failed to create optional builder directory", "dir", dir, "error", err)
			} else {
				log.Debug("created optional builder directory", "dir", dir)
			}
		} else {
			log.Debug("skipping optional directory not present on host", "dir", dir)
		}
	}

	log.Debug("created adaptive builder directory structure for distribution")
	return nil
}

// mountHostFilesystem mounts the entire host filesystem excluding /opt/joblet.
// This provides the full OS environment needed for compilation and builds.
func (f *JobFilesystem) mountHostFilesystem() error {
	log := f.logger.WithField("operation", "mount-host-filesystem")

	// Read host root directory
	hostEntries, err := f.platform.ReadDir("/")
	if err != nil {
		return fmt.Errorf("failed to read host root directory: %w", err)
	}

	for _, entry := range hostEntries {
		dirName := entry.Name()
		hostPath := "/" + dirName
		targetPath := filepath.Join(f.RootDir, dirName)

		// Skip special filesystems and /opt/joblet to prevent recursion
		switch dirName {
		case "proc", "sys", "dev":
			log.Debug("skipping special filesystem", "dir", dirName)
			continue
		case "opt":
			// Special handling for /opt - mount everything except /opt/joblet
			if err := f.mountOptDirectory(hostPath, targetPath); err != nil {
				log.Warn("failed to mount /opt directory", "error", err)
			}
			continue
		case "tmp":
			// Handle tmp specially - it will be isolated
			log.Debug("skipping /tmp - will be isolated", "dir", dirName)
			continue
		}

		// Create target directory
		if err := f.platform.MkdirAll(targetPath, 0755); err != nil {
			log.Warn("failed to create target directory", "target", targetPath, "error", err)
			continue
		}

		// Bind mount host directory
		if err := f.platform.Mount(hostPath, targetPath, "", uintptr(syscall.MS_BIND), ""); err != nil {
			log.Warn("failed to bind mount host directory", "host", hostPath, "target", targetPath, "error", err)
			continue
		}

		log.Debug("mounted host directory", "host", hostPath, "target", targetPath)
	}

	return nil
}

// mountOptDirectory mounts /opt directory excluding /opt/joblet to prevent recursion
func (f *JobFilesystem) mountOptDirectory(hostOptPath, targetOptPath string) error {
	log := f.logger.WithField("operation", "mount-opt-directory")

	// Create /opt target directory
	if err := f.platform.MkdirAll(targetOptPath, 0755); err != nil {
		return fmt.Errorf("failed to create /opt target directory: %w", err)
	}

	// Read /opt directory contents
	optEntries, err := f.platform.ReadDir(hostOptPath)
	if err != nil {
		return fmt.Errorf("failed to read /opt directory: %w", err)
	}

	for _, entry := range optEntries {
		dirName := entry.Name()

		// Skip joblet directory to prevent recursion
		if dirName == "joblet" {
			log.Debug("skipping /opt/joblet to prevent recursion")
			continue
		}

		hostPath := filepath.Join(hostOptPath, dirName)
		targetPath := filepath.Join(targetOptPath, dirName)

		// Create target directory
		if err := f.platform.MkdirAll(targetPath, 0755); err != nil {
			log.Warn("failed to create /opt target", "target", targetPath, "error", err)
			continue
		}

		// Bind mount
		if err := f.platform.Mount(hostPath, targetPath, "", uintptr(syscall.MS_BIND), ""); err != nil {
			log.Warn("failed to bind mount /opt subdirectory", "host", hostPath, "target", targetPath, "error", err)
			continue
		}

		log.Debug("mounted /opt subdirectory", "host", hostPath, "target", targetPath)
	}

	return nil
}

// mountRuntimesDirectory bind mounts /opt/joblet/runtimes as read-write for builds
func (f *JobFilesystem) mountRuntimesDirectory() error {
	log := f.logger.WithField("operation", "mount-runtimes-directory")

	hostRuntimesPath := f.config.Runtime.BasePath

	// Create /opt/joblet/runtimes directory in builder chroot
	targetRuntimesPath := filepath.Join(f.RootDir, "opt", "joblet", "runtimes")
	if err := f.platform.MkdirAll(targetRuntimesPath, 0755); err != nil {
		return fmt.Errorf("failed to create runtimes target directory: %w", err)
	}

	// Bind mount as read-write for runtime installation
	if err := f.platform.Mount(hostRuntimesPath, targetRuntimesPath, "", uintptr(syscall.MS_BIND), ""); err != nil {
		return fmt.Errorf("failed to bind mount runtimes directory: %w", err)
	}

	log.Debug("mounted runtimes directory", "host", hostRuntimesPath, "target", targetRuntimesPath)
	return nil
}
