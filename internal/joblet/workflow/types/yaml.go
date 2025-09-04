// Package types defines data structures for workflow YAML parsing and job specifications.
package types

// WorkflowYAML represents the complete structure of a workflow YAML file.
// This is the top-level structure that contains all job definitions for a workflow.
// The Jobs map uses job names as keys and JobSpec structures as values,
// allowing for named job definitions that can reference each other in dependencies.
// Example YAML:
//
//	environment:
//	  PIPELINE_VERSION: "1.0.0"
//	  LOG_LEVEL: "INFO"
//	secret_environment:
//	  API_TOKEN: "secret-token"
//	jobs:
//	  extract-data:
//	    command: "python3"
//	    args: ["extract.py"]
//	    volumes: ["data-pipeline"]
//	    environment:
//	      NODE_ENV: "production"  # Inherits global vars + this override
type WorkflowYAML struct {
	// Name is an optional workflow name for better identification
	Name string `yaml:"name,omitempty"`
	// Description is an optional workflow description
	Description string `yaml:"description,omitempty"`
	// Environment defines global environment variables inherited by all jobs (visible in logs)
	Environment map[string]string `yaml:"environment,omitempty"`
	// SecretEnvironment defines global secret environment variables inherited by all jobs (hidden from logs)
	SecretEnvironment map[string]string `yaml:"secret_environment,omitempty"`
	// Jobs maps job names to their specifications
	// Key: job name (used for dependency references)
	// Value: complete job specification
	Jobs map[string]JobSpec `yaml:"jobs"`
}

// JobSpec defines the complete specification for a single job within a workflow.
// Contains all necessary information for job execution including:
// - Command and arguments to execute
// - Runtime environment (e.g., "python-3.11-ml")
// - File uploads and volume mounts
// - Dependency requirements on other jobs
// - Resource limits (CPU, memory, I/O)
// This structure is parsed from YAML and converted to internal job representations.
type JobSpec struct {
	// Command is the executable to run (e.g., "python3", "java", "node")
	Command string `yaml:"command"`
	// Args are the command-line arguments passed to the command
	Args []string `yaml:"args"`
	// Runtime specifies the execution environment (e.g., "python-3.11-ml")
	Runtime string `yaml:"runtime"`
	// Network specifies the network for job isolation (e.g., "bridge", "isolated", "none", "custom-network")
	Network string `yaml:"network"`
	// Uploads defines files to be uploaded to the job's workspace
	Uploads *JobUploads `yaml:"uploads"`
	// Volumes lists the volumes to mount for data persistence
	Volumes []string `yaml:"volumes"`
	// Requires defines dependencies on other jobs (e.g., [{"job-name": "COMPLETED"}])
	Requires []map[string]string `yaml:"requires"`
	// Resources specifies computational limits for the job
	Resources JobResources `yaml:"resources"`
	// Environment defines regular environment variables for the job (visible in logs)
	Environment map[string]string `yaml:"environment,omitempty"`
	// SecretEnvironment defines secret environment variables for the job (hidden from logs)
	SecretEnvironment map[string]string `yaml:"secret_environment,omitempty"`
}

// JobUploads specifies which files should be uploaded to the job's execution environment.
// The Files slice contains relative file paths that will be uploaded from the client
// and made available in the job's working directory during execution.
// Essential for workflows where jobs need access to scripts, data files, or configurations.
type JobUploads struct {
	// Files lists the file paths to upload to the job's working directory
	Files []string `yaml:"files"`
}

// JobResources defines the computational resource limits for a job's execution.
// Provides fine-grained control over:
// - MaxCPU: CPU percentage limit (0-100)
// - MaxMemory: Memory limit in megabytes
// - MaxIOBPS: I/O bandwidth limit in bytes per second
// - CPUCores: Specific CPU cores to bind to (e.g., "0-3" or "0,2,4")
// These limits are enforced by the job execution system using cgroups.
type JobResources struct {
	// MaxCPU limits CPU usage as a percentage (0-100)
	MaxCPU int `yaml:"max_cpu"`
	// MaxMemory limits memory usage in megabytes
	MaxMemory int `yaml:"max_memory"`
	// MaxIOBPS limits I/O bandwidth in bytes per second
	MaxIOBPS int `yaml:"max_io_bps"`
	// CPUCores specifies CPU core binding (e.g., "0-3", "0,2,4")
	CPUCores string `yaml:"cpu_cores"`
}
