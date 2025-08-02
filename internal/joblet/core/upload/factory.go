package upload

import (
	"joblet/internal/joblet/domain"
	"joblet/pkg/logger"
	"joblet/pkg/platform"
)

// Factory creates upload streaming contexts
type Factory struct {
	platform platform.Platform
	logger   *logger.Logger
}

// NewFactory creates a new upload factory
func NewFactory(platform platform.Platform, logger *logger.Logger) *Factory {
	return &Factory{
		platform: platform,
		logger:   logger,
	}
}

// CreateStreamContext creates a new stream context
func (f *Factory) CreateStreamContext(session *domain.UploadSession, pipePath string, jobID string) domain.UploadStreamer {
	return NewStreamContext(session, pipePath, jobID, f.platform, f.logger)
}

// Factory provides upload streaming functionality
