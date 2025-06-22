package worker

import (
	"runtime"
	"worker/internal/worker/interfaces"
	"worker/internal/worker/jobworker"
)

// NewWorker creates a platform-specific worker implementation
// This function works on all platforms and calls the appropriate platform-specific constructor
func NewWorker(store interfaces.Store) interfaces.Worker {
	switch runtime.GOOS {
	case "linux":
		return jobworker.NewLinuxWorker(store)
	case "darwin":
		return nil
	default:
		return nil
	}
}
