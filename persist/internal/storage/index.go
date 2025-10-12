package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// jobIndex manages the job metadata index
type jobIndex struct {
	indexPath string
	mu        sync.RWMutex
	jobs      map[string]*JobInfo
}

// newJobIndex creates a new job index
func newJobIndex(indexPath string) *jobIndex {
	return &jobIndex{
		indexPath: indexPath,
		jobs:      make(map[string]*JobInfo),
	}
}

// Load loads the index from disk
func (ji *jobIndex) Load() error {
	ji.mu.Lock()
	defer ji.mu.Unlock()

	data, err := os.ReadFile(ji.indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Index doesn't exist yet, start fresh
		}
		return fmt.Errorf("failed to read index file: %w", err)
	}

	if err := json.Unmarshal(data, &ji.jobs); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	return nil
}

// Save saves the index to disk
func (ji *jobIndex) Save() error {
	ji.mu.RLock()
	defer ji.mu.RUnlock()

	data, err := json.MarshalIndent(ji.jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(ji.indexPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	return nil
}

// UpdateJob updates job statistics
func (ji *jobIndex) UpdateJob(jobID string, logCount, metricCount int64) {
	ji.mu.Lock()
	defer ji.mu.Unlock()

	now := time.Now().Unix()

	info, exists := ji.jobs[jobID]
	if !exists {
		info = &JobInfo{
			JobID:     jobID,
			CreatedAt: now,
		}
		ji.jobs[jobID] = info
	}

	info.LastUpdated = now
	info.LogCount += logCount
	info.MetricCount += metricCount

	// Periodically save (every 100 updates)
	if (info.LogCount+info.MetricCount)%100 == 0 {
		go ji.Save()
	}
}

// DeleteJob removes a job from the index
func (ji *jobIndex) DeleteJob(jobID string) {
	ji.mu.Lock()
	defer ji.mu.Unlock()

	delete(ji.jobs, jobID)

	// Save immediately on deletion
	go ji.Save()
}

// GetJobInfo returns information about a job
func (ji *jobIndex) GetJobInfo(jobID string) (*JobInfo, error) {
	ji.mu.RLock()
	defer ji.mu.RUnlock()

	info, exists := ji.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}

	// Return a copy
	return &JobInfo{
		JobID:       info.JobID,
		CreatedAt:   info.CreatedAt,
		LastUpdated: info.LastUpdated,
		LogCount:    info.LogCount,
		MetricCount: info.MetricCount,
		SizeBytes:   info.SizeBytes,
	}, nil
}

// ListJobs lists all jobs matching the filter
func (ji *jobIndex) ListJobs(filter *JobFilter) ([]string, error) {
	ji.mu.RLock()
	defer ji.mu.RUnlock()

	result := make([]string, 0, len(ji.jobs))

	for jobID, info := range ji.jobs {
		// Apply filters
		if filter.Since != nil && info.CreatedAt < *filter.Since {
			continue
		}
		if filter.Until != nil && info.CreatedAt > *filter.Until {
			continue
		}

		result = append(result, jobID)
	}

	// Apply pagination
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return []string{}, nil
		}
		result = result[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}
