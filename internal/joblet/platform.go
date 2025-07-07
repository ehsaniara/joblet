package joblet

import (
	"joblet/internal/joblet/core"
	"joblet/internal/joblet/core/interfaces"
	"joblet/internal/joblet/state"
	"joblet/pkg/config"
)

// NewJoblet creates a platform-specific joblet implementation
func NewJoblet(store state.Store, cfg *config.Config) interfaces.Joblet {
	return core.NewJoblet(store, cfg)
}
