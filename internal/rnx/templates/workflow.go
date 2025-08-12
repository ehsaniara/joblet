package templates

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowJobConfig extends JobConfig with dependency information
type WorkflowJobConfig struct {
	JobConfig  `yaml:",inline"`
	Requires   []JobRequirement `yaml:"requires"`
	Conditions []Condition      `yaml:"conditions"`
	MaxWait    string           `yaml:"max_wait"`
}

// JobRequirement represents a dependency on another job
type JobRequirement struct {
	JobName    string `yaml:"-"`
	Status     string `yaml:"-"`
	Expression string `yaml:"expression"`
}

// UnmarshalYAML custom unmarshaler for JobRequirement
func (jr *JobRequirement) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a map first (structured format)
	var m map[string]string
	if err := unmarshal(&m); err == nil {
		for k, v := range m {
			if k == "expression" {
				jr.Expression = v
			} else {
				jr.JobName = k
				jr.Status = v
			}
		}
		return nil
	}

	// Try to unmarshal as a string (expression format)
	var expr string
	if err := unmarshal(&expr); err == nil {
		jr.Expression = expr
		return nil
	}

	return fmt.Errorf("invalid job requirement format")
}

// Condition represents a conditional requirement for job execution
type Condition struct {
	Type  string `yaml:"type"`
	Value string `yaml:"value"`
}

// UnmarshalYAML custom unmarshaler for Condition
func (c *Condition) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Try to unmarshal as a map
	var m map[string]string
	if err := unmarshal(&m); err == nil {
		for k, v := range m {
			c.Type = k
			c.Value = v
			break // Take the first key-value pair
		}
		return nil
	}

	return fmt.Errorf("invalid condition format")
}

// WorkflowJobSet extends JobSet with workflow capabilities
type WorkflowJobSet struct {
	Version   string                       `yaml:"version"`
	Defaults  JobConfig                    `yaml:"defaults"`
	Jobs      map[string]WorkflowJobConfig `yaml:"jobs"`
	Workflows map[string]WorkflowDef       `yaml:"workflows"`
}

// WorkflowDef represents a named workflow definition
type WorkflowDef struct {
	Name        string                       `yaml:"name"`
	Description string                       `yaml:"description"`
	Jobs        map[string]WorkflowJobConfig `yaml:"jobs"`
}

// WorkflowExecutionMode determines how the template should be executed
type WorkflowExecutionMode int

const (
	ModeSingleJob WorkflowExecutionMode = iota
	ModeParallelJobs
	ModeWorkflow
	ModeNamedWorkflow
	ModeError
)

// WorkflowMetadata contains workflow tracking information
type WorkflowMetadata struct {
	ID            int               // Sequential workflow ID
	Name          string            // Human-readable name
	Template      string            // Source template file
	Selector      string            // Template selector used
	Status        WorkflowStatus    // Derived from job states
	JobIDs        []string          // Ordered list of job IDs
	JobMapping    map[string]string // Internal name to job ID mapping
	CreatedAt     int64             // Unix timestamp
	StartedAt     *int64            // Unix timestamp of first job start
	CompletedAt   *int64            // Unix timestamp of completion
	TotalJobs     int               // Total number of jobs
	CompletedJobs int               // Number of completed jobs
	FailedJobs    int               // Number of failed jobs
}

// WorkflowStatus represents the overall workflow state
type WorkflowStatus string

const (
	WorkflowStatusPending   WorkflowStatus = "PENDING"
	WorkflowStatusRunning   WorkflowStatus = "RUNNING"
	WorkflowStatusCompleted WorkflowStatus = "COMPLETED"
	WorkflowStatusFailed    WorkflowStatus = "FAILED"
	WorkflowStatusCanceled  WorkflowStatus = "CANCELED"
	WorkflowStatusStopped   WorkflowStatus = "STOPPED"
)

// DetectExecutionMode analyzes the template to determine execution mode
func DetectExecutionMode(templatePath string, selector string) (WorkflowExecutionMode, string, error) {
	// Parse selector if present
	parts := strings.Split(templatePath, ":")
	if len(parts) == 2 {
		templatePath = parts[0]
		selector = parts[1]
	}

	// Load the template
	config, err := LoadWorkflowConfig(templatePath)
	if err != nil {
		return ModeError, "", err
	}

	// If selector is present, it's single job or named workflow
	if selector != "" {
		// Check if it's a job name
		if _, exists := config.Jobs[selector]; exists {
			return ModeSingleJob, selector, nil
		}

		// Check if it's a workflow name
		if config.Workflows != nil {
			if _, exists := config.Workflows[selector]; exists {
				return ModeNamedWorkflow, selector, nil
			}
		}

		return ModeError, "", fmt.Errorf("selector '%s' not found in template", selector)
	}

	// No selector - check for dependencies
	hasDependencies := false
	for _, job := range config.Jobs {
		if len(job.Requires) > 0 {
			hasDependencies = true
			break
		}
	}

	// If we have multiple workflows and no selector, error
	if len(config.Workflows) > 0 {
		return ModeError, "", fmt.Errorf("multiple workflows found, please specify which to run")
	}

	// If dependencies exist, it's a workflow
	if hasDependencies {
		return ModeWorkflow, "", nil
	}

	// Otherwise, parallel execution
	return ModeParallelJobs, "", nil
}

