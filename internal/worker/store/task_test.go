package store

import (
	"testing"
	"time"
	"worker/internal/worker/domain"
)

func TestTask_NewTask(t *testing.T) {
	job := &domain.Job{
		Id:      "task-test-1",
		Command: "echo",
		Args:    []string{"hello"},
		Status:  domain.StatusInitializing,
	}

	task := NewTask(job)

	if task.id != job.Id {
		t.Errorf("Expected task ID %v, got %v", job.Id, task.id)
	}

	retrievedJob := task.GetJob()
	if retrievedJob.Id != job.Id {
		t.Errorf("Expected job ID %v, got %v", job.Id, retrievedJob.Id)
	}

	// Verify it's a deep copy
	if retrievedJob == job {
		t.Error("Expected deep copy, got same reference")
	}
}

func TestTask_GetJob(t *testing.T) {
	job := &domain.Job{
		Id:      "get-job-test",
		Command: "echo",
		Status:  domain.StatusRunning,
		Pid:     1234,
	}

	task := NewTask(job)
	retrievedJob := task.GetJob()

	// Verify all fields are copied correctly
	if retrievedJob.Id != job.Id {
		t.Errorf("Expected ID %v, got %v", job.Id, retrievedJob.Id)
	}
	if retrievedJob.Command != job.Command {
		t.Errorf("Expected command %v, got %v", job.Command, retrievedJob.Command)
	}
	if retrievedJob.Status != job.Status {
		t.Errorf("Expected status %v, got %v", job.Status, retrievedJob.Status)
	}
	if retrievedJob.Pid != job.Pid {
		t.Errorf("Expected PID %v, got %v", job.Pid, retrievedJob.Pid)
	}

	// Verify independence
	retrievedJob.Status = domain.StatusCompleted
	secondRetrieved := task.GetJob()
	if secondRetrieved.Status == domain.StatusCompleted {
		t.Error("Expected job to be independent of returned copy")
	}
}

func TestTask_UpdateJob(t *testing.T) {
	originalJob := &domain.Job{
		Id:      "update-test",
		Command: "echo",
		Status:  domain.StatusInitializing,
	}

	task := NewTask(originalJob)

	// Update job
	updatedJob := originalJob.DeepCopy()
	updatedJob.MarkAsRunning(5678)

	task.UpdateJob(updatedJob)

	// Verify update
	retrievedJob := task.GetJob()
	if retrievedJob.Status != domain.StatusRunning {
		t.Errorf("Expected status RUNNING, got %v", retrievedJob.Status)
	}
	if retrievedJob.Pid != 5678 {
		t.Errorf("Expected PID 5678, got %v", retrievedJob.Pid)
	}
}

