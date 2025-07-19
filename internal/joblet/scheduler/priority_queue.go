package scheduler

import (
	"container/heap"
	"joblet/internal/joblet/domain"
	"sync"
	"time"
)

// ScheduledJob wraps a job with its scheduled execution time for the priority queue
type ScheduledJob struct {
	Job           *domain.Job
	ScheduledTime time.Time
	Index         int // Index in the heap (required by heap.Interface)
}

// PriorityQueue implements a thread-safe priority queue for scheduled jobs
// Jobs are ordered by their scheduled execution time (earliest first)
type PriorityQueue struct {
	items []*ScheduledJob
	mutex sync.RWMutex
}

// NewPriorityQueue creates a new priority queue for scheduled jobs
func NewPriorityQueue() *PriorityQueue {
	pq := &PriorityQueue{
		items: make([]*ScheduledJob, 0),
	}
	heap.Init(pq)
	return pq
}

// Len returns the number of items in the queue (heap.Interface)
func (pq *PriorityQueue) Len() int {
	return len(pq.items)
}

// Less compares two items by their scheduled time (heap.Interface)
func (pq *PriorityQueue) Less(i, j int) bool {
	return pq.items[i].ScheduledTime.Before(pq.items[j].ScheduledTime)
}

// Swap exchanges two items in the queue (heap.Interface)
func (pq *PriorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].Index = i
	pq.items[j].Index = j
}

// Push adds an item to the queue (heap.Interface)
func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*ScheduledJob)
	item.Index = len(pq.items)
	pq.items = append(pq.items, item)
}

// Pop removes and returns the item with the earliest scheduled time (heap.Interface)
func (pq *PriorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.Index = -1 // for safety
	pq.items = old[0 : n-1]
	return item
}

// Add adds a job to the priority queue (thread-safe)
func (pq *PriorityQueue) Add(job *domain.Job) {
	if job.ScheduledTime == nil {
		return // Only scheduled jobs should be added to this queue
	}

	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	scheduledJob := &ScheduledJob{
		Job:           job,
		ScheduledTime: *job.ScheduledTime,
	}

	heap.Push(pq, scheduledJob)
}

// Peek returns the next job to be executed without removing it (thread-safe)
func (pq *PriorityQueue) Peek() *domain.Job {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	if len(pq.items) == 0 {
		return nil
	}

	return pq.items[0].Job
}

// Next removes and returns the next job to be executed (thread-safe)
func (pq *PriorityQueue) Next() *domain.Job {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	if len(pq.items) == 0 {
		return nil
	}

	scheduledJob := heap.Pop(pq).(*ScheduledJob)
	return scheduledJob.Job
}

// Remove removes a specific job from the queue by ID (thread-safe)
func (pq *PriorityQueue) Remove(jobID string) bool {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	for i, item := range pq.items {
		if item.Job.Id == jobID {
			heap.Remove(pq, i)
			return true
		}
	}
	return false
}

// Update updates the scheduled time for a job already in the queue (thread-safe)
func (pq *PriorityQueue) Update(jobID string, newScheduledTime time.Time) bool {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	for _, item := range pq.items {
		if item.Job.Id == jobID {
			item.ScheduledTime = newScheduledTime
			item.Job.ScheduledTime = &newScheduledTime
			heap.Fix(pq, item.Index)
			return true
		}
	}
	return false
}

// GetAll returns all scheduled jobs (thread-safe, returns copies)
func (pq *PriorityQueue) GetAll() []*domain.Job {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	jobs := make([]*domain.Job, 0, len(pq.items))
	for _, item := range pq.items {
		jobs = append(jobs, item.Job.DeepCopy())
	}
	return jobs
}

// IsEmpty returns true if the queue is empty (thread-safe)
func (pq *PriorityQueue) IsEmpty() bool {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return len(pq.items) == 0
}

// Size returns the number of jobs in the queue (thread-safe)
func (pq *PriorityQueue) Size() int {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()
	return len(pq.items)
}

// GetNextExecutionTime returns when the next job should execute (thread-safe)
func (pq *PriorityQueue) GetNextExecutionTime() *time.Time {
	pq.mutex.RLock()
	defer pq.mutex.RUnlock()

	if len(pq.items) == 0 {
		return nil
	}

	return &pq.items[0].ScheduledTime
}
