package workflow

import (
	"joblet/internal/joblet/domain"
	"testing"
)

func TestNewDependencyResolver(t *testing.T) {
	dr := NewDependencyResolver()

	if dr == nil {
		t.Fatal("NewDependencyResolver() returned nil")
	}

	if dr.workflows == nil {
		t.Error("workflows map not initialized")
	}

	if dr.jobToWorkflow == nil {
		t.Error("jobToWorkflow map not initialized")
	}

	if dr.jobStateCache == nil {
		t.Error("jobStateCache map not initialized")
	}

	if dr.expressionCache == nil {
		t.Error("expressionCache map not initialized")
	}
}

func TestDependencyResolver_CreateWorkflow(t *testing.T) {
	dr := NewDependencyResolver()

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

	workflowID, err := dr.CreateWorkflow("test-workflow", jobs, order)
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	if workflowID != 1 {
		t.Errorf("CreateWorkflow() workflowID = %d, want 1", workflowID)
	}

	// Verify workflow was created
	dr.mu.RLock()
	workflow, exists := dr.workflows[workflowID]
	dr.mu.RUnlock()

	if !exists {
		t.Fatal("Workflow not found")
	}

	if workflow.Status != WorkflowPending {
		t.Errorf("workflow.Status = %v, want %v", workflow.Status, WorkflowPending)
	}

	if workflow.TotalJobs != 2 {
		t.Errorf("workflow.TotalJobs = %d, want 2", workflow.TotalJobs)
	}
}

func TestDependencyResolver_EvaluateRequirement(t *testing.T) {
	dr := NewDependencyResolver()

	// Set up job state cache
	dr.jobStateCache = map[string]domain.JobStatus{
		"job1": domain.StatusCompleted,
		"job2": domain.StatusFailed,
		"job3": domain.StatusRunning,
	}

	tests := []struct {
		name        string
		requirement Requirement
		expected    bool
	}{
		{
			name: "simple requirement satisfied",
			requirement: Requirement{
				Type:   RequirementSimple,
				JobID:  "job1",
				Status: "COMPLETED",
			},
			expected: true,
		},
		{
			name: "simple requirement not satisfied",
			requirement: Requirement{
				Type:   RequirementSimple,
				JobID:  "job1",
				Status: "FAILED",
			},
			expected: false,
		},
		{
			name: "job not found",
			requirement: Requirement{
				Type:   RequirementSimple,
				JobID:  "nonexistent",
				Status: "COMPLETED",
			},
			expected: false,
		},
		{
			name: "expression requirement satisfied",
			requirement: Requirement{
				Type:       RequirementExpression,
				Expression: "job1=COMPLETED AND job2=FAILED",
			},
			expected: true,
		},
		{
			name: "expression requirement not satisfied",
			requirement: Requirement{
				Type:       RequirementExpression,
				Expression: "job1=COMPLETED AND job2=COMPLETED",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dr.evaluateRequirement(tt.requirement)
			if result != tt.expected {
				t.Errorf("evaluateRequirement() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDependencyResolver_ParseAndEvaluateExpression(t *testing.T) {
	dr := NewDependencyResolver()

	// Set up job state cache
	dr.jobStateCache = map[string]domain.JobStatus{
		"job1": domain.StatusCompleted,
		"job2": domain.StatusFailed,
		"job3": domain.StatusRunning,
	}

	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{
			name:     "simple equality",
			expr:     "job1=COMPLETED",
			expected: true,
		},
		{
			name:     "AND operation",
			expr:     "job1=COMPLETED AND job2=FAILED",
			expected: true,
		},
		{
			name:     "OR operation",
			expr:     "job1=COMPLETED OR job3=COMPLETED",
			expected: true,
		},
		{
			name:     "IN operation",
			expr:     "job1 IN (COMPLETED,FAILED)",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := dr.parseAndEvaluateExpression(tt.expr)
			if result != tt.expected {
				t.Errorf("parseAndEvaluateExpression(%q) = %v, want %v", tt.expr, result, tt.expected)
			}
		})
	}
}

func TestDependencyResolver_EvaluateExpression(t *testing.T) {
	dr := NewDependencyResolver()

	// Set up job state cache
	dr.jobStateCache = map[string]domain.JobStatus{
		"job1": domain.StatusCompleted,
	}

	// Test caching
	expr := "job1=COMPLETED"

	// First call should evaluate and cache
	result1 := dr.evaluateExpression(expr)
	if !result1 {
		t.Error("First evaluation should return true")
	}

	// Second call should use cache
	result2 := dr.evaluateExpression(expr)
	if !result2 {
		t.Error("Second evaluation should return true (from cache)")
	}

	// Check that result is cached
	if cachedResult, exists := dr.expressionCache[expr]; !exists || !cachedResult {
		t.Error("Expression result should be cached")
	}
}

func TestDependencyResolver_OnJobStateChange(t *testing.T) {
	dr := NewDependencyResolver()

	// Create a simple workflow
	jobs := map[string]*JobDependency{
		"job1": {
			JobID:        "job1",
			InternalName: "job1",
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
	}

	workflowID, err := dr.CreateWorkflow("test-workflow", jobs, []string{"job1"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	// Update job state using OnJobStateChange
	dr.OnJobStateChange("job1", domain.StatusRunning)

	// Check that job state was updated in cache (using internal name)
	if state, exists := dr.jobStateCache["job1"]; !exists || state != domain.StatusRunning {
		t.Errorf("Job state not updated correctly: got %v, want %v", state, domain.StatusRunning)
	}

	// Check that workflow job status was updated
	dr.mu.RLock()
	workflow := dr.workflows[workflowID]
	jobDep := workflow.Jobs["job1"]
	dr.mu.RUnlock()

	if jobDep.Status != domain.StatusRunning {
		t.Errorf("Workflow job status not updated: got %v, want %v", jobDep.Status, domain.StatusRunning)
	}
}

func TestDependencyResolver_GetWorkflowStatus(t *testing.T) {
	dr := NewDependencyResolver()

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
			Requirements: []Requirement{},
			Status:       domain.StatusPending,
		},
	}

	workflowID, err := dr.CreateWorkflow("test-workflow", jobs, []string{"job1", "job2"})
	if err != nil {
		t.Fatalf("CreateWorkflow() error = %v", err)
	}

	// Get initial workflow status
	status, err := dr.GetWorkflowStatus(workflowID)
	if err != nil {
		t.Fatalf("GetWorkflowStatus() error = %v", err)
	}
	if status.Status != WorkflowPending {
		t.Errorf("Workflow status should be pending initially, got %v", status.Status)
	}

	// Complete one job
	dr.OnJobStateChange("job1", domain.StatusCompleted)
	status, _ = dr.GetWorkflowStatus(workflowID)
	if status.Status == WorkflowCompleted {
		t.Error("Workflow should not be complete with only one job completed")
	}

	// Complete second job
	dr.OnJobStateChange("job2", domain.StatusCompleted)
	status, _ = dr.GetWorkflowStatus(workflowID)
	if status.Status != WorkflowCompleted {
		t.Error("Workflow should be complete when all jobs are completed")
	}
}
