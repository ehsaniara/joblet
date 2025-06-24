//go:build darwin

package jobinit

import (
	"worker/pkg/logger"
)

// newDarwinJobInitializer creates a Darwin job initializer
func newDarwinJobInitializer() JobInitializer {
	return &darwinJobInitializer{
		logger: logger.New(),
	}
}

// newLinuxJobInitializer creates a Linux job initializer (stub for Darwin)
func newLinuxJobInitializer() JobInitializer {
	panic("Linux initializer called on Darwin")
}
