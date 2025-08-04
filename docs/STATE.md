# Joblet State Management Design

## Overview

The Joblet state management module provides a high-performance, thread-safe, in-memory state store for managing job
lifecycle, resource allocation, and system state. It serves as the single source of truth for all runtime state in the
Joblet system, ensuring consistency across concurrent operations while maintaining low latency for state queries and
updates.

## Architecture

### Core Components

```
State Management System
├── Job State Store
│   ├── Job Records (status, metadata, resources)
│   ├── State Transitions (lifecycle management)
│   └── Job Indexing (fast lookups)
├── Task Abstraction
│   ├── Task Queue (scheduled jobs)
│   ├── Task Execution (job runner integration)
│   └── Task Results (completion tracking)
├── Volume State Store
│   ├── Volume Registry
│   ├── Mount Tracking
│   └── Usage Statistics
├── Network State Store
│   ├── Network Registry
│   ├── IP Allocation
│   └── Connection Tracking
└── System State
    ├── Resource Usage
    ├── Concurrent Job Limits
    └── Health Metrics
```

## State Stores

### Job State Store

The job state store maintains all job-related state information with optimized data structures for different access
patterns.

```go
type JobStore struct {
mu          sync.RWMutex
jobs        map[string]*Job // Primary storage by ID
jobsByState map[JobState][]*Job // Index by state
jobsByNode  map[string][]*Job // Index by node (future)
activeJobs  int32             // Atomic counter

// Event handling
listeners   []JobStateListener
eventChan   chan JobEvent

// Metrics
metrics     *StateMetrics
}

type Job struct {
ID          string
State       JobState
Task        *Task
Command     []string
Environment map[string]string
Resources   ResourceLimits
Network     NetworkConfig
Volumes     []VolumeMount

// Runtime state
PID         int
StartTime   time.Time
EndTime     time.Time
ExitCode    int
Error       string

// Resource tracking
CPUUsage    float64
MemoryUsage int64
IOStats     IOStatistics

// Metadata
CreatedAt   time.Time
UpdatedAt   time.Time
Labels      map[string]string
}
```

### Job State Lifecycle

```
INITIALIZING → SCHEDULED → RUNNING → COMPLETED
     ↓            ↓          ↓         
   FAILED      CANCELLED   STOPPED    
```

#### State Definitions

- **INITIALIZING**: Job accepted, resources being allocated
- **SCHEDULED**: Job queued for future execution
- **RUNNING**: Job actively executing
- **COMPLETED**: Job finished successfully (exit code 0)
- **FAILED**: Job terminated with error
- **STOPPED**: Job terminated by user request
- **CANCELLED**: Scheduled job cancelled before execution

#### State Transition Rules

```go
var validTransitions = map[JobState][]JobState{
JobStateInitializing: {JobStateScheduled, JobStateRunning, JobStateFailed},
JobStateScheduled:    {JobStateRunning, JobStateCancelled},
JobStateRunning:      {JobStateCompleted, JobStateFailed, JobStateStopped},
JobStateCompleted:    {}, // Terminal state
JobStateFailed:       {}, // Terminal state
JobStateStopped:      {}, // Terminal state
JobStateCancelled:    {}, // Terminal state
}

func (j *Job) TransitionTo(newState JobState) error {
j.mu.Lock()
defer j.mu.Unlock()

// Validate transition
validStates, ok := validTransitions[j.State]
if !ok {
return fmt.Errorf("invalid current state: %s", j.State)
}

allowed := false
for _, state := range validStates {
if state == newState {
allowed = true
break
}
}

if !allowed {
return fmt.Errorf("invalid transition: %s → %s", j.State, newState)
}

// Update state
oldState := j.State
j.State = newState
j.UpdatedAt = time.Now()

// Trigger state change events
j.notifyListeners(oldState, newState)

return nil
}
```

### Task Abstraction

The Task abstraction provides a higher-level interface for job scheduling and execution.

```go
type Task struct {
ID          string
Type        TaskType
Status      TaskStatus
Priority    int
ScheduledAt time.Time

// Execution details
Handler     TaskHandler
Context     context.Context
CancelFunc  context.CancelFunc

// Results
Result      interface{}
Error       error
CompletedAt time.Time

// Retry configuration
MaxRetries  int
RetryCount  int
RetryDelay  time.Duration
}

type TaskQueue struct {
mu       sync.Mutex
tasks    []*Task
ready    chan *Task
workers  int

// Priority queue implementation
heap     taskHeap
}

// Task execution flow
func (tq *TaskQueue) Execute(task *Task) error {
// 1. Validate task
if err := task.Validate(); err != nil {
return err
}

// 2. Add to queue
tq.mu.Lock()
heap.Push(&tq.heap, task)
tq.mu.Unlock()

// 3. Signal ready channel
select {
case tq.ready <- task:
default:
// Queue full, task remains in heap
}

return nil
}
```

