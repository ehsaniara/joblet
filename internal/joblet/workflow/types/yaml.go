// Package types defines data structures for workflow YAML parsing and job specifications.
package types

// WorkflowYAML represents the complete structure of a workflow YAML file.
// SIMPLIFIED: Removed global environment inheritance and secret_environment separation
// to reduce complexity. Each job now defines its own complete environment.
// Example YAML:
//
//	jobs:
//	  extract-data:
//	    command: "python3"
//	    args: ["extract.py"]
//	    volumes: ["data-pipeline"]
//	    environment:
//	      NODE_ENV: "production"
//	      API_TOKEN: "secret-token"  # Use env var prefix or naming convention for secrets
type WorkflowYAML struct {
	// Name is an optional workflow name for better identification
	Name string `yaml:"name,omitempty"`
	// Description is an optional workflow description
	Description string `yaml:"description,omitempty"`
	// Jobs maps job names to their specifications
	// Key: job name (used for dependency references)
	// Value: complete job specification
	Jobs map[string]JobSpec `yaml:"jobs"`

	// DEPRECATED: These fields are kept for backward compatibility but are no longer used
	// Jobs should define their own environment variables directly
	Environment       map[string]string `yaml:"environment,omitempty"`        // Deprecated
	SecretEnvironment map[string]string `yaml:"secret_environment,omitempty"` // Deprecated
}

// JobSpec defines the complete specification for a single job within a workflow.
// SIMPLIFIED: Merged environment and secret_environment into a single field.
// Use naming conventions (e.g., SECRET_ prefix) to identify sensitive variables.
type JobSpec struct {
	// Command is the executable to run (e.g., "python3", "java", "node")
	Command string `yaml:"command"`
	// Args are the command-line arguments passed to the command
	Args []string `yaml:"args"`
	// Runtime specifies the execution environment (e.g., "python-3.11-ml")
	Runtime string `yaml:"runtime"`
	// Network specifies the network for job isolation (e.g., "bridge", "isolated", "none")
	Network string `yaml:"network"`
	// Uploads defines files to be uploaded to the job's workspace
	Uploads *JobUploads `yaml:"uploads"`
	// Volumes lists the volumes to mount for data persistence
	Volumes []string `yaml:"volumes"`
	// Requires defines dependencies on other jobs (e.g., [{"job-name": "COMPLETED"}])
	Requires []map[string]string `yaml:"requires"`
	// Resources specifies computational limits for the job
	Resources JobResources `yaml:"resources"`
	// Environment defines all environment variables for the job
	// Use naming conventions for secrets (e.g., SECRET_ or _TOKEN suffix)
	Environment map[string]string `yaml:"environment,omitempty"`

	// DEPRECATED: Kept for backward compatibility
	SecretEnvironment map[string]string `yaml:"secret_environment,omitempty"` // Deprecated - use Environment
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
// - GPUCount: Number of GPUs to allocate (requires GPU support enabled)
// - GPUMemoryMB: Minimum GPU memory requirement in megabytes
// These limits are enforced by the job execution system using cgroups and device controllers.
type JobResources struct {
	// MaxCPU limits CPU usage as a percentage (0-100)
	MaxCPU int `yaml:"max_cpu"`
	// MaxMemory limits memory usage in megabytes
	MaxMemory int `yaml:"max_memory"`
	// MaxIOBPS limits I/O bandwidth in bytes per second
	MaxIOBPS int `yaml:"max_io_bps"`
	// CPUCores specifies CPU core binding (e.g., "0-3", "0,2,4")
	CPUCores string `yaml:"cpu_cores"`
	// GPUCount specifies the number of GPUs to allocate (0 = no GPU)
	GPUCount int `yaml:"gpu_count"`
	// GPUMemoryMB specifies minimum GPU memory requirement in MB (0 = any)
	GPUMemoryMB int `yaml:"gpu_memory_mb"`
}
