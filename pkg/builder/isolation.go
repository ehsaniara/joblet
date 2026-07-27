//go:build linux

package builder

import (
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
)

// IsolatedEnvironment manages an OverlayFS-based chroot for isolated package installation
type IsolatedEnvironment struct {
	// Directories
	baseDir   string // Base directory for all overlay components
	upperDir  string // Upper layer (captures writes)
	workDir   string // Work directory (required by overlayfs)
	mergedDir string // Merged view (the chroot)
	// Note: lowerDir is always "/" (host root, read-only) - not stored as field

	// State
	mounted bool
	logger  BuildLogger

	// System operations interface (for testing)
	sysOps SystemOps
}

// NewIsolatedEnvironment creates a new isolated environment for package installation
func NewIsolatedEnvironment(baseDir string, logger BuildLogger) (*IsolatedEnvironment, error) {
	return NewIsolatedEnvironmentWithOps(baseDir, logger, NewRealSystemOps())
}

// NewIsolatedEnvironmentWithOps creates a new isolated environment with custom system operations
// This is useful for testing with mocked system operations
func NewIsolatedEnvironmentWithOps(baseDir string, logger BuildLogger, sysOps SystemOps) (*IsolatedEnvironment, error) {
	if logger == nil {
		logger = NewBuildLogger(false)
	}
	if sysOps == nil {
		sysOps = NewRealSystemOps()
	}

	env := &IsolatedEnvironment{
		baseDir:   baseDir,
		upperDir:  filepath.Join(baseDir, "upper"),
		workDir:   filepath.Join(baseDir, "work"),
		mergedDir: filepath.Join(baseDir, "merged"),
		logger:    logger,
		sysOps:    sysOps,
	}

	return env, nil
}

// Setup creates the overlay filesystem for isolated package installation
func (e *IsolatedEnvironment) Setup() error {
	e.logger.Info("Setting up isolated build environment with OverlayFS")

	// Create directories
	dirs := []string{e.upperDir, e.workDir, e.mergedDir}
	for _, dir := range dirs {
		if err := e.sysOps.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Mount overlayfs
	// lowerdir=/ (host root, read-only)
	// upperdir=upper (captures all writes)
	// workdir=work (required by overlayfs)
	opts := fmt.Sprintf("lowerdir=/,upperdir=%s,workdir=%s", e.upperDir, e.workDir)

	if err := e.sysOps.Mount("overlay", e.mergedDir, "overlay", 0, opts); err != nil {
		return fmt.Errorf("failed to mount overlayfs: %w (are you running as root?)", err)
	}
	e.mounted = true

	e.logger.Debug("OverlayFS mounted at %s", e.mergedDir)

	// Mount essential filesystems inside the overlay
	if err := e.mountEssentialFS(); err != nil {
		_ = e.Cleanup() // Best effort cleanup, ignore error
		return fmt.Errorf("failed to mount essential filesystems: %w", err)
	}

	// Setup DNS resolution - this is critical for package installation
	if err := e.setupDNS(); err != nil {
		_ = e.Cleanup()
		return fmt.Errorf("failed to setup DNS resolution: %w (pip/npm installs require network access)", err)
	}

	// Verify DNS is working by testing resolution
	if err := e.verifyDNS(); err != nil {
		_ = e.Cleanup()
		return fmt.Errorf("DNS verification failed: %w (check network connectivity and /etc/resolv.conf)", err)
	}

	e.logger.Info("Isolated environment ready")
	return nil
}

// verifyDNS tests that DNS resolution works inside the chroot
func (e *IsolatedEnvironment) verifyDNS() error {
	e.logger.Debug("Verifying DNS resolution in isolated environment")

	// Try to resolve a well-known domain using getent or nslookup
	// We use getent as it's more commonly available
	output, err := e.RunInChroot("getent", "hosts", "google.com")
	if err != nil {
		// Try nslookup as fallback
		output, err = e.RunInChroot("nslookup", "google.com")
		if err != nil {
			// Try a simple ping (just DNS resolution, not full ping)
			output, err = e.RunInChroot("host", "google.com")
			if err != nil {
				return fmt.Errorf("cannot resolve DNS names - tried getent, nslookup, host: %s", string(output))
			}
		}
	}

	e.logger.Debug("DNS resolution verified: %s", strings.TrimSpace(string(output)))
	return nil
}

// mountEssentialFS mounts /proc, /sys, /dev inside the overlay
func (e *IsolatedEnvironment) mountEssentialFS() error {
	// Mount /proc
	procPath := filepath.Join(e.mergedDir, "proc")
	if err := e.sysOps.MkdirAll(procPath, 0755); err != nil {
		return fmt.Errorf("failed to create /proc: %w", err)
	}
	if err := e.sysOps.Mount("proc", procPath, "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount /proc: %w", err)
	}

	// Mount /sys (read-only)
	sysPath := filepath.Join(e.mergedDir, "sys")
	if err := e.sysOps.MkdirAll(sysPath, 0755); err != nil {
		return fmt.Errorf("failed to create /sys: %w", err)
	}
	if err := e.sysOps.Mount("sysfs", sysPath, "sysfs", syscall.MS_RDONLY, ""); err != nil {
		e.logger.Warn("Failed to mount /sys: %v", err)
		// Non-fatal - continue
	}

	// Bind mount /dev
	devPath := filepath.Join(e.mergedDir, "dev")
	if err := e.sysOps.MkdirAll(devPath, 0755); err != nil {
		return fmt.Errorf("failed to create /dev: %w", err)
	}
	if err := e.sysOps.Mount("/dev", devPath, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to bind mount /dev: %w", err)
	}

	return nil
}

