//go:build linux

// Package network provides network namespace operations for Linux systems.
// This package handles the low-level operations required to create, manage,
// and clean up network namespaces for job isolation. It uses Linux-specific
// system calls like setns, mount, and unmount.
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
	// SysSetnsX86_64 is the x86_64 syscall number for setns
	// This system call allows a process to join an existing namespace
	// See: man 2 setns
	SysSetnsX86_64 = 308 // x86_64 syscall number for setns
)

// NamespaceOperations handles network namespace creation, joining, and cleanup.
// This struct encapsulates all the low-level namespace operations needed for
// job isolation, including:
//
// - Creating namespace references (symlinks or bind mounts)
// - Joining existing namespaces via setns syscall
// - Cleaning up namespace artifacts when no longer needed
// - Validating namespace paths for security
//
// The operations support two types of namespace persistence:
// 1. Symlinks - for isolated jobs (temporary, cleaned up per job)
// 2. Bind mounts - for network groups (persistent across multiple jobs)
type NamespaceOperations struct {
	syscall     osinterface.SyscallInterface // Interface for system calls (mount, unmount, setns)
	osInterface osinterface.OsInterface      // Interface for OS operations (file I/O, stat)
	paths       *NetworkPaths                // Configured paths for namespace storage
	logger      *logger.Logger               // Structured logger for debugging namespace operations
}

// NamespaceInfo contains comprehensive information about a network namespace.
// This structure is used for introspection, monitoring, and administrative
// operations on existing namespaces.
type NamespaceInfo struct {
	Path      string // Full filesystem path to the namespace reference
	GroupID   string // Network group ID (extracted from path basename)
	CreatedAt string // Creation timestamp in RFC3339 format
	IsBound   bool   // true for bind mounts (persistent), false for symlinks (temporary)
}

// NewNamespaceOperations creates a new namespace operations handler.
// This factory function sets up all dependencies needed for namespace management.
//
// Parameters:
//   - syscall: Interface for system calls (allows mocking in tests)
//   - osInterface: Interface for OS operations (allows mocking in tests)
//   - paths: Network paths configuration (uses defaults if nil)
//
// Returns: Configured NamespaceOperations ready for use
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

// JoinNetworkNamespace joins an existing network namespace using setns syscall.
// This operation moves the calling process into the specified network namespace,
// allowing it to share network interfaces, routing tables, and firewall rules
// with other processes in the same namespace.
//
// The process flow:
// 1. Validate namespace path exists
// 2. Open namespace file descriptor
// 3. Call setns syscall to join the namespace
// 4. Verify operation succeeded
//
// This is typically called by job-init processes before exec'ing the user command,
// ensuring the user process runs in the correct network context.
//
// Parameters:
//   - nsPath: Path to namespace file (symlink or bind mount)
//
// Returns: nil on success, error with detailed context on failure
func (no *NamespaceOperations) JoinNetworkNamespace(nsPath string) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	log := no.logger.WithField("nsPath", nsPath)
	log.Debug("attempting to join network namespace")

	// Verify namespace file exists and is accessible
	if _, err := no.osInterface.Stat(nsPath); err != nil {
		if no.osInterface.IsNotExist(err) {
			return fmt.Errorf("namespace file does not exist: %s", nsPath)
		}
		return fmt.Errorf("failed to stat namespace file %s: %w", nsPath, err)
	}

	log.Debug("opening namespace file")

	// Open the namespace file in read-only mode
	// The file descriptor will be used with the setns syscall
	fd, err := syscall.Open(nsPath, syscall.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open namespace file %s: %w", nsPath, err)
	}

	// Ensure file descriptor is closed regardless of success/failure
	defer func() {
		if closeErr := syscall.Close(fd); closeErr != nil {
			log.Warn("failed to close namespace file descriptor", "error", closeErr)
		}
	}()

	log.Debug("calling setns syscall", "fd", fd)

	// Call setns system call to join the network namespace
	// Parameters:
	//   - fd: file descriptor of namespace to join
	//   - CLONE_NEWNET: flag indicating we want to join a network namespace
	_, _, errno := syscall.Syscall(SysSetnsX86_64, uintptr(fd), syscall.CLONE_NEWNET, 0)
	if errno != 0 {
		return fmt.Errorf("setns syscall failed for %s: %v", nsPath, errno)
	}

	log.Info("successfully joined network namespace")
	return nil
}

