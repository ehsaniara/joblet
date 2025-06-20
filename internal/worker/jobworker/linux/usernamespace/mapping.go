//go:build linux

package usernamespace

import (
	"context"
	"syscall"
)

// UserNamespaceManager handles user namespace operations
//
//counterfeiter:generate . UserNamespaceManager
type UserNamespaceManager interface {
	// CreateUserMapping creates UID/GID mappings for a job
	CreateUserMapping(ctx context.Context, jobID string) (*UserMapping, error)

	// CleanupUserMapping removes user mappings for a job
	CleanupUserMapping(jobID string) error

	// GetJobUID returns the UID that should be used inside the namespace
	GetJobUID(jobID string) (uint32, error)

	// GetJobGID returns the GID that should be used inside the namespace
	GetJobGID(jobID string) (uint32, error)

	// ConfigureSysProcAttr adds user namespace flags to syscall attributes
	ConfigureSysProcAttr(attr *syscall.SysProcAttr, mapping *UserMapping) *syscall.SysProcAttr

	// ValidateSubUIDGID to validate Sub UID and GID
	ValidateSubUIDGID() error
}

// UserMapping represents the UID/GID mapping for a job
type UserMapping struct {
	JobID        string
	NamespaceUID uint32 // UID inside the user namespace (usually 0 for root)
	NamespaceGID uint32 // GID inside the user namespace (usually 0 for root)
	HostUID      uint32 // Mapped UID on the host (unprivileged)
	HostGID      uint32 // Mapped GID on the host (unprivileged)
	UIDMapPath   string // Path to uid_map file
	GIDMapPath   string // Path to gid_map file
	SubUIDRange  *UIDRange
	SubGIDRange  *GIDRange
}

// UIDRange represents a range of UIDs
type UIDRange struct {
	Start uint32
	Count uint32
}

// GIDRange represents a range of GIDs
type GIDRange struct {
	Start uint32
	Count uint32
}

// UserNamespaceConfig contains configuration for user namespaces
type UserNamespaceConfig struct {
	// Base UID/GID range for job isolation (e.g., 100000-165535)
	BaseUID uint32
	BaseGID uint32

	// Range size per job (default: 65536)
	RangeSize uint32

	// Maximum concurrent jobs (for UID allocation)
	MaxJobs uint32

	// SubUID/SubGID file paths
	SubUIDFile string
	SubGIDFile string

	// Whether to use setuid helper for mapping
	UseSetuidHelper  bool
	SetuidHelperPath string
}

// DefaultUserNamespaceConfig returns default configuration
func DefaultUserNamespaceConfig() *UserNamespaceConfig {
	return &UserNamespaceConfig{
		BaseUID:          100000,
		BaseGID:          100000,
		RangeSize:        65536,
		MaxJobs:          100,
		SubUIDFile:       "/etc/subuid",
		SubGIDFile:       "/etc/subgid",
		UseSetuidHelper:  false,
		SetuidHelperPath: "/usr/bin/newuidmap",
	}
}
