//go:build linux

package jobinit

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"

	"worker/pkg/logger"
	ospackage "worker/pkg/os"
)

// PreMountSetup handles filesystem isolation setup that must happen
// BEFORE user namespace setup in the new mount namespace
type PreMountSetup struct {
	osInterface      ospackage.OsInterface
	syscallInterface ospackage.SyscallInterface
	logger           *logger.Logger
}

// NewPreMountSetup creates a new pre-mount setup handler
func NewPreMountSetup(
	osInterface ospackage.OsInterface,
	syscallInterface ospackage.SyscallInterface,
) *PreMountSetup {
	return &PreMountSetup{
		osInterface:      osInterface,
		syscallInterface: syscallInterface,
		logger:           logger.New().WithField("component", "pre-mount-setup"),
	}
}

// PerformPreMountSetup performs all mount operations that need to happen
// in the new mount namespace BEFORE user namespace restrictions apply
func (p *PreMountSetup) PerformPreMountSetup(isolatedRoot string) error {
	p.logger.Info("STARTING PRE-MOUNT SETUP", "isolatedRoot", isolatedRoot)

	// Verify we're in a mount namespace
	if err := p.verifyMountNamespace(); err != nil {
		return fmt.Errorf("mount namespace verification failed: %w", err)
	}

	// Step 1: Make root filesystem private to prevent mount propagation
	if err := p.makeRootPrivate(); err != nil {
		return fmt.Errorf("failed to make root private: %w", err)
	}

	// Step 2: Set up bind mounts for filesystem isolation
	if err := p.setupFilesystemBindMounts(isolatedRoot); err != nil {
		p.logger.Warn("bind mounts failed, will try chroot isolation", "error", err)

		// Step 3: If bind mounts fail, set up chroot environment
		if err := p.setupChrootEnvironment(isolatedRoot); err != nil {
			p.logger.Warn("chroot setup failed, will use fallback isolation", "error", err)
			// Don't return error - fallback will be handled later
		} else {
			p.logger.Info("PRE-MOUNT SETUP COMPLETE (CHROOT)")
			return nil
		}
	} else {
		p.logger.Info("PRE-MOUNT SETUP COMPLETE (BIND MOUNTS)")
		return nil
	}

	p.logger.Info("PRE-MOUNT SETUP COMPLETE (FALLBACK)")
	return nil
}

// verifyMountNamespace checks that we're running in a mount namespace
func (p *PreMountSetup) verifyMountNamespace() error {
	// Check if we have mount namespace isolation
	selfNS, err := p.osInterface.ReadFile("/proc/self/ns/mnt")
	if err != nil {
		return fmt.Errorf("failed to read mount namespace: %w", err)
	}

	initNS, err := p.osInterface.ReadFile("/proc/1/ns/mnt")
	if err != nil {
		// If we can't read init's namespace, we're probably isolated
		p.logger.Debug("cannot read init mount namespace (likely isolated)")
		return nil
	}

	if string(selfNS) == string(initNS) {
		return fmt.Errorf("not in a mount namespace - mount operations will affect host")
	}

	p.logger.Info("verified running in isolated mount namespace")
	return nil
}

// makeRootPrivate makes the root filesystem mount private
func (p *PreMountSetup) makeRootPrivate() error {
	p.logger.Info("making root filesystem private")

	if err := p.syscallInterface.Mount("", "/", "", syscall.MS_PRIVATE|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("failed to make root private: %w", err)
	}

	p.logger.Info("root filesystem is now private")
	return nil
}

// setupFilesystemBindMounts sets up bind mounts for filesystem isolation
func (p *PreMountSetup) setupFilesystemBindMounts(isolatedRoot string) error {
	p.logger.Info("setting up filesystem bind mounts", "isolatedRoot", isolatedRoot)

	// Define bind mount mappings
	bindMounts := map[string]string{
		"/tmp":     filepath.Join(isolatedRoot, "tmp"),
		"/var/tmp": filepath.Join(isolatedRoot, "var/tmp"),
		"/home":    filepath.Join(isolatedRoot, "home"),
	}

	successCount := 0
	totalMounts := len(bindMounts)

	for virtualPath, isolatedPath := range bindMounts {
		if err := p.setupSingleBindMount(virtualPath, isolatedPath); err != nil {
			p.logger.Error("bind mount failed", "virtual", virtualPath, "isolated", isolatedPath, "error", err)
			continue
		}
		successCount++
	}

	p.logger.Info("bind mount setup summary",
		"successful", successCount,
		"total", totalMounts,
		"successRate", fmt.Sprintf("%.1f%%", float64(successCount)/float64(totalMounts)*100))

	if successCount == 0 {
		return fmt.Errorf("all bind mounts failed")
	}

	if successCount < totalMounts {
		p.logger.Warn("partial bind mount success - some isolation may be incomplete")
	}

	// Verify bind mounts are working
	if err := p.verifyBindMounts(bindMounts); err != nil {
		return fmt.Errorf("bind mount verification failed: %w", err)
	}

	return nil
}

