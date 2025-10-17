//go:build linux

package core

import (
	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/core/interfaces"
	"github.com/ehsaniara/joblet/pkg/config"
)

// NewJoblet creates a Linux joblet
func NewJoblet(store adapters.JobStorer, metricsStore *adapters.MetricsStoreAdapter, cfg *config.Config, networkStoreAdapter adapters.NetworkStorer) interfaces.Joblet {
	return NewPlatformJoblet(store, metricsStore, cfg, networkStoreAdapter)
}
