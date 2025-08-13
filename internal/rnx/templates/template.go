package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// JobConfig represents a job configuration from YAML
type JobConfig struct {
	Description string            `yaml:"description"`
	Command     string            `yaml:"command"`
	Args        []string          `yaml:"args"`
	Resources   ResourceConfig    `yaml:"resources"`
	Uploads     UploadConfig      `yaml:"uploads"`
	Volumes     []string          `yaml:"volumes"`
	Network     string            `yaml:"network"`
	Runtime     string            `yaml:"runtime"`
	Schedule    string            `yaml:"schedule"`
	Environment map[string]string `yaml:"environment"`
	WorkDir     string            `yaml:"workdir"`
	Extends     string            `yaml:"extends"`
}

// ResourceConfig defines resource limits
type ResourceConfig struct {
	MaxCPU    int    `yaml:"max_cpu"`
	MaxMemory int    `yaml:"max_memory"`
	MaxIOBPS  int    `yaml:"max_iobps"`
	CPUCores  string `yaml:"cpu_cores"`
}

// UploadConfig defines file uploads
type UploadConfig struct {
	Files       []string `yaml:"files"`
	Directories []string `yaml:"directories"`
}

// JobSet represents a collection of job configurations
type JobSet struct {
	Version  string               `yaml:"version"`
	Defaults JobConfig            `yaml:"defaults"`
	Jobs     map[string]JobConfig `yaml:"jobs"`
}

// LoadConfig loads a YAML configuration file
func LoadConfig(path string) (*JobSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config JobSet
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Apply defaults and inheritance
	for name, job := range config.Jobs {
		if job.Extends != "" {
			if parent, exists := config.Jobs[job.Extends]; exists {
				job = mergeConfigs(parent, job)
			}
		}
		job = mergeConfigs(config.Defaults, job)
		config.Jobs[name] = job
	}

	return &config, nil
}

// LoadJobConfig loads a single job configuration
func LoadJobConfig(path string, jobName string) (*JobConfig, error) {
	jobSet, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}

	job, exists := jobSet.Jobs[jobName]
	if !exists {
		return nil, fmt.Errorf("job '%s' not found in configuration", jobName)
	}

	return &job, nil
}

// BuildCommand builds the rnx command from a job configuration
func (j *JobConfig) BuildCommand() []string {
	var cmd []string
	cmd = append(cmd, "rnx", "run")

	// Add resource limits
	if j.Resources.MaxCPU > 0 {
		cmd = append(cmd, fmt.Sprintf("--max-cpu=%d", j.Resources.MaxCPU))
	}
	if j.Resources.MaxMemory > 0 {
		cmd = append(cmd, fmt.Sprintf("--max-memory=%d", j.Resources.MaxMemory))
	}
	if j.Resources.MaxIOBPS > 0 {
		cmd = append(cmd, fmt.Sprintf("--max-iobps=%d", j.Resources.MaxIOBPS))
	}
	if j.Resources.CPUCores != "" {
		cmd = append(cmd, fmt.Sprintf("--cpu-cores=%s", j.Resources.CPUCores))
	}

	// Add uploads
	for _, file := range j.Uploads.Files {
		cmd = append(cmd, fmt.Sprintf("--upload=%s", file))
	}
	for _, dir := range j.Uploads.Directories {
		cmd = append(cmd, fmt.Sprintf("--upload-dir=%s", dir))
	}

	// Add volumes
	for _, volume := range j.Volumes {
		cmd = append(cmd, fmt.Sprintf("--volume=%s", volume))
	}

	// Add network
	if j.Network != "" {
		cmd = append(cmd, fmt.Sprintf("--network=%s", j.Network))
	}

	// Add runtime
	if j.Runtime != "" {
		cmd = append(cmd, fmt.Sprintf("--runtime=%s", j.Runtime))
	}

	// Add schedule
	if j.Schedule != "" {
		cmd = append(cmd, fmt.Sprintf("--schedule=%s", j.Schedule))
	}

	// Add command and args
	if j.Command != "" {
		cmd = append(cmd, j.Command)
	}
	cmd = append(cmd, j.Args...)

	return cmd
}

// GetCommandString returns the command as a single string
func (j *JobConfig) GetCommandString() string {
	parts := j.BuildCommand()
	// Quote parts that contain spaces
	for i, part := range parts {
		if strings.Contains(part, " ") && !strings.HasPrefix(part, "--") {
			parts[i] = fmt.Sprintf("\"%s\"", part)
		}
	}
	return strings.Join(parts, " ")
}

// mergeConfigs merges two job configurations, with child overriding parent
func mergeConfigs(parent, child JobConfig) JobConfig {
	result := parent

	if child.Description != "" {
		result.Description = child.Description
	}
	if child.Command != "" {
		result.Command = child.Command
	}
	if len(child.Args) > 0 {
		result.Args = child.Args
	}
	if child.Resources.MaxCPU > 0 {
		result.Resources.MaxCPU = child.Resources.MaxCPU
	}
	if child.Resources.MaxMemory > 0 {
		result.Resources.MaxMemory = child.Resources.MaxMemory
	}
	if child.Resources.MaxIOBPS > 0 {
		result.Resources.MaxIOBPS = child.Resources.MaxIOBPS
	}
	if child.Resources.CPUCores != "" {
		result.Resources.CPUCores = child.Resources.CPUCores
	}
	if len(child.Uploads.Files) > 0 {
		result.Uploads.Files = append(result.Uploads.Files, child.Uploads.Files...)
	}
	if len(child.Uploads.Directories) > 0 {
		result.Uploads.Directories = append(result.Uploads.Directories, child.Uploads.Directories...)
	}
	if len(child.Volumes) > 0 {
		result.Volumes = append(result.Volumes, child.Volumes...)
	}
	if child.Network != "" {
		result.Network = child.Network
	}
	if child.Runtime != "" {
		result.Runtime = child.Runtime
	}
	if child.Schedule != "" {
		result.Schedule = child.Schedule
	}
	if child.WorkDir != "" {
		result.WorkDir = child.WorkDir
	}
	if len(child.Environment) > 0 {
		if result.Environment == nil {
			result.Environment = make(map[string]string)
		}
		for k, v := range child.Environment {
			result.Environment[k] = v
		}
	}

	return result
}

// FindConfigFile searches for a config file in current and parent directories
func FindConfigFile(startPath string) (string, error) {
	configNames := []string{"joblet.yaml", "joblet.yml", ".joblet.yaml", ".joblet.yml"}

	currentPath := startPath
	for {
		for _, name := range configNames {
			configPath := filepath.Join(currentPath, name)
			if _, err := os.Stat(configPath); err == nil {
				return configPath, nil
			}
		}

		parent := filepath.Dir(currentPath)
		if parent == currentPath {
			break
		}
		currentPath = parent
	}

	return "", fmt.Errorf("no joblet configuration file found")
}