// LoadWorkflowConfig loads a workflow-enhanced configuration
func LoadWorkflowConfig(path string) (*WorkflowJobSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WorkflowJobSet
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults and inheritance for jobs
	for name, job := range config.Jobs {
		if job.Extends != "" {
			if parent, exists := config.Jobs[job.Extends]; exists {
				job.JobConfig = mergeConfigs(parent.JobConfig, job.JobConfig)
			}
		}
		job.JobConfig = mergeConfigs(config.Defaults, job.JobConfig)
		config.Jobs[name] = job
	}

	// Apply defaults to workflow jobs
	for wfName, workflow := range config.Workflows {
		for jobName, job := range workflow.Jobs {
			job.JobConfig = mergeConfigs(config.Defaults, job.JobConfig)
			workflow.Jobs[jobName] = job
		}
		config.Workflows[wfName] = workflow
	}

	return &config, nil
}

// ValidateDependencies checks for circular dependencies in the workflow
func ValidateDependencies(jobs map[string]WorkflowJobConfig) error {
	// Build dependency graph
	graph := make(map[string][]string)
	for name, job := range jobs {
		var deps []string
		for _, req := range job.Requires {
			if req.JobName != "" {
				deps = append(deps, req.JobName)
			} else if req.Expression != "" {
				// Parse expression to extract job names
				jobNames := extractJobNamesFromExpression(req.Expression)
				deps = append(deps, jobNames...)
			}
		}
		graph[name] = deps
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, dep := range graph[node] {
			if !visited[dep] {
				if hasCycle(dep) {
					return true
				}
			} else if recStack[dep] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for node := range graph {
		if !visited[node] {
			if hasCycle(node) {
				return fmt.Errorf("circular dependency detected in workflow")
			}
		}
	}

	return nil
}

// extractJobNamesFromExpression parses a boolean expression to extract job names
func extractJobNamesFromExpression(expr string) []string {
	var jobNames []string

	// Simple regex-like parsing for job names
	// This is a simplified version - a full implementation would use a proper parser
	tokens := strings.FieldsFunc(expr, func(r rune) bool {
		return r == '(' || r == ')' || r == ' ' || r == '=' || r == '!' || r == '<' || r == '>'
	})

	for _, token := range tokens {
		token = strings.TrimSpace(token)
		// Skip operators and keywords
		if token == "" || token == "AND" || token == "OR" || token == "NOT" ||
			token == "IN" || token == "NOT_IN" || token == "&&" || token == "||" {
			continue
		}
		// Skip status values
		if token == "COMPLETED" || token == "FAILED" || token == "CANCELED" ||
			token == "STOPPED" || token == "RUNNING" || token == "PENDING" || token == "SCHEDULED" {
			continue
		}
		// Remaining tokens should be job names
		jobNames = append(jobNames, token)
	}

	return jobNames
}

// BuildDependencyGraph creates a topological ordering of jobs
func BuildDependencyGraph(jobs map[string]WorkflowJobConfig) ([]string, error) {
	// Validate dependencies first
	if err := ValidateDependencies(jobs); err != nil {
		return nil, err
	}

	// Build adjacency list
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// Initialize all jobs
	for name := range jobs {
		graph[name] = []string{}
		inDegree[name] = 0
	}

	// Build the graph
	for name, job := range jobs {
		for _, req := range job.Requires {
			if req.JobName != "" {
				graph[req.JobName] = append(graph[req.JobName], name)
				inDegree[name]++
			} else if req.Expression != "" {
				// Parse expression to extract job names
				jobNames := extractJobNamesFromExpression(req.Expression)
				for _, depName := range jobNames {
					graph[depName] = append(graph[depName], name)
					inDegree[name]++
				}
			}
		}
	}

	// Topological sort using Kahn's algorithm
	var result []string
	queue := []string{}

	// Find all jobs with no dependencies
	for name, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, neighbor := range graph[current] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(jobs) {
		return nil, fmt.Errorf("unable to create valid job ordering")
	}

	return result, nil
}