### Volume State Store

Manages volume lifecycle and tracks volume-to-job associations.

```go
type VolumeStore struct {
mu          sync.RWMutex
volumes     map[string]*Volume
volumesByJob map[string][]*Volume

// Usage tracking
usageStats  map[string]*VolumeUsage
}

type Volume struct {
ID          string
Name        string
Type        VolumeType
Size        int64
Path        string

// State
Status      VolumeStatus
InUse       bool
MountCount  int32

// Associations
JobIDs      []string
Labels      map[string]string

// Timestamps
CreatedAt   time.Time
LastUsed    time.Time
}

type VolumeUsage struct {
VolumeID    string
UsedBytes   int64
FreeBytes   int64
TotalBytes  int64
IOStats     IOStatistics
LastUpdated time.Time
}
```

### Network State Store

Tracks network configurations and IP allocations.

```go
type NetworkStore struct {
mu         sync.RWMutex
networks   map[string]*Network
ipAllocator *IPAllocator

// Connection tracking
connections map[string]*NetworkConnection
}

type Network struct {
ID          string
Name        string
Type        NetworkType
CIDR        string
Bridge      string

// State
Status      NetworkStatus
JobCount    int32

// IP management
Gateway     net.IP
IPRange     *net.IPNet
AllocatedIPs map[string]net.IP

// Metadata
CreatedAt   time.Time
Labels      map[string]string
}

type IPAllocator struct {
mu          sync.Mutex
pools       map[string]*IPPool
allocations map[string]net.IP
}
```

## Concurrency and Synchronization

### Locking Strategy

The state module uses a hierarchical locking strategy to minimize contention:

```go
// Coarse-grained locks for store-level operations
type StoreLocks struct {
jobs    sync.RWMutex // Job store lock
volumes sync.RWMutex // Volume store lock
network sync.RWMutex // Network store lock
}

// Fine-grained locks for individual resources
type ResourceLock struct {
mu      sync.RWMutex
holders map[string]time.Time
}

// Lock ordering to prevent deadlocks
// Always acquire in order: jobs → volumes → network
func (s *StateManager) UpdateJobWithVolume(jobID, volumeID string) error {
s.locks.jobs.Lock()
defer s.locks.jobs.Unlock()

s.locks.volumes.Lock()
defer s.locks.volumes.Unlock()

// Safe to operate on both stores
return s.linkJobToVolume(jobID, volumeID)
}
```

### Atomic Operations

Critical counters use atomic operations for lock-free updates:

```go
type Metrics struct {
ActiveJobs    atomic.Int32
TotalJobs     atomic.Int64
FailedJobs    atomic.Int64
CompletedJobs atomic.Int64
}

func (m *Metrics) IncrementActive() {
m.ActiveJobs.Add(1)
m.TotalJobs.Add(1)
}

func (m *Metrics) DecrementActive(failed bool) {
m.ActiveJobs.Add(-1)
if failed {
m.FailedJobs.Add(1)
} else {
m.CompletedJobs.Add(1)
}
}
```

### Event System

Asynchronous event propagation for state changes:

```go
type EventBus struct {
subscribers map[EventType][]EventHandler
eventQueue  chan Event
workers     int
}

type Event struct {
Type      EventType
Timestamp time.Time
Source    string
Data      interface{}
}

func (eb *EventBus) Publish(event Event) {
select {
case eb.eventQueue <- event:
default:
// Queue full, drop event or handle overflow
log.Warn("Event queue full, dropping event", "type", event.Type)
}
}

func (eb *EventBus) processEvents() {
for event := range eb.eventQueue {
handlers := eb.subscribers[event.Type]
for _, handler := range handlers {
go handler(event) // Async processing
}
}
}
```

## State Queries

### Query Interface

Flexible query system for state retrieval:

```go
type QueryBuilder struct {
filters []Filter
sort    SortOption
limit   int
offset  int
}

type Filter interface {
Apply(job *Job) bool
}

// Example filters
type StateFilter struct {
States []JobState
}

type LabelFilter struct {
Key   string
Value string
}

type TimeRangeFilter struct {
Field string // "created", "started", "completed"
Start time.Time
End   time.Time
}

// Query execution
func (js *JobStore) Query(qb *QueryBuilder) ([]*Job, error) {
js.mu.RLock()
defer js.mu.RUnlock()

// Apply filters
results := make([]*Job, 0)
for _, job := range js.jobs {
match := true
for _, filter := range qb.filters {
if !filter.Apply(job) {
match = false
break
}
}
if match {
results = append(results, job)
}
}

// Apply sorting
if qb.sort != nil {
sort.Slice(results, qb.sort.Less)
}

// Apply pagination
start := qb.offset
end := start + qb.limit
if end > len(results) {
end = len(results)
}

return results[start:end], nil
}
```