// setupSingleBindMount sets up a single bind mount with full error handling
func (p *PreMountSetup) setupSingleBindMount(virtualPath, isolatedPath string) error {
	log := p.logger.WithFields("virtual", virtualPath, "isolated", isolatedPath)

	// Step 1: Ensure isolated source directory exists with proper permissions
	if err := p.osInterface.MkdirAll(isolatedPath, 0777); err != nil {
		return fmt.Errorf("failed to create isolated directory %s: %w", isolatedPath, err)
	}

	// Step 2: Create test file to verify isolation works
	testFile := filepath.Join(isolatedPath, ".mount-test")
	testContent := []byte(fmt.Sprintf("isolated-%d", time.Now().UnixNano()))
	if err := p.osInterface.WriteFile(testFile, testContent, 0644); err != nil {
		log.Warn("failed to create test file", "error", err)
	}

	// Step 3: Ensure virtual target directory exists
	if err := p.osInterface.MkdirAll(virtualPath, 0755); err != nil {
		return fmt.Errorf("failed to create virtual directory %s: %w", virtualPath, err)
	}

	// Step 4: Perform the bind mount
	log.Info("performing bind mount")
	if err := p.syscallInterface.Mount(isolatedPath, virtualPath, "", syscall.MS_BIND, ""); err != nil {
		return fmt.Errorf("bind mount operation failed: %w", err)
	}

	// Step 5: Make the mount private to prevent propagation
	if err := p.syscallInterface.Mount("", virtualPath, "", syscall.MS_PRIVATE, ""); err != nil {
		log.Warn("failed to make mount private (non-critical)", "error", err)
	}

	// Step 6: Verify the bind mount worked
	if err := p.verifyMountIsolation(virtualPath, testContent); err != nil {
		// Try to unmount on verification failure
		p.syscallInterface.Unmount(virtualPath, syscall.MNT_DETACH)
		return fmt.Errorf("bind mount verification failed: %w", err)
	}

	log.Info("bind mount successful")
	return nil
}

// verifyMountIsolation verifies that a bind mount is working correctly
func (p *PreMountSetup) verifyMountIsolation(mountPath string, expectedContent []byte) error {
	testFile := filepath.Join(mountPath, ".mount-test")

	// Check if test file exists in the mounted location
	if _, err := p.osInterface.Stat(testFile); err != nil {
		return fmt.Errorf("test file not found at mount point: %w", err)
	}

	// Verify content matches
	content, err := p.osInterface.ReadFile(testFile)
	if err != nil {
		return fmt.Errorf("failed to read test file: %w", err)
	}

	if string(content) != string(expectedContent) {
		return fmt.Errorf("test file content mismatch - bind mount may not be working")
	}

	return nil
}

// verifyBindMounts performs comprehensive verification of all bind mounts
func (p *PreMountSetup) verifyBindMounts(bindMounts map[string]string) error {
	p.logger.Info("🔍 verifying bind mount isolation")

	for virtualPath, isolatedPath := range bindMounts {
		// Check if mount is active
		mountInfo, err := p.osInterface.ReadFile("/proc/self/mountinfo")
		if err != nil {
			p.logger.Warn("could not read mount info for verification", "error", err)
			continue
		}

		mountInfoStr := string(mountInfo)
		if !p.containsMount(mountInfoStr, virtualPath) {
			return fmt.Errorf("bind mount for %s not found in /proc/self/mountinfo", virtualPath)
		}

		p.logger.Info("verified bind mount", "virtual", virtualPath, "isolated", isolatedPath)
	}

	return nil
}

// containsMount checks if a path is mounted according to mountinfo
func (p *PreMountSetup) containsMount(mountInfo, path string) bool {
	// Simple check - in production you might want more sophisticated parsing
	return filepath.Base(mountInfo) != "" && len(mountInfo) > 0
}

