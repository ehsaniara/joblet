package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExecutor implements JobExecutor for testing
type mockExecutor struct {
	mu            sync.Mutex
	executedJobs  []string
	executionErr  error
	executionWait time.Duration
}

func (m *mockExecutor) ExecuteScheduledJob(ctx context.Context, job *domain.Job) error {
	if m.executionWait > 0 {
		time.Sleep(m.executionWait)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executedJobs = append(m.executedJobs, job.Uuid)
	return m.executionErr
}

func (m *mockExecutor) GetExecutedJobs() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string{}, m.executedJobs...)
}

func createTestJob(uuid string, scheduledTime time.Time) *domain.Job {
	return &domain.Job{
		Uuid:          uuid,
		Command:       "test",
		ScheduledTime: &scheduledTime,
		Status:        domain.StatusScheduled,
	}
}

func TestNew(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	assert.NotNil(t, s)
	assert.NotNil(t, s.queue)
	assert.NotNil(t, s.executor)
	assert.NotNil(t, s.logger)
	assert.NotNil(t, s.newJobSignal)
	assert.NotNil(t, s.stopSignal)
	assert.False(t, s.running)
}

func TestScheduler_StartStop(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Start scheduler
	err := s.Start()
	assert.NoError(t, err)
	assert.True(t, s.IsRunning())

	// Starting again should be idempotent
	err = s.Start()
	assert.NoError(t, err)

	// Stop scheduler
	err = s.Stop()
	assert.NoError(t, err)
	assert.False(t, s.IsRunning())

	// Stopping again should be idempotent
	err = s.Stop()
	assert.NoError(t, err)
}

func TestScheduler_AddJob(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Add a scheduled job
	scheduledTime := time.Now().Add(1 * time.Hour)
	job := createTestJob("job-1", scheduledTime)

	err := s.AddJob(job)
	assert.NoError(t, err)
	assert.Equal(t, 1, s.GetQueueSize())

	// Add another job
	job2 := createTestJob("job-2", scheduledTime.Add(1*time.Hour))
	err = s.AddJob(job2)
	assert.NoError(t, err)
	assert.Equal(t, 2, s.GetQueueSize())
}

func TestScheduler_AddJob_NotScheduled(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Add a job without scheduled time
	job := &domain.Job{
		Uuid:    "job-1",
		Command: "test",
		// No ScheduledTime
	}

	err := s.AddJob(job)
	assert.NoError(t, err)
	assert.Equal(t, 0, s.GetQueueSize(), "Job without scheduled time should not be added")
}

func TestScheduler_RemoveJob(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Add jobs
	scheduledTime := time.Now().Add(1 * time.Hour)
	job1 := createTestJob("job-1", scheduledTime)
	job2 := createTestJob("job-2", scheduledTime.Add(1*time.Hour))

	err := s.AddJob(job1)
	require.NoError(t, err)
	err = s.AddJob(job2)
	require.NoError(t, err)
	assert.Equal(t, 2, s.GetQueueSize())

	// Remove first job
	removed := s.RemoveJob("job-1")
	assert.True(t, removed)
	assert.Equal(t, 1, s.GetQueueSize())

	// Try to remove non-existent job
	removed = s.RemoveJob("non-existent")
	assert.False(t, removed)

	// Remove second job
	removed = s.RemoveJob("job-2")
	assert.True(t, removed)
	assert.Equal(t, 0, s.GetQueueSize())
}

func TestScheduler_GetScheduledJobs(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Add jobs
	now := time.Now()
	job1 := createTestJob("job-1", now.Add(1*time.Hour))
	job2 := createTestJob("job-2", now.Add(2*time.Hour))
	job3 := createTestJob("job-3", now.Add(30*time.Minute))

	err := s.AddJob(job1)
	require.NoError(t, err)
	err = s.AddJob(job2)
	require.NoError(t, err)
	err = s.AddJob(job3)
	require.NoError(t, err)

	jobs := s.GetScheduledJobs()
	assert.Len(t, jobs, 3)

	// Verify jobs are returned (order may vary based on deep copy)
	jobIDs := make(map[string]bool)
	for _, j := range jobs {
		jobIDs[j.Uuid] = true
	}
	assert.True(t, jobIDs["job-1"])
	assert.True(t, jobIDs["job-2"])
	assert.True(t, jobIDs["job-3"])
}