// setupDNS copies resolv.conf for DNS resolution inside the chroot
func (e *IsolatedEnvironment) setupDNS() error {
	// The overlay already has /etc/resolv.conf from the host
	// But on EC2/systemd systems, it's often a symlink to /run/systemd/resolve/stub-resolv.conf
	// which doesn't exist in the overlay. We need to replace it with a real file.
	resolvPath := filepath.Join(e.mergedDir, "etc", "resolv.conf")

	// Check if resolv.conf exists and is readable (not a broken symlink)
	if content, err := e.sysOps.ReadFile(resolvPath); err == nil && len(content) > 0 {
		return nil // Already exists and readable from overlay
	}

	// Create /etc if needed
	etcPath := filepath.Join(e.mergedDir, "etc")
	if err := e.sysOps.MkdirAll(etcPath, 0755); err != nil {
		return fmt.Errorf("failed to create /etc: %w", err)
	}

	// Remove any existing broken symlink (common on EC2/systemd systems)
	// On systemd systems, /etc/resolv.conf is often a symlink to:
	// /run/systemd/resolve/stub-resolv.conf (which doesn't exist in overlay)
	_ = e.sysOps.RemoveAll(resolvPath) // Ignore error if doesn't exist

	// Read DNS config from host (follow symlinks to get actual content)
	content, err := e.sysOps.ReadFile("/etc/resolv.conf")
	if err != nil {
		// Create a fallback with public DNS servers
		e.logger.Debug("Could not read host /etc/resolv.conf, using fallback DNS servers")
		content = []byte("nameserver 8.8.8.8\nnameserver 8.8.4.4\n")
	}

	if err := e.sysOps.WriteFile(resolvPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write resolv.conf: %w", err)
	}

	e.logger.Debug("DNS configured in isolated environment")
	return nil
}

