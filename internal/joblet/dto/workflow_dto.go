package dto

import "time"

// WorkflowDTO represents a workflow for data transfer
type WorkflowDTO struct {
	ID           string                  `json:"id"`
	Name         string                  `json:"name"`
	Status       string                  `json:"status"`
	Jobs         []WorkflowJobDTO        `json:"jobs"`
	CreatedTime  time.Time               `json:"created_time"`
	StartTime    *time.Time              `json:"start_time,omitempty"`
	EndTime      *time.Time              `json:"end_time,omitempty"`
	Environment  map[string]string       `json:"environment,omitempty"`
	Networks     []WorkflowNetworkDTO    `json:"networks,omitempty"`
	Volumes      []WorkflowVolumeDTO     `json:"volumes,omitempty"`
	Dependencies []WorkflowDependencyDTO `json:"dependencies,omitempty"`
}

// WorkflowJobDTO represents a job within a workflow
type WorkflowJobDTO struct {
	Name              string            `json:"name"`
	Command           string            `json:"command"`
	Args              []string          `json:"args,omitempty"`
	Status            string            `json:"status"`
	JobUuid           string            `json:"job_uuid,omitempty"` // Set when job is actually created
	Dependencies      []string          `json:"dependencies,omitempty"`
	Environment       map[string]string `json:"environment,omitempty"`
	SecretEnvironment map[string]string `json:"secret_environment,omitempty"`
	Resources         ResourceLimitsDTO `json:"resources,omitempty"`
	Network           string            `json:"network,omitempty"`
	Volumes           []string          `json:"volumes,omitempty"`
	Runtime           string            `json:"runtime,omitempty"`
	StartTime         *time.Time        `json:"start_time,omitempty"`
	EndTime           *time.Time        `json:"end_time,omitempty"`
	ExitCode          int32             `json:"exit_code,omitempty"`
}

// WorkflowNetworkDTO represents a network definition within a workflow
type WorkflowNetworkDTO struct {
	Name string `json:"name"`
	CIDR string `json:"cidr,omitempty"`
}

// WorkflowVolumeDTO represents a volume definition within a workflow
type WorkflowVolumeDTO struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"` // "filesystem" or "memory"
	Size string `json:"size,omitempty"` // Readable size
}

// WorkflowDependencyDTO represents a dependency between workflow jobs
type WorkflowDependencyDTO struct {
	JobName   string   `json:"job_name"`
	DependsOn []string `json:"depends_on"`
	Condition string   `json:"condition,omitempty"` // "success", "failure", "completion"
}

// StartWorkflowRequestDTO for starting workflows
type StartWorkflowRequestDTO struct {
	Name        string            `json:"name"`
	Content     []byte            `json:"content"`               // YAML content
	Environment map[string]string `json:"environment,omitempty"` // Global environment variables
}

// WorkflowStatusDTO represents workflow execution status
type WorkflowStatusDTO struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	JobsTotal     int    `json:"jobs_total"`
	JobsCompleted int    `json:"jobs_completed"`
	JobsRunning   int    `json:"jobs_running"`
	JobsFailed    int    `json:"jobs_failed"`
	StartTime     string `json:"start_time,omitempty"` // ISO 8601 format
	EndTime       string `json:"end_time,omitempty"`   // ISO 8601 format
	Duration      string `json:"duration,omitempty"`   // Readable duration
}

// WorkflowListItemDTO represents a workflow in list responses
type WorkflowListItemDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	JobCount    int    `json:"job_count"`
	CreatedTime string `json:"created_time"` // ISO 8601 format
	Duration    string `json:"duration,omitempty"`
}