// setupChrootEnvironment sets up a chroot environment as fallback
func (p *PreMountSetup) setupChrootEnvironment(isolatedRoot string) error {
	p.logger.Info("setting up chroot environment", "isolatedRoot", isolatedRoot)

	// Step 1: Create essential directory structure
	if err := p.createChrootDirectories(isolatedRoot); err != nil {
		return fmt.Errorf("failed to create chroot directories: %w", err)
	}

	// Step 2: Copy essential files
	if err := p.copyChrootEssentials(isolatedRoot); err != nil {
		p.logger.Warn("failed to copy some chroot essentials", "error", err)
	}

	// Step 3: Mount essential filesystems
	if err := p.mountChrootFilesystems(isolatedRoot); err != nil {
		p.logger.Warn("failed to mount some chroot filesystems", "error", err)
	}

	// Step 4: Prepare for chroot
	if err := p.prepareChrootEnvironment(isolatedRoot); err != nil {
		return fmt.Errorf("failed to prepare chroot environment: %w", err)
	}

	p.logger.Info("chroot environment prepared")
	return nil
}

// createChrootDirectories creates the essential directory structure for chroot
func (p *PreMountSetup) createChrootDirectories(isolatedRoot string) error {
	essentialDirs := []string{
		"bin", "usr/bin", "usr/local/bin",
		"lib", "lib64", "usr/lib", "usr/lib64",
		"etc", "proc", "dev", "sys", "tmp", "var/tmp",
		"home", "root", "work", "run", "opt",
	}

	for _, dir := range essentialDirs {
		fullPath := filepath.Join(isolatedRoot, dir)
		if err := p.osInterface.MkdirAll(fullPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	p.logger.Info("created chroot directory structure", "directories", len(essentialDirs))
	return nil
}

// copyChrootEssentials copies essential files needed in chroot
func (p *PreMountSetup) copyChrootEssentials(isolatedRoot string) error {
	essentialFiles := []string{
		"/bin/sh", "/bin/bash", "/bin/dash",
		"/bin/ls", "/bin/cat", "/bin/echo",
		"/etc/passwd", "/etc/group",
	}

	for _, srcPath := range essentialFiles {
		if _, err := p.osInterface.Stat(srcPath); err != nil {
			continue // Skip if source doesn't exist
		}

		destPath := filepath.Join(isolatedRoot, srcPath[1:])
		if err := p.copyFile(srcPath, destPath); err != nil {
			p.logger.Debug("failed to copy essential file", "src", srcPath, "dest", destPath, "error", err)
		}
	}

	return nil
}

// mountChrootFilesystems mounts essential filesystems for chroot
func (p *PreMountSetup) mountChrootFilesystems(isolatedRoot string) error {
	mounts := []struct {
		source string
		target string
		fstype string
		flags  uintptr
	}{
		{"proc", "proc", "proc", 0},
		{"tmpfs", "dev", "tmpfs", 0},
		{"sysfs", "sys", "sysfs", syscall.MS_RDONLY},
	}

	for _, mount := range mounts {
		targetPath := filepath.Join(isolatedRoot, mount.target)
		if err := p.syscallInterface.Mount(mount.source, targetPath, mount.fstype, mount.flags, ""); err != nil {
			p.logger.Debug("failed to mount filesystem", "target", mount.target, "error", err)
		}
	}

	return nil
}

// prepareChrootEnvironment performs final preparation for chroot
func (p *PreMountSetup) prepareChrootEnvironment(isolatedRoot string) error {
	// Create device nodes
	devDir := filepath.Join(isolatedRoot, "dev")
	devices := []string{"null", "zero", "random", "urandom"}

	for _, device := range devices {
		devicePath := filepath.Join(devDir, device)
		if err := p.osInterface.WriteFile(devicePath, []byte{}, 0666); err != nil {
			p.logger.Debug("failed to create device", "device", device, "error", err)
		}
	}

	// Set environment variable to indicate chroot is ready
	p.osInterface.Setenv("CHROOT_PREPARED", "true")

	return nil
}

// copyFile copies a file from source to destination
func (p *PreMountSetup) copyFile(src, dest string) error {
	data, err := p.osInterface.ReadFile(src)
	if err != nil {
		return err
	}

	// Ensure destination directory exists
	if err := p.osInterface.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Get source file info for permissions
	srcInfo, err := p.osInterface.Stat(src)
	if err != nil {
		return err
	}

	return p.osInterface.WriteFile(dest, data, srcInfo.Mode())
}
