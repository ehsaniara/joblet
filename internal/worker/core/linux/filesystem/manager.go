package filesystem

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"worker/internal/worker/core/interfaces"
	"worker/pkg/logger"
	osinterface "worker/pkg/os"
)

type Manager struct {
	baseDir     string
	osInterface osinterface.OsInterface
	logger      *logger.Logger
}

// Ensure Manager implements the interface
var _ interfaces.FilesystemManager = (*Manager)(nil)

func NewManager(osInterface osinterface.OsInterface) interfaces.FilesystemManager {
	return &Manager{
		baseDir:     "/tmp/worker/fs",
		osInterface: osInterface,
		logger:      logger.New().WithField("component", "filesystem-manager"),
	}
}

// SetupIsolatedFilesystem creates and prepares an isolated filesystem for a job
// Enhanced for true filesystem virtualization with bind mount support
func (m *Manager) SetupIsolatedFilesystem(ctx context.Context, jobID string) (string, error) {
	isolatedRoot := m.GetIsolatedRoot(jobID)

	m.logger.Info("setting up isolated filesystem for virtualization", "jobID", jobID, "root", isolatedRoot)

	// Create isolated root directory
	if err := m.osInterface.MkdirAll(isolatedRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create isolated root: %w", err)
	}

	// Create directories specifically designed for bind mounting
	if err := m.createDirectoriesForBindMounting(isolatedRoot); err != nil {
		// Cleanup on failure
		m.osInterface.RemoveAll(isolatedRoot)
		return "", fmt.Errorf("failed to create directories for bind mounting: %w", err)
	}

	// Copy essential binaries and libraries
	if err := m.copyEssentialBinaries(isolatedRoot); err != nil {
		m.logger.Warn("failed to copy some essential binaries", "error", err)
		// Don't fail completely - job might not need them
	}

	// Create device nodes (simplified)
	if err := m.createBasicDeviceNodes(isolatedRoot); err != nil {
		m.logger.Warn("failed to create device nodes", "error", err)
		// Don't fail completely
	}

	// Create basic configuration files for isolated environment
	if err := m.createBasicConfigFiles(isolatedRoot); err != nil {
		m.logger.Warn("failed to create basic config files", "error", err)
		// Don't fail completely
	}

	m.logger.Info("isolated filesystem setup complete for virtualization", "root", isolatedRoot)
	return isolatedRoot, nil
}

// createDirectoriesForBindMounting creates directories specifically for bind mounting
// This replaces createEssentialDirectories with a focus on virtualization
func (m *Manager) createDirectoriesForBindMounting(isolatedRoot string) error {
	// Primary directories that will be bind-mounted to provide isolation
	bindMountDirs := []string{
		"tmp",     // For /tmp - critical for isolation
		"var/tmp", // For /var/tmp - secondary temp directory
		"home",    // For /home - user home directories
		"work",    // Working directory for job execution
	}

	// Additional system directories that may be needed for full compatibility
	systemDirs := []string{
		"proc",          // Process information (for potential future proc mounting)
		"dev",           // Device nodes
		"sys",           // System information (for potential future sys mounting)
		"run",           // Runtime data
		"etc",           // Configuration files
		"usr/bin",       // Additional binaries
		"usr/lib",       // Additional libraries
		"usr/lib64",     // 64-bit libraries
		"usr/local/bin", // Local binaries
		"bin",           // Essential binaries
		"lib",           // Essential libraries
		"lib64",         // 64-bit libraries
		"opt",           // Optional software
		"root",          // Root home directory
		"srv",           // Service data
		"mnt",           // Mount points
		"media",         // Removable media
		"var/log",       // Log files (isolated)
		"var/cache",     // Cache files (isolated)
		"var/spool",     // Spool files (isolated)
	}

	allDirs := append(bindMountDirs, systemDirs...)

	successCount := 0
	for _, dir := range allDirs {
		fullPath := filepath.Join(isolatedRoot, dir)
		if err := m.osInterface.MkdirAll(fullPath, 0755); err != nil {
			m.logger.Debug("failed to create directory", "path", fullPath, "error", err)
			// Continue with other directories
		} else {
			m.logger.Debug("created directory for isolation", "path", fullPath)
			successCount++
		}
	}

	// Ensure critical bind mount directories exist
	criticalDirs := []string{"tmp", "home", "work"}
	for _, criticalDir := range criticalDirs {
		criticalPath := filepath.Join(isolatedRoot, criticalDir)
		if _, err := m.osInterface.Stat(criticalPath); err != nil {
			return fmt.Errorf("critical directory creation failed: %s (%w)", criticalDir, err)
		}
	}

	m.logger.Info("directories for bind mounting created",
		"successful", successCount,
		"total", len(allDirs),
		"critical", len(criticalDirs))

	return nil
}