func TestTask_WriteToBuffer(t *testing.T) {
	job := &domain.Job{
		Id:      "buffer-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Write data to buffer
	testData := []byte("Hello, World!")
	task.WriteToBuffer(testData)

	// Retrieve buffer
	buffer := task.GetBuffer()
	if string(buffer) != string(testData) {
		t.Errorf("Expected buffer %v, got %v", string(testData), string(buffer))
	}
}

func TestTask_WriteToBufferMultiple(t *testing.T) {
	job := &domain.Job{
		Id:      "multi-buffer-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Write multiple chunks
	chunks := [][]byte{
		[]byte("First "),
		[]byte("Second "),
		[]byte("Third"),
	}

	for _, chunk := range chunks {
		task.WriteToBuffer(chunk)
	}

	// Verify accumulated buffer
	buffer := task.GetBuffer()
	expected := "First Second Third"
	if string(buffer) != expected {
		t.Errorf("Expected buffer %v, got %v", expected, string(buffer))
	}
}

func TestTask_WriteToBufferEmpty(t *testing.T) {
	job := &domain.Job{
		Id:      "empty-buffer-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Write empty data (should be ignored)
	task.WriteToBuffer([]byte{})

	// Buffer should remain empty
	buffer := task.GetBuffer()
	if buffer != nil {
		t.Errorf("Expected nil buffer, got %v", buffer)
	}
}

func TestTask_GetBufferEmpty(t *testing.T) {
	job := &domain.Job{
		Id:      "empty-get-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Get empty buffer
	buffer := task.GetBuffer()
	if buffer != nil {
		t.Errorf("Expected nil buffer, got %v", buffer)
	}
}

func TestTask_SubscribeAndPublish(t *testing.T) {
	job := &domain.Job{
		Id:      "subscribe-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Subscribe to updates
	updates, unsubscribe := task.Subscribe()
	defer unsubscribe()

	// Publish an update
	testUpdate := Update{
		JobID:    "subscribe-test",
		LogChunk: []byte("test log"),
		Status:   "RUNNING",
	}

	go task.Publish(testUpdate)

	// Receive update
	select {
	case receivedUpdate := <-updates:
		if receivedUpdate.JobID != testUpdate.JobID {
			t.Errorf("Expected JobID %v, got %v", testUpdate.JobID, receivedUpdate.JobID)
		}
		if string(receivedUpdate.LogChunk) != string(testUpdate.LogChunk) {
			t.Errorf("Expected LogChunk %v, got %v", string(testUpdate.LogChunk), string(receivedUpdate.LogChunk))
		}
		if receivedUpdate.Status != testUpdate.Status {
			t.Errorf("Expected Status %v, got %v", testUpdate.Status, receivedUpdate.Status)
		}
	case <-time.After(1 * time.Second):
		t.Error("Did not receive update in time")
	}
}

func TestTask_MultipleSubscribers(t *testing.T) {
	job := &domain.Job{
		Id:      "multi-subscribe-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Create multiple subscribers
	numSubscribers := 3
	subscribers := make([]<-chan Update, numSubscribers)
	unsubscribers := make([]func(), numSubscribers)

	for i := 0; i < numSubscribers; i++ {
		updates, unsubscribe := task.Subscribe()
		subscribers[i] = updates
		unsubscribers[i] = unsubscribe
	}

	defer func() {
		for _, unsubscribe := range unsubscribers {
			unsubscribe()
		}
	}()

	// Publish update
	testUpdate := Update{
		JobID:    "multi-subscribe-test",
		LogChunk: []byte("broadcast message"),
	}

	go task.Publish(testUpdate)

	// All subscribers should receive the update
	received := 0
	timeout := time.After(1 * time.Second)

	for i := 0; i < numSubscribers; i++ {
		select {
		case update := <-subscribers[i]:
			if string(update.LogChunk) == string(testUpdate.LogChunk) {
				received++
			}
		case <-timeout:
			t.Error("Timeout waiting for updates")
			return
		}
	}

	if received != numSubscribers {
		t.Errorf("Expected %v subscribers to receive update, got %v", numSubscribers, received)
	}
}

func TestTask_UnsubscribeRemovesSubscriber(t *testing.T) {
	job := &domain.Job{
		Id:      "unsubscribe-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Subscribe and immediately unsubscribe
	_, unsubscribe := task.Subscribe()
	unsubscribe()

	// Publish update - should not block
	testUpdate := Update{
		JobID:    "unsubscribe-test",
		LogChunk: []byte("should not block"),
	}

	// This should complete quickly since there are no subscribers
	done := make(chan bool)
	go func() {
		task.Publish(testUpdate)
		done <- true
	}()

	select {
	case <-done:
		// Success - publish completed quickly
	case <-time.After(100 * time.Millisecond):
		t.Error("Publish took too long - subscriber not properly removed")
	}
}

func TestTask_IsRunning(t *testing.T) {
	tests := []struct {
		status   domain.JobStatus
		expected bool
	}{
		{domain.StatusInitializing, false},
		{domain.StatusRunning, true},
		{domain.StatusCompleted, false},
		{domain.StatusFailed, false},
		{domain.StatusStopped, false},
	}

	for _, tt := range tests {
		job := &domain.Job{
			Id:     "running-test",
			Status: tt.status,
		}

		task := NewTask(job)
		if task.IsRunning() != tt.expected {
			t.Errorf("IsRunning() for status %v: expected %v, got %v",
				tt.status, tt.expected, task.IsRunning())
		}
	}
}

func TestTask_Shutdown(t *testing.T) {
	job := &domain.Job{
		Id:      "shutdown-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Create subscriber
	updates, unsubscribe := task.Subscribe()
	defer unsubscribe()

	// Shutdown task
	go task.Shutdown()

	// Channel should be closed
	select {
	case _, ok := <-updates:
		if ok {
			t.Error("Expected channel to be closed")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Channel was not closed in time")
	}
}

func TestTask_WriteToBufferWithSubscriber(t *testing.T) {
	job := &domain.Job{
		Id:      "write-subscriber-test",
		Command: "echo",
		Status:  domain.StatusRunning,
	}

	task := NewTask(job)

	// Subscribe to updates
	updates, unsubscribe := task.Subscribe()
	defer unsubscribe()

	// Write to buffer (should trigger update)
	testData := []byte("Hello, Subscriber!")
	task.WriteToBuffer(testData)

	// Should receive update
	select {
	case update := <-updates:
		if string(update.LogChunk) != string(testData) {
			t.Errorf("Expected LogChunk %v, got %v", string(testData), string(update.LogChunk))
		}
		if update.JobID != job.Id {
			t.Errorf("Expected JobID %v, got %v", job.Id, update.JobID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Did not receive update from WriteToBuffer")
	}
}
