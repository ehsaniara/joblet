package state_test

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/state"
	"github.com/ehsaniara/joblet/internal/joblet/state/statefakes"
	"github.com/ehsaniara/joblet/pkg/logger"
)

func TestBatcher_CreateAsync(t *testing.T) {
	fakeClient := &statefakes.FakeStateClient{}
	log := logger.WithField("test", "batcher")

	batcher := state.NewBatcher(fakeClient, log)
	defer batcher.Close()

	// Create jobs asynchronously
	const numJobs = 100
	for i := 0; i < numJobs; i++ {
		job := &domain.Job{
			Uuid:      fmt.Sprintf("batch-job-%d", i),
			Command:   "echo",
			Args:      []string{"test"},
			Status:    "SCHEDULED",
			StartTime: time.Now(),
		}
		batcher.CreateAsync(job)
	}

	// Give time for batches to process
	time.Sleep(200 * time.Millisecond)

	// Should have called Sync at least once
	if fakeClient.SyncCallCount() == 0 {
		t.Error("Expected Sync to be called at least once")
	}

	// Check stats
	stats := batcher.Stats()
	t.Logf("Batcher stats: %+v", stats)

	if stats["batches_sent"].(uint64) == 0 {
		t.Error("Expected at least one batch to be sent")
	}

	if stats["operations_processed"].(uint64) < uint64(numJobs) {
		t.Errorf("Expected at least %d operations processed, got %v",
			numJobs, stats["operations_processed"])
	}
}

func TestBatcher_CreateSync(t *testing.T) {
	fakeClient := &statefakes.FakeStateClient{}
	log := logger.WithField("test", "batcher")

	// Configure fake to succeed
	fakeClient.SyncReturns(nil)

	batcher := state.NewBatcher(fakeClient, log)
	defer batcher.Close()

	ctx := context.Background()
	job := &domain.Job{
		Uuid:      "sync-job-1",
		Command:   "echo",
		Args:      []string{"test"},
		Status:    "SCHEDULED",
		StartTime: time.Now(),
	}

	err := batcher.Create(ctx, job)
	if err != nil {
		t.Errorf("Create failed: %v", err)
	}

	// Give time for batch to process
	time.Sleep(50 * time.Millisecond)

	// Should have called Sync
	if fakeClient.SyncCallCount() == 0 {
		t.Error("Expected Sync to be called")
	}
}

func TestBatcher_Batching(t *testing.T) {
	fakeClient := &statefakes.FakeStateClient{}
	log := logger.WithField("test", "batcher")

	var syncCallCount atomic.Int32
	fakeClient.SyncStub = func(ctx context.Context, jobs []*domain.Job) error {
		syncCallCount.Add(1)
		t.Logf("Sync called with %d jobs", len(jobs))
		return nil
	}

	batcher := state.NewBatcher(fakeClient, log)
	defer batcher.Close()

	// Create exactly 25 jobs (max batch size)
	const numJobs = 25
	for i := 0; i < numJobs; i++ {
		job := &domain.Job{
			Uuid:      fmt.Sprintf("batch-job-%d", i),
			Command:   "echo",
			Args:      []string{"test"},
			Status:    "SCHEDULED",
			StartTime: time.Now(),
		}
		batcher.CreateAsync(job)
	}

	// Give time for batch to process
	time.Sleep(200 * time.Millisecond)

	// Should have called Sync at least once
	calls := syncCallCount.Load()
	if calls == 0 {
		t.Error("Expected Sync to be called at least once")
	}

	t.Logf("Sync was called %d times for %d jobs", calls, numJobs)
}

