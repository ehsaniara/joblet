//go:build darwin

package filesystem

import (
	"fmt"
	"runtime"
	"worker/pkg/config"
	"worker/pkg/logger"
	"worker/pkg/platform"
)

// Darwin stub implementation for filesystem isolation
type Isolator struct {
	platform platform.Platform
	config   config.FilesystemConfig
	logger   *logger.Logger
}

func NewIsolator(cfg config.FilesystemConfig, platform platform.Platform) *Isolator {
	return &Isolator{
		platform: platform,
		config:   cfg,
		logger:   logger.New().WithField("component", "filesystem-isolator-darwin"),
	}
}

// JobFilesystem represents a stub filesystem for Darwin
type JobFilesystem struct {
	JobID    string
	RootDir  string
	TmpDir   string
	WorkDir  string
	platform platform.Platform
	config   config.FilesystemConfig
	logger   *logger.Logger
}

// CreateJobFilesystem creates a stub filesystem for Darwin (no isolation)
func (i *Isolator) CreateJobFilesystem(jobID string) (*JobFilesystem, error) {
	i.logger.Warn("filesystem isolation not available on Darwin", "platform", runtime.GOOS)

	return &JobFilesystem{
		JobID:    jobID,
		RootDir:  "/tmp", // Use system tmp as fallback
		TmpDir:   "/tmp",
		WorkDir:  "/tmp",
		platform: i.platform,
		config:   i.config,
		logger:   i.logger,
	}, nil
}

// Setup is a no-op on Darwin
func (f *JobFilesystem) Setup() error {
	f.logger.Warn("filesystem isolation setup skipped on Darwin")
	return fmt.Errorf("filesystem isolation not supported on Darwin")
}

// Cleanup is a no-op on Darwin
func (f *JobFilesystem) Cleanup() error {
	f.logger.Debug("filesystem cleanup skipped on Darwin")
	return nil
}
