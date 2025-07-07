//go:build linux

package unprivileged

import (
	"joblet/pkg/logger"
	"joblet/pkg/platform"
	"syscall"
)

type JobIsolation struct {
	platform platform.Platform
	logger   *logger.Logger
}

func NewJobIsolation() *JobIsolation {
	return &JobIsolation{
		platform: platform.NewPlatform(),
		logger:   logger.New().WithField("component", "native-isolation"),
	}
}

// CreateIsolatedSysProcAttr uses Go's native syscall package for maximum compatibility
func (ji *JobIsolation) CreateIsolatedSysProcAttr() *syscall.SysProcAttr {
	sysProcAttr := &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // Process isolation (native)
		syscall.CLONE_NEWNS | // Mount isolation (native)
		syscall.CLONE_NEWIPC | // IPC isolation (native)
		syscall.CLONE_NEWUTS // UTS isolation (native)

	ji.logger.Debug("created native Go isolation attributes",
		"approach", "native-go-syscalls",
		"pidNamespace", true,
		"mountNamespace", true,
		"userComplexity", false,
		"reliability", "high")

	return sysProcAttr
}
