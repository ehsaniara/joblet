package workflow

import (
	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"testing"
)

func TestNewWorkflowManager(t *testing.T) {
	wm := NewWorkflowManager()

	if wm == nil {
		t.Fatal("NewWorkflowManager() returned nil")
	}

	if wm.workflows == nil {
		t.Error("workflows map not initialized")
	}

	if wm.jobToWorkflow == nil {
		t.Error("jobToWorkflow map not initialized")
	}

	if wm.resolver == nil {
		t.Error("resolver not initialized")
	}

	if wm.workflowCounter != 0 {
		t.Errorf("workflowCounter = %d, want 0", wm.workflowCounter)
	}
}

func TestWorkflowManager_CreateWorkflow(t *testing.T) {
	wm := NewWorkflowManager()

	// Test data
	jobs := map[string]*JobDependency{
		"job1": {
			JobID:        "job1",
			InternalName: "job1",
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
		"job2": {
			JobID:        "job2",
			InternalName: "job2",
			Requirements: []Requirement{
				{
					Type:   RequirementSimple,
					JobID:  "job1",
					Status: "COMPLETED",
				},
			},
			Status: domain.StatusPending,
		},
	}
	order := []string{"job1", "job2"}

	// Create workflow
	workflowID, err := wm.CreateWorkflow("test-workflow", jobs, order)
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	if workflowID != 1 {
		t.Errorf("CreateWorkflow() workflowID = %d, want 1", workflowID)
	}

	// Check workflow was stored
	wm.mu.RLock()
	workflow, exists := wm.workflows[workflowID]
	wm.mu.RUnlock()

	if !exists {
		t.Fatal("Workflow not found in workflows map")
	}

	if workflow.ID != workflowID {
		t.Errorf("workflow.ID = %d, want %d", workflow.ID, workflowID)
	}

	if workflow.Workflow != "test-workflow" {
		t.Errorf("workflow.Workflow = %q, want %q", workflow.Workflow, "test-workflow")
	}

	if len(workflow.Jobs) != 2 {
		t.Errorf("len(workflow.Jobs) = %d, want 2", len(workflow.Jobs))
	}
}

func TestWorkflowManager_CreateWorkflowWithYaml(t *testing.T) {
	wm := NewWorkflowManager()

	jobs := map[string]*JobDependency{
		"job1": {
			JobID:        "job1",
			InternalName: "job1",
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
	}
	order := []string{"job1"}
	yamlContent := "jobs:\n  job1:\n    command: echo test"

	workflowID, err := wm.CreateWorkflowWithYaml("test-workflow", yamlContent, jobs, order)
	if err != nil {
		t.Fatalf("CreateWorkflowWithYaml() error = %v", err)
	}

	// Check YAML content was stored
	wm.mu.RLock()
	workflow, exists := wm.workflows[workflowID]
	wm.mu.RUnlock()

	if !exists {
		t.Fatal("Workflow not found")
	}

	if workflow.YamlContent != yamlContent {
		t.Errorf("workflow.YamlContent = %q, want %q", workflow.YamlContent, yamlContent)
	}
}

func TestWorkflowManager_UpdateJobID(t *testing.T) {
	wm := NewWorkflowManager()

	jobs := map[string]*JobDependency{
		"job1": {
			JobID:        "job1",
			InternalName: "job1",
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
	}

	workflowID, err := wm.CreateWorkflow("test-workflow", jobs, []string{"job1"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	// Update job ID
	err = wm.UpdateJobID("job1", "actual-job-123")
	if err != nil {
		t.Fatalf("UpdateJobID() error = %v", err)
	}

	// Check job-to-workflow mapping
	wm.mu.RLock()
	mappedWorkflowID, exists := wm.jobToWorkflow["actual-job-123"]
	wm.mu.RUnlock()

	if !exists {
		t.Fatal("Job ID not found in jobToWorkflow mapping")
	}

	if mappedWorkflowID != workflowID {
		t.Errorf("jobToWorkflow[actual-job-123] = %d, want %d", mappedWorkflowID, workflowID)
	}
}

func TestWorkflowManager_GetWorkflowStatus(t *testing.T) {
	wm := NewWorkflowManager()

	jobs := map[string]*JobDependency{
		"job1": {
			JobID:        "job1",
			InternalName: "job1",
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
	}

	workflowID, err := wm.CreateWorkflow("test-workflow", jobs, []string{"job1"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	// Get workflow status
	status, err := wm.GetWorkflowStatus(workflowID)
	if err != nil {
		t.Fatalf("GetWorkflowStatus() error = %v", err)
	}

	if status == nil {
		t.Fatal("GetWorkflowStatus() returned nil")
	}

	if status.ID != workflowID {
		t.Errorf("status.ID = %d, want %d", status.ID, workflowID)
	}

	// Test non-existent workflow
	_, err = wm.GetWorkflowStatus(999)
	if err == nil {
		t.Error("GetWorkflowStatus(999) should return error for non-existent workflow")
	}
}

func TestWorkflowManager_IsJobPartOfWorkflow(t *testing.T) {
	wm := NewWorkflowManager()

	jobs := map[string]*JobDependency{
		"job1": {
			JobID:        "job1",
			InternalName: "job1",
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
	}

	_, err := wm.CreateWorkflow("test-workflow", jobs, []string{"job1"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	err = wm.UpdateJobID("job1", "actual-job-123")
	if err != nil {
		t.Fatalf("UpdateJobID() error = %v", err)
	}

	// Test job is part of workflow
	isPartOf := wm.IsJobPartOfWorkflow("actual-job-123")
	if !isPartOf {
		t.Error("IsJobPartOfWorkflow(actual-job-123) = false, want true")
	}

	// Test job is not part of workflow
	isPartOf = wm.IsJobPartOfWorkflow("non-existent-job")
	if isPartOf {
		t.Error("IsJobPartOfWorkflow(non-existent-job) = true, want false")
	}
}

func TestWorkflowManager_ListWorkflows(t *testing.T) {
	wm := NewWorkflowManager()

	// Initially should be empty
	workflows := wm.ListWorkflows()
	if len(workflows) != 0 {
		t.Errorf("len(ListWorkflows()) = %d, want 0", len(workflows))
	}

	// Add some workflows
	jobs := map[string]*JobDependency{
		"job1": {
			JobID:        "job1",
			InternalName: "job1",
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
	}

	_, err := wm.CreateWorkflow("workflow1", jobs, []string{"job1"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	_, err = wm.CreateWorkflow("workflow2", jobs, []string{"job1"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	workflows = wm.ListWorkflows()
	if len(workflows) != 2 {
		t.Errorf("len(ListWorkflows()) = %d, want 2", len(workflows))
	}
}
