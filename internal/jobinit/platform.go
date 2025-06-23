package jobinit

import "runtime"

// JobInitializer interface that both platforms will implement
type JobInitializer interface {
	Run() error
	LoadConfigFromEnv() (*JobConfig, error)
	ExecuteJob(config *JobConfig) error
}

// JobConfig is shared across platforms - updated for single-process approach
type JobConfig struct {
	JobID      string
	Command    string
	Args       []string
	CgroupPath string

	// User namespace configuration (Linux only)
	UserNamespaceEnabled bool
	NamespaceUID         uint32
	NamespaceGID         uint32
}

// Run is the entry point that works on all platforms
func Run() error {
	switch runtime.GOOS {
	case "linux":
		initializer := NewLinuxJobInitializer()
		return initializer.Run()
	case "darwin":
		return nil
	default:
		return nil
	}
}
