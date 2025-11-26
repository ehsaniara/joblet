package adapters

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/pubsub"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/stretchr/testify/assert"
)

// TestWriteToBuffer_PersistEnabled verifies that logs ARE buffered when persist is enabled
func TestWriteToBuffer_PersistEnabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log) // persistClient=nil, stateClient=nil, persistEnabled = true
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-123"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write to buffer
	testData := []byte("test log chunk")
	jobStoreAdapter.WriteToBuffer(jobID, testData)

	// Verify: Data should be in buffer (persist enabled)
	chunks := buffer.ReadAll()
	assert.Equal(t, 1, len(chunks), "Buffer should contain 1 chunk when persist enabled")
	assert.Equal(t, testData, chunks[0], "Buffered data should match written data")
}

// TestWriteToBuffer_PersistDisabled verifies that logs are NOT buffered when persist is disabled
func TestWriteToBuffer_PersistDisabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, false, log) // persistClient=nil, stateClient=nil, persistEnabled = false
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-456"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write to buffer
	testData := []byte("test log chunk")
	jobStoreAdapter.WriteToBuffer(jobID, testData)

	// Verify: Data should NOT be in buffer (persist disabled)
	chunks := buffer.ReadAll()
	assert.Equal(t, 0, len(chunks), "Buffer should be empty when persist disabled (no buffering)")
}

// TestWriteToBuffer_MultipleWrites_PersistEnabled verifies multiple writes are buffered
func TestWriteToBuffer_MultipleWrites_PersistEnabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log) // persistClient=nil, stateClient=nil, persistEnabled = true
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-789"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write multiple chunks
	testData1 := []byte("chunk 1")
	testData2 := []byte("chunk 2")
	testData3 := []byte("chunk 3")

	jobStoreAdapter.WriteToBuffer(jobID, testData1)
	jobStoreAdapter.WriteToBuffer(jobID, testData2)
	jobStoreAdapter.WriteToBuffer(jobID, testData3)

	// Verify: All chunks should be in buffer
	chunks := buffer.ReadAll()
	assert.Equal(t, 3, len(chunks), "Buffer should contain 3 chunks when persist enabled")
	assert.Equal(t, testData1, chunks[0])
	assert.Equal(t, testData2, chunks[1])
	assert.Equal(t, testData3, chunks[2])
}

// TestWriteToBuffer_MultipleWrites_PersistDisabled verifies multiple writes skip buffering
func TestWriteToBuffer_MultipleWrites_PersistDisabled(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, false, log) // persistClient=nil, stateClient=nil, persistEnabled = false
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "test-job-000"
	job := &domain.Job{
		Uuid:   jobID,
		Status: "RUNNING",
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Write multiple chunks
	jobStoreAdapter.WriteToBuffer(jobID, []byte("chunk 1"))
	jobStoreAdapter.WriteToBuffer(jobID, []byte("chunk 2"))
	jobStoreAdapter.WriteToBuffer(jobID, []byte("chunk 3"))

	// Verify: Buffer should remain empty (all writes skipped)
	chunks := buffer.ReadAll()
	assert.Equal(t, 0, len(chunks), "Buffer should remain empty when persist disabled (no buffering)")
}

// TestUpdateJob_Concurrent verifies that concurrent UpdateJob calls don't race
func TestUpdateJob_Concurrent(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log)
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "concurrent-update-job"
	job := &domain.Job{
		Uuid:   jobID,
		Status: domain.StatusRunning,
	}

	// Add job to store first
	_ = store.Create(context.Background(), jobID, job)

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job.DeepCopy(),
			logBuffer: buffer,
			pubsub:    ps,
		},
	}

	// Test: Concurrent UpdateJob calls
	var wg sync.WaitGroup
	numGoroutines := 10
	numUpdates := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numUpdates; j++ {
				updatedJob := &domain.Job{
					Uuid:   jobID,
					Status: domain.StatusRunning,
				}
				jobStoreAdapter.UpdateJob(updatedJob)
			}
		}(i)
	}

	wg.Wait()

	// Verify: Job should still exist and be accessible
	jobStoreAdapter.tasksMutex.RLock()
	task, exists := jobStoreAdapter.tasks[jobID]
	jobStoreAdapter.tasksMutex.RUnlock()

	assert.True(t, exists, "Job should still exist after concurrent updates")
	assert.NotNil(t, task.job, "Task job should not be nil")
}

// TestOutput_Concurrent verifies that concurrent Output calls don't race
func TestOutput_Concurrent(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log)
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "concurrent-output-job"
	job := &domain.Job{
		Uuid:   jobID,
		Status: domain.StatusRunning,
	}

	// Create task with buffer and some data
	buffer := NewSimpleLogBuffer(jobID)
	_ = buffer.Write([]byte("test data"))
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
		},
	}

	// Test: Concurrent Output calls while updating job status
	var wg sync.WaitGroup
	numReaders := 10
	numReads := 100

	// Spawn readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numReads; j++ {
				data, isRunning, err := jobStoreAdapter.Output(jobID)
				// Just verify no panic occurs and results are consistent
				_ = data
				_ = isRunning
				_ = err
			}
		}()
	}

	// Spawn writers that update job status
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < numReads; j++ {
			jobStoreAdapter.tasksMutex.Lock()
			if task, exists := jobStoreAdapter.tasks[jobID]; exists {
				if j%2 == 0 {
					task.job.Status = domain.StatusRunning
				} else {
					task.job.Status = domain.StatusCompleted
				}
			}
			jobStoreAdapter.tasksMutex.Unlock()
		}
	}()

	wg.Wait()

	// Verify: No panics occurred (test passes if we reach here)
	assert.True(t, true, "Concurrent Output calls completed without race")
}

