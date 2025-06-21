//go:build linux

package usernamespace

import (
	"context"
	"fmt"
	"job-worker/pkg/logger"
	osinterface "job-worker/pkg/os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
)

// userNamespaceManager implements UserNamespaceManager with UID recycling
type userNamespaceManager struct {
	config      *UserNamespaceConfig
	osInterface osinterface.OsInterface
	logger      *logger.Logger

	// UID/GID allocation tracking
	allocatedUIDs map[string]*UserMapping
	allocatedMu   sync.RWMutex

	// Track available UIDs for recycling
	availableUIDs []uint32
	nextUID       uint32
}

// NewUserNamespaceManager creates a new user namespace manager
func NewUserNamespaceManager(config *UserNamespaceConfig, osInterface osinterface.OsInterface) UserNamespaceManager {
	if config == nil {
		config = DefaultUserNamespaceConfig()
	}

	return &userNamespaceManager{
		config:        config,
		osInterface:   osInterface,
		logger:        logger.New().WithField("component", "user-namespace"),
		allocatedUIDs: make(map[string]*UserMapping),
		availableUIDs: make([]uint32, 0),
		nextUID:       config.BaseUID,
	}
}

// CreateUserMapping allocates unique UID/GID range with recycling support
func (m *userNamespaceManager) CreateUserMapping(ctx context.Context, jobID string) (*UserMapping, error) {
	m.allocatedMu.Lock()
	defer m.allocatedMu.Unlock()

	log := m.logger.WithField("jobID", jobID)

	// Check if mapping already exists
	if existing, exists := m.allocatedUIDs[jobID]; exists {
		log.Debug("user mapping already exists", "hostUID", existing.HostUID)
		return existing, nil
	}

	// Reuse released UID ranges before allocating new ones
	var hostUID uint32
	if len(m.availableUIDs) > 0 {
		// Reuse the first available UID
		hostUID = m.availableUIDs[0]
		m.availableUIDs = m.availableUIDs[1:] // Remove from available list
		log.Debug("reusing available UID", "hostUID", hostUID)
	} else {
		// Allocate new UID range if none available
		hostUID = m.nextUID

		// Ensure we don't exceed limits
		maxUID := m.config.BaseUID + (m.config.MaxJobs * m.config.RangeSize)
		if hostUID+m.config.RangeSize > maxUID {
			return nil, fmt.Errorf("UID range exhausted: no available UIDs to recycle and cannot allocate new ones (used %d/%d slots)",
				len(m.allocatedUIDs), m.config.MaxJobs)
		}

		m.nextUID += m.config.RangeSize
		log.Debug("allocated new UID", "hostUID", hostUID, "nextUID", m.nextUID)
	}

	hostGID := m.config.BaseGID + (hostUID - m.config.BaseUID)

	mapping := &UserMapping{
		JobID:        jobID,
		NamespaceUID: 0, // Always map to root inside namespace
		NamespaceGID: 0, // Always map to root inside namespace
		HostUID:      hostUID,
		HostGID:      hostGID,
		SubUIDRange: &UIDRange{
			Start: hostUID,
			Count: m.config.RangeSize,
		},
		SubGIDRange: &GIDRange{
			Start: hostGID,
			Count: m.config.RangeSize,
		},
	}

	// Store the mapping
	m.allocatedUIDs[jobID] = mapping

	log.Info("created user mapping with recycling",
		"namespaceUID", mapping.NamespaceUID,
		"hostUID", mapping.HostUID,
		"hostGID", mapping.HostGID,
		"rangeSize", m.config.RangeSize,
		"allocatedUIDs", len(m.allocatedUIDs),
		"availableUIDs", len(m.availableUIDs))

	return mapping, nil
}

// CleanupUserMapping removes user mappings for a job and makes UID available for reuse
func (m *userNamespaceManager) CleanupUserMapping(jobID string) error {
	m.allocatedMu.Lock()
	defer m.allocatedMu.Unlock()

	log := m.logger.WithField("jobID", jobID)

	mapping, exists := m.allocatedUIDs[jobID]
	if !exists {
		log.Debug("no user mapping to cleanup")
		return nil
	}

	// Remove from tracking
	delete(m.allocatedUIDs, jobID)

	// Add UID back to available pool for recycling
	m.availableUIDs = append(m.availableUIDs, mapping.HostUID)

	// Keep available UIDs sorted for better debugging
	sort.Slice(m.availableUIDs, func(i, j int) bool {
		return m.availableUIDs[i] < m.availableUIDs[j]
	})

	// Clean up any namespace files if they exist
	if mapping.UIDMapPath != "" {
		if err := m.osInterface.Remove(mapping.UIDMapPath); err != nil && !m.osInterface.IsNotExist(err) {
			log.Warn("failed to remove uid_map file", "path", mapping.UIDMapPath, "error", err)
		}
	}

	if mapping.GIDMapPath != "" {
		if err := m.osInterface.Remove(mapping.GIDMapPath); err != nil && !m.osInterface.IsNotExist(err) {
			log.Warn("failed to remove gid_map file", "path", mapping.GIDMapPath, "error", err)
		}
	}

	log.Info("cleaned up user mapping and recycled UID",
		"hostUID", mapping.HostUID,
		"allocatedUIDs", len(m.allocatedUIDs),
		"availableUIDs", len(m.availableUIDs))

	return nil
}

