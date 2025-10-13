package ipc

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/adapters"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	ipcpb "github.com/ehsaniara/joblet/internal/proto/gen/ipc"
	"github.com/ehsaniara/joblet/pkg/logger"
)

// Subscriber subscribes to pub/sub and forwards to IPC writer
type Subscriber struct {
	writer *Writer
	pubsub pubsub.PubSub[adapters.JobEvent]
	logger *logger.Logger

	// Metrics
	eventsProcessed atomic.Uint64
	logsSent        atomic.Uint64
	errors          atomic.Uint64

	// Lifecycle
	ctx         context.Context
	cancel      context.CancelFunc
	unsubscribe func()
}

// NewSubscriber creates a new IPC subscriber
func NewSubscriber(writer *Writer, ps pubsub.PubSub[adapters.JobEvent], log *logger.Logger) *Subscriber {
	ctx, cancel := context.WithCancel(context.Background())

	return &Subscriber{
		writer: writer,
		pubsub: ps,
		logger: log.WithField("component", "ipc-subscriber"),
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start begins subscribing to all job events
func (s *Subscriber) Start() error {
	// Subscribe to the single "jobs" topic (all jobs publish here)
	updates, unsubscribe, err := s.pubsub.Subscribe(s.ctx, "jobs")
	if err != nil {
		return err
	}

	s.unsubscribe = unsubscribe
	s.logger.Info("IPC subscriber started, listening to job events")

	// Process events in background
	go s.processEvents(updates)

	return nil
}

// processEvents handles incoming pub/sub events
func (s *Subscriber) processEvents(updates <-chan pubsub.Message[adapters.JobEvent]) {
	sequence := make(map[string]uint64) // jobID -> sequence number

	for {
		select {
		case <-s.ctx.Done():
			return
		case msg, ok := <-updates:
			if !ok {
				s.logger.Info("Pub/sub channel closed")
				return
			}

			s.eventsProcessed.Add(1)
			event := msg.Payload

			// Only process LOG_CHUNK events
			if event.Type != "LOG_CHUNK" {
				continue
			}

			if len(event.LogChunk) == 0 {
				continue
			}

			// Get or initialize sequence for this job
			jobID := event.JobID
			seq := sequence[jobID]
			sequence[jobID] = seq + 1

			// Determine stream type (we don't have this info in JobEvent currently, default to stdout)
			// TODO: JobEvent should include stream type (stdout/stderr)
			streamType := ipcpb.StreamType_STREAM_TYPE_STDOUT

			// Send to IPC writer
			timestamp := time.Now().UnixNano()
			if event.Timestamp > 0 {
				timestamp = event.Timestamp * 1000000000 // Convert seconds to nanos
			}

			if err := s.writer.WriteLog(jobID, streamType, timestamp, seq, event.LogChunk); err != nil {
				s.errors.Add(1)
				s.logger.Warn("Failed to write log to IPC",
					"jobID", jobID,
					"error", err,
					"chunkSize", len(event.LogChunk))
			} else {
				s.logsSent.Add(1)
				s.logger.Info("Forwarded log chunk to IPC",
					"jobID", jobID,
					"sequence", seq,
					"size", len(event.LogChunk))
			}
		}
	}
}

// Stop stops the subscriber
func (s *Subscriber) Stop() {
	s.logger.Info("Stopping IPC subscriber")
	s.cancel()

	if s.unsubscribe != nil {
		s.unsubscribe()
	}

	s.logger.Info("IPC subscriber stopped",
		"eventsProcessed", s.eventsProcessed.Load(),
		"logsSent", s.logsSent.Load(),
		"errors", s.errors.Load())
}

// Stats returns subscriber statistics
func (s *Subscriber) Stats() SubscriberStats {
	return SubscriberStats{
		EventsProcessed: s.eventsProcessed.Load(),
		LogsSent:        s.logsSent.Load(),
		Errors:          s.errors.Load(),
	}
}

// SubscriberStats represents subscriber statistics
type SubscriberStats struct {
	EventsProcessed uint64
	LogsSent        uint64
	Errors          uint64
}