// RunInChroot executes a command inside the isolated chroot environment
func (e *IsolatedEnvironment) RunInChroot(name string, args ...string) ([]byte, error) {
	if !e.mounted {
		return nil, fmt.Errorf("isolated environment not mounted")
	}

	// Use chroot to run the command
	chrootArgs := append([]string{e.mergedDir, name}, args...)
	cmd := e.sysOps.Command("chroot", chrootArgs...)

	// Set environment for package managers
	cmd.SetEnv([]string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"DEBIAN_FRONTEND=noninteractive",
		"LANG=C.UTF-8",
	})

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("chroot command failed: %w\nOutput: %s", err, string(output))
	}

	return output, nil
}

// AddDeadsnakesPPA adds the deadsnakes PPA for newer Python versions on Ubuntu
// IMPORTANT: This runs inside the OverlayFS chroot - the host system is NOT modified.
// The PPA is added to the overlay's upper layer which is discarded after build.
func (e *IsolatedEnvironment) AddDeadsnakesPPA() error {
	if !e.mounted {
		return fmt.Errorf("cannot add PPA: isolated environment not mounted")
	}

	e.logger.Info("Adding deadsnakes PPA in isolated environment (host is protected by OverlayFS)")
	e.logger.Debug("PPA changes will be written to overlay upper layer: %s", e.upperDir)

	// Install software-properties-common for add-apt-repository
	// This runs inside chroot, so it installs to the overlay, not the host
	if output, err := e.RunInChroot("apt-get", "install", "-y", "software-properties-common"); err != nil {
		return fmt.Errorf("failed to install software-properties-common: %w\nOutput: %s", err, string(output))
	}

	// Add deadsnakes PPA inside the chroot
	// The PPA file will be created in the overlay's upper layer at:
	// {upperDir}/etc/apt/sources.list.d/deadsnakes-ubuntu-ppa-*.list
	if output, err := e.RunInChroot("add-apt-repository", "-y", "ppa:deadsnakes/ppa"); err != nil {
		return fmt.Errorf("failed to add deadsnakes PPA: %w\nOutput: %s", err, string(output))
	}

	e.logger.Info("Deadsnakes PPA added successfully (isolated, host unchanged)")
	return nil
}

// needsDeadsnakesPPA checks if any package requires the deadsnakes PPA
func needsDeadsnakesPPA(packages []string) bool {
	for _, pkg := range packages {
		// Check for python3.X packages where X >= 10 (python3.10, python3.11, python3.12, etc.)
		if strings.HasPrefix(pkg, "python3.1") || strings.HasPrefix(pkg, "python3.2") {
			return true
		}
	}
	return false
}

