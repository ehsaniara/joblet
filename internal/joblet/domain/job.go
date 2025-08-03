package domain

import (
	"errors"
	"time"
)

// JobStatus represents the current state of a job
type JobStatus string

const (
	StatusPending      JobStatus = "PENDING"
	StatusRunning      JobStatus = "RUNNING"
	StatusCompleted    JobStatus = "COMPLETED"
	StatusFailed       JobStatus = "FAILED"
	StatusStopped      JobStatus = "STOPPED"
	StatusScheduled    JobStatus = "SCHEDULED"
	StatusInitializing JobStatus = "INITIALIZING"
)

var (
	// ErrInvalidCommand is returned when job command is empty
	ErrInvalidCommand = errors.New("job command cannot be empty")
)

// Job represents a job with improved resource limits using value objects
type Job struct {
	Id            string         // Unique identifier for job tracking
	Command       string         // Executable command path
	Args          []string       // Command line arguments
	Limits        ResourceLimits // CPU/memory/IO constraints using value objects
	Status        JobStatus      // Current execution state
	Pid           int32          // Process ID when running
	CgroupPath    string         // Filesystem path for resource limits
	StartTime     time.Time      // Job creation timestamp
	EndTime       *time.Time     // Completion timestamp (nil if running)
	ExitCode      int32          // Process exit status
	ScheduledTime *time.Time     // When the job should start (nil for immediate execution)
	Network       string         // Network name
	Volumes       []string       // Volume names to mount
}

// IsRunning returns true if the job is currently running
func (j *Job) IsRunning() bool {
	return j.Status == StatusRunning
}

// IsCompleted returns true if the job has completed execution
func (j *Job) IsCompleted() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusStopped
}

// IsScheduled returns true if the job is scheduled for future execution
func (j *Job) IsScheduled() bool {
	return j.Status == StatusScheduled
}

// HasResourceLimits returns true if any resource limits are set
func (j *Job) HasResourceLimits() bool {
	return j.Limits.HasCPULimit() ||
		j.Limits.HasMemoryLimit() ||
		j.Limits.HasIOLimit() ||
		j.Limits.HasCoreRestriction()
}

// GetDuration returns the job execution duration
func (j *Job) GetDuration() time.Duration {
	if j.EndTime == nil {
		if j.IsRunning() {
			return time.Since(j.StartTime)
		}
		return 0
	}
	return j.EndTime.Sub(j.StartTime)
}

// GetScheduleDelay returns how long until the job is scheduled to run
func (j *Job) GetScheduleDelay() time.Duration {
	if j.ScheduledTime == nil || !j.IsScheduled() {
		return 0
	}
	delay := time.Until(*j.ScheduledTime)
	if delay < 0 {
		return 0 // Already past scheduled time
	}
	return delay
}

// Validate validates the job configuration
func (j *Job) Validate() error {
	if j.Command == "" {
		return ErrInvalidCommand
	}

	// Validate is built into the ResourceLimits through the builder
	// No additional validation needed here

	return nil
}

// DeepCopy creates a deep copy of the job
func (j *Job) DeepCopy() *Job {
	if j == nil {
		return nil
	}

	jobCopy := &Job{
		Id:         j.Id,
		Command:    j.Command,
		Args:       make([]string, len(j.Args)),
		Limits:     j.Limits, // Value objects are safe to copy
		Status:     j.Status,
		Pid:        j.Pid,
		CgroupPath: j.CgroupPath,
		StartTime:  j.StartTime,
		ExitCode:   j.ExitCode,
		Network:    j.Network,
		Volumes:    make([]string, len(j.Volumes)),
	}

	// Copy slices
	copy(jobCopy.Args, j.Args)
	copy(jobCopy.Volumes, j.Volumes)

	// Copy pointers
	if j.EndTime != nil {
		endTime := *j.EndTime
		jobCopy.EndTime = &endTime
	}
	if j.ScheduledTime != nil {
		scheduledTime := *j.ScheduledTime
		jobCopy.ScheduledTime = &scheduledTime
	}

	return jobCopy
}
