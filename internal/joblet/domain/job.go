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

// Stop forcefully terminates a running job
func (j *Job) Stop() {
	j.Status = StatusStopped
	j.ExitCode = -1
	now := time.Now()
	j.EndTime = &now
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
		Args:       copyStringSlice(j.Args),
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

func copyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}

	if len(src) == 0 {
		return []string{}
	}

	dst := make([]string, len(src))
	copy(dst, src)

	return dst
}
