package domain

import (
	"errors"
	"fmt"
	"job-worker/internal/worker/utils"
	"time"
)

type JobStatus string

const (
	StatusInitializing JobStatus = "INITIALIZING"
	StatusRunning      JobStatus = "RUNNING"
	StatusCompleted    JobStatus = "COMPLETED"
	StatusFailed       JobStatus = "FAILED"
	StatusStopped      JobStatus = "STOPPED"
)

type ResourceLimits struct {
	MaxCPU    int32
	MaxMemory int32
	MaxIOBPS  int32
}

type Job struct {
	Id         string         // Unique identifier for job tracking
	Command    string         // Executable command path
	Args       []string       // Command line arguments
	Limits     ResourceLimits // CPU/memory/IO constraints
	Status     JobStatus      // Current execution state
	Pid        int32          // Process ID when running
	CgroupPath string         // Filesystem path for resource limits
	StartTime  time.Time      // Job creation timestamp
	EndTime    *time.Time     // Completion timestamp (nil if running)
	ExitCode   int32          // Process exit status
}

func (j *Job) IsRunning() bool {
	return j.Status == StatusRunning
}

func (j *Job) IsCompleted() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusStopped
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

// Complete marks job as successfully finished with given exit code
func (j *Job) Complete(exitCode int32) error {

	if j.Status != StatusRunning {
		return fmt.Errorf("cannot complete job: current status is %s, expected %s", j.Status, StatusRunning)
	}

	j.Status = StatusCompleted
	j.ExitCode = exitCode
	now := time.Now()
	j.EndTime = &now
	return nil
}

// Fail marks job as failed with given exit code (allows INITIALIZING->FAILED transition)
func (j *Job) Fail(exitCode int32) error {

	if j.Status != StatusRunning && j.Status != StatusInitializing {
		return fmt.Errorf("cannot fail job: current status is %s, expected %s or %s", j.Status, StatusRunning, StatusInitializing)
	}

	j.Status = StatusFailed
	j.ExitCode = exitCode
	now := time.Now()
	j.EndTime = &now
	return nil
}

// Stop forcefully terminates a running job
func (j *Job) Stop() error {

	if j.Status != StatusRunning {
		return fmt.Errorf("cannot stop job: current status is %s, expected %s", j.Status, StatusRunning)
	}

	j.Status = StatusStopped
	j.ExitCode = -1
	now := time.Now()
	j.EndTime = &now
	return nil
}

// DeepCopy creates independent copy to prevent concurrent modification issues
func (j *Job) DeepCopy() *Job {
	var endTimeCopy *time.Time
	if j.EndTime != nil {
		cp := *j.EndTime
		endTimeCopy = &cp
	}

	return &Job{
		Id:         j.Id,
		Command:    j.Command,
		Args:       utils.CopyStringSlice(j.Args),
		Limits:     j.Limits,
		Status:     j.Status,
		Pid:        j.Pid,
		CgroupPath: j.CgroupPath,
		StartTime:  j.StartTime,
		EndTime:    endTimeCopy,
		ExitCode:   j.ExitCode,
	}
}

// Duration calculates job runtime (current time if still running)
func (j *Job) Duration() time.Duration {
	if j.EndTime != nil {
		return j.EndTime.Sub(j.StartTime)
	}
	if j.Status == StatusRunning {
		return time.Since(j.StartTime)
	}
	return 0
}
