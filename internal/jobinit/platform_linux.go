//go:build linux

package jobinit

import (
	"worker/pkg/logger"
	osinterface "worker/pkg/os"
)

// newLinuxJobInitializer creates a new Linux job initializer
func newLinuxJobInitializer() JobInitializer {
	osInterface := &osinterface.DefaultOs{}
	syscallInterface := &osinterface.DefaultSyscall{}
	execInterface := &osinterface.DefaultExec{}

	return &linuxJobInitializer{
		osInterface:      osInterface,
		execInterface:    execInterface,
		syscallInterface: syscallInterface,
		logger:           logger.New().WithField("component", "job-init-linux"),
	}
}

// newDarwinJobInitializer creates a Darwin job initializer (stub for Linux)
func newDarwinJobInitializer() JobInitializer {
	panic("Darwin initializer called on Linux")
}