// InstallPackagesIsolated installs system packages inside the isolated environment
func (e *IsolatedEnvironment) InstallPackagesIsolated(pkgManager string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	e.logger.Info("Installing packages in isolated environment: %s", strings.Join(packages, ", "))

	switch pkgManager {
	case "apt":
		// Update package list - this is required for fresh installs
		e.logger.Debug("Running apt-get update in chroot")
		if output, err := e.RunInChroot("apt-get", "update", "-qq"); err != nil {
			return fmt.Errorf("apt-get update failed: %w\nOutput: %s\n\nThis usually means:\n  - Network connectivity issues\n  - DNS resolution problems\n  - Invalid apt sources in /etc/apt/sources.list", err, string(output))
		}

		// Check if we need deadsnakes PPA for Python packages
		if needsDeadsnakesPPA(packages) {
			// Try to install packages first - if it fails, add PPA and retry
			args := append([]string{"install", "-y", "--no-install-recommends"}, packages...)
			if _, err := e.RunInChroot("apt-get", args...); err != nil {
				e.logger.Info("Package not found in default repos, adding deadsnakes PPA...")
				if err := e.AddDeadsnakesPPA(); err != nil {
					return fmt.Errorf("failed to add deadsnakes PPA: %w", err)
				}
				// Update after adding PPA
				if output, err := e.RunInChroot("apt-get", "update", "-qq"); err != nil {
					return fmt.Errorf("apt-get update after PPA failed: %w\nOutput: %s", err, string(output))
				}
			} else {
				// First attempt succeeded
				return nil
			}
		}

		// Install packages
		args := append([]string{"install", "-y", "--no-install-recommends"}, packages...)
		output, err := e.RunInChroot("apt-get", args...)
		if err != nil {
			return fmt.Errorf("apt-get install failed: %w\nOutput: %s", err, string(output))
		}
		e.logger.Debug("apt-get install output: %s", string(output))

	case "yum":
		// Clean and update yum cache
		e.logger.Debug("Running yum makecache in chroot")
		if output, err := e.RunInChroot("yum", "makecache", "-q"); err != nil {
			return fmt.Errorf("yum makecache failed: %w\nOutput: %s\n\nThis usually means:\n  - Network connectivity issues\n  - DNS resolution problems\n  - Invalid yum repositories", err, string(output))
		}

		args := append([]string{"install", "-y"}, packages...)
		output, err := e.RunInChroot("yum", args...)
		if err != nil {
			return fmt.Errorf("yum install failed: %w\nOutput: %s", err, string(output))
		}
		e.logger.Debug("yum install output: %s", string(output))

	case "dnf":
		// Clean and update dnf cache
		e.logger.Debug("Running dnf makecache in chroot")
		if output, err := e.RunInChroot("dnf", "makecache", "-q"); err != nil {
			return fmt.Errorf("dnf makecache failed: %w\nOutput: %s\n\nThis usually means:\n  - Network connectivity issues\n  - DNS resolution problems\n  - Invalid dnf repositories", err, string(output))
		}

		args := append([]string{"install", "-y"}, packages...)
		output, err := e.RunInChroot("dnf", args...)
		if err != nil {
			return fmt.Errorf("dnf install failed: %w\nOutput: %s", err, string(output))
		}
		e.logger.Debug("dnf install output: %s", string(output))

	default:
		return fmt.Errorf("unsupported package manager: %s", pkgManager)
	}

	e.logger.Info("Packages installed successfully in isolated environment")
	return nil
}

// InstallPipPackagesIsolated installs Python packages using pip inside the isolated environment
// This ensures we use the Python/pip that was installed in phase 7, not the host's Python
func (e *IsolatedEnvironment) InstallPipPackagesIsolated(packages []string, pipOptions string, pythonVersion string) error {
	if len(packages) == 0 {
		return nil
	}

	e.logger.Info("Installing pip packages in isolated environment: %s", strings.Join(packages, ", "))

	// Build the pip install command
	// Use python3.X -m pip to ensure we use the correct Python version
	pythonBinary := fmt.Sprintf("python%s", pythonVersion)

	// First, verify Python is available in the isolated environment
	e.logger.Debug("Verifying Python is available in isolated environment")
	if output, err := e.RunInChroot(pythonBinary, "--version"); err != nil {
		return fmt.Errorf("python %s not found in isolated environment: %w\nOutput: %s\n\nMake sure the base Python package was installed in phase 7", pythonVersion, err, string(output))
	}

	// Ensure pip is available
	e.logger.Debug("Ensuring pip is available in isolated environment")
	if output, err := e.RunInChroot(pythonBinary, "-m", "ensurepip", "--upgrade"); err != nil {
		e.logger.Debug("ensurepip output: %s", string(output))
		// ensurepip might fail if pip is already installed, verify pip works
	}

	// Verify pip is actually available
	if output, err := e.RunInChroot(pythonBinary, "-m", "pip", "--version"); err != nil {
		return fmt.Errorf("pip not available in isolated environment: %w\nOutput: %s\n\nTried ensurepip but pip is still not working, check if python3-pip package is installed", err, string(output))
	}

	// Build pip install arguments
	// --ignore-installed allows upgrading system packages that were installed by apt/yum
	args := []string{"-m", "pip", "install", "--no-cache-dir", "--ignore-installed"}

	// Add pip options if specified (e.g., --index-url)
	if pipOptions != "" {
		optionParts := strings.Fields(pipOptions)
		args = append(args, optionParts...)
	}

	// Add packages
	args = append(args, packages...)

	// Run pip install inside the chroot
	output, err := e.RunInChroot(pythonBinary, args...)
	if err != nil {
		return fmt.Errorf("pip install failed in isolated environment: %w\nOutput: %s", err, string(output))
	}

	e.logger.Debug("Pip installation output: %s", string(output))
	e.logger.Info("Pip packages installed successfully in isolated environment")
	return nil
}

