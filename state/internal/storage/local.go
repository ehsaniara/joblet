package storage

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// LocalConfig holds local filesystem storage configuration
type LocalConfig struct {
	Directory    string `yaml:"directory" json:"directory"`
	SyncInterval string `yaml:"sync_interval" json:"sync_interval"` // How often to sync to disk
}

// localBackend implements Backend using local filesystem storage.
// Jobs are stored as gzipped JSONL (one JSON object per line).
// Data persists across restarts.
type localBackend struct {
	mu       sync.RWMutex
	jobs     map[string]*domain.Job
	config   *LocalConfig
	filePath string

	// Background sync
	dirty      bool
	syncTicker *time.Ticker
	done       chan struct{}
	closed     bool
}

// NewLocalBackend creates a new local filesystem storage backend
func NewLocalBackend(cfg *LocalConfig) (Backend, error) {
	if cfg == nil {
		cfg = &LocalConfig{
			Directory:    "/opt/joblet/state",
			SyncInterval: "5s",
		}
	}

	if cfg.Directory == "" {
		cfg.Directory = "/opt/joblet/state"
	}

	if cfg.SyncInterval == "" {
		cfg.SyncInterval = "5s"
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	filePath := filepath.Join(cfg.Directory, "jobs.jsonl.gz")

	lb := &localBackend{
		jobs:     make(map[string]*domain.Job),
		config:   cfg,
		filePath: filePath,
		done:     make(chan struct{}),
	}

	// Load existing jobs from disk
	if err := lb.loadFromDisk(); err != nil {
		// Log warning but don't fail - start fresh if file doesn't exist or is corrupted
		fmt.Printf("Warning: could not load existing state: %v\n", err)
	}

	// Parse sync interval
	syncInterval, err := time.ParseDuration(cfg.SyncInterval)
	if err != nil {
		syncInterval = 5 * time.Second
	}

	// Start background sync goroutine
	lb.syncTicker = time.NewTicker(syncInterval)
	go lb.backgroundSync()

	return lb, nil
}

// loadFromDisk reads all jobs from the gzipped JSONL file
func (lb *localBackend) loadFromDisk() error {
	file, err := os.Open(lb.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet - this is fine for first run
			return nil
		}
		return fmt.Errorf("failed to open state file: %w", err)
	}
	defer file.Close()

	// Handle multi-stream gzip files (each write creates a new gzip stream)
	for {
		gzReader, err := gzip.NewReader(file)
		if err != nil {
			if err == io.EOF {
				// No more gzip streams - done
				break
			}
			// For other errors, try to continue
			return fmt.Errorf("failed to create gzip reader: %w", err)
		}

		scanner := bufio.NewScanner(gzReader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 64KB initial, 1MB max

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var job domain.Job
			if err := json.Unmarshal(line, &job); err != nil {
				// Log warning but continue loading other jobs
				fmt.Printf("Warning: failed to unmarshal job: %v\n", err)
				continue
			}

			// Store the job (later entries override earlier ones - this handles updates)
			lb.jobs[job.Uuid] = &job
		}

		if err := scanner.Err(); err != nil {
			gzReader.Close()
			return fmt.Errorf("error reading state file: %w", err)
		}

		gzReader.Close()
	}

	fmt.Printf("Loaded %d jobs from local state\n", len(lb.jobs))
	return nil
}

// saveToDisk writes all jobs to a fresh gzipped JSONL file
func (lb *localBackend) saveToDisk() error {
	// Write to temp file first for atomicity
	tempPath := lb.filePath + ".tmp"

	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create temp state file: %w", err)
	}

	gzWriter := gzip.NewWriter(file)

	// Write all jobs
	for _, job := range lb.jobs {
		data, err := json.Marshal(job)
		if err != nil {
			gzWriter.Close()
			file.Close()
			os.Remove(tempPath)
			return fmt.Errorf("failed to marshal job %s: %w", job.Uuid, err)
		}

		data = append(data, '\n')
		if _, err := gzWriter.Write(data); err != nil {
			gzWriter.Close()
			file.Close()
			os.Remove(tempPath)
			return fmt.Errorf("failed to write job %s: %w", job.Uuid, err)
		}
	}

	// Close gzip writer to write trailer
	if err := gzWriter.Close(); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to sync state file: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close state file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempPath, lb.filePath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename state file: %w", err)
	}

	return nil
}

