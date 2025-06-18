package jobinit

// JobInitializer interface that both platforms will implement
type JobInitializer interface {
	Run() error
	LoadConfigFromEnv() (*JobConfig, error)
	ExecuteJob(config *JobConfig) error
}

// JobConfig is shared across platforms (simplified for Option 3)
type JobConfig struct {
	JobID             string
	Command           string
	Args              []string
	CgroupPath        string
	NetworkGroupID    string // Network group identifier
	IsNewNetworkGroup bool   // Whether this is a new network group (for setup)
}

// Run is the entry point that works on all platforms
func Run() error {
	ji := NewJobInitializer()
	return ji.Run()
}
