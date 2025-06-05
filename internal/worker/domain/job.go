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
	Id         string
	Command    string
	Args       []string
	Limits     ResourceLimits
	Status     JobStatus
	Pid        int32
	CgroupPath string
	StartTime  time.Time
	EndTime    *time.Time
	ExitCode   int32
}

func (j *Job) IsRunning() bool {
	return j.Status == StatusRunning
}

func (j *Job) IsCompleted() bool {
	return j.Status == StatusCompleted || j.Status == StatusFailed || j.Status == StatusStopped
}

func (j *Job) IsInitializing() bool {
	return j.Status == StatusInitializing
}

// MarkAsRunning from INITIALIZING to RUNNING
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

func (j *Job) Duration() time.Duration {
	if j.EndTime != nil {
		return j.EndTime.Sub(j.StartTime)
	}
	if j.Status == StatusRunning {
		return time.Since(j.StartTime)
	}
	return 0
}
