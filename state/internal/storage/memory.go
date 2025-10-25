package storage

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// memoryBackend is an in-memory implementation of Backend interface.
// Used as a fallback when persistent storage is disabled or unavailable.
// All data is lost on restart - suitable for development/testing only.
type memoryBackend struct {
	mu   sync.RWMutex
	jobs map[string]*domain.Job
}

// NewMemoryBackend creates a new in-memory storage backend
func NewMemoryBackend() Backend {
	return &memoryBackend{
		jobs: make(map[string]*domain.Job),
	}
}

func (m *memoryBackend) Create(ctx context.Context, job *domain.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[job.Uuid]; exists {
		return ErrJobAlreadyExists
	}

	// Create a copy to avoid external mutations
	jobCopy := *job
	if jobCopy.StartTime.IsZero() {
		jobCopy.StartTime = time.Now()
	}

	m.jobs[job.Uuid] = &jobCopy
	return nil
}

func (m *memoryBackend) Get(ctx context.Context, jobID string) (*domain.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[jobID]
	if !exists {
		return nil, ErrJobNotFound
	}

	// Return a copy to prevent external mutations
	jobCopy := *job
	return &jobCopy, nil
}

func (m *memoryBackend) Update(ctx context.Context, job *domain.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[job.Uuid]; !exists {
		return ErrJobNotFound
	}

	// Create a copy to avoid external mutations
	jobCopy := *job

	m.jobs[job.Uuid] = &jobCopy
	return nil
}

func (m *memoryBackend) Delete(ctx context.Context, jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.jobs[jobID]; !exists {
		return ErrJobNotFound
	}

	delete(m.jobs, jobID)
	return nil
}

func (m *memoryBackend) List(ctx context.Context, filter *Filter) ([]*domain.Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*domain.Job

	// Collect matching jobs
	for _, job := range m.jobs {
		if matchesFilter(job, filter) {
			jobCopy := *job
			result = append(result, &jobCopy)
		}
	}

	// Sort results
	if filter != nil && filter.SortBy != "" {
		sortJobs(result, filter.SortBy, filter.SortDesc)
	}

	// Apply limit
	if filter != nil && filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (m *memoryBackend) Sync(ctx context.Context, jobs []*domain.Job) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Bulk replace all jobs (used for reconciliation)
	m.jobs = make(map[string]*domain.Job, len(jobs))
	for _, job := range jobs {
		jobCopy := *job
		m.jobs[job.Uuid] = &jobCopy
	}

	return nil
}

func (m *memoryBackend) Close() error {
	// No resources to clean up for memory backend
	return nil
}

func (m *memoryBackend) HealthCheck(ctx context.Context) error {
	// Memory backend is always healthy
	return nil
}

// Helper functions

func matchesFilter(job *domain.Job, filter *Filter) bool {
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

func sortJobs(jobs []*domain.Job, sortBy string, descending bool) {
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