// backgroundSync periodically syncs dirty state to disk
func (lb *localBackend) backgroundSync() {
	for {
		select {
		case <-lb.syncTicker.C:
			lb.mu.Lock()
			if lb.dirty {
				if err := lb.saveToDisk(); err != nil {
					fmt.Printf("Error saving state to disk: %v\n", err)
				} else {
					lb.dirty = false
				}
			}
			lb.mu.Unlock()
		case <-lb.done:
			return
		}
	}
}

// markDirty marks the state as needing sync
func (lb *localBackend) markDirty() {
	lb.dirty = true
}

func (lb *localBackend) Create(ctx context.Context, job *domain.Job) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, exists := lb.jobs[job.Uuid]; exists {
		return ErrJobAlreadyExists
	}

	// Create a copy to avoid external mutations
	jobCopy := *job
	if jobCopy.StartTime.IsZero() {
		jobCopy.StartTime = time.Now()
	}

	lb.jobs[job.Uuid] = &jobCopy
	lb.markDirty()

	return nil
}

func (lb *localBackend) Get(ctx context.Context, jobID string) (*domain.Job, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	job, exists := lb.jobs[jobID]
	if !exists {
		return nil, ErrJobNotFound
	}

	// Return a copy to prevent external mutations
	jobCopy := *job
	return &jobCopy, nil
}

func (lb *localBackend) Update(ctx context.Context, job *domain.Job) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, exists := lb.jobs[job.Uuid]; !exists {
		return ErrJobNotFound
	}

	// Create a copy to avoid external mutations
	jobCopy := *job
	lb.jobs[job.Uuid] = &jobCopy
	lb.markDirty()

	return nil
}

func (lb *localBackend) Delete(ctx context.Context, jobID string) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if _, exists := lb.jobs[jobID]; !exists {
		return ErrJobNotFound
	}

	delete(lb.jobs, jobID)
	lb.markDirty()

	return nil
}

func (lb *localBackend) List(ctx context.Context, filter *Filter) ([]*domain.Job, error) {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	var result []*domain.Job

	// Collect matching jobs
	for _, job := range lb.jobs {
		if lb.matchesFilter(job, filter) {
			jobCopy := *job
			result = append(result, &jobCopy)
		}
	}

	// Sort results
	if filter != nil && filter.SortBy != "" {
		lb.sortJobs(result, filter.SortBy, filter.SortDesc)
	}

	// Apply limit
	if filter != nil && filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (lb *localBackend) Sync(ctx context.Context, jobs []*domain.Job) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	// Bulk replace all jobs (used for reconciliation)
	lb.jobs = make(map[string]*domain.Job, len(jobs))
	for _, job := range jobs {
		jobCopy := *job
		lb.jobs[job.Uuid] = &jobCopy
	}
	lb.markDirty()

	// Force immediate save for bulk sync
	return lb.saveToDisk()
}

func (lb *localBackend) Close() error {
	lb.mu.Lock()
	if lb.closed {
		lb.mu.Unlock()
		return nil
	}
	lb.closed = true
	lb.mu.Unlock()

	// Stop background sync
	lb.syncTicker.Stop()
	close(lb.done)

	// Final save
	lb.mu.Lock()
	defer lb.mu.Unlock()

	if lb.dirty {
		if err := lb.saveToDisk(); err != nil {
			return fmt.Errorf("failed to save state on close: %w", err)
		}
	}

	return nil
}

func (lb *localBackend) HealthCheck(ctx context.Context) error {
	// Check if we can write to the state directory
	testPath := filepath.Join(lb.config.Directory, ".health")
	if err := os.WriteFile(testPath, []byte("ok"), 0644); err != nil {
		return fmt.Errorf("state directory not writable: %w", err)
	}
	os.Remove(testPath)
	return nil
}

// Helper functions

func (lb *localBackend) matchesFilter(job *domain.Job, filter *Filter) bool {
	if filter == nil {
		return true
	}

	// Filter by status
	if filter.Status != "" && string(job.Status) != filter.Status {
		return false
	}

	// Filter by multiple statuses (OR condition)
	if len(filter.Statuses) > 0 {
		found := false
		for _, status := range filter.Statuses {
			if string(job.Status) == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by node ID
	if filter.NodeID != "" && job.NodeId != filter.NodeID {
		return false
	}

	return true
}

func (lb *localBackend) sortJobs(jobs []*domain.Job, sortBy string, descending bool) {
	sort.Slice(jobs, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "createdAt", "startTime":
			less = jobs[i].StartTime.Before(jobs[j].StartTime)
		case "status":
			less = jobs[i].Status < jobs[j].Status
		default:
			// Default: sort by StartTime
			less = jobs[i].StartTime.Before(jobs[j].StartTime)
		}

		if descending {
			return !less
		}
		return less
	})
}
