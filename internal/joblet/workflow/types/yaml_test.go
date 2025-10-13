package types

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// Simple test for WorkflowYAML parsing
func TestWorkflowYAML_Parse(t *testing.T) {
	yamlData := `
name: "test-workflow"
description: "A simple test workflow"
jobs:
  job1:
    command: "echo"
    args: ["hello"]
    runtime: "ubuntu"
    environment:
      NODE_ENV: "production"
`

	var workflow WorkflowYAML
	err := yaml.Unmarshal([]byte(yamlData), &workflow)
	if err != nil {
		t.Fatalf("Failed to unmarshal YAML: %v", err)
	}

	if workflow.Name != "test-workflow" {
		t.Errorf("Name = %q, want %q", workflow.Name, "test-workflow")
	}

	if workflow.Description != "A simple test workflow" {
		t.Errorf("Description = %q, want %q", workflow.Description, "A simple test workflow")
	}

	if len(workflow.Jobs) != 1 {
		t.Errorf("len(Jobs) = %d, want 1", len(workflow.Jobs))
	}

	job1, exists := workflow.Jobs["job1"]
	if !exists {
		t.Fatal("job1 not found in workflow")
	}

	if job1.Command != "echo" {
		t.Errorf("job1.Command = %q, want %q", job1.Command, "echo")
	}

	if len(job1.Args) != 1 || job1.Args[0] != "hello" {
		t.Errorf("job1.Args = %v, want [hello]", job1.Args)
	}

	if job1.Environment["NODE_ENV"] != "production" {
		t.Errorf("job1.Environment[NODE_ENV] = %q, want %q", job1.Environment["NODE_ENV"], "production")
	}
}

func TestJobSpec_UnmarshalYAML(t *testing.T) {
	yamlData := `
command: "python3"
args: ["script.py", "--input", "data.csv"]
runtime: "python-3.11-ml"
resources:
  max_memory: 2048
  max_cpu: 200
volumes: ["data-vol"]
network: "backend"
requires:
  - job1: "COMPLETED"
  - expression: "job2=COMPLETED AND job3=FAILED"
environment:
  NODE_ENV: "production"
  DEBUG: "false"
  SECRET_API_KEY: "secret123"
uploads:
  files:
    - "script.py"
    - "config.json"
`

	var jobSpec JobSpec
	err := yaml.Unmarshal([]byte(yamlData), &jobSpec)
	if err != nil {
		t.Fatalf("UnmarshalYAML() error = %v", err)
	}

	// Test basic fields
	if jobSpec.Command != "python3" {
		t.Errorf("Command = %q, want %q", jobSpec.Command, "python3")
	}

	if len(jobSpec.Args) != 3 {
		t.Errorf("len(Args) = %d, want 3", len(jobSpec.Args))
	}

	if jobSpec.Runtime != "python-3.11-ml" {
		t.Errorf("Runtime = %q, want %q", jobSpec.Runtime, "python-3.11-ml")
	}

	// Test resources
	if jobSpec.Resources.MaxMemory != 2048 {
		t.Errorf("Resources.MaxMemory = %d, want 2048", jobSpec.Resources.MaxMemory)
	}

	if jobSpec.Resources.MaxCPU != 200 {
		t.Errorf("Resources.MaxCPU = %d, want 200", jobSpec.Resources.MaxCPU)
	}

	// Test arrays
	if len(jobSpec.Volumes) != 1 || jobSpec.Volumes[0] != "data-vol" {
		t.Errorf("Volumes = %v, want [data-vol]", jobSpec.Volumes)
	}

	if jobSpec.Network != "backend" {
		t.Errorf("Network = %q, want %q", jobSpec.Network, "backend")
	}

	// Test requirements (simplified - just check they exist)
	if len(jobSpec.Requires) != 2 {
		t.Errorf("len(Requires) = %d, want 2", len(jobSpec.Requires))
	}

	// Test environment variables (including secrets using naming convention)
	if jobSpec.Environment["NODE_ENV"] != "production" {
		t.Errorf("Environment[NODE_ENV] = %q, want production", jobSpec.Environment["NODE_ENV"])
	}

	if jobSpec.Environment["SECRET_API_KEY"] != "secret123" {
		t.Errorf("Environment[SECRET_API_KEY] = %q, want secret123", jobSpec.Environment["SECRET_API_KEY"])
	}

	// Test upload files
	if jobSpec.Uploads == nil || len(jobSpec.Uploads.Files) != 2 {
		t.Errorf("len(Uploads.Files) = %d, want 2", len(jobSpec.Uploads.Files))
	}
}

func TestWorkflowYAML_UnmarshalYAML(t *testing.T) {
	yamlData := `
jobs:
  data-extraction:
    command: "python3"
    args: ["extract.py"]
    runtime: "python-3.11-ml"
    resources:
      max_memory: 2048
      max_cpu: 100

  model-training:
    command: "python3"
    args: ["train.py"]
    runtime: "python-3.11-ml" 
    requires:
      - data-extraction: "COMPLETED"
    resources:
      max_memory: 8192
      max_cpu: 400

  deployment:
    command: "python3"
    args: ["deploy.py"]
    requires:
      - expression: "data-extraction=COMPLETED AND model-training=COMPLETED"
`

	var workflow WorkflowYAML
	err := yaml.Unmarshal([]byte(yamlData), &workflow)
	if err != nil {
		t.Fatalf("UnmarshalYAML() error = %v", err)
	}

	// Test that all jobs were parsed
	if len(workflow.Jobs) != 3 {
		t.Errorf("len(workflow.Jobs) = %d, want 3", len(workflow.Jobs))
	}

	// Test data-extraction job
	dataJob, exists := workflow.Jobs["data-extraction"]
	if !exists {
		t.Fatal("data-extraction job not found")
	}

	if dataJob.Command != "python3" {
		t.Errorf("data-extraction.Command = %q, want python3", dataJob.Command)
	}

	if len(dataJob.Args) != 1 || dataJob.Args[0] != "extract.py" {
		t.Errorf("data-extraction.Args = %v, want [extract.py]", dataJob.Args)
	}

	// Test model-training job with dependency
	modelJob, exists := workflow.Jobs["model-training"]
	if !exists {
		t.Fatal("model-training job not found")
	}

	if len(modelJob.Requires) != 1 {
		t.Errorf("len(model-training.Requires) = %d, want 1", len(modelJob.Requires))
	}

	// Test deployment job with expression requirement
	deployJob, exists := workflow.Jobs["deployment"]
	if !exists {
		t.Fatal("deployment job not found")
	}

	if len(deployJob.Requires) != 1 {
		t.Errorf("len(deployment.Requires) = %d, want 1", len(deployJob.Requires))
	}
}
