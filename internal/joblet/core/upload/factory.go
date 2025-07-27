package upload

import (
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
)

// Factory creates upload streaming contexts
type Factory struct {
	logger *logger.Logger
}

// NewFactory creates a new upload factory
func NewFactory(logger *logger.Logger) *Factory {
	return &Factory{
		logger: logger,
	}
}

// CreateStreamContext creates a new stream context
func (f *Factory) CreateStreamContext(session *domain.UploadSession, pipePath string, jobID string) domain.UploadStreamer {
	return NewStreamContext(session, pipePath, jobID, f.logger)
}

// Ensure Factory implements domain.UploadSessionFactory
var _ domain.UploadSessionFactory = (*Factory)(nil)