// InstallNpmPackagesIsolated installs Node.js packages using npm inside the isolated environment
// This ensures we use the npm that was installed in phase 7, not the host's npm
func (e *IsolatedEnvironment) InstallNpmPackagesIsolated(packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	e.logger.Info("Installing npm packages in isolated environment: %s", strings.Join(packages, ", "))

	// First, verify npm is available in the isolated environment
	e.logger.Debug("Verifying npm is available in isolated environment")
	if output, err := e.RunInChroot("npm", "--version"); err != nil {
		return fmt.Errorf("npm not found in isolated environment: %w\nOutput: %s\n\nMake sure nodejs/npm packages were installed in phase 7", err, string(output))
	}

	// Build npm install arguments - install globally
	args := []string{"install", "-g"}
	args = append(args, packages...)

	// Run npm install inside the chroot
	output, err := e.RunInChroot("npm", args...)
	if err != nil {
		return fmt.Errorf("npm install failed in isolated environment: %w\nOutput: %s", err, string(output))
	}

	e.logger.Debug("npm installation output: %s", string(output))
	e.logger.Info("npm packages installed successfully in isolated environment")
	return nil
}

// GetMergedPath returns the path to a file in the merged overlay view
func (e *IsolatedEnvironment) GetMergedPath(path string) string {
	return filepath.Join(e.mergedDir, path)
}

// GetUpperPath returns the path to a file in the upper layer (changes only)
func (e *IsolatedEnvironment) GetUpperPath(path string) string {
	return filepath.Join(e.upperDir, path)
}

// Cleanup unmounts the overlay and removes temporary directories
func (e *IsolatedEnvironment) Cleanup() error {
	e.logger.Debug("Cleaning up isolated environment")

	var errs []string

	if e.mounted {
		// Unmount in reverse order
		// First unmount /dev, /sys, /proc
		devPath := filepath.Join(e.mergedDir, "dev")
		if err := e.sysOps.Unmount(devPath, syscall.MNT_DETACH); err != nil {
			errs = append(errs, fmt.Sprintf("unmount /dev: %v", err))
		}

		sysPath := filepath.Join(e.mergedDir, "sys")
		if err := e.sysOps.Unmount(sysPath, syscall.MNT_DETACH); err != nil {
			// Non-fatal
			e.logger.Debug("unmount /sys: %v", err)
		}

		procPath := filepath.Join(e.mergedDir, "proc")
		if err := e.sysOps.Unmount(procPath, syscall.MNT_DETACH); err != nil {
			errs = append(errs, fmt.Sprintf("unmount /proc: %v", err))
		}

		// Unmount the overlay itself
		if err := e.sysOps.Unmount(e.mergedDir, syscall.MNT_DETACH); err != nil {
			errs = append(errs, fmt.Sprintf("unmount overlay: %v", err))
		}

		e.mounted = false
	}

	// Remove temporary directories
	if err := e.sysOps.RemoveAll(e.baseDir); err != nil {
		errs = append(errs, fmt.Sprintf("remove base dir: %v", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %s", strings.Join(errs, "; "))
	}

	e.logger.Debug("Isolated environment cleaned up")
	return nil
}

// IsMounted returns whether the overlay is currently mounted
func (e *IsolatedEnvironment) IsMounted() bool {
	return e.mounted
}
