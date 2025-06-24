//go:build !linux && !darwin

package jobinit

import (
	"worker/pkg/logger"
	osinterface "worker/pkg/os"
)

// newLinuxJobInitializer creates a Linux job initializer (fallback for other platforms)
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

// newDarwinJobInitializer creates a Darwin job initializer (stub for other platforms)
func newDarwinJobInitializer() JobInitializer {
	panic("Darwin initializer not supported on this platform")
}