### Indexing Strategy

Multiple indexes for efficient lookups:

```go
type JobIndexes struct {
byState    map[JobState]map[string]*Job
byLabel    map[string]map[string]*Job
bySchedule *timeWheel // Time-based index for scheduled jobs
}

// Time wheel for efficient scheduled job lookup
type timeWheel struct {
mu       sync.RWMutex
slots    []*timeSlot
interval time.Duration
current  int
}

type timeSlot struct {
time time.Time
jobs []*Job
}
```

## State Persistence

While Joblet uses in-memory state by design, the architecture supports optional persistence:

### Snapshot Mechanism

```go
type StateSnapshot struct {
Version   string
Timestamp time.Time
Jobs      []*Job
Volumes   []*Volume
Networks  []*Network
Checksum  string
}

func (sm *StateManager) CreateSnapshot() (*StateSnapshot, error) {
// Acquire read locks on all stores
sm.locks.jobs.RLock()
defer sm.locks.jobs.RUnlock()
sm.locks.volumes.RLock()
defer sm.locks.volumes.RUnlock()
sm.locks.network.RLock()
defer sm.locks.network.RUnlock()

snapshot := &StateSnapshot{
Version:   "1.0",
Timestamp: time.Now(),
Jobs:      sm.jobStore.GetAll(),
Volumes:   sm.volumeStore.GetAll(),
Networks:  sm.networkStore.GetAll(),
}

// Calculate checksum
snapshot.Checksum = calculateChecksum(snapshot)

return snapshot, nil
}
```

### Recovery Process

```go
func (sm *StateManager) RestoreFromSnapshot(snapshot *StateSnapshot) error {
// Validate snapshot
if err := validateSnapshot(snapshot); err != nil {
return err
}

// Acquire write locks
sm.locks.jobs.Lock()
defer sm.locks.jobs.Unlock()
sm.locks.volumes.Lock()
defer sm.locks.volumes.Unlock()
sm.locks.network.Lock()
defer sm.locks.network.Unlock()

// Clear existing state
sm.clearAll()

// Restore state
for _, job := range snapshot.Jobs {
sm.jobStore.Add(job)
}
for _, volume := range snapshot.Volumes {
sm.volumeStore.Add(volume)
}
for _, network := range snapshot.Networks {
sm.networkStore.Add(network)
}

return nil
}
```

## Performance Optimization

### Memory Management

```go
// Object pooling for frequently allocated objects
var jobPool = sync.Pool{
New: func () interface{} {
return &Job{
Environment: make(map[string]string),
Labels:      make(map[string]string),
}
},
}

func NewJob() *Job {
return jobPool.Get().(*Job)
}

func ReleaseJob(job *Job) {
job.Reset()
jobPool.Put(job)
}
```

### Cache Strategy

```go
type StateCache struct {
mu         sync.RWMutex
jobCache   *lru.Cache
queryCache *lru.Cache
ttl        time.Duration
}

func (sc *StateCache) GetJob(id string) (*Job, bool) {
sc.mu.RLock()
defer sc.mu.RUnlock()

if val, ok := sc.jobCache.Get(id); ok {
return val.(*Job), true
}
return nil, false
}
```

### Batch Operations

```go
func (js *JobStore) BatchUpdate(updates []JobUpdate) error {
js.mu.Lock()
defer js.mu.Unlock()

// Validate all updates first
for _, update := range updates {
if _, exists := js.jobs[update.ID]; !exists {
return fmt.Errorf("job %s not found", update.ID)
}
}

// Apply updates
for _, update := range updates {
job := js.jobs[update.ID]
update.Apply(job)
}

// Single notification for batch
js.notifyBatchUpdate(updates)

return nil
}
```

## Monitoring and Metrics

### State Metrics

```go
type StateMetrics struct {
// Counters
TotalJobs        prometheus.Counter
ActiveJobs       prometheus.Gauge
CompletedJobs    prometheus.Counter
FailedJobs       prometheus.Counter

// Histograms
JobDuration      prometheus.Histogram
StateTransition  prometheus.Histogram

// Resource usage
MemoryUsage      prometheus.Gauge
StorageUsage     prometheus.Gauge
}

func (sm *StateMetrics) RecordJobCompletion(job *Job) {
duration := job.EndTime.Sub(job.StartTime)
sm.JobDuration.Observe(duration.Seconds())

if job.ExitCode == 0 {
sm.CompletedJobs.Inc()
} else {
sm.FailedJobs.Inc()
}

sm.ActiveJobs.Dec()
}
```

### Health Checks

