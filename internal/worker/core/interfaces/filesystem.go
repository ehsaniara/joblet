package interfaces

import "context"

// FilesystemManager handles job filesystem isolation
//
//counterfeiter:generate . FilesystemManager
type FilesystemManager interface {
	// SetupIsolatedFilesystem creates and prepares an isolated filesystem for a job
	SetupIsolatedFilesystem(ctx context.Context, jobID string) (string, error)

	// CleanupIsolatedFilesystem removes the isolated filesystem for a job
	CleanupIsolatedFilesystem(jobID string) error

	// GetIsolatedRoot returns the root path for a job's isolated filesystem
	GetIsolatedRoot(jobID string) string
}
