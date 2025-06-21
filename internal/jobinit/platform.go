package jobinit

// JobInitializer interface that both platforms will implement
type JobInitializer interface {
	Run() error
	LoadConfigFromEnv() (*JobConfig, error)
	ExecuteJob(config *JobConfig) error
}

// JobConfig is shared across platforms
type JobConfig struct {
	JobID      string
	Command    string
	Args       []string
	CgroupPath string

	// User namespace configuration
	UserNamespaceEnabled bool
	NamespaceUID         uint32
	NamespaceGID         uint32
}

// Run is the entry point that works on all platforms
func Run() error {
	ji := NewJobInitializer()
	return ji.Run()
}
