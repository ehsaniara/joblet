package core

import (
	"joblet/internal/joblet/adapters"
)

// OutputWriter provides an io.Writer implementation that streams job output
// to the job storage buffer system for real-time log streaming.
// Thread-safe for concurrent writes from multiple goroutines.
type OutputWriter struct {
	jobId string
	store adapters.JobStoreAdapter
}

// NewWrite creates a new OutputWriter for the specified job.
// The writer will send all output to the job's buffer for real-time streaming.
//
// Parameters:
//   - store: Job storage adapter for buffer management
//   - jobId: Unique identifier for the job
//
// Returns: OutputWriter instance configured for the specified job
func NewWrite(store adapters.JobStoreAdapter, jobId string) *OutputWriter {
	return &OutputWriter{store: store, jobId: jobId}
}

// Write implements the io.Writer interface for job output streaming.
// Creates a copy of the input data to prevent race conditions with buffer reuse.
// Sends the data to the job's buffer for real-time log streaming to clients.
// Always returns success to prevent command execution failures due to logging issues.
//
// Parameters:
//   - p: Byte slice containing the output data to write
//
// Returns: Number of bytes written (always len(p)), and nil error
func (w *OutputWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Create a copy of the data to prevent races
	// The underlying buffer p might be reused by the caller
	chunk := make([]byte, len(p))
	copy(chunk, p)

	w.store.WriteToBuffer(w.jobId, chunk)

	// Return the number of bytes written (always successful)
	return len(p), nil
}