// CleanupIsolatedFilesystem removes the isolated filesystem for a job
func (m *Manager) CleanupIsolatedFilesystem(jobID string) error {
	isolatedRoot := m.GetIsolatedRoot(jobID)

	m.logger.Info("cleaning up isolated filesystem", "jobID", jobID, "root", isolatedRoot)

	// Check if isolated root exists
	if _, err := m.osInterface.Stat(isolatedRoot); err != nil {
		if m.osInterface.IsNotExist(err) {
			m.logger.Debug("isolated root already removed", "root", isolatedRoot)
			return nil
		}
		return fmt.Errorf("failed to stat isolated root: %w", err)
	}

	// Get directory size for logging
	if size, err := m.getDirectorySize(isolatedRoot); err == nil {
		m.logger.Debug("cleaning up isolated filesystem", "size", size, "root", isolatedRoot)
	}

	// Remove isolated filesystem directory tree
	if err := m.osInterface.RemoveAll(isolatedRoot); err != nil {
		m.logger.Error("failed to remove isolated filesystem", "error", err, "root", isolatedRoot)
		return err
	}

	// Cleanup any job-specific global directories we might have created
	m.cleanupJobSpecificGlobalDirectories(jobID)

	m.logger.Info("isolated filesystem cleanup complete", "jobID", jobID)
	return nil
}

// GetIsolatedRoot returns the root path for a job's isolated filesystem
func (m *Manager) GetIsolatedRoot(jobID string) string {
	return filepath.Join(m.baseDir, fmt.Sprintf("job-%s-root", jobID))
}

// createEssentialDirectories is maintained for backward compatibility
// but now calls the enhanced createDirectoriesForBindMounting
func (m *Manager) createEssentialDirectories(isolatedRoot string) error {
	return m.createDirectoriesForBindMounting(isolatedRoot)
}

