package upload

import (
	"context"
	"fmt"
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
)

// StreamContext holds information about an active upload streaming session
type StreamContext struct {
	Session  *domain.UploadSession
	PipePath string
	JobID    string
	manager  *Manager
	logger   *logger.Logger
}

// StartStreaming starts the background streaming of files
func (sc *StreamContext) StartStreaming() error {
	if sc == nil || sc.Session == nil || len(sc.Session.Files) == 0 {
		return nil
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), UploadTimeout)
		defer cancel()

		if err := sc.manager.StreamAllFiles(ctx, sc.Session, sc.PipePath); err != nil {
			sc.logger.Error("failed to stream files", "error", err, "jobID", sc.JobID)
		}

		// Cleanup pipe after streaming
		_ = sc.manager.CleanupPipe(sc.PipePath)
	}()

	sc.logger.Debug("started background file streaming",
		"jobID", sc.JobID,
		"fileCount", len(sc.Session.Files),
		"pipePath", sc.PipePath)

	return nil
}

// SetManager sets the upload manager for the stream context
func (sc *StreamContext) SetManager(m *Manager) {
	sc.manager = m
	sc.logger = m.logger.WithField("streamContext", sc.JobID)
}

// StreamConfig contains configuration for streaming uploads
type StreamConfig struct {
	JobID        string
	Uploads      []domain.FileUpload
	MemoryLimit  int32
	WorkspaceDir string
}

// PrepareStreamingSession prepares a streaming session with proper resource constraints
func (m *Manager) PrepareStreamingSession(config *StreamConfig) (*StreamContext, error) {
	// Prepare upload session
	session, err := m.PrepareUploadSession(config.JobID, config.Uploads, config.MemoryLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare upload session: %w", err)
	}

	// Create upload pipe if files are present
	if len(session.Files) == 0 {
		return &StreamContext{
			Session: session,
			JobID:   config.JobID,
		}, nil
	}

	pipePath, err := m.CreateUploadPipe(config.JobID)
	if err != nil {
		return nil, fmt.Errorf("failed to create upload pipe: %w", err)
	}

	ctx := &StreamContext{
		Session:  session,
		PipePath: pipePath,
		JobID:    config.JobID,
	}
	ctx.SetManager(m)

	return ctx, nil
}

// ProcessDirectUploads processes uploads directly to workspace (for scheduled jobs or immediate processing)
func (m *Manager) ProcessDirectUploads(ctx context.Context, config *StreamConfig) error {
	if len(config.Uploads) == 0 {
		return nil
	}

	log := m.logger.WithField("operation", "process-direct-uploads")
	log.Debug("processing direct uploads",
		"jobID", config.JobID,
		"uploadCount", len(config.Uploads),
		"workspace", config.WorkspaceDir)

	// Prepare session
	session, err := m.PrepareUploadSession(config.JobID, config.Uploads, config.MemoryLimit)
	if err != nil {
		return fmt.Errorf("failed to prepare upload session: %w", err)
	}

	// Process files directly to workspace
	if err := m.ProcessAllFiles(session, config.WorkspaceDir); err != nil {
		return fmt.Errorf("failed to process files: %w", err)
	}

	log.Debug("direct upload processing completed", "filesProcessed", len(session.Files))
	return nil
}
