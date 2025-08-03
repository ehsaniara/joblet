package state_test

import (
	"context"
	"fmt"
	"joblet/internal/joblet/domain"
	"joblet/internal/joblet/state"
	"joblet/internal/joblet/state/statefakes"
	"sync"
	"testing"
	"time"
)

func TestStore_CreateAndGetJob(t *testing.T) {
	store := state.New()

	job := &domain.Job{
		Id:        "test-job-1",
		Command:   "echo",
		Args:      []string{"hello"},
		Status:    domain.StatusInitializing,
		Limits:    *domain.NewResourceLimitsFromParams(100, "", 512, 1000),
		StartTime: time.Now(),
	}

	// Test creating a job
	store.CreateNewJob(job)

	// Test getting the job
	retrievedJob, exists := store.GetJob("test-job-1")
	if !exists {
		t.Fatal("Expected job to exist")
	}

	if retrievedJob.Id != job.Id {
		t.Errorf("Expected job ID %v, got %v", job.Id, retrievedJob.Id)
	}
	if retrievedJob.Command != job.Command {
		t.Errorf("Expected command %v, got %v", job.Command, retrievedJob.Command)
	}

	// Test getting non-existent job
	_, exists = store.GetJob("non-existent")
	if exists {
		t.Error("Expected job to not exist")
	}
}

func TestStore_CreateDuplicateJob(t *testing.T) {
	store := state.New()

	job := &domain.Job{
		Id:      "duplicate-job",
		Command: "echo",
		Status:  domain.StatusInitializing,
	}

	// Create job twice
	store.CreateNewJob(job)
	store.CreateNewJob(job)

	// Should still only have one job
	jobs := store.ListJobs()
	if len(jobs) != 1 {
		t.Errorf("Expected 1 job, got %v", len(jobs))
	}
}

func TestStore_UpdateJob(t *testing.T) {
	s := state.New()

	job := &domain.Job{
		Id:      "update-test",
		Command: "echo",
		Status:  domain.StatusInitializing,
	}

	s.CreateNewJob(job)

	// Update the job
	updatedJob := job.DeepCopy()
	updatedJob.Status = domain.StatusRunning
	updatedJob.Pid = 1234

	s.UpdateJob(updatedJob)

	// Retrieve and verify update
	retrievedJob, exists := s.GetJob("update-test")
	if !exists {
		t.Fatal("Expected job to exist")
	}

	if retrievedJob.Status != domain.StatusRunning {
		t.Errorf("Expected status RUNNING, got %v", retrievedJob.Status)
	}
	if retrievedJob.Pid != 1234 {
		t.Errorf("Expected PID 1234, got %v", retrievedJob.Pid)
	}
}

func TestStore_ListJobs(t *testing.T) {
	store := state.New()

	// Create multiple jobs
	jobs := []*domain.Job{
		{Id: "job-1", Command: "echo", Status: domain.StatusRunning},
		{Id: "job-2", Command: "ls", Status: domain.StatusCompleted},
		{Id: "job-3", Command: "pwd", Status: domain.StatusFailed},
	}

	for _, job := range jobs {
		store.CreateNewJob(job)
	}

	// List all jobs
	listedJobs := store.ListJobs()
	if len(listedJobs) != 3 {
		t.Errorf("Expected 3 jobs, got %v", len(listedJobs))
	}

	// Check that all jobs are present
	jobIds := make(map[string]bool)
	for _, job := range listedJobs {
		jobIds[job.Id] = true
	}

	for _, expectedJob := range jobs {
		if !jobIds[expectedJob.Id] {
			t.Errorf("Expected job %v to be in list", expectedJob.Id)
		}
	}
}

