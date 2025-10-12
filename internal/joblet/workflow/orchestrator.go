package workflow

import (
	"fmt"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// WorkflowManager manages workflows without requiring store changes
type WorkflowManager struct {
	mu              sync.RWMutex
	workflows       map[int]*WorkflowState
	jobToWorkflow   map[string]int
	workflowCounter int
	resolver        *DependencyResolver
}

// NewWorkflowManager creates a new workflow manager
func NewWorkflowManager() *WorkflowManager {
	return &WorkflowManager{
		workflows:     make(map[int]*WorkflowState),
		jobToWorkflow: make(map[string]int),
		resolver:      NewDependencyResolver(),
	}
}

// CreateWorkflow creates a new workflow with the given name, workflow file, and job dependencies.
// Returns the assigned workflow ID and any error that occurred during creation.
// The jobs map contains job IDs mapped to their dependency information.
// The order slice defines the intended execution order for jobs without dependencies.
func (wm *WorkflowManager) CreateWorkflow(workflow string, jobs map[string]*JobDependency, order []string) (int, error) {
	return wm.CreateWorkflowWithYaml(workflow, "", jobs, order)
}

// CreateWorkflowWithYaml creates a new workflow with YAML content for client access.
// This is the preferred method for workflows that need to store original YAML content.
func (wm *WorkflowManager) CreateWorkflowWithYaml(workflow string, yamlContent string, jobs map[string]*JobDependency, order []string) (int, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.workflowCounter++
	workflowID := wm.workflowCounter

	workflowState := &WorkflowState{
		ID:          workflowID,
		Workflow:    workflow,
		YamlContent: yamlContent,
		Jobs:        jobs,
		JobOrder:    order,
		Status:      WorkflowPending,
		CreatedAt:   time.Now(),
		TotalJobs:   len(jobs),
	}

	wm.workflows[workflowID] = workflowState

	// Map jobs to workflow
	for jobID := range jobs {
		wm.jobToWorkflow[jobID] = workflowID
	}

	// Create in resolver
	_, err := wm.resolver.CreateWorkflowWithYaml(workflow, yamlContent, jobs, order)
	return workflowID, err
}

// OnJobStateChange handles job state changes and updates the corresponding workflow.
// This method is called by the job execution system whenever a job status changes.
// It automatically propagates the job status to the dependency resolver and updates
// the workflow's overall status based on completion of its constituent jobs.
func (wm *WorkflowManager) OnJobStateChange(jobID string, newStatus domain.JobStatus) {
	wm.resolver.OnJobStateChange(jobID, newStatus)

	wm.mu.Lock()
	defer wm.mu.Unlock()

	workflowID, exists := wm.jobToWorkflow[jobID]
	if !exists {
		return
	}

	if workflow, exists := wm.workflows[workflowID]; exists {
		// Update job status in workflow
		if job, exists := workflow.Jobs[jobID]; exists {
			job.Status = newStatus
		}

		// Update workflow state
		if updatedWF, err := wm.resolver.GetWorkflowStatus(workflowID); err == nil {
			// Copy updated state
			workflow.Status = updatedWF.Status
			workflow.CompletedJobs = updatedWF.CompletedJobs
			workflow.FailedJobs = updatedWF.FailedJobs
			workflow.StartedAt = updatedWF.StartedAt
			workflow.CompletedAt = updatedWF.CompletedAt
		}
	}
}

// UpdateJobID updates the job ID mapping when a workflow job is started.
//
// RESPONSIBILITY:
// - Maps workflow job names to actual job IDs returned by the joblet service
// - Updates internal data structures to use actual job IDs instead of job names
// - Maintains consistency between workflow manager and dependency resolver mappings
// - Ensures proper job status tracking using real job identifiers
//
// WORKFLOW:
// 1. Validates that the job name exists in the workflow system
// 2. Updates the JobDependency.JobID field from job name to actual job ID
// 3. Remaps workflow.Jobs dictionary from jobName key to actualJobID key
// 4. Updates jobToWorkflow mapping to use actual job ID
// 5. Synchronizes changes with the dependency resolver
//
// PARAMETERS:
// - jobName: Original job name from workflow YAML (e.g., "setup-data", "process-data")
// - actualJobID: Unique job identifier returned by joblet.StartJob (e.g., "42", "43")
//
// RETURNS:
// - error: If job name not found in workflow or update fails
//
// THREAD SAFETY:
// - Uses write lock to ensure atomic updates across all data structures
// - Safe for concurrent access with other workflow manager operations
func (wm *WorkflowManager) UpdateJobID(jobName string, actualJobID string) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// Find the workflow that contains this job name
	workflowID, exists := wm.jobToWorkflow[jobName]
	if !exists {
		return fmt.Errorf("job name %s not found in any workflow", jobName)
	}

	workflow, exists := wm.workflows[workflowID]
	if !exists {
		return fmt.Errorf("workflow %d not found", workflowID)
	}

	// Find the job dependency entry
	jobDep, exists := workflow.Jobs[jobName]
	if !exists {
		return fmt.Errorf("job %s not found in workflow %d", jobName, workflowID)
	}

	// Update the JobID field to the actual job ID
	jobDep.JobID = actualJobID

	// Update workflow.Jobs map: move from jobName key to actualJobID key
	delete(workflow.Jobs, jobName)
	workflow.Jobs[actualJobID] = jobDep

	// Update the jobToWorkflow mapping
	delete(wm.jobToWorkflow, jobName)
	wm.jobToWorkflow[actualJobID] = workflowID

	// Update resolver mappings as well
	wm.resolver.UpdateJobID(jobName, actualJobID)

	return nil
}

