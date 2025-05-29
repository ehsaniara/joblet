package resource

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

import (
	"fmt"
	"os"
)

//counterfeiter:generate . Resource
type Resource interface {
	Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error
}
type cgroup struct {
}

func New() Resource {
	return &cgroup{}
}

// Create to create cgroups
func (cg *cgroup) Create(cgroupJobDir string, maxCPU int32, maxMemory int32, maxIOBPS int32) error {

	if err := os.MkdirAll(cgroupJobDir, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup directory: %v", err)
	}

	// using cpu.max for cgroup v2
	if err := cg.SetCPULimit(cgroupJobDir, int(maxCPU)); err != nil {
		return err
	}

	if err := cg.SetMemoryLimit(cgroupJobDir, int(maxMemory)); err != nil {
		return err
	}

	if maxIOBPS > 0 {
		err := cg.SetIOLimit(cgroupJobDir, int(maxIOBPS))
		if err != nil {
			return err
		}
	}
	return nil
}

// SetIOLimit sets IO limits for a cgroup
func (cg *cgroup) SetIOLimit(cgroupPath string, ioBPS int) error {
	//TODO implement me
	panic("implement me")
}

// SetCPULimit sets CPU limits for the cgroup
func (cg *cgroup) SetCPULimit(cgroupPath string, cpuLimit int) error {
	//TODO implement me
	panic("implement me")
}

// SetMemoryLimit sets memory limits for the cgroup
func (cg *cgroup) SetMemoryLimit(cgroupPath string, memoryLimitMB int) error {
	//TODO implement me
	panic("implement me")
}
