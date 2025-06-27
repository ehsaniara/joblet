package worker

import (
	"runtime"
	"worker/internal/worker/core"
	"worker/internal/worker/core/interfaces"
	"worker/internal/worker/state"
)

// NewWorker creates a platform-specific worker implementation
// This function works on all platforms and calls the appropriate platform-specific constructor
func NewWorker(store state.Store) interfaces.Worker {
	switch runtime.GOOS {
	case "linux":
		return core.NewLinuxWorker(store)
	case "darwin":
		return nil
	default:
		return nil
	}
}
