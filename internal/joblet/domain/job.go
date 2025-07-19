package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type JobStatus string

const (
	StatusScheduled    JobStatus = "SCHEDULED"
	StatusInitializing JobStatus = "INITIALIZING"
	StatusRunning      JobStatus = "RUNNING"
	StatusCompleted    JobStatus = "COMPLETED"
	StatusFailed       JobStatus = "FAILED"
	StatusStopped      JobStatus = "STOPPED"
)

type ResourceLimits struct {
	MaxCPU    int32  // Percentage of CPU time (0-100+ for multiple cores)
	MaxMemory int32  // Memory in MB
	MaxIOBPS  int32  // IO bandwidth
	CPUCores  string // Core specification: "0-3", "1,3,5", "2", or "" for no limit
}

type Job struct {
	Id            string         // Unique identifier for job tracking
	Command       string         // Executable command path
	Args          []string       // Command line arguments
	Limits        ResourceLimits // CPU/memory/IO constraints
	Status        JobStatus      // Current execution state
	Pid           int32          // Process ID when running
	CgroupPath    string         // Filesystem path for resource limits
	StartTime     time.Time      // Job creation timestamp
	EndTime       *time.Time     // Completion timestamp (nil if running)
	ExitCode      int32          // Process exit status
	ScheduledTime *time.Time     // When the job should start (nil for immediate execution)
}

func (r *ResourceLimits) HasCoreRestriction() bool {
	return r.CPUCores != ""
}

func (r *ResourceLimits) ParseCoreCount() int {
	if r.CPUCores == "" {
		return 0
	}

	// Parse "0-3" -> 4 cores, "1,3,5" -> 3 cores, "2" -> 1 core
	cores := strings.Split(strings.ReplaceAll(r.CPUCores, "-", ","), ",")

	// Handle ranges like "0-3"
	if strings.Contains(r.CPUCores, "-") {
		parts := strings.Split(r.CPUCores, "-")
		if len(parts) == 2 {
			start, _ := strconv.Atoi(parts[0])
			end, _ := strconv.Atoi(parts[1])
			return end - start + 1
		}
	}

	return len(cores)
}

func (j *Job) IsRunning() bool {
	return j.Status == StatusRunning
}

func (j *Job) IsCompleted() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusStopped
}

// IsScheduled returns true if the job is scheduled for future execution
func (j *Job) IsScheduled() bool {
	return j.Status == StatusScheduled
}

// IsDue returns true if a scheduled job is ready to execute
func (j *Job) IsDue() bool {
	if !j.IsScheduled() || j.ScheduledTime == nil {
		return false
	}
	return time.Now().After(*j.ScheduledTime) || time.Now().Equal(*j.ScheduledTime)
}

// GetExecutionTime returns the time when this job should execute
// For immediate jobs, returns StartTime. For scheduled jobs, returns ScheduledTime.
func (j *Job) GetExecutionTime() time.Time {
	if j.ScheduledTime != nil {
		return *j.ScheduledTime
	}
	return j.StartTime
}

// MarkAsRunning transitions job from INITIALIZING to RUNNING state with given PID
func (j *Job) MarkAsRunning(pid int32) error {
	if j.Status != StatusInitializing {
		return fmt.Errorf("cannot mark job as running: current status is %s, expected %s", j.Status, StatusInitializing)
	}

	if pid <= 0 {
		return errors.New("PID must be positive")
	}

	j.Status = StatusRunning
	j.Pid = pid
	return nil
}

// MarkAsInitializing transitions job from SCHEDULED to INITIALIZING state
func (j *Job) MarkAsInitializing() error {
	if j.Status != StatusScheduled {
		return fmt.Errorf("cannot mark job as initializing: current status is %s, expected %s", j.Status, StatusScheduled)
	}

	j.Status = StatusInitializing
	return nil
}

// Complete marks job as successfully finished with given exit code
func (j *Job) Complete(exitCode int32) {
	j.Status = StatusCompleted
	j.ExitCode = exitCode
	now := time.Now()
	j.EndTime = &now
}

// Fail marks job as failed with given exit code
func (j *Job) Fail(exitCode int32) {
	j.Status = StatusFailed
	j.ExitCode = exitCode
	now := time.Now()
	j.EndTime = &now
}

// Stop marks job as stopped (terminated by user)
func (j *Job) Stop() {
	j.Status = StatusStopped
	j.ExitCode = -1 // Conventional exit code for terminated processes
	now := time.Now()
	j.EndTime = &now
}

// Duration returns how long the job has been running or took to complete
func (j *Job) Duration() time.Duration {
	if j.EndTime != nil {
		return j.EndTime.Sub(j.StartTime)
	}
	return time.Since(j.StartTime)
}

// DeepCopy creates a deep copy of the job
func (j *Job) DeepCopy() *Job {
	if j == nil {
		return nil
	}

	cp := &Job{
		Id:         j.Id,
		Command:    j.Command,
		Args:       append([]string(nil), j.Args...),
		Limits:     j.Limits,
		Status:     j.Status,
		Pid:        j.Pid,
		CgroupPath: j.CgroupPath,
		StartTime:  j.StartTime,
		ExitCode:   j.ExitCode,
	}

	if j.EndTime != nil {
		endTimeCopy := *j.EndTime
		cp.EndTime = &endTimeCopy
	}

	if j.ScheduledTime != nil {
		scheduledTimeCopy := *j.ScheduledTime
		cp.ScheduledTime = &scheduledTimeCopy
	}

	return cp
}
