package worker

import (
	"worker/internal/worker/core"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/state"
	"worker/pkg/config"
)

// NewWorker creates a platform-specific worker implementation
func NewWorker(store state.Store, cfg *config.Config) interfaces.Worker {
	return core.NewWorker(store, cfg)
}
