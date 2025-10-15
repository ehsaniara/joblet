//go:build linux

package unprivileged

import (
	"syscall"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/pkg/logger"
	"github.com/ehsaniara/joblet/pkg/platform"
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
	return ji.CreateIsolatedSysProcAttrForJobType(domain.JobTypeStandard)
}

// CreateIsolatedSysProcAttrForJobType creates isolation attributes based on job type
func (ji *JobIsolation) CreateIsolatedSysProcAttrForJobType(jobType domain.JobType) *syscall.SysProcAttr {
	sysProcAttr := &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	// Base isolation: PID, Mount, IPC, UTS (always enabled)
	sysProcAttr.Cloneflags = syscall.CLONE_NEWPID | // Process isolation (native)
		syscall.CLONE_NEWNS | // Mount isolation (native)
		syscall.CLONE_NEWIPC | // IPC isolation (native)
		syscall.CLONE_NEWUTS // UTS isolation (native)

	// Network isolation: Enable for production jobs, disable for runtime builds
	var networkIsolation bool
	if jobType.IsRuntimeBuild() {
		// Runtime build jobs need internet access to download packages
		// They use builder chroot for filesystem isolation but need host networking
		networkIsolation = false
		ji.logger.Info("DISABLING NETWORK ISOLATION FOR RUNTIME BUILD JOB", "jobType", jobType)
	} else {
		// Production jobs get full network isolation
		sysProcAttr.Cloneflags |= syscall.CLONE_NEWNET
		networkIsolation = true
		ji.logger.Info("ENABLING NETWORK ISOLATION FOR PRODUCTION JOB", "jobType", jobType)
	}

	ji.logger.Debug("created native Go isolation attributes",
		"approach", "native-go-syscalls",
		"jobType", jobType,
		"pidNamespace", true,
		"mountNamespace", true,
		"networkNamespace", networkIsolation,
		"userComplexity", false,
		"reliability", "high")

	return sysProcAttr
}