func TestStore_WriteToBuffer(t *testing.T) {
	store := state.New()

	job := &domain.Job{
		Id:      "buffer-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	store.CreateNewJob(job)

	// Write to buffer
	testData := []byte("Hello, World!")
	store.WriteToBuffer("buffer-test", testData)

	// Get output
	output, isRunning, err := store.GetOutput("buffer-test")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if !isRunning {
		t.Error("Expected job to be running")
	}
	if string(output) != string(testData) {
		t.Errorf("Expected output %v, got %v", string(testData), string(output))
	}
}

func TestStore_WriteToNonExistentJob(t *testing.T) {
	store := state.New()

	// Should not panic when writing to non-existent job
	store.WriteToBuffer("non-existent", []byte("test"))

	// This test passes if no panic occurs
}

func TestStore_GetOutputNonExistentJob(t *testing.T) {
	store := state.New()

	_, _, err := store.GetOutput("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent job")
	}
}

func TestStore_SendUpdatesToClientCancellation(t *testing.T) {
	store := state.New()

	job := &domain.Job{
		Id:      "cancel-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	store.CreateNewJob(job)

	ctx, cancel := context.WithCancel(context.Background())
	fakeStreamer := &statefakes.FakeDomainStreamer{}
	fakeStreamer.ContextReturns(ctx)

	// Start streaming
	errCh := make(chan error, 1)
	go func() {
		err := store.SendUpdatesToClient(ctx, "cancel-test", fakeStreamer)
		errCh <- err
	}()

	// Cancel context
	cancel()

	// Should receive cancellation error
	select {
	case err := <-errCh:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Error("Stream did not handle cancellation in time")
	}
}

func TestStore_SendUpdatesToNonExistentJob(t *testing.T) {
	store := state.New()

	ctx := context.Background()
	fakeStreamer := &statefakes.FakeDomainStreamer{}
	fakeStreamer.ContextReturns(ctx)

	err := store.SendUpdatesToClient(ctx, "non-existent", fakeStreamer)
	if err == nil {
		t.Error("Expected error for non-existent job")
	}
}

func TestStore_SendUpdatesToNonRunningJob(t *testing.T) {
	store := state.New()

	job := &domain.Job{
		Id:      "completed-job",
		Command: "echo",
		Status:  domain.StatusCompleted,
	}

	store.CreateNewJob(job)

	ctx := context.Background()
	fakeStreamer := &statefakes.FakeDomainStreamer{}
	fakeStreamer.ContextReturns(ctx)

	err := store.SendUpdatesToClient(ctx, "completed-job", fakeStreamer)
	if err != nil {
		t.Errorf("Expected no error for completed job, got %v", err)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := state.New()

	// Test concurrent job creation
	var wg sync.WaitGroup
	numJobs := 100

	for i := 0; i < numJobs; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			job := &domain.Job{
				Id:      fmt.Sprintf("concurrent-%d", id),
				Command: "echo",
				Status:  domain.StatusInitializing,
			}
			store.CreateNewJob(job)
		}(i)
	}

	wg.Wait()

	// Verify all jobs were created
	jobs := store.ListJobs()
	if len(jobs) != numJobs {
		t.Errorf("Expected %v jobs, got %v", numJobs, len(jobs))
	}
}

// Benchmark tests
func BenchmarkStore_CreateJob(b *testing.B) {
	store := state.New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		job := &domain.Job{
			Id:      fmt.Sprintf("bench-%d", i),
			Command: "echo",
			Status:  domain.StatusInitializing,
		}
		store.CreateNewJob(job)
	}
}

func BenchmarkStore_GetJob(b *testing.B) {
	store := state.New()

	// Create test job
	job := &domain.Job{
		Id:      "bench-get",
		Command: "echo",
		Status:  domain.StatusInitializing,
	}
	store.CreateNewJob(job)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetJob("bench-get")
	}
}

func BenchmarkStore_WriteToBuffer(b *testing.B) {
	store := state.New()

	job := &domain.Job{
		Id:      "bench-write",
		Command: "echo",
		Status:  domain.StatusRunning,
	}
	store.CreateNewJob(job)

	testData := []byte("benchmark data")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.WriteToBuffer("bench-write", testData)
	}
}