```go
type HealthChecker struct {
checks []HealthCheck
}

type HealthCheck interface {
Name() string
Check() error
}

type StateHealthCheck struct {
stateManager *StateManager
}

func (shc *StateHealthCheck) Check() error {
// Check job store consistency
if err := shc.stateManager.jobStore.Validate(); err != nil {
return fmt.Errorf("job store unhealthy: %w", err)
}

// Check for stuck jobs
stuckJobs := shc.stateManager.GetStuckJobs(5 * time.Minute)
if len(stuckJobs) > 0 {
return fmt.Errorf("%d jobs stuck in running state", len(stuckJobs))
}

return nil
}
```

## Error Handling

### State Recovery

```go
func (sm *StateManager) RecoverFromPanic() {
if r := recover(); r != nil {
log.Error("State manager panic", "error", r, "stack", debug.Stack())

// Attempt to restore consistency
sm.performEmergencyRecovery()

// Notify administrators
sm.alertAdmin("State manager panic recovered", r)
}
}

func (sm *StateManager) performEmergencyRecovery() {
// Lock all stores
sm.lockAll()
defer sm.unlockAll()

// Validate and repair state
sm.jobStore.Repair()
sm.volumeStore.Repair()
sm.networkStore.Repair()

// Reset metrics
sm.metrics.Reset()
}
```

### Consistency Checks

```go
func (js *JobStore) Validate() error {
js.mu.RLock()
defer js.mu.RUnlock()

errors := make([]error, 0)

// Check index consistency
for state, jobs := range js.jobsByState {
for _, job := range jobs {
if job.State != state {
errors = append(errors, fmt.Errorf(
"job %s in wrong state index: expected %s, got %s",
job.ID, state, job.State,
))
}
}
}

// Check resource references
for _, job := range js.jobs {
for _, vol := range job.Volumes {
if !js.volumeExists(vol.VolumeID) {
errors = append(errors, fmt.Errorf(
"job %s references non-existent volume %s",
job.ID, vol.VolumeID,
))
}
}
}

if len(errors) > 0 {
return fmt.Errorf("validation failed: %v", errors)
}

return nil
}
```

## Testing Strategy

### Unit Tests

```go
func TestJobStateTransitions(t *testing.T) {
job := NewJob()
job.State = JobStateInitializing

// Valid transition
err := job.TransitionTo(JobStateRunning)
assert.NoError(t, err)
assert.Equal(t, JobStateRunning, job.State)

// Invalid transition
err = job.TransitionTo(JobStateInitializing)
assert.Error(t, err)
}

func TestConcurrentAccess(t *testing.T) {
store := NewJobStore()

// Concurrent writes
var wg sync.WaitGroup
for i := 0; i < 100; i++ {
wg.Add(1)
go func (id int) {
defer wg.Done()
job := NewJob()
job.ID = fmt.Sprintf("job-%d", id)
store.Add(job)
}(i)
}
wg.Wait()

assert.Equal(t, 100, store.Count())
}
```

### Integration Tests

```go
func TestStateManagerIntegration(t *testing.T) {
sm := NewStateManager()

// Create job with volume
job := &Job{
ID: "test-job",
Volumes: []VolumeMount{{
VolumeID: "test-volume",
Path:     "/data",
}},
}

volume := &Volume{
ID:   "test-volume",
Name: "test",
Type: VolumeTypeFilesystem,
}

// Add resources
err := sm.AddVolume(volume)
assert.NoError(t, err)

err = sm.AddJob(job)
assert.NoError(t, err)

// Verify associations
jobVolumes := sm.GetJobVolumes(job.ID)
assert.Len(t, jobVolumes, 1)
assert.Equal(t, volume.ID, jobVolumes[0].ID)
}
```

### Benchmark Tests

```go
func BenchmarkJobStore_Add(b *testing.B) {
store := NewJobStore()

b.ResetTimer()
b.RunParallel(func (pb *testing.PB) {
i := 0
for pb.Next() {
job := NewJob()
job.ID = fmt.Sprintf("job-%d", i)
store.Add(job)
i++
}
})
}

func BenchmarkJobStore_Query(b *testing.B) {
store := setupLargeJobStore(10000)

query := &QueryBuilder{
filters: []Filter{
&StateFilter{States: []JobState{JobStateRunning}},
},
limit: 100,
}

b.ResetTimer()
for i := 0; i < b.N; i++ {
store.Query(query)
}
}
```

## Future Enhancements

### Planned Features

1. **Distributed State**
    - Multi-node state synchronization
    - Raft consensus for consistency
    - State sharding for scalability

2. **Advanced Querying**
    - SQL-like query language
    - Full-text search on job logs
    - Time-series queries for metrics

3. **State Persistence Options**
    - Pluggable storage backends
    - Write-ahead logging
    - Point-in-time recovery

4. **Enhanced Monitoring**
    - Real-time state change streams
    - Predictive analytics
    - Anomaly detection

5. **Performance Improvements**
    - Lock-free data structures
    - NUMA-aware memory allocation
    - Compressed state storage