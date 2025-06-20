package worker

import (
	"job-worker/internal/worker/interfaces"
	"job-worker/internal/worker/jobworker"
	"runtime"
)

// New creates a platform-specific worker implementation
// This function works on all platforms and calls the appropriate platform-specific constructor
func New(store interfaces.Store) interfaces.JobWorker {
	switch runtime.GOOS {
	case "linux":
		return jobworker.NewLinuxWorker(store)
	case "darwin":
		return nil
	default:
		return nil
	}
}
