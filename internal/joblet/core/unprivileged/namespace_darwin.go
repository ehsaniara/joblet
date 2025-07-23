//go:build darwin

package unprivileged

import "syscall"

// JobIsolation provides job isolation capabilities
type JobIsolation struct{}

// NewJobIsolation creates a new job isolation instance
func NewJobIsolation() *JobIsolation {
	return &JobIsolation{}
}

// CreateIsolatedSysProcAttr creates syscall attributes for process isolation
func (j *JobIsolation) CreateIsolatedSysProcAttr() *syscall.SysProcAttr {
	// Return minimal attributes for Darwin
	return &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
}
