package upload

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

const (
	UploadTimeout    = 5 * time.Minute
	DefaultChunkSize = 64 * 1024 // Default chunk size
)

// Ensure Manager implements domain.UploadManager
var _ domain.UploadManager = (*Manager)(nil)

// Manager handles streaming file uploads with memory-aware chunking
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

// PrepareUploadSession creates an upload session optimized for the given memory limit
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

// CreateUploadPipe creates a named pipe for streaming files
func (m *Manager) CreateUploadPipe(jobID string) (string, error) {
	pipeDir := fmt.Sprintf("/opt/joblet/jobs/%s/pipes", jobID)

	// Create directory with proper permissions
	if err := m.platform.MkdirAll(pipeDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create pipe directory: %w", err)
	}

	pipePath := filepath.Join(pipeDir, "upload.fifo")

	// Create named pipe
	if err := syscall.Mkfifo(pipePath, 0600); err != nil {
		return "", fmt.Errorf("failed to create named pipe: %w", err)
	}

	m.logger.Debug("upload pipe created", "pipePath", pipePath)
	return pipePath, nil
}

// StreamAllFiles streams all files through the named pipe with memory monitoring
func (m *Manager) StreamAllFiles(ctx context.Context, session *domain.UploadSession, pipePath string) error {
	if len(session.Files) == 0 {
		return nil
	}

	log := m.logger.WithField("operation", "stream-all-files")
	log.Debug("starting file streaming", "fileCount", len(session.Files))

	// Open pipe for writing (this will block until reader opens it)
	pipe, err := m.platform.OpenFile(pipePath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("failed to open upload pipe: %w", err)
	}
	defer pipe.Close()

	// Stream each file using the same mechanism
	for _, file := range session.Files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := m.streamSingleFile(ctx, pipe, file, session.ChunkSize); err != nil {
			return fmt.Errorf("failed to stream file %s: %w", file.Path, err)
		}
	}

	return nil
}

// streamSingleFile streams a single file with chunking and memory monitoring
func (m *Manager) streamSingleFile(ctx context.Context, pipe io.Writer, file domain.FileUpload, chunkSize int) error {
	// Write file header (path, size, mode, isDirectory)
	header := fmt.Sprintf("FILE:%s:%d:%d:%t\n", file.Path, file.Size, file.Mode, file.IsDirectory)
	if _, err := pipe.Write([]byte(header)); err != nil {
		return fmt.Errorf("failed to write file header: %w", err)
	}

	// For directories, just write the header
	if file.IsDirectory {
		return nil
	}

	// Stream file content in chunks
	content := file.Content
	totalWritten := 0

	for totalWritten < len(content) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check memory pressure before each chunk
		if err := m.checkMemoryPressure(); err != nil {
			m.logger.Warn("memory pressure detected, reducing chunk size", "error", err)
			chunkSize = chunkSize / 2
			if chunkSize < 1024 {
				return fmt.Errorf("memory pressure too high for upload")
			}
		}

		// Calculate chunk size for this iteration
		remaining := len(content) - totalWritten
		currentChunkSize := chunkSize
		if remaining < currentChunkSize {
			currentChunkSize = remaining
		}

		// Write chunk
		chunk := content[totalWritten : totalWritten+currentChunkSize]
		written, err := pipe.Write(chunk)
		if err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}

		totalWritten += written

		// Small delay to prevent overwhelming the receiver
		if currentChunkSize == chunkSize {
			time.Sleep(time.Millisecond)
		}
	}

	return nil
}

// checkMemoryPressure monitors cgroup memory usage
func (m *Manager) checkMemoryPressure() error {
	// Read memory.pressure (cgroups v2) or memory.usage_in_bytes (cgroups v1)
	pressureFile := "/sys/fs/cgroup/memory.pressure"
	if _, err := m.platform.Stat(pressureFile); err == nil {
		// cgroups v2
		data, err := m.platform.ReadFile(pressureFile)
		if err != nil {
			return nil // Ignore errors, continue upload
		}

		// Parse pressure data - if "some avg60" > 50, we have pressure
		// Format: "some avg10=X.XX avg60=Y.YY avg300=Z.ZZ total=NNNN"
		pressureStr := string(data)
		if len(pressureStr) > 0 {
			// Simple heuristic: if file is not empty, there's some pressure
			return fmt.Errorf("memory pressure")
		}
	}

	return nil
}

// ProcessAllFiles processes all files directly in the workspace
func (m *Manager) ProcessAllFiles(session *domain.UploadSession, workspacePath string) error {
	log := m.logger.WithField("operation", "process-all-files")

	for _, file := range session.Files {
		fullPath := filepath.Join(workspacePath, file.Path)

		if file.IsDirectory {
			// Create directory
			mode := os.FileMode(file.Mode)
			if mode == 0 {
				mode = 0755
			}
			if err := m.platform.MkdirAll(fullPath, mode); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", file.Path, err)
			}
			log.Debug("created directory", "path", file.Path)
		} else {
			// Create parent directory
			parentDir := filepath.Dir(fullPath)
			if err := m.platform.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Write file
			mode := os.FileMode(file.Mode)
			if mode == 0 {
				mode = 0644
			}
			if err := m.platform.WriteFile(fullPath, file.Content, mode); err != nil {
				return fmt.Errorf("failed to write file %s: %w", file.Path, err)
			}
			log.Debug("wrote file", "path", file.Path, "size", len(file.Content))
		}
	}

	log.Info("processed all files", "count", len(session.Files), "workspacePath", workspacePath)
	return nil
}

// CleanupPipe removes the upload pipe
func (m *Manager) CleanupPipe(pipePath string) error {
	if pipePath == "" {
		return nil
	}

	if err := m.platform.Remove(pipePath); err != nil && !m.platform.IsNotExist(err) {
		m.logger.Warn("failed to cleanup upload pipe", "path", pipePath, "error", err)
		return err
	}

	// Remove pipe directory if empty
	pipeDir := filepath.Dir(pipePath)
	_ = m.platform.Remove(pipeDir) // Ignore errors, directory might not be empty

	return nil
}