// GetJobUID returns the UID that should be used inside the namespace
func (m *userNamespaceManager) GetJobUID(jobID string) (uint32, error) {
	m.allocatedMu.RLock()
	defer m.allocatedMu.RUnlock()

	mapping, exists := m.allocatedUIDs[jobID]
	if !exists {
		return 0, fmt.Errorf("no user mapping found for job %s", jobID)
	}

	return mapping.NamespaceUID, nil
}

// GetJobGID returns the GID that should be used inside the namespace
func (m *userNamespaceManager) GetJobGID(jobID string) (uint32, error) {
	m.allocatedMu.RLock()
	defer m.allocatedMu.RUnlock()

	mapping, exists := m.allocatedUIDs[jobID]
	if !exists {
		return 0, fmt.Errorf("no user mapping found for job %s", jobID)
	}

	return mapping.NamespaceGID, nil
}

// ConfigureSysProcAttr adds user namespace flags to syscall attributes
func (m *userNamespaceManager) ConfigureSysProcAttr(attr *syscall.SysProcAttr, mapping *UserMapping) *syscall.SysProcAttr {
	if attr == nil {
		attr = &syscall.SysProcAttr{}
	}

	// Add user namespace flag
	attr.Cloneflags |= syscall.CLONE_NEWUSER

	// Set up UID/GID mappings
	attr.UidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: int(mapping.NamespaceUID),
			HostID:      int(mapping.HostUID),
			Size:        int(mapping.SubUIDRange.Count),
		},
	}

	attr.GidMappings = []syscall.SysProcIDMap{
		{
			ContainerID: int(mapping.NamespaceGID),
			HostID:      int(mapping.HostGID),
			Size:        int(mapping.SubGIDRange.Count),
		},
	}

	// Ensure we deny setgroups for security
	attr.GidMappingsEnableSetgroups = false

	m.logger.Debug("configured user namespace attributes",
		"uidMapping", attr.UidMappings[0],
		"gidMapping", attr.GidMappings[0],
		"setgroupsDenied", true)

	return attr
}

// writeMapping writes content to a mapping file with proper error handling
func (m *userNamespaceManager) writeMapping(path, content string) error {
	// Check if file exists and is writable
	if _, err := m.osInterface.Stat(path); err != nil {
		return fmt.Errorf("mapping file not accessible: %s (%w)", path, err)
	}

	// Write the mapping
	if err := m.osInterface.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write to %s: %w", path, err)
	}

	m.logger.Debug("wrote user namespace mapping", "path", path, "content", content)
	return nil
}

// ValidateSubUIDGID validates that the system has proper subuid/subgid configuration
func (m *userNamespaceManager) ValidateSubUIDGID() error {
	// Check if files exist
	if _, err := m.osInterface.Stat(m.config.SubUIDFile); err != nil {
		return fmt.Errorf("subuid file not found: %s\n"+
			"Create it with: sudo touch /etc/subuid", m.config.SubUIDFile)
	}

	if _, err := m.osInterface.Stat(m.config.SubGIDFile); err != nil {
		return fmt.Errorf("subgid file not found: %s\n"+
			"Create it with: sudo touch /etc/subgid", m.config.SubGIDFile)
	}

	// Determine expected user
	expectedUser := "job-worker"
	if user := m.osInterface.Getenv("USER"); user != "" && user != "root" {
		expectedUser = user
	}

	// Check subuid content
	if err := m.checkUserInFile(m.config.SubUIDFile, expectedUser, "subuid"); err != nil {
		return err
	}

	// Check subgid content
	if err := m.checkUserInFile(m.config.SubGIDFile, expectedUser, "subgid"); err != nil {
		return err
	}

	m.logger.Info("subuid/subgid validation passed",
		"user", expectedUser,
		"subuidFile", m.config.SubUIDFile,
		"subgidFile", m.config.SubGIDFile)

	return nil
}

// checkUserInFile checks if a user has an entry in subuid/subgid file
func (m *userNamespaceManager) checkUserInFile(filePath, username, fileType string) error {
	data, err := m.osInterface.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", filePath, err)
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	var userEntries []string
	totalUIDs := uint32(0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, username+":") {
			userEntries = append(userEntries, line)

			// Try to parse the count for basic validation
			parts := strings.Split(line, ":")
			if len(parts) >= 3 {
				if count, err := strconv.ParseUint(parts[2], 10, 32); err == nil {
					totalUIDs += uint32(count)
				}
			}
		}
	}

	if len(userEntries) == 0 {
		example := fmt.Sprintf("%s:100000:65536", username)
		return fmt.Errorf("user '%s' not found in %s\n"+
			"Add entry like: %s\n"+
			"Run: echo '%s' | sudo tee -a %s",
			username, filePath, example, example, filePath)
	}

	// Basic check: ensure we have enough UIDs/GIDs for our needs
	requiredUIDs := m.config.MaxJobs * m.config.RangeSize
	if totalUIDs < requiredUIDs {
		return fmt.Errorf("insufficient %s allocation for user '%s'\n"+
			"Need: %d UIDs (%d jobs × %d UIDs each)\n"+
			"Have: %d UIDs\n"+
			"Current entries: %s\n"+
			"Consider increasing the count in %s",
			fileType, username, requiredUIDs, m.config.MaxJobs, m.config.RangeSize,
			totalUIDs, strings.Join(userEntries, ", "), filePath)
	}

	m.logger.Debug("user found in file",
		"user", username,
		"file", filePath,
		"entries", len(userEntries),
		"totalUIDs", totalUIDs)

	return nil
}
