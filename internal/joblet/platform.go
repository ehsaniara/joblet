//go:build linux

package joblet

import (
	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/core"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/pkg/config"
)

// NewJoblet creates a platform-specific joblet implementation
func NewJoblet(store adapters.JobStorer, metricsStore *adapters.MetricsStoreAdapter, cfg *config.Config, networkStore adapters.NetworkStorer) interfaces.Joblet {
	return core.NewJoblet(store, metricsStore, cfg, networkStore)
}