func TestScheduler_ExecutesJobAtScheduledTime(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Start scheduler
	err := s.Start()
	require.NoError(t, err)
	defer func() { _ = s.Stop() }()

	// Schedule a job for immediate execution (in the past)
	pastTime := time.Now().Add(-1 * time.Second)
	job := createTestJob("immediate-job", pastTime)

	err = s.AddJob(job)
	require.NoError(t, err)

	// Wait for execution
	time.Sleep(200 * time.Millisecond)

	// Verify job was executed
	executedJobs := executor.GetExecutedJobs()
	assert.Contains(t, executedJobs, "immediate-job")
	assert.Equal(t, 0, s.GetQueueSize(), "Job should be removed from queue after execution")
}

func TestScheduler_ExecutesJobsInOrder(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Start scheduler
	err := s.Start()
	require.NoError(t, err)
	defer func() { _ = s.Stop() }()

	now := time.Now()

	// Add jobs in reverse order (job-3 earliest, job-1 latest)
	job1 := createTestJob("job-1", now.Add(150*time.Millisecond))
	job2 := createTestJob("job-2", now.Add(100*time.Millisecond))
	job3 := createTestJob("job-3", now.Add(50*time.Millisecond))

	err = s.AddJob(job1)
	require.NoError(t, err)
	err = s.AddJob(job2)
	require.NoError(t, err)
	err = s.AddJob(job3)
	require.NoError(t, err)

	// Wait for all executions
	time.Sleep(300 * time.Millisecond)

	// Verify all jobs executed
	executedJobs := executor.GetExecutedJobs()
	assert.Len(t, executedJobs, 3)

	// First job executed should be job-3 (earliest)
	assert.Equal(t, "job-3", executedJobs[0])
}

func TestScheduler_HandleExecutionError(t *testing.T) {
	executor := &mockExecutor{
		executionErr: errors.New("execution failed"),
	}
	s := New(executor)

	// Start scheduler
	err := s.Start()
	require.NoError(t, err)
	defer func() { _ = s.Stop() }()

	// Schedule job for immediate execution
	pastTime := time.Now().Add(-1 * time.Second)
	job := createTestJob("failing-job", pastTime)

	err = s.AddJob(job)
	require.NoError(t, err)

	// Wait for execution attempt
	time.Sleep(200 * time.Millisecond)

	// Verify job was attempted (even though it failed)
	executedJobs := executor.GetExecutedJobs()
	assert.Contains(t, executedJobs, "failing-job")
}

func TestScheduler_NewJobWakesUpScheduler(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Start scheduler
	err := s.Start()
	require.NoError(t, err)
	defer func() { _ = s.Stop() }()

	// Give scheduler time to enter wait state
	time.Sleep(50 * time.Millisecond)

	// Add job that should execute immediately
	pastTime := time.Now().Add(-1 * time.Second)
	job := createTestJob("wake-up-job", pastTime)

	err = s.AddJob(job)
	require.NoError(t, err)

	// Wait for execution
	time.Sleep(200 * time.Millisecond)

	// Verify job was executed
	executedJobs := executor.GetExecutedJobs()
	assert.Contains(t, executedJobs, "wake-up-job")
}

func TestScheduler_StopDuringExecution(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	// Start scheduler
	err := s.Start()
	require.NoError(t, err)

	// Add a future job
	futureTime := time.Now().Add(10 * time.Second)
	job := createTestJob("future-job", futureTime)
	err = s.AddJob(job)
	require.NoError(t, err)

	// Stop immediately
	err = s.Stop()
	assert.NoError(t, err)

	// Verify scheduler stopped
	assert.False(t, s.IsRunning())

	// Job should not have been executed
	executedJobs := executor.GetExecutedJobs()
	assert.Empty(t, executedJobs)
}

func TestScheduler_IsRunning(t *testing.T) {
	executor := &mockExecutor{}
	s := New(executor)

	assert.False(t, s.IsRunning())

	err := s.Start()
	require.NoError(t, err)
	assert.True(t, s.IsRunning())

	err = s.Stop()
	require.NoError(t, err)
	assert.False(t, s.IsRunning())
}

// Priority Queue Tests

func TestPriorityQueue_New(t *testing.T) {
	pq := NewPriorityQueue()

	assert.NotNil(t, pq)
	assert.True(t, pq.IsEmpty())
	assert.Equal(t, 0, pq.Size())
}

func TestPriorityQueue_AddAndPeek(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	job1 := createTestJob("job-1", now.Add(2*time.Hour))
	job2 := createTestJob("job-2", now.Add(1*time.Hour)) // Earlier

	pq.Add(job1)
	pq.Add(job2)

	// Peek should return earliest job
	peeked := pq.Peek()
	assert.Equal(t, "job-2", peeked.Uuid, "Peek should return job with earliest scheduled time")
	assert.Equal(t, 2, pq.Size(), "Peek should not remove item")
}

