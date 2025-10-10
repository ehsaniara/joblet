package cloud

import (
	"context"

	"joblet/internal/joblet/monitoring/domain"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// CloudDetector provides cloud environment detection capabilities
//
//counterfeiter:generate . CloudDetector
type CloudDetector interface {
	// DetectCloudEnvironment detects the current cloud environment
	DetectCloudEnvironment(ctx context.Context) (*domain.CloudInfo, error)

	// GetCachedInfo returns the cached cloud information without re-detection
	GetCachedInfo() *domain.CloudInfo
}
