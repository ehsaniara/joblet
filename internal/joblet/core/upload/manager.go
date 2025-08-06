package upload

import (
	"fmt"
	"path/filepath"
	"syscall"

	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
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

// Streamer implements the improved upload streamer interface
type Streamer struct {
	jobID     string
	session   *domain.UploadSession
	transport domain.UploadTransport
	manager   domain.UploadManager
	logger    *logger.Logger
	platform  platform.Platform
	running   bool
}

// Ensure Streamer implements domain.UploadStreamer
var _ domain.UploadStreamer = (*Streamer)(nil)

// NewStreamer creates a new upload streamer
func NewStreamer(jobID string, session *domain.UploadSession, transport domain.UploadTransport, platform platform.Platform, logger *logger.Logger) *Streamer {
	return &Streamer{
		jobID:     jobID,
		session:   session,
		transport: transport,
		logger:    logger.WithField("component", "upload-streamer"),
		platform:  platform,
	}
}

// Start begins the streaming process
func (s *Streamer) Start() error {
	if s.running {
		return fmt.Errorf("streamer already running")
	}

	s.running = true
	s.logger.Debug("starting upload streaming", "jobID", s.jobID)

	// Get writer from transport
	writer, err := s.transport.GetWriter()
	if err != nil {
		s.running = false
		return fmt.Errorf("failed to get transport writer: %w", err)
	}
	defer writer.Close()

	// Stream each file
	for _, file := range s.session.Files {
		if err := s.streamSingleFile(writer, file); err != nil {
			s.running = false
			return fmt.Errorf("failed to stream file %s: %w", file.Path, err)
		}
	}

	s.logger.Debug("upload streaming completed", "jobID", s.jobID)
	s.running = false
	return nil
}

// Stop gracefully stops the streaming
func (s *Streamer) Stop() error {
	if !s.running {
		return nil
	}

	s.running = false
	s.logger.Debug("stopping upload streaming", "jobID", s.jobID)

	// Transport cleanup is handled elsewhere
	return nil
}

// SetManager sets the upload manager
func (s *Streamer) SetManager(manager domain.UploadManager) {
	s.manager = manager
}

// GetJobID returns the job ID this streamer is associated with
func (s *Streamer) GetJobID() string {
	return s.jobID
}

// GetTransport returns the transport mechanism
func (s *Streamer) GetTransport() domain.UploadTransport {
	return s.transport
}

// streamSingleFile streams a single file with the improved format
func (s *Streamer) streamSingleFile(writer interface{ Write([]byte) (int, error) }, file domain.FileUpload) error {
	// Write file header (path, size, mode, isDirectory)
	header := fmt.Sprintf("FILE:%s:%d:%d:%t\n", file.Path, file.Size, file.Mode, file.IsDirectory)
	if _, err := writer.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write file header: %w", err)
	}

	// For directories, just write the header
	if file.IsDirectory {
		return nil
	}

	// Stream file content in chunks
	content := file.Content
	chunkSize := s.session.ChunkSize
	totalWritten := 0

	for totalWritten < len(content) {
		// Calculate chunk size for this iteration
		remaining := len(content) - totalWritten
		currentChunkSize := chunkSize
		if remaining < currentChunkSize {
			currentChunkSize = remaining
		}

		// Write chunk
		chunk := content[totalWritten : totalWritten+currentChunkSize]
		written, err := writer.Write(chunk)
		if err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}

		totalWritten += written
	}

	return nil
}
