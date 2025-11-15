package upload

import (
	"fmt"
	"path/filepath"
	"syscall"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
)

// Ensure Manager implements domain.UploadManager
var _ domain.UploadManager = (*Manager)(nil)

// Manager implements the improved upload manager interface
type Manager struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewManager creates a new upload manager
func NewManager(platform platform.Platform, logger *logger.Logger) *Manager {
	return &Manager{
		platform: platform,
		logger:   logger.WithField("component", "upload-manager"),
	}
}

// PrepareUploadSession creates and optimizes an upload session for the given memory constraints.
// It validates upload content, calculates total file sizes, and configures memory-aware processing
// to ensure uploads fit within the specified memory limits without system resource exhaustion.
func (m *Manager) PrepareUploadSession(jobID string, uploads []domain.FileUpload, memoryLimitMB int32) (*domain.UploadSession, error) {
	session := &domain.UploadSession{
		JobID: jobID,
		Files: make([]domain.FileUpload, 0, len(uploads)),
	}

	// Optimize for memory constraints
	session.OptimizeForMemory(memoryLimitMB)

	// Process all files uniformly
	var totalSize int64
	for _, upload := range uploads {
		upload.Size = int64(len(upload.Content))
		totalSize += upload.Size
		session.TotalFiles++
		session.Files = append(session.Files, upload)
	}

	session.TotalSize = totalSize

	// Validate the session
	if err := session.ValidateUpload(); err != nil {
		return nil, fmt.Errorf("upload validation failed: %w", err)
	}

	m.logger.Debug("upload session prepared",
		"jobID", jobID,
		"totalFiles", session.TotalFiles,
		"totalSize", totalSize,
		"chunkSize", session.ChunkSize)

	return session, nil
}

// CreateTransport creates a transport for uploads (replaces CreateUploadPipe)
func (m *Manager) CreateTransport(jobID string) (domain.UploadTransport, error) {
	pipeDir := fmt.Sprintf("/opt/joblet/jobs/%s/pipes", jobID)

	// Create directory with proper permissions
	if err := m.platform.MkdirAll(pipeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create pipe directory: %w", err)
	}

	pipePath := filepath.Join(pipeDir, "upload.fifo")

	// Create named pipe
	if err := syscall.Mkfifo(pipePath, 0600); err != nil {
		return nil, fmt.Errorf("failed to create named pipe: %w", err)
	}

	transport := domain.NewPipeTransport(pipePath, m.platform, m.logger)
	m.logger.Debug("transport created", "pipePath", pipePath)

	return transport, nil
}

// CleanupTransport cleans up transport resources
func (m *Manager) CleanupTransport(transport domain.UploadTransport) error {
	if transport == nil {
		return nil
	}

	// Close the transport (which will clean up the pipe)
	if err := transport.Close(); err != nil {
		m.logger.Warn("failed to cleanup transport", "error", err)
		return err
	}

	return nil
}
