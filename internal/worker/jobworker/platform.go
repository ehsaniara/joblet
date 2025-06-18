package jobworker

import (
	"job-worker/internal/worker/interfaces"
	"runtime"
)

// New creates a platform-specific worker implementation
// This function works on all platforms and calls the appropriate platform-specific constructor
func New(store interfaces.Store) interfaces.JobWorker {
	switch runtime.GOOS {
	case "linux":
		return NewLinuxWorker(store)
	case "darwin":
		return nil
	default:
		return nil
	}
}