// CreateNamespaceSymlink creates a symlink to a process's network namespace.
// Symlinks are used for isolated jobs that don't share namespaces with other jobs.
// The symlink points to /proc/PID/ns/net and is automatically cleaned up when
// the process exits and /proc/PID disappears.
//
// Symlink lifecycle:
// 1. Process starts with new network namespace
// 2. Symlink created pointing to /proc/PID/ns/net
// 3. Other processes can join via the symlink
// 4. When process exits, /proc/PID/ns/net disappears
// 5. Symlink becomes broken and can be removed
//
// Parameters:
//   - pid: Process ID whose namespace to reference
//   - targetPath: Where to create the symlink
//
// Returns: nil on success, error with context on failure
func (no *NamespaceOperations) CreateNamespaceSymlink(pid int32, targetPath string) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	if targetPath == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	log := no.logger.WithFields("pid", pid, "targetPath", targetPath)

	// Ensure the target directory exists before creating symlink
	if err := no.ensureDir(targetPath); err != nil {
		return fmt.Errorf("failed to ensure target directory: %w", err)
	}

	// Construct the source path in /proc filesystem
	// This path will exist as long as the process is running
	nsSource := fmt.Sprintf("/proc/%d/ns/net", pid)

	log.Debug("creating namespace symlink", "source", nsSource)

	// Create the symlink from source to target
	if err := no.osInterface.Symlink(nsSource, targetPath); err != nil {
		return fmt.Errorf("failed to create namespace symlink from %s to %s: %w",
			nsSource, targetPath, err)
	}

	log.Info("namespace symlink created successfully")
	return nil
}

// CreateNamespaceBindMount creates a bind mount for a network namespace.
// Bind mounts are used for persistent network groups where multiple jobs
// share the same namespace. The bind mount persists even after the original
// process exits, allowing subsequent processes to join the same namespace.
//
// Bind mount lifecycle:
// 1. Process starts with new network namespace
// 2. Empty file created at target path
// 3. /proc/PID/ns/net bind mounted to target file
// 4. Original process can exit - namespace persists via bind mount
// 5. Other processes join via the bind mount
// 6. When no longer needed, unmount and remove file
//
// This is more complex than symlinks but provides namespace persistence
// beyond the lifetime of the creating process.
//
// Parameters:
//   - pid: Process ID whose namespace to bind mount
//   - targetPath: Where to create the bind mount
//
// Returns: nil on success, error with context on failure
func (no *NamespaceOperations) CreateNamespaceBindMount(pid int32, targetPath string) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: %d", pid)
	}
	if targetPath == "" {
		return fmt.Errorf("target path cannot be empty")
	}

	log := no.logger.WithFields("pid", pid, "targetPath", targetPath)

	// Ensure the target directory exists
	if err := no.ensureDir(targetPath); err != nil {
		return fmt.Errorf("failed to ensure target directory: %w", err)
	}

	// Construct source path in /proc filesystem
	nsSource := fmt.Sprintf("/proc/%d/ns/net", pid)

	log.Debug("creating namespace bind mount", "source", nsSource)

	// Create an empty file at the target path
	// Bind mounts require a file (not directory) as target
	if err := no.osInterface.WriteFile(targetPath, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create target file %s: %w", targetPath, err)
	}

	// Create the bind mount from source to target
	// MS_BIND flag indicates this is a bind mount operation
	if err := no.syscall.Mount(nsSource, targetPath, "", syscall.MS_BIND, ""); err != nil {
		// Clean up the target file if bind mount fails
		no.osInterface.Remove(targetPath)
		return fmt.Errorf("failed to bind mount namespace from %s to %s: %w",
			nsSource, targetPath, err)
	}

	log.Info("namespace bind mount created successfully")
	return nil
}

