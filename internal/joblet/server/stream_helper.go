package server

import (
	"context"

	"github.com/ehsaniara/joblet/pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StreamConfig holds the configuration for unified streaming behavior.
// This abstraction handles the common pattern across logs, metrics, and telematics:
// 1. For completed jobs: send all historical data, then return
// 2. For jobs not found locally: query persist for historical data
// 3. For running jobs: send historical data first, then stream live updates
type StreamConfig struct {
	// JobUUID is the resolved job UUID
	JobUUID string

	// Logger for this stream operation
	Logger *logger.Logger

	// SendHistorical sends buffered and persisted historical data.
	// Returns the count of events sent and any error.
	SendHistorical func() (int, error)

	// QueryPersistOnly queries persist when job is not found locally.
	// Returns the count of events found and any error.
	QueryPersistOnly func() (int, error)

	// StreamLive streams live updates for running jobs.
	// Should block until job completes or context is cancelled.
	StreamLive func() error
}

// JobState represents the state of a job for streaming decisions
type JobState int

const (
	// JobStateRunning means the job is currently running
	JobStateRunning JobState = iota
	// JobStateCompleted means the job has finished (completed, failed, or stopped)
	JobStateCompleted
	// JobStateNotFound means the job was not found locally
	JobStateNotFound
)

// StreamWithHistory implements the unified streaming pattern for logs, metrics, and telematics.
// The flow is:
//   - Completed jobs: send historical data only
//   - Not found locally: query persist only
//   - Running jobs: send historical first, then stream live
func StreamWithHistory(ctx context.Context, cfg StreamConfig, state JobState) error {
	log := cfg.Logger

	switch state {
	case JobStateCompleted:
		// Job completed - send all historical data
		log.Debug("job completed, sending historical data only")
		_, err := cfg.SendHistorical()
		return err

	case JobStateNotFound:
		// Job not found locally - query persist for historical data
		log.Debug("job not found locally, querying persist for historical data")
		if cfg.QueryPersistOnly == nil {
			return status.Errorf(codes.NotFound, "job not found: %s", cfg.JobUUID)
		}
		count, err := cfg.QueryPersistOnly()
		if err != nil {
			return err
		}
		if count == 0 {
			return status.Errorf(codes.NotFound, "job not found: %s", cfg.JobUUID)
		}
		return nil

	case JobStateRunning:
		// Job running - send historical first, then stream live
		log.Debug("job running, sending historical data then streaming live")

		// Send historical data (don't fail if this errors, continue to live)
		if _, err := cfg.SendHistorical(); err != nil {
			log.Warn("failed to send historical data", "error", err)
			// Continue to live streaming even if historical fails
		}

		// Stream live updates
		if cfg.StreamLive == nil {
			log.Warn("no live streaming configured")
			return nil
		}
		return cfg.StreamLive()

	default:
		return status.Errorf(codes.Internal, "unknown job state")
	}
}

// DetermineJobState determines the streaming state for a job
func DetermineJobState(exists bool, isCompleted bool) JobState {
	if !exists {
		return JobStateNotFound
	}
	if isCompleted {
		return JobStateCompleted
	}
	return JobStateRunning
}
