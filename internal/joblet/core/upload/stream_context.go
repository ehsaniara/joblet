package upload

import (
	"context"
	"fmt"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"os"
	"sync"
	"syscall"
	"time"
)

// Ensure StreamContext implements domain.UploadStreamer
var _ domain.UploadStreamer = (*StreamContext)(nil)

// StreamContext holds information about an active upload streaming session
type StreamContext struct {
	Session  *domain.UploadSession
	PipePath string
	JobID    string
	manager  domain.UploadManager
	logger   *logger.Logger

	// Synchronization fields
	streamingReady chan struct{}
	once           sync.Once
}

// NewStreamContext creates a new stream context
func NewStreamContext(session *domain.UploadSession, pipePath string, jobID string, logger *logger.Logger) *StreamContext {
	return &StreamContext{
		Session:  session,
		PipePath: pipePath,
		JobID:    jobID,
		logger:   logger.WithField("component", "streamContext"),
	}
}

// GetPipePath returns the pipe path
func (sc *StreamContext) GetPipePath() string {
	return sc.PipePath
}

// GetJobID returns the job ID
func (sc *StreamContext) GetJobID() string {
	return sc.JobID
}

// SetManager sets the upload manager for the stream context
func (sc *StreamContext) SetManager(manager domain.UploadManager) {
	sc.manager = manager
}

// StartStreaming starts the background streaming of files with proper synchronization
func (sc *StreamContext) StartStreaming() error {
	if sc.Session == nil || len(sc.Session.Files) == 0 {
		return nil
	}

	if sc.manager == nil {
		return fmt.Errorf("manager not set")
	}

	log := sc.logger

	// Initialize synchronization
	sc.streamingReady = make(chan struct{})

	log.Info("starting upload streaming",
		"fileCount", len(sc.Session.Files),
		"pipePath", sc.PipePath)

	// Start the streaming goroutine
	go sc.streamingGoroutine()

	// Wait for the goroutine to be ready (but not for pipe to be fully open)
	select {
	case <-sc.streamingReady:
		log.Info("streaming goroutine is ready")
		return nil
	case <-time.After(2 * time.Second):
		// Don't fail if streaming setup takes time
		log.Warn("streaming setup is taking longer than expected, continuing anyway")
		return nil
	}
}

func (sc *StreamContext) streamingGoroutine() {
	log := sc.logger.WithField("goroutine", "streaming")

	// Signal that goroutine has started (not that pipe is open)
	sc.once.Do(func() {
		close(sc.streamingReady)
	})

	ctx, cancel := context.WithTimeout(context.Background(), UploadTimeout)
	defer cancel()

	// Try to open pipe with retry logic
	pipe, err := sc.openPipeWithRetry(ctx)
	if err != nil {
		log.Error("failed to open pipe after retries", "error", err)
		return
	}
	defer pipe.Close()

	// Stream files
	if err := sc.streamFilesToPipe(ctx, pipe); err != nil {
		log.Error("failed to stream files", "error", err)
	} else {
		log.Info("file streaming completed successfully")
	}

	// Cleanup pipe after streaming
	if err := sc.manager.CleanupPipe(sc.PipePath); err != nil {
		log.Warn("failed to cleanup pipe", "error", err)
	}
}

func (sc *StreamContext) openPipeWithRetry(ctx context.Context) (*os.File, error) {
	log := sc.logger.WithField("operation", "open-pipe-retry")

	retryInterval := 100 * time.Millisecond
	maxRetryInterval := 2 * time.Second

	for {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Try to open in non-blocking mode
		pipe, err := os.OpenFile(sc.PipePath, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if err == nil {
			// Successfully opened - now set to blocking mode for actual writes
			if e := syscall.SetNonblock(int(pipe.Fd()), false); e != nil {
				pipe.Close()
				return nil, fmt.Errorf("failed to set blocking mode: %w", e)
			}
			log.Debug("pipe opened successfully")
			return pipe, nil
		}

		// Check if error is ENXIO (no reader)
		if pathErr, ok := err.(*os.PathError); ok {
			if errno, ok := pathErr.Err.(syscall.Errno); ok && errno == syscall.ENXIO {
				log.Debug("no reader on pipe yet, retrying...", "retryInterval", retryInterval)

				// Wait before retry
				select {
				case <-time.After(retryInterval):
					// Exponential backoff
					retryInterval = retryInterval * 2
					if retryInterval > maxRetryInterval {
						retryInterval = maxRetryInterval
					}
					continue
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			}
		}

		// Other errors - return immediately
		return nil, fmt.Errorf("failed to open pipe: %w", err)
	}
}

// streamFilesToPipe handles the actual streaming to an open pipe
func (sc *StreamContext) streamFilesToPipe(ctx context.Context, pipe *os.File) error {
	if len(sc.Session.Files) == 0 {
		return nil
	}

	log := sc.logger.WithField("operation", "stream-files-to-pipe")
	log.Debug("streaming files to pipe", "fileCount", len(sc.Session.Files))

	// Stream each file
	for _, file := range sc.Session.Files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Write file header
		header := fmt.Sprintf("FILE:%s:%d:%d:%t\n",
			file.Path, file.Size, file.Mode, file.IsDirectory)

		if _, err := pipe.Write([]byte(header)); err != nil {
			return fmt.Errorf("failed to write file header: %w", err)
		}

		// Write file content in chunks
		if !file.IsDirectory && len(file.Content) > 0 {
			chunkSize := sc.Session.ChunkSize
			if chunkSize == 0 {
				chunkSize = DefaultChunkSize
			}

			for offset := 0; offset < len(file.Content); offset += chunkSize {
				end := offset + chunkSize
				if end > len(file.Content) {
					end = len(file.Content)
				}

				chunk := file.Content[offset:end]
				if _, err := pipe.Write(chunk); err != nil {
					return fmt.Errorf("failed to write chunk at offset %d: %w", offset, err)
				}

				// Small delay between chunks to prevent overwhelming the pipe
				if offset+chunkSize < len(file.Content) {
					time.Sleep(1 * time.Millisecond)
				}
			}
		}
	}

	log.Debug("all files streamed successfully")
	return nil
}

// StreamConfig contains configuration for streaming uploads
type StreamConfig struct {
	JobID        string
	Uploads      []domain.FileUpload
	MemoryLimit  int32
	WorkspaceDir string
}

// ProcessDirectUploads processes uploads directly to workspace (for scheduled jobs or immediate processing)
func (m *Manager) ProcessDirectUploads(ctx context.Context, config *StreamConfig) error {
	if len(config.Uploads) == 0 {
		return nil
	}

	log := m.logger.WithField("operation", "process-direct-uploads")
	log.Debug("processing direct uploads", "jobID", config.JobID,
		"uploadCount", len(config.Uploads), "workspace", config.WorkspaceDir)

	// Prepare session
	session, err := m.PrepareUploadSession(config.JobID, config.Uploads, config.MemoryLimit)
	if err != nil {
		return fmt.Errorf("failed to prepare upload session: %w", err)
	}

	// Process files directly to workspace
	if e := m.ProcessAllFiles(session, config.WorkspaceDir); e != nil {
		return fmt.Errorf("failed to process files: %w", e)
	}

	log.Debug("direct upload processing completed", "filesProcessed", len(session.Files))
	return nil
}