// RemoveNamespace removes a namespace file or symlink.
// This method handles cleanup for both types of namespace references:
//
// For bind mounts (persistent namespaces):
// 1. Unmount the bind mount to release the namespace reference
// 2. Remove the underlying file
//
// For symlinks (temporary namespaces):
// 1. Simply remove the symlink file
//
// The isBound parameter determines which cleanup strategy to use.
//
// Parameters:
//   - nsPath: Path to namespace reference to remove
//   - isBound: true for bind mounts, false for symlinks
//
// Returns: nil on success, error with context on failure
func (no *NamespaceOperations) RemoveNamespace(nsPath string, isBound bool) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	log := no.logger.WithFields("nsPath", nsPath, "isBound", isBound)

	// Check if the namespace path exists before attempting removal
	if _, err := no.osInterface.Stat(nsPath); err != nil {
		if no.osInterface.IsNotExist(err) {
			log.Debug("namespace path does not exist, nothing to remove")
			return nil
		}
		return fmt.Errorf("failed to stat namespace path %s: %w", nsPath, err)
	}

	if isBound {
		// For bind mounts, unmount first to release the namespace reference
		log.Debug("unmounting bind mount")
		if err := no.syscall.Unmount(nsPath, 0); err != nil {
			log.Warn("failed to unmount namespace bind mount", "error", err)
			// Continue with removal attempt even if unmount fails
			// The file removal might still succeed
		}
	}

	// Remove the file/symlink from filesystem
	log.Debug("removing namespace file")
	if err := no.osInterface.Remove(nsPath); err != nil {
		return fmt.Errorf("failed to remove namespace file %s: %w", nsPath, err)
	}

	log.Info("namespace removed successfully")
	return nil
}

// GetNamespaceInfo returns comprehensive information about a namespace.
// This method inspects an existing namespace reference and returns metadata
// useful for monitoring, debugging, and administrative operations.
//
// The method determines whether a namespace reference is a bind mount or
// symlink by examining the file mode bits.
//
// Parameters:
//   - nsPath: Path to namespace reference to inspect
//
// Returns: NamespaceInfo structure with metadata, or error if inspection fails
func (no *NamespaceOperations) GetNamespaceInfo(nsPath string) (*NamespaceInfo, error) {
	if nsPath == "" {
		return nil, fmt.Errorf("namespace path cannot be empty")
	}

	// Get file information from filesystem
	fileInfo, err := no.osInterface.Stat(nsPath)
	if err != nil {
		if no.osInterface.IsNotExist(err) {
			return nil, fmt.Errorf("namespace does not exist: %s", nsPath)
		}
		return nil, fmt.Errorf("failed to stat namespace %s: %w", nsPath, err)
	}

	// Determine namespace type by examining file mode
	// Regular files indicate bind mounts, symlinks have different mode bits
	isBound := fileInfo.Mode().IsRegular()

	// Extract group ID from the path basename
	// Assumes naming convention like /var/run/netns/groupID
	groupID := filepath.Base(nsPath)

	info := &NamespaceInfo{
		Path:      nsPath,
		GroupID:   groupID,
		CreatedAt: fileInfo.ModTime().Format("2006-01-02T15:04:05Z07:00"),
		IsBound:   isBound,
	}

	return info, nil
}