func TestPriorityQueue_Next(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	job1 := createTestJob("job-1", now.Add(2*time.Hour))
	job2 := createTestJob("job-2", now.Add(1*time.Hour))
	job3 := createTestJob("job-3", now.Add(3*time.Hour))

	pq.Add(job1)
	pq.Add(job2)
	pq.Add(job3)

	// Next should return and remove items in order
	next := pq.Next()
	assert.Equal(t, "job-2", next.Uuid)
	assert.Equal(t, 2, pq.Size())

	next = pq.Next()
	assert.Equal(t, "job-1", next.Uuid)
	assert.Equal(t, 1, pq.Size())

	next = pq.Next()
	assert.Equal(t, "job-3", next.Uuid)
	assert.Equal(t, 0, pq.Size())

	// Next on empty queue
	next = pq.Next()
	assert.Nil(t, next)
}

func TestPriorityQueue_Remove(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	job1 := createTestJob("job-1", now.Add(1*time.Hour))
	job2 := createTestJob("job-2", now.Add(2*time.Hour))

	pq.Add(job1)
	pq.Add(job2)

	// Remove existing job
	removed := pq.Remove("job-1")
	assert.True(t, removed)
	assert.Equal(t, 1, pq.Size())

	// Verify job-2 is still there
	peeked := pq.Peek()
	assert.Equal(t, "job-2", peeked.Uuid)

	// Remove non-existent job
	removed = pq.Remove("non-existent")
	assert.False(t, removed)
}

func TestPriorityQueue_Update(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	job1 := createTestJob("job-1", now.Add(2*time.Hour))
	job2 := createTestJob("job-2", now.Add(1*time.Hour))

	pq.Add(job1)
	pq.Add(job2)

	// job-2 should be first
	assert.Equal(t, "job-2", pq.Peek().Uuid)

	// Update job-1 to be earlier
	newTime := now.Add(30 * time.Minute)
	updated := pq.Update("job-1", newTime)
	assert.True(t, updated)

	// Now job-1 should be first
	assert.Equal(t, "job-1", pq.Peek().Uuid)

	// Update non-existent job
	updated = pq.Update("non-existent", newTime)
	assert.False(t, updated)
}

func TestPriorityQueue_GetAll(t *testing.T) {
	pq := NewPriorityQueue()

	now := time.Now()
	job1 := createTestJob("job-1", now.Add(1*time.Hour))
	job2 := createTestJob("job-2", now.Add(2*time.Hour))

	pq.Add(job1)
	pq.Add(job2)

	jobs := pq.GetAll()
	assert.Len(t, jobs, 2)

	// Verify GetAll returns copies (modifying returned jobs shouldn't affect queue)
	jobs[0].Uuid = "modified"
	assert.Equal(t, "job-1", pq.Peek().Uuid, "GetAll should return copies")
}

func TestPriorityQueue_GetNextExecutionTime(t *testing.T) {
	pq := NewPriorityQueue()

	// Empty queue
	nextTime := pq.GetNextExecutionTime()
	assert.Nil(t, nextTime)

	// Add jobs
	now := time.Now()
	earliestTime := now.Add(1 * time.Hour)
	job1 := createTestJob("job-1", earliestTime)
	job2 := createTestJob("job-2", now.Add(2*time.Hour))

	pq.Add(job1)
	pq.Add(job2)

	nextTime = pq.GetNextExecutionTime()
	assert.NotNil(t, nextTime)
	assert.Equal(t, earliestTime.Unix(), nextTime.Unix())
}

func TestPriorityQueue_ThreadSafety(t *testing.T) {
	pq := NewPriorityQueue()
	now := time.Now()

	var wg sync.WaitGroup
	numGoroutines := 10
	jobsPerGoroutine := 100

	// Concurrent adds
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < jobsPerGoroutine; j++ {
				jobID := time.Now().Format("job-15:04:05.000000000") + string(rune(goroutineID)) + string(rune(j))
				job := createTestJob(jobID, now.Add(time.Duration(j)*time.Minute))
				pq.Add(job)
			}
		}(i)
	}

	wg.Wait()

	// All jobs should be added
	assert.Equal(t, numGoroutines*jobsPerGoroutine, pq.Size())
}

func TestPriorityQueue_AddNilScheduledTime(t *testing.T) {
	pq := NewPriorityQueue()

	job := &domain.Job{
		Uuid:    "job-no-schedule",
		Command: "test",
		// No ScheduledTime
	}

	pq.Add(job)
	assert.Equal(t, 0, pq.Size(), "Job without scheduled time should not be added")
}

func TestPriorityQueue_PeekEmpty(t *testing.T) {
	pq := NewPriorityQueue()

	result := pq.Peek()
	assert.Nil(t, result)
}
