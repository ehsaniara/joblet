//go:build linux

package joblet

import (
	"joblet/internal/joblet/adapters"
	"joblet/internal/joblet/core"
	"joblet/internal/joblet/core/interfaces"
	"joblet/pkg/config"
)

// NewJoblet creates a platform-specific joblet implementation
func NewJoblet(store adapters.JobStoreAdapter, cfg *config.Config, networkStore adapters.NetworkStoreAdapter) interfaces.Joblet {
	return core.NewJoblet(store, cfg, networkStore)
}
