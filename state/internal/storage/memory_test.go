package storage

import (
	"context"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

func TestMemoryBackend_CreateAndGet(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	job := &domain.Job{
		Uuid:      "test-job-123",
		Status:    "RUNNING",
		Command:   "echo test",
		NodeId:    "node-1",
		StartTime: time.Now(),
	}

	// Test Create
	err := backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Test Get
	retrieved, err := backend.Get(ctx, job.Uuid)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.Uuid != job.Uuid {
		t.Errorf("Expected Uuid %s, got %s", job.Uuid, retrieved.Uuid)
	}
	if retrieved.Status != job.Status {
		t.Errorf("Expected Status %s, got %s", job.Status, retrieved.Status)
	}
	if retrieved.Command != job.Command {
		t.Errorf("Expected Command %s, got %s", job.Command, retrieved.Command)
	}
}

func TestMemoryBackend_CreateDuplicate(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	job := &domain.Job{
		Uuid:    "test-job-duplicate",
		Status:  "RUNNING",
		Command: "echo test",
	}

	// First create should succeed
	err := backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("First create failed: %v", err)
	}

	// Second create should fail
	err = backend.Create(ctx, job)
	if err != ErrJobAlreadyExists {
		t.Errorf("Expected ErrJobAlreadyExists, got %v", err)
	}
}

func TestMemoryBackend_Update(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	job := &domain.Job{
		Uuid:    "test-job-update",
		Status:  "RUNNING",
		Command: "echo test",
	}

	// Create job
	err := backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Update job
	job.Status = "COMPLETED"
	job.ExitCode = 0
	endTime := time.Now()
	job.EndTime = &endTime

	err = backend.Update(ctx, job)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Verify update
	retrieved, err := backend.Get(ctx, job.Uuid)
	if err != nil {
		t.Fatalf("Get after update failed: %v", err)
	}

	if retrieved.Status != "COMPLETED" {
		t.Errorf("Expected status COMPLETED, got %s", retrieved.Status)
	}
	if retrieved.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", retrieved.ExitCode)
	}
	if retrieved.EndTime == nil {
		t.Error("Expected EndTime to be set")
	}
}

func TestMemoryBackend_UpdateNonExistent(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	job := &domain.Job{
		Uuid:   "non-existent",
		Status: "RUNNING",
	}

	err := backend.Update(ctx, job)
	if err != ErrJobNotFound {
		t.Errorf("Expected ErrJobNotFound, got %v", err)
	}
}

func TestMemoryBackend_Delete(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	job := &domain.Job{
		Uuid:    "test-job-delete",
		Status:  "COMPLETED",
		Command: "echo test",
	}

	// Create job
	err := backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Delete job
	err = backend.Delete(ctx, job.Uuid)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = backend.Get(ctx, job.Uuid)
	if err != ErrJobNotFound {
		t.Errorf("Expected ErrJobNotFound after delete, got %v", err)
	}
}

func TestMemoryBackend_List(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Create multiple jobs
	jobs := []*domain.Job{
		{Uuid: "job-1", Status: "RUNNING", Command: "cmd1", StartTime: time.Now()},
		{Uuid: "job-2", Status: "COMPLETED", Command: "cmd2", StartTime: time.Now().Add(1 * time.Second)},
		{Uuid: "job-3", Status: "FAILED", Command: "cmd3", StartTime: time.Now().Add(2 * time.Second)},
		{Uuid: "job-4", Status: "RUNNING", Command: "cmd4", StartTime: time.Now().Add(3 * time.Second)},
	}

	for _, job := range jobs {
		if err := backend.Create(ctx, job); err != nil {
			t.Fatalf("Failed to create job %s: %v", job.Uuid, err)
		}
	}

	// List all jobs
	allJobs, err := backend.List(ctx, nil)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(allJobs) != 4 {
		t.Errorf("Expected 4 jobs, got %d", len(allJobs))
	}

	// Filter by status
	runningJobs, err := backend.List(ctx, &Filter{Status: "RUNNING"})
	if err != nil {
		t.Fatalf("List with filter failed: %v", err)
	}
	if len(runningJobs) != 2 {
		t.Errorf("Expected 2 RUNNING jobs, got %d", len(runningJobs))
	}

	// Filter with limit
	limitedJobs, err := backend.List(ctx, &Filter{Limit: 2})
	if err != nil {
		t.Fatalf("List with limit failed: %v", err)
	}
	if len(limitedJobs) != 2 {
		t.Errorf("Expected 2 jobs with limit, got %d", len(limitedJobs))
	}
}

func TestMemoryBackend_Sync(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	// Create initial jobs
	job1 := &domain.Job{Uuid: "job-1", Status: "RUNNING", Command: "cmd1"}
	backend.Create(ctx, job1)

	// Sync with new set of jobs
	newJobs := []*domain.Job{
		{Uuid: "job-2", Status: "PENDING", Command: "cmd2"},
		{Uuid: "job-3", Status: "COMPLETED", Command: "cmd3"},
	}

	err := backend.Sync(ctx, newJobs)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Verify old job is replaced
	_, err = backend.Get(ctx, "job-1")
	if err != ErrJobNotFound {
		t.Error("Expected old job to be removed after sync")
	}

	// Verify new jobs exist
	_, err = backend.Get(ctx, "job-2")
	if err != nil {
		t.Error("Expected new job to exist after sync")
	}

	_, err = backend.Get(ctx, "job-3")
	if err != nil {
		t.Error("Expected new job to exist after sync")
	}
}

func TestMemoryBackend_HealthCheck(t *testing.T) {
	backend := NewMemoryBackend()
	ctx := context.Background()

	err := backend.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck failed: %v", err)
	}
}

func TestMemoryBackend_Close(t *testing.T) {
	backend := NewMemoryBackend()

	err := backend.Close()
	if err != nil {
		t.Errorf("Close failed: %v", err)
	}
}