func (m *Manager) copyEssentialBinaries(isolatedRoot string) error {
	// Essential binaries that jobs commonly need
	essentialBinaries := map[string]string{
		// Shell and basic commands
		"/bin/sh":     "bin/sh",
		"/bin/bash":   "bin/bash",
		"/bin/dash":   "bin/dash",
		"/bin/ls":     "bin/ls",
		"/bin/cat":    "bin/cat",
		"/bin/echo":   "bin/echo",
		"/bin/pwd":    "bin/pwd",
		"/bin/chmod":  "bin/chmod",
		"/bin/mkdir":  "bin/mkdir",
		"/bin/rmdir":  "bin/rmdir",
		"/bin/rm":     "bin/rm",
		"/bin/cp":     "bin/cp",
		"/bin/mv":     "bin/mv",
		"/bin/touch":  "bin/touch",
		"/bin/grep":   "bin/grep",
		"/bin/sed":    "bin/sed",
		"/bin/awk":    "bin/awk",
		"/bin/cut":    "bin/cut",
		"/bin/sort":   "bin/sort",
		"/bin/head":   "bin/head",
		"/bin/tail":   "bin/tail",
		"/bin/wc":     "bin/wc",
		"/bin/find":   "bin/find",
		"/bin/which":  "bin/which",
		"/bin/whoami": "bin/whoami",
		"/bin/id":     "bin/id",
		"/bin/env":    "bin/env",
		"/bin/sleep":  "bin/sleep",
		"/bin/date":   "bin/date",

		// Programming languages and interpreters
		"/usr/bin/python3": "usr/bin/python3",
		"/usr/bin/python":  "usr/bin/python",
		"/usr/bin/python2": "usr/bin/python2",
		"/usr/bin/node":    "usr/bin/node",
		"/usr/bin/ruby":    "usr/bin/ruby",
		"/usr/bin/perl":    "usr/bin/perl",
		"/usr/bin/php":     "usr/bin/php",
		"/usr/bin/java":    "usr/bin/java",
		"/usr/bin/javac":   "usr/bin/javac",
		"/usr/bin/go":      "usr/bin/go",
		"/usr/bin/gcc":     "usr/bin/gcc",
		"/usr/bin/g++":     "usr/bin/g++",
		"/usr/bin/make":    "usr/bin/make",
		"/usr/bin/cmake":   "usr/bin/cmake",

		// Network and file utilities
		"/usr/bin/curl":   "usr/bin/curl",
		"/usr/bin/wget":   "usr/bin/wget",
		"/usr/bin/tar":    "usr/bin/tar",
		"/usr/bin/gzip":   "usr/bin/gzip",
		"/usr/bin/gunzip": "usr/bin/gunzip",
		"/usr/bin/unzip":  "usr/bin/unzip",
		"/usr/bin/zip":    "usr/bin/zip",

		// Text editors
		"/usr/bin/nano": "usr/bin/nano",
		"/usr/bin/vim":  "usr/bin/vim",
		"/usr/bin/vi":   "usr/bin/vi",

		// Git and version control
		"/usr/bin/git": "usr/bin/git",
	}

	copiedCount := 0
	for srcPath, destRelPath := range essentialBinaries {
		destPath := filepath.Join(isolatedRoot, destRelPath)

		// Check if source exists
		if _, err := m.osInterface.Stat(srcPath); err != nil {
			continue // Skip if binary doesn't exist on host
		}

		// Copy binary
		if err := m.copyFile(srcPath, destPath); err != nil {
			m.logger.Debug("failed to copy binary", "src", srcPath, "dest", destPath, "error", err)
			continue
		}

		copiedCount++
	}

	m.logger.Info("essential binaries copied", "copiedCount", copiedCount, "totalAvailable", len(essentialBinaries))

	// Copy essential libraries
	if err := m.copyEssentialLibraries(isolatedRoot); err != nil {
		m.logger.Debug("failed to copy essential libraries", "error", err)
	}

	return nil
}

func (m *Manager) copyEssentialLibraries(isolatedRoot string) error {
	// Essential libraries for basic program execution
	essentialLibs := []string{
		// Dynamic linker
		"/lib64/ld-linux-x86-64.so.2",
		"/lib/ld-linux.so.2",

		// Core C libraries
		"/lib/x86_64-linux-gnu/libc.so.6",
		"/lib/x86_64-linux-gnu/libdl.so.2",
		"/lib/x86_64-linux-gnu/libpthread.so.0",
		"/lib/x86_64-linux-gnu/libm.so.6",
		"/lib/x86_64-linux-gnu/librt.so.1",
		"/lib/x86_64-linux-gnu/libresolv.so.2",
		"/lib/x86_64-linux-gnu/libnss_files.so.2",
		"/lib/x86_64-linux-gnu/libnss_dns.so.2",

		// Common additional libraries
		"/lib/x86_64-linux-gnu/libz.so.1",
		"/lib/x86_64-linux-gnu/libssl.so.1.1",
		"/lib/x86_64-linux-gnu/libcrypto.so.1.1",
		"/lib/x86_64-linux-gnu/libcurl.so.4",

		// Alternative paths for different distributions
		"/usr/lib/x86_64-linux-gnu/libc.so.6",
		"/usr/lib64/libc.so.6",
		"/lib64/libc.so.6",
	}

	copiedCount := 0
	for _, libPath := range essentialLibs {
		if _, err := m.osInterface.Stat(libPath); err != nil {
			continue // Skip if library doesn't exist
		}

		// Determine destination path
		var destPath string
		if filepath.Dir(libPath) == "/lib64" {
			destPath = filepath.Join(isolatedRoot, "lib64", filepath.Base(libPath))
		} else if strings.Contains(libPath, "/usr/lib64/") {
			destPath = filepath.Join(isolatedRoot, "usr/lib64", filepath.Base(libPath))
		} else if strings.Contains(libPath, "/usr/lib/") {
			destPath = filepath.Join(isolatedRoot, "usr/lib", filepath.Base(libPath))
		} else {
			destPath = filepath.Join(isolatedRoot, "lib", filepath.Base(libPath))
		}

		if err := m.copyFile(libPath, destPath); err != nil {
			m.logger.Debug("failed to copy library", "src", libPath, "dest", destPath, "error", err)
			continue
		}

		copiedCount++
	}

	m.logger.Debug("essential libraries copied", "copiedCount", copiedCount)
	return nil
}

