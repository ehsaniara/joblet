//go:build linux

package joblet

import (
	"joblet/internal/joblet/adapters"
	"joblet/internal/joblet/core"
	"joblet/internal/joblet/core/interfaces"
	"joblet/pkg/config"
)

// NewJoblet creates a platform-specific joblet implementation
func NewJoblet(store adapters.JobStorer, cfg *config.Config, networkStore adapters.NetworkStorer) interfaces.Joblet {
	return core.NewJoblet(store, cfg, networkStore)
}