// ListNamespaces lists all network namespaces in the configured paths.
// This method scans both temporary and persistent namespace directories
// and returns information about all found namespace references.
//
// This is useful for:
// - Administrative overview of active namespaces
// - Cleanup of orphaned namespace references
// - Monitoring namespace usage across the system
//
// Returns: Slice of NamespaceInfo for all found namespaces
func (no *NamespaceOperations) ListNamespaces() ([]*NamespaceInfo, error) {
	var allNamespaces []*NamespaceInfo

	// Check both configured namespace paths
	paths := []string{no.paths.NetnsPath, no.paths.VarRunNetns}

	for _, basePath := range paths {
		namespaces, err := no.listNamespacesInPath(basePath)
		if err != nil {
			no.logger.Warn("failed to list namespaces in path",
				"path", basePath,
				"error", err)
			continue // Continue with other paths even if one fails
		}
		allNamespaces = append(allNamespaces, namespaces...)
	}

	return allNamespaces, nil
}

// BuildNamespacePath builds a namespace path for a given group ID and type.
// This method encapsulates the path construction logic and ensures consistent
// namespace path generation across the system.
//
// Parameters:
//   - groupID: Network group identifier
//   - persistent: true for persistent namespaces (bind mounts), false for temporary (symlinks)
//
// Returns: Full filesystem path for the namespace reference
func (no *NamespaceOperations) BuildNamespacePath(groupID string, persistent bool) string {
	if persistent {
		// Persistent namespaces go in /var/run/netns/ (standard Linux location)
		return filepath.Join(no.paths.VarRunNetns, groupID)
	}
	// Temporary namespaces go in /tmp/shared-netns/
	return filepath.Join(no.paths.NetnsPath, groupID)
}

// ensureDir ensures that the directory for a given path exists.
// This helper method creates any missing parent directories needed
// for namespace file creation.
//
// Parameters:
//   - targetPath: Full path to file (directory portion will be created)
//
// Returns: nil on success, error if directory creation fails
func (no *NamespaceOperations) ensureDir(targetPath string) error {
	dir := filepath.Dir(targetPath)

	// Check if directory already exists
	if _, err := no.osInterface.Stat(dir); err != nil {
		if no.osInterface.IsNotExist(err) {
			// Directory doesn't exist, create it with appropriate permissions
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

// listNamespacesInPath lists namespaces in a specific directory.
// This helper method scans a single directory for namespace references
// and returns information about each found namespace.
//
// Parameters:
//   - basePath: Directory to scan for namespace references
//
// Returns: Slice of NamespaceInfo for namespaces in the directory
func (no *NamespaceOperations) listNamespacesInPath(basePath string) ([]*NamespaceInfo, error) {
	// Check if directory exists before attempting to read it
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

	// Process each entry in the directory
	for _, entry := range entries {
		nsPath := filepath.Join(basePath, entry.Name())

		// Get detailed information about this namespace
		info, err := no.GetNamespaceInfo(nsPath)
		if err != nil {
			no.logger.Warn("failed to get namespace info",
				"path", nsPath,
				"error", err)
			continue // Skip problematic entries
		}

		namespaces = append(namespaces, info)
	}

	return namespaces, nil
}

// ValidateNamespacePath validates that a namespace path is safe to use.
// This security method prevents path traversal attacks and ensures that
// namespace operations only affect files in allowed directories.
//
// Validation checks:
// 1. Path is not empty
// 2. No path traversal attempts (../ sequences)
// 3. Path is within allowed namespace directories
//
// Parameters:
//   - nsPath: Namespace path to validate
//
// Returns: nil if path is safe, error describing the security issue
func (no *NamespaceOperations) ValidateNamespacePath(nsPath string) error {
	if nsPath == "" {
		return fmt.Errorf("namespace path cannot be empty")
	}

	// Clean the path and check for traversal attempts
	// filepath.Clean removes redundant elements like ../ and ./
	cleanPath := filepath.Clean(nsPath)
	if cleanPath != nsPath {
		return fmt.Errorf("invalid namespace path (potential path traversal): %s", nsPath)
	}

	// Ensure path is within allowed namespace directories
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
