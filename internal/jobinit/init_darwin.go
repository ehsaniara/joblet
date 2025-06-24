//go:build darwin

package jobinit

import (
	"worker/pkg/logger"
)

type darwinJobInitializer struct {
	logger *logger.Logger
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
