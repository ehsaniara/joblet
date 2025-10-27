package state

import (
	"context"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
)

const (
	// Default batch settings
	defaultMaxBatchSize     = 25 // Max items per batch (DynamoDB limit)
	defaultBatchTimeout     = 100 * time.Millisecond
	defaultBatchChannelSize = 10000 // Buffer for pending operations
)

// BatchOperation represents a batched state operation
type BatchOperation struct {
	Type      string      // "create", "update", "delete"
	Job       *domain.Job // For create/update
	JobID     string      // For delete
	Timestamp time.Time
	Result    chan error // Optional result channel
}

// Batcher batches state operations for improved throughput
type Batcher struct {
	client       StateClient
	operations   chan *BatchOperation
	maxBatchSize int
	batchTimeout time.Duration
	logger       *logger.Logger
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc

	// Metrics
	batchesSent    uint64
	operationsProc uint64
	errors         uint64
}

// NewBatcher creates a new operation batcher
func NewBatcher(client StateClient, logger *logger.Logger) *Batcher {
	if logger == nil {
		logger = logger.WithField("component", "state-batcher")
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &Batcher{
		client:       client,
		operations:   make(chan *BatchOperation, defaultBatchChannelSize),
		maxBatchSize: defaultMaxBatchSize,
		batchTimeout: defaultBatchTimeout,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
	}

	// Start batch processor
	b.wg.Add(1)
	go b.processBatches()

	return b
}

// Create queues a create operation
func (b *Batcher) Create(ctx context.Context, job *domain.Job) error {
	op := &BatchOperation{
		Type:      "create",
		Job:       job,
		Timestamp: time.Now(),
		Result:    make(chan error, 1),
	}

	select {
	case b.operations <- op:
		// Wait for result
		select {
		case err := <-op.Result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-b.ctx.Done():
		return b.ctx.Err()
	}
}

// Update queues an update operation
func (b *Batcher) Update(ctx context.Context, job *domain.Job) error {
	op := &BatchOperation{
		Type:      "update",
		Job:       job,
		Timestamp: time.Now(),
		Result:    make(chan error, 1),
	}

	select {
	case b.operations <- op:
		// Wait for result
		select {
		case err := <-op.Result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-b.ctx.Done():
		return b.ctx.Err()
	}
}

// Delete queues a delete operation
func (b *Batcher) Delete(ctx context.Context, jobID string) error {
	op := &BatchOperation{
		Type:      "delete",
		JobID:     jobID,
		Timestamp: time.Now(),
		Result:    make(chan error, 1),
	}

	select {
	case b.operations <- op:
		// Wait for result
		select {
		case err := <-op.Result:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	case <-ctx.Done():
		return ctx.Err()
	case <-b.ctx.Done():
		return b.ctx.Err()
	}
}

// CreateAsync queues a create operation without waiting for result (fire-and-forget)
func (b *Batcher) CreateAsync(job *domain.Job) {
	op := &BatchOperation{
		Type:      "create",
		Job:       job,
		Timestamp: time.Now(),
	}

	select {
	case b.operations <- op:
		// Queued successfully
	default:
		// Channel full, log warning
		b.logger.Warn("batch queue full, dropping create operation", "jobId", job.Uuid)
	}
}

// UpdateAsync queues an update operation without waiting for result
func (b *Batcher) UpdateAsync(job *domain.Job) {
	op := &BatchOperation{
		Type:      "update",
		Job:       job,
		Timestamp: time.Now(),
	}

	select {
	case b.operations <- op:
		// Queued successfully
	default:
		b.logger.Warn("batch queue full, dropping update operation", "jobId", job.Uuid)
	}
}

// DeleteAsync queues a delete operation without waiting for result
func (b *Batcher) DeleteAsync(jobID string) {
	op := &BatchOperation{
		Type:      "delete",
		JobID:     jobID,
		Timestamp: time.Now(),
	}

	select {
	case b.operations <- op:
		// Queued successfully
	default:
		b.logger.Warn("batch queue full, dropping delete operation", "jobId", jobID)
	}
}

// processBatches processes operations in batches
func (b *Batcher) processBatches() {
	defer b.wg.Done()

	batch := make([]*BatchOperation, 0, b.maxBatchSize)
	timer := time.NewTimer(b.batchTimeout)
	defer timer.Stop()

	for {
		select {
		case op := <-b.operations:
			batch = append(batch, op)

			// Send batch if full
			if len(batch) >= b.maxBatchSize {
				b.sendBatch(batch)
				batch = make([]*BatchOperation, 0, b.maxBatchSize)
				timer.Reset(b.batchTimeout)
			}

		case <-timer.C:
			// Send batch on timeout
			if len(batch) > 0 {
				b.sendBatch(batch)
				batch = make([]*BatchOperation, 0, b.maxBatchSize)
			}
			timer.Reset(b.batchTimeout)

		case <-b.ctx.Done():
			// Flush remaining batch on shutdown
			if len(batch) > 0 {
				b.sendBatch(batch)
			}
			return
		}
	}
}

// sendBatch sends a batch of operations
func (b *Batcher) sendBatch(batch []*BatchOperation) {
	if len(batch) == 0 {
		return
	}

	b.batchesSent++
	b.operationsProc += uint64(len(batch))

	// Group operations by type
	creates := make([]*domain.Job, 0, len(batch))
	updates := make([]*domain.Job, 0, len(batch))
	deletes := make([]string, 0, len(batch))

	createOps := make([]*BatchOperation, 0, len(batch))
	updateOps := make([]*BatchOperation, 0, len(batch))
	deleteOps := make([]*BatchOperation, 0, len(batch))

	for _, op := range batch {
		switch op.Type {
		case "create":
			creates = append(creates, op.Job)
			createOps = append(createOps, op)
		case "update":
			updates = append(updates, op.Job)
			updateOps = append(updateOps, op)
		case "delete":
			deletes = append(deletes, op.JobID)
			deleteOps = append(deleteOps, op)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send creates as batch
	if len(creates) > 0 {
		if err := b.client.Sync(ctx, creates); err != nil {
			b.logger.Error("batch create failed", "count", len(creates), "error", err)
			b.errors++
			// Notify waiters
			for _, op := range createOps {
				if op.Result != nil {
					select {
					case op.Result <- err:
					default:
					}
				}
			}
		} else {
			// Success - notify waiters
			for _, op := range createOps {
				if op.Result != nil {
					select {
					case op.Result <- nil:
					default:
					}
				}
			}
		}
	}

	// Send updates as batch
	if len(updates) > 0 {
		if err := b.client.Sync(ctx, updates); err != nil {
			b.logger.Error("batch update failed", "count", len(updates), "error", err)
			b.errors++
			for _, op := range updateOps {
				if op.Result != nil {
					select {
					case op.Result <- err:
					default:
					}
				}
			}
		} else {
			for _, op := range updateOps {
				if op.Result != nil {
					select {
					case op.Result <- nil:
					default:
					}
				}
			}
		}
	}

	// Send deletes individually (no batch delete in current API)
	if len(deletes) > 0 {
		for i, jobID := range deletes {
			if err := b.client.Delete(ctx, jobID); err != nil {
				b.logger.Error("batch delete failed", "jobId", jobID, "error", err)
				b.errors++
				if deleteOps[i].Result != nil {
					select {
					case deleteOps[i].Result <- err:
					default:
					}
				}
			} else {
				if deleteOps[i].Result != nil {
					select {
					case deleteOps[i].Result <- nil:
					default:
					}
				}
			}
		}
	}

	b.logger.Debug("batch processed",
		"total", len(batch),
		"creates", len(creates),
		"updates", len(updates),
		"deletes", len(deletes))
}

// Close shuts down the batcher
func (b *Batcher) Close() error {
	b.cancel()
	b.wg.Wait()

	b.logger.Info("batcher closed",
		"batches_sent", b.batchesSent,
		"operations_processed", b.operationsProc,
		"errors", b.errors)

	return nil
}

// Stats returns batcher statistics
func (b *Batcher) Stats() map[string]interface{} {
	return map[string]interface{}{
		"batches_sent":         b.batchesSent,
		"operations_processed": b.operationsProc,
		"errors":               b.errors,
		"queue_size":           len(b.operations),
		"queue_capacity":       cap(b.operations),
	}
}