// TestWriteToBuffer_Concurrent verifies that concurrent WriteToBuffer calls don't race
func TestWriteToBuffer_Concurrent(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log)
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create a test job
	jobID := "concurrent-write-job"
	job := &domain.Job{
		Uuid:   jobID,
		Status: domain.StatusRunning,
	}

	// Create task with buffer
	buffer := NewSimpleLogBuffer(jobID)
	jobStoreAdapter.tasks = map[string]*taskWrapper{
		jobID: {
			job:       job,
			logBuffer: buffer,
			pubsub:    ps,
		},
	}

	// Test: Concurrent WriteToBuffer calls
	var wg sync.WaitGroup
	numGoroutines := 10
	numWrites := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numWrites; j++ {
				data := []byte("test data from goroutine")
				jobStoreAdapter.WriteToBuffer(jobID, data)
			}
		}(i)
	}

	wg.Wait()

	// Verify: All writes should have completed
	chunks := buffer.ReadAll()
	expectedChunks := numGoroutines * numWrites
	assert.Equal(t, expectedChunks, len(chunks), "All concurrent writes should be buffered")
}

// TestWriteToBuffer_ConcurrentWithDeletion verifies WriteToBuffer handles concurrent task deletion
func TestWriteToBuffer_ConcurrentWithDeletion(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log)
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create multiple test jobs
	numJobs := 10
	for i := 0; i < numJobs; i++ {
		jobID := fmt.Sprintf("delete-test-job-%d", i)
		job := &domain.Job{
			Uuid:   jobID,
			Status: domain.StatusRunning,
		}

		buffer := NewSimpleLogBuffer(jobID)
		jobStoreAdapter.tasksMutex.Lock()
		jobStoreAdapter.tasks[jobID] = &taskWrapper{
			job:       job,
			logBuffer: buffer,
			pubsub:    ps,
		}
		jobStoreAdapter.tasksMutex.Unlock()
	}

	// Test: Concurrent writes while deleting tasks
	var wg sync.WaitGroup

	// Writers
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jobID := fmt.Sprintf("delete-test-job-%d", idx)
			for j := 0; j < 100; j++ {
				jobStoreAdapter.WriteToBuffer(jobID, []byte("test data"))
			}
		}(i)
	}

	// Deleters - delete tasks while writers are running
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jobID := fmt.Sprintf("delete-test-job-%d", idx)
			// Small delay to let some writes happen
			for j := 0; j < 50; j++ {
				// Spin
			}
			jobStoreAdapter.tasksMutex.Lock()
			delete(jobStoreAdapter.tasks, jobID)
			jobStoreAdapter.tasksMutex.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify: No panics occurred (test passes if we reach here)
	assert.True(t, true, "Concurrent writes with deletion completed without race")
}

// TestSubscribeCleanup_ConcurrentWithDeletion verifies subscription cleanup handles concurrent task deletion
func TestSubscribeCleanup_ConcurrentWithDeletion(t *testing.T) {
	// Setup
	log := logger.New()
	store := &SimpleJobStore{
		jobs:   make(map[string]*domain.Job),
		logger: log,
	}
	logMgr := NewSimpleLogManager()
	ps := pubsub.NewPubSub[JobEvent]()

	adapter := NewJobStorer(store, logMgr, ps, nil, nil, true, log)
	jobStoreAdapter := adapter.(*jobStoreAdapter)

	// Create test jobs with subscribers map
	numJobs := 5
	for i := 0; i < numJobs; i++ {
		jobID := fmt.Sprintf("subscribe-test-job-%d", i)
		job := &domain.Job{
			Uuid:   jobID,
			Status: domain.StatusRunning,
		}

		buffer := NewSimpleLogBuffer(jobID)
		jobStoreAdapter.tasksMutex.Lock()
		jobStoreAdapter.tasks[jobID] = &taskWrapper{
			job:         job,
			logBuffer:   buffer,
			subscribers: make(map[string]*subscriptionContext),
			pubsub:      ps,
		}
		jobStoreAdapter.tasksMutex.Unlock()
	}

	// Test: Concurrent subscriber registration/cleanup while deleting tasks
	var wg sync.WaitGroup

	// Subscriber registrations and cleanups
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jobID := fmt.Sprintf("subscribe-test-job-%d", idx)

			for j := 0; j < 50; j++ {
				subID := fmt.Sprintf("sub_%d_%d", idx, j)

				// Register subscriber
				jobStoreAdapter.tasksMutex.RLock()
				if task, exists := jobStoreAdapter.tasks[jobID]; exists {
					task.subMutex.Lock()
					task.subscribers[subID] = &subscriptionContext{id: subID}
					task.subMutex.Unlock()
				}
				jobStoreAdapter.tasksMutex.RUnlock()

				// Cleanup subscriber
				jobStoreAdapter.tasksMutex.RLock()
				if task, exists := jobStoreAdapter.tasks[jobID]; exists {
					task.subMutex.Lock()
					delete(task.subscribers, subID)
					task.subMutex.Unlock()
				}
				jobStoreAdapter.tasksMutex.RUnlock()
			}
		}(i)
	}

	// Deleters
	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			jobID := fmt.Sprintf("subscribe-test-job-%d", idx)
			// Small delay
			for j := 0; j < 25; j++ {
				// Spin
			}
			jobStoreAdapter.tasksMutex.Lock()
			delete(jobStoreAdapter.tasks, jobID)
			jobStoreAdapter.tasksMutex.Unlock()
		}(i)
	}

	wg.Wait()

	// Verify: No panics occurred (test passes if we reach here)
	assert.True(t, true, "Concurrent subscription operations with deletion completed without race")
}