func (m *Manager) createBasicDeviceNodes(isolatedRoot string) error {
	devDir := filepath.Join(isolatedRoot, "dev")

	// Create basic device files (just empty files for compatibility)
	deviceFiles := []string{
		"null",
		"zero",
		"random",
		"urandom",
		"stdin",
		"stdout",
		"stderr",
	}

	for _, deviceName := range deviceFiles {
		devicePath := filepath.Join(devDir, deviceName)

		// Create empty file
		if err := m.osInterface.WriteFile(devicePath, []byte{}, 0666); err != nil {
			m.logger.Debug("failed to create device file", "device", deviceName, "error", err)
		}
	}

	m.logger.Debug("basic device nodes created", "count", len(deviceFiles))
	return nil
}

// Helper method to create configuration files if needed
func (m *Manager) createBasicConfigFiles(isolatedRoot string) error {
	etcDir := filepath.Join(isolatedRoot, "etc")

	// Create basic /etc/passwd for user lookups
	passwdContent := `root:x:0:0:root:/root:/bin/bash
nobody:x:65534:65534:nobody:/nonexistent:/usr/sbin/nologin
worker:x:1000:1000:worker:/work:/bin/bash
`

	if err := m.osInterface.WriteFile(filepath.Join(etcDir, "passwd"), []byte(passwdContent), 0644); err != nil {
		m.logger.Debug("failed to create passwd file", "error", err)
	}

	// Create basic /etc/group for group lookups
	groupContent := `root:x:0:
nogroup:x:65534:
worker:x:1000:
`

	if err := m.osInterface.WriteFile(filepath.Join(etcDir, "group"), []byte(groupContent), 0644); err != nil {
		m.logger.Debug("failed to create group file", "error", err)
	}

	m.logger.Debug("basic config files created")
	return nil
}

func (m *Manager) copyFile(src, dest string) error {
	// Ensure destination directory exists
	if err := m.osInterface.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}

	// Read source file
	data, err := m.osInterface.ReadFile(src)
	if err != nil {
		return err
	}

	// Get source file permissions
	srcInfo, err := m.osInterface.Stat(src)
	if err != nil {
		return err
	}

	// Write to destination with same permissions
	if err := m.osInterface.WriteFile(dest, data, srcInfo.Mode()); err != nil {
		return err
	}

	return nil
}

// getDirectorySize gets directory size for logging
func (m *Manager) getDirectorySize(dirPath string) (string, error) {
	// Simple approach - count files/directories
	entries, err := m.osInterface.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	fileCount := 0
	dirCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			dirCount++
		} else {
			fileCount++
		}
	}

	return fmt.Sprintf("%d files, %d directories", fileCount, dirCount), nil
}

// cleanupJobSpecificGlobalDirectories cleans up any job-specific global directories
func (m *Manager) cleanupJobSpecificGlobalDirectories(jobID string) {
	// If we created job-specific global directories, clean them up
	jobSpecificDirs := []string{
		fmt.Sprintf("/work-%s", jobID), // If we used job-specific global work dirs
		fmt.Sprintf("/tmp-%s", jobID),  // If we used job-specific global tmp dirs
		fmt.Sprintf("/home-%s", jobID), // If we used job-specific global home dirs
	}

	for _, dir := range jobSpecificDirs {
		if _, err := m.osInterface.Stat(dir); err == nil {
			if err := m.osInterface.RemoveAll(dir); err != nil {
				m.logger.Debug("failed to remove job-specific global directory", "dir", dir, "error", err)
			} else {
				m.logger.Debug("removed job-specific global directory", "dir", dir)
			}
		}
	}
}