func TestBatcher_ErrorHandling(t *testing.T) {
	fakeClient := &statefakes.FakeStateClient{}
	log := logger.WithField("test", "batcher")

	// Configure fake to fail
	expectedErr := fmt.Errorf("sync failed")
	fakeClient.SyncReturns(expectedErr)

	batcher := state.NewBatcher(fakeClient, log)
	defer batcher.Close()

	ctx := context.Background()
	job := &domain.Job{
		Uuid:      "error-job-1",
		Command:   "echo",
		Args:      []string{"test"},
		Status:    "SCHEDULED",
		StartTime: time.Now(),
	}

	err := batcher.Create(ctx, job)
	if err == nil {
		t.Error("Expected error from Create")
	}

	if !strings.Contains(err.Error(), "sync failed") {
		t.Errorf("Expected error containing 'sync failed', got: %v", err)
	}
}

func TestBatcher_QueueFull(t *testing.T) {
	fakeClient := &statefakes.FakeStateClient{}
	log := logger.WithField("test", "batcher")

	// Don't process anything so queue fills up
	fakeClient.SyncStub = func(ctx context.Context, jobs []*domain.Job) error {
		time.Sleep(10 * time.Second) // Block forever
		return nil
	}

	batcher := state.NewBatcher(fakeClient, log)
	defer batcher.Close()

	// Try to overflow the queue (defaultBatchChannelSize = 10000)
	const numJobs = 10000 + 1000
	for i := 0; i < numJobs; i++ {
		job := &domain.Job{
			Uuid:      fmt.Sprintf("overflow-job-%d", i),
			Command:   "echo",
			Args:      []string{"test"},
			Status:    "SCHEDULED",
			StartTime: time.Now(),
		}
		batcher.CreateAsync(job) // Should drop some
	}

	// Check stats
	stats := batcher.Stats()
	queueSize := stats["queue_size"].(int)

	// Queue should be full or close to it (defaultBatchChannelSize = 10000)
	const expectedCapacity = 10000
	t.Logf("Queue size: %d / %d", queueSize, expectedCapacity)

	if queueSize < expectedCapacity-100 {
		t.Errorf("Expected queue to be nearly full, got %d / %d",
			queueSize, expectedCapacity)
	}
}

func TestBatcher_Shutdown(t *testing.T) {
	fakeClient := &statefakes.FakeStateClient{}
	log := logger.WithField("test", "batcher")

	batcher := state.NewBatcher(fakeClient, log)

	// Add some jobs
	for i := 0; i < 10; i++ {
		job := &domain.Job{
			Uuid:      fmt.Sprintf("shutdown-job-%d", i),
			Command:   "echo",
			Args:      []string{"test"},
			Status:    "SCHEDULED",
			StartTime: time.Now(),
		}
		batcher.CreateAsync(job)
	}

	// Give batch processor time to pick up jobs
	time.Sleep(150 * time.Millisecond)

	// Close should flush remaining jobs
	err := batcher.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Should have processed jobs
	if fakeClient.SyncCallCount() == 0 {
		t.Error("Expected Sync to be called during shutdown")
	}
}

func TestBatcher_ConcurrentWrites(t *testing.T) {
	fakeClient := &statefakes.FakeStateClient{}
	log := logger.WithField("test", "batcher")

	fakeClient.SyncReturns(nil)

	batcher := state.NewBatcher(fakeClient, log)
	defer batcher.Close()

	const numGoroutines = 10
	const opsPerGoroutine = 10

	done := make(chan struct{})

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < opsPerGoroutine; j++ {
				job := &domain.Job{
					Uuid:      fmt.Sprintf("concurrent-job-%d-%d", id, j),
					Command:   "echo",
					Args:      []string{"test"},
					Status:    "SCHEDULED",
					StartTime: time.Now(),
				}
				batcher.CreateAsync(job)
			}
			done <- struct{}{}
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Give time for batches to process
	time.Sleep(300 * time.Millisecond)

	stats := batcher.Stats()
	t.Logf("Stats after concurrent writes: %+v", stats)

	expectedOps := uint64(numGoroutines * opsPerGoroutine)
	if stats["operations_processed"].(uint64) < expectedOps {
		t.Errorf("Expected at least %d operations processed, got %v",
			expectedOps, stats["operations_processed"])
	}
}