// GetReadyJobs returns a list of job IDs that are ready to execute for the given workflow.
// A job is considered ready when all of its dependencies have completed successfully.
// This method is used by the workflow execution engine to determine which jobs to start next.
func (wm *WorkflowManager) GetReadyJobs(workflowID int) []string {
	return wm.resolver.GetReadyJobs(workflowID)
}

// GetWorkflowStatus returns a copy of the current workflow status for the given workflow ID.
// Returns error if the workflow is not found. The returned WorkflowState is a copy to
// prevent race conditions when accessing workflow data from multiple goroutines.
func (wm *WorkflowManager) GetWorkflowStatus(workflowID int) (*WorkflowState, error) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflow, exists := wm.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow %d not found", workflowID)
	}

	// Check for nil before dereferencing
	if workflow == nil {
		return nil, fmt.Errorf("workflow %d is nil", workflowID)
	}

	// Create a copy to avoid race conditions
	copy := *workflow
	return &copy, nil
}

// ListWorkflows returns a list of all workflows managed by this WorkflowManager.
// Each returned WorkflowState is a copy to prevent external modifications to internal state.
// The list includes workflows in all states (pending, running, completed, failed, canceled).
func (wm *WorkflowManager) ListWorkflows() []*WorkflowState {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	var result []*WorkflowState
	for _, wf := range wm.workflows {
		if wf == nil {
			continue
		}
		copy := *wf
		result = append(result, &copy)
	}

	return result
}

// GetJobWorkflow returns the workflow ID that contains the given job.
// Returns the workflow ID and true if the job is part of a workflow,
// or 0 and false if the job is not associated with any workflow.
func (wm *WorkflowManager) GetJobWorkflow(jobID string) (int, bool) {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	workflowID, exists := wm.jobToWorkflow[jobID]
	return workflowID, exists
}

// IsJobPartOfWorkflow checks if the given job ID belongs to any workflow.
// This is used to determine whether job status changes should trigger workflow updates.
func (wm *WorkflowManager) IsJobPartOfWorkflow(jobID string) bool {
	_, exists := wm.GetJobWorkflow(jobID)
	return exists
}
