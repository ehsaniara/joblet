//go:build linux

package network

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
)

const (
	SysSetnsX86_64 = 308 // x86_64 syscall number for setns
)

// NamespaceOperations handles network namespace creation, joining, and cleanup
type NamespaceOperations struct {
	syscall     osinterface.SyscallInterface
	osInterface osinterface.OsInterface
	paths       *NetworkPaths
	logger      *logger.Logger
}

// NamespaceInfo contains information about a network namespace
type NamespaceInfo struct {
	Path      string
	GroupID   string
	CreatedAt string
	IsBound   bool // Whether it's a bind mount or symlink
}

// NewNamespaceOperations creates a new namespace operations handler
func NewNamespaceOperations(
	syscall osinterface.SyscallInterface,
	osInterface osinterface.OsInterface,
	paths *NetworkPaths,
) *NamespaceOperations {
	if paths == nil {
		paths = NewDefaultPaths()
	}

	return &NamespaceOperations{
		syscall:     syscall,
		osInterface: osInterface,
		paths:       paths,
		logger:      logger.New().WithField("component", "namespace-ops"),
	}
}

// JoinNetworkNamespace joins an existing network namespace using setns syscall
func (no *NamespaceOperations) JoinNetworkNamespace(nsPath string) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	log := no.logger.WithField("nsPath", nsPath)
	log.Debug("attempting to join network namespace")

	// Check if namespace file exists
	if _, err := no.osInterface.Stat(nsPath); err != nil {
		if no.osInterface.IsNotExist(err) {
			return fmt.Errorf("namespace file does not exist: %s", nsPath)
		}
		return fmt.Errorf("failed to stat namespace file %s: %w", nsPath, err)
	}

	log.Debug("opening namespace file")

	// Open the namespace file
	fd, err := syscall.Open(nsPath, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open namespace file %s: %w", nsPath, err)
	}
	defer func() {
		if closeErr := syscall.Close(fd); closeErr != nil {
			log.Warn("failed to close namespace file descriptor", "error", closeErr)
		}
	}()

	log.Debug("calling setns syscall", "fd", fd)

	// Call setns system call
	_, _, errno := syscall.Syscall(SysSetnsX86_64, uintptr(fd), syscall.CLONE_NEWNET, 0)
	if errno != 0 {
		return fmt.Errorf("setns syscall failed for %s: %v", nsPath, errno)
	}

	log.Info("successfully joined network namespace")
	return nil
}

// CreateNamespaceSymlink creates a symlink to a process's network namespace
func (no *NamespaceOperations) CreateNamespaceSymlink(pid int32, targetPath string) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	if targetPath == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	log := no.logger.WithFields("pid", pid, "targetPath", targetPath)

	// Ensure target directory exists
	if err := no.ensureDir(targetPath); err != nil {
		return fmt.Errorf("failed to ensure target directory: %w", err)
	}

	nsSource := fmt.Sprintf("/proc/%d/ns/net", pid)

	log.Debug("creating namespace symlink", "source", nsSource)

	// Create symlink
	if err := no.osInterface.Symlink(nsSource, targetPath); err != nil {
		return fmt.Errorf("failed to create namespace symlink from %s to %s: %w",
			nsSource, targetPath, err)
	}

	log.Info("namespace symlink created successfully")
	return nil
}

// CreateNamespaceBindMount creates a bind mount for a network namespace
func (no *NamespaceOperations) CreateNamespaceBindMount(pid int32, targetPath string) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	if targetPath == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	log := no.logger.WithFields("pid", pid, "targetPath", targetPath)

	// Ensure target directory exists
	if err := no.ensureDir(targetPath); err != nil {
		return fmt.Errorf("failed to ensure target directory: %w", err)
	}

	nsSource := fmt.Sprintf("/proc/%d/ns/net", pid)

	log.Debug("creating namespace bind mount", "source", nsSource)

	// Create the target file first
	if err := no.osInterface.WriteFile(targetPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create target file %s: %w", targetPath, err)
	}

	// Create bind mount
	if err := no.syscall.Mount(nsSource, targetPath, "", syscall.MS_BIND, ""); err != nil {
		// Clean up the target file if bind mount fails
		no.osInterface.Remove(targetPath)
		return fmt.Errorf("failed to bind mount namespace from %s to %s: %w",
			nsSource, targetPath, err)
	}

	log.Info("namespace bind mount created successfully")
	return nil
}

