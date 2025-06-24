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

// NewJobInitializer creates a platform-specific job initializer
// This calls the existing platform-specific constructors
func NewJobInitializer() JobInitializer {
	switch runtime.GOOS {
	case "linux":
		return newLinuxJobInitializer()
	case "darwin":
		// Use the existing Darwin implementation
		return newDarwinJobInitializer()
	default:
		// Fallback - assume Linux for unknown platforms
		return newLinuxJobInitializer()
	}
}

// Run is the entry point that works on all platforms
func Run() error {
	ji := NewJobInitializer()
	return ji.Run()
}
