//go:build darwin

package jobinit

import "worker/pkg/logger"

type darwinJobInitializer struct {
	logger *logger.Logger
}

// NewDarwinJobInitializer creates a Darwin job initializer (placeholder)
func NewDarwinJobInitializer() JobInitializer {
	return &darwinJobInitializer{
		logger: logger.New().WithField("component", "job-init-darwin"),
	}
}

func (d darwinJobInitializer) Run() error {
	//TODO implement me
	panic("implement me")
}

func (d darwinJobInitializer) LoadConfigFromEnv() (*JobConfig, error) {
	//TODO implement me
	panic("implement me")
}

func (d darwinJobInitializer) ExecuteJob(config *JobConfig) error {
	//TODO implement me
	panic("implement me")
}

// Ensure linuxJobInitializer implements JobInitializer
var _ JobInitializer = (*darwinJobInitializer)(nil)