// RemoveNamespace removes a namespace file or symlink
func (no *NamespaceOperations) RemoveNamespace(nsPath string, isBound bool) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	log := no.logger.WithFields("nsPath", nsPath, "isBound", isBound)

	// Check if the namespace path exists
	if _, err := no.osInterface.Stat(nsPath); err != nil {
		if no.osInterface.IsNotExist(err) {
			log.Debug("namespace path does not exist, nothing to remove")
			return nil
		}
		return fmt.Errorf("failed to stat namespace path %s: %w", nsPath, err)
	}

	if isBound {
		// Unmount bind mount first
		log.Debug("unmounting bind mount")
		if err := no.syscall.Unmount(nsPath, 0); err != nil {
			log.Warn("failed to unmount namespace bind mount", "error", err)
			// Continue with removal attempt
		}
	}

	// Remove the file/symlink
	log.Debug("removing namespace file")
	if err := no.osInterface.Remove(nsPath); err != nil {
		return fmt.Errorf("failed to remove namespace file %s: %w", nsPath, err)
	}

	log.Info("namespace removed successfully")
	return nil
}

// GetNamespaceInfo returns information about a namespace
func (no *NamespaceOperations) GetNamespaceInfo(nsPath string) (*NamespaceInfo, error) {
	if nsPath == "" {
		return nil, fmt.Errorf("namespace path cannot be empty")
	}

	// Check if path exists
	fileInfo, err := no.osInterface.Stat(nsPath)
	if err != nil {
		if no.osInterface.IsNotExist(err) {
			return nil, fmt.Errorf("namespace does not exist: %s", nsPath)
		}
		return nil, fmt.Errorf("failed to stat namespace %s: %w", nsPath, err)
	}

	// Determine if it's a bind mount by checking if it's a regular file
	// Symlinks have different mode bits
	isBound := fileInfo.Mode().IsRegular()

	// Extract group ID from path (assuming format like /var/run/netns/groupID)
	groupID := filepath.Base(nsPath)

	info := &NamespaceInfo{
		Path:      nsPath,
		GroupID:   groupID,
		CreatedAt: fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		IsBound:   isBound,
	}

	return info, nil
}

// ListNamespaces lists all network namespaces in the configured paths
func (no *NamespaceOperations) ListNamespaces() ([]*NamespaceInfo, error) {
	var allNamespaces []*NamespaceInfo

	// Check both netns paths
	paths := []string{no.paths.NetnsPath, no.paths.VarRunNetns}

	for _, basePath := range paths {
		namespaces, err := no.listNamespacesInPath(basePath)
		if err != nil {
			no.logger.Warn("failed to list namespaces in path",
				"path", basePath,
				"error", err)
			continue
		}
		allNamespaces = append(allNamespaces, namespaces...)
	}

	return allNamespaces, nil
}

// BuildNamespacePath builds a namespace path for a given group ID and type
func (no *NamespaceOperations) BuildNamespacePath(groupID string, persistent bool) string {
	if persistent {
		return filepath.Join(no.paths.VarRunNetns, groupID)
	}
	return filepath.Join(no.paths.NetnsPath, groupID)
}

// ensureDir ensures that the directory for a given path exists
func (no *NamespaceOperations) ensureDir(targetPath string) error {
	dir := filepath.Dir(targetPath)

	// Check if directory exists
	if _, err := no.osInterface.Stat(dir); err != nil {
		if no.osInterface.IsNotExist(err) {
			// Directory doesn't exist, create it
			if mkdirErr := no.osInterface.MkdirAll(dir, 0755); mkdirErr != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, mkdirErr)
			}
			no.logger.Debug("created directory", "path", dir)
		} else {
			return fmt.Errorf("failed to stat directory %s: %w", dir, err)
		}
	}

	return nil
}

// listNamespacesInPath lists namespaces in a specific directory
func (no *NamespaceOperations) listNamespacesInPath(basePath string) ([]*NamespaceInfo, error) {
	// Check if directory exists
	if _, err := no.osInterface.Stat(basePath); err != nil {
		if no.osInterface.IsNotExist(err) {
			return nil, nil // Directory doesn't exist, return empty list
		}
		return nil, fmt.Errorf("failed to stat directory %s: %w", basePath, err)
	}

	// Read directory contents
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", basePath, err)
	}

	var namespaces []*NamespaceInfo

	for _, entry := range entries {
		nsPath := filepath.Join(basePath, entry.Name())

		info, err := no.GetNamespaceInfo(nsPath)
		if err != nil {
			no.logger.Warn("failed to get namespace info",
				"path", nsPath,
				"error", err)
			continue
		}

		namespaces = append(namespaces, info)
	}

	return namespaces, nil
}

// ValidateNamespacePath validates that a namespace path is safe to use
func (no *NamespaceOperations) ValidateNamespacePath(nsPath string) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	// Check for path traversal attempts
	cleanPath := filepath.Clean(nsPath)
	if cleanPath != nsPath {
		return fmt.Errorf("invalid namespace path (potential path traversal): %s", nsPath)
	}

	// Ensure path is within allowed directories
	allowedPrefixes := []string{no.paths.NetnsPath, no.paths.VarRunNetns}

	validPath := false
	for _, prefix := range allowedPrefixes {
		if filepath.HasPrefix(cleanPath, prefix) {
			validPath = true
			break
		}
	}

	if !validPath {
		return fmt.Errorf("namespace path not in allowed directories: %s", nsPath)
	}

	return nil
}
