package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

func TestLocalBackend_CreateAndGet(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Create a job
	job := &domain.Job{
		Uuid:      "test-job-123",
		Command:   "echo hello",
		Status:    domain.StatusRunning,
		StartTime: time.Now(),
		NodeId:    "node-1",
	}

	err = backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Get the job
	retrieved, err := backend.Get(ctx, "test-job-123")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if retrieved.Uuid != job.Uuid {
		t.Errorf("Expected Uuid %s, got %s", job.Uuid, retrieved.Uuid)
	}
	if retrieved.Command != job.Command {
		t.Errorf("Expected Command %s, got %s", job.Command, retrieved.Command)
	}
	if retrieved.Status != job.Status {
		t.Errorf("Expected Status %s, got %s", job.Status, retrieved.Status)
	}
}

func TestLocalBackend_CreateDuplicate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "echo hello",
		Status:  domain.StatusRunning,
	}

	err = backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Try to create duplicate
	err = backend.Create(ctx, job)
	if err != ErrJobAlreadyExists {
		t.Errorf("Expected ErrJobAlreadyExists, got %v", err)
	}
}

func TestLocalBackend_Update(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "echo hello",
		Status:  domain.StatusRunning,
	}

	err = backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Update the job
	job.Status = domain.StatusCompleted
	job.ExitCode = 0

	err = backend.Update(ctx, job)
	if err != nil {
		t.Fatalf("Failed to update job: %v", err)
	}

	// Get the updated job
	retrieved, err := backend.Get(ctx, "test-job-123")
	if err != nil {
		t.Fatalf("Failed to get job: %v", err)
	}

	if retrieved.Status != domain.StatusCompleted {
		t.Errorf("Expected Status COMPLETED, got %s", retrieved.Status)
	}
	if retrieved.ExitCode != 0 {
		t.Errorf("Expected ExitCode 0, got %d", retrieved.ExitCode)
	}
}

func TestLocalBackend_UpdateNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	job := &domain.Job{
		Uuid:    "non-existent-job",
		Command: "echo hello",
		Status:  domain.StatusRunning,
	}

	err = backend.Update(ctx, job)
	if err != ErrJobNotFound {
		t.Errorf("Expected ErrJobNotFound, got %v", err)
	}
}

func TestLocalBackend_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	job := &domain.Job{
		Uuid:    "test-job-123",
		Command: "echo hello",
		Status:  domain.StatusRunning,
	}

	err = backend.Create(ctx, job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	err = backend.Delete(ctx, "test-job-123")
	if err != nil {
		t.Fatalf("Failed to delete job: %v", err)
	}

	// Verify job is deleted
	_, err = backend.Get(ctx, "test-job-123")
	if err != ErrJobNotFound {
		t.Errorf("Expected ErrJobNotFound, got %v", err)
	}
}

func TestLocalBackend_List(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Create multiple jobs
	jobs := []*domain.Job{
		{Uuid: "job-1", Command: "cmd1", Status: domain.StatusRunning, NodeId: "node-1"},
		{Uuid: "job-2", Command: "cmd2", Status: domain.StatusCompleted, NodeId: "node-1"},
		{Uuid: "job-3", Command: "cmd3", Status: domain.StatusRunning, NodeId: "node-2"},
	}

	for _, job := range jobs {
		if err := backend.Create(ctx, job); err != nil {
			t.Fatalf("Failed to create job: %v", err)
		}
	}

	// List all jobs
	allJobs, err := backend.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}
	if len(allJobs) != 3 {
		t.Errorf("Expected 3 jobs, got %d", len(allJobs))
	}

	// List by status
	runningJobs, err := backend.List(ctx, &Filter{Status: "RUNNING"})
	if err != nil {
		t.Fatalf("Failed to list running jobs: %v", err)
	}
	if len(runningJobs) != 2 {
		t.Errorf("Expected 2 running jobs, got %d", len(runningJobs))
	}

	// List by node ID
	node1Jobs, err := backend.List(ctx, &Filter{NodeID: "node-1"})
	if err != nil {
		t.Fatalf("Failed to list node-1 jobs: %v", err)
	}
	if len(node1Jobs) != 2 {
		t.Errorf("Expected 2 jobs for node-1, got %d", len(node1Jobs))
	}
}

func TestLocalBackend_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	ctx := context.Background()

	// Create backend and add a job
	backend1, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}

	job := &domain.Job{
		Uuid:      "persistent-job-123",
		Command:   "echo hello",
		Status:    domain.StatusCompleted,
		ExitCode:  42,
		StartTime: time.Now(),
		NodeId:    "node-1",
	}

	err = backend1.Create(ctx, job)
	if err != nil {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Close the backend (triggers save)
	backend1.Close()

	// Verify the file was created
	filePath := filepath.Join(tmpDir, "jobs.jsonl.gz")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("State file was not created")
	}

	// Create a new backend (should load from disk)
	backend2, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create second backend: %v", err)
	}
	defer backend2.Close()

	// Get the job from the new backend
	retrieved, err := backend2.Get(ctx, "persistent-job-123")
	if err != nil {
		t.Fatalf("Failed to get job from new backend: %v", err)
	}

	if retrieved.Uuid != job.Uuid {
		t.Errorf("Expected Uuid %s, got %s", job.Uuid, retrieved.Uuid)
	}
	if retrieved.Command != job.Command {
		t.Errorf("Expected Command %s, got %s", job.Command, retrieved.Command)
	}
	if retrieved.Status != job.Status {
		t.Errorf("Expected Status %s, got %s", job.Status, retrieved.Status)
	}
	if retrieved.ExitCode != job.ExitCode {
		t.Errorf("Expected ExitCode %d, got %d", job.ExitCode, retrieved.ExitCode)
	}
}

func TestLocalBackend_HealthCheck(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	err = backend.HealthCheck(ctx)
	if err != nil {
		t.Errorf("Health check failed: %v", err)
	}
}

func TestLocalBackend_Sync(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "joblet-state-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &LocalConfig{
		Directory:    tmpDir,
		SyncInterval: "100ms",
	}

	backend, err := NewLocalBackend(cfg)
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	ctx := context.Background()

	// Create initial job
	initialJob := &domain.Job{
		Uuid:    "initial-job",
		Command: "initial",
		Status:  domain.StatusRunning,
	}
	backend.Create(ctx, initialJob)

	// Sync with new jobs (replace all)
	syncJobs := []*domain.Job{
		{Uuid: "sync-job-1", Command: "cmd1", Status: domain.StatusCompleted},
		{Uuid: "sync-job-2", Command: "cmd2", Status: domain.StatusRunning},
	}

	err = backend.Sync(ctx, syncJobs)
	if err != nil {
		t.Fatalf("Failed to sync: %v", err)
	}

	// Verify initial job is gone
	_, err = backend.Get(ctx, "initial-job")
	if err != ErrJobNotFound {
		t.Errorf("Expected initial job to be removed after sync")
	}

	// Verify sync jobs exist
	allJobs, err := backend.List(ctx, nil)
	if err != nil {
		t.Fatalf("Failed to list: %v", err)
	}
	if len(allJobs) != 2 {
		t.Errorf("Expected 2 jobs after sync, got %d", len(allJobs))
	}
}
