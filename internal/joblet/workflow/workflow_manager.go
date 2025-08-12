package workflow

import (
	"fmt"
	"sync"
	"time"

	"joblet/internal/joblet/domain"
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

// CreateWorkflow creates a new workflow with the given name, template, and job dependencies.
// Returns the assigned workflow ID and any error that occurred during creation.
// The jobs map contains job IDs mapped to their dependency information.
// The order slice defines the intended execution order for jobs without dependencies.
func (wm *WorkflowManager) CreateWorkflow(name, template string, jobs map[string]*JobDependency, order []string) (int, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wm.workflowCounter++
	workflowID := wm.workflowCounter

	workflow := &WorkflowState{
		ID:        workflowID,
		Name:      name,
		Template:  template,
		Jobs:      jobs,
		JobOrder:  order,
		Status:    WorkflowPending,
		CreatedAt: time.Now(),
		TotalJobs: len(jobs),
	}

	wm.workflows[workflowID] = workflow

	// Map jobs to workflow
	for jobID := range jobs {
		wm.jobToWorkflow[jobID] = workflowID
	}

	// Create in resolver
	_, err := wm.resolver.CreateWorkflow(name, template, jobs, order)
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
		copy := *wf
		result = append(result, &copy)
	}

	return result
}

// CancelWorkflow cancels a workflow and all of its pending jobs.
// Already running jobs will continue to completion, but no new jobs will be started.
// Returns error if the workflow is not found or if cancellation fails.
func (wm *WorkflowManager) CancelWorkflow(workflowID int) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	wf, exists := wm.workflows[workflowID]
	if !exists {
		return fmt.Errorf("workflow %d not found", workflowID)
	}

	// Cancel through resolver
	err := wm.resolver.CancelWorkflow(workflowID)
	if err != nil {
		return err
	}

	// Update local state
	wf.Status = WorkflowCanceled
	if wf.CompletedAt == nil {
		now := time.Now()
		wf.CompletedAt = &now
	}

	return nil
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

// Deprecated: GetGlobalWorkflowManager - use dependency injection instead
// This function exists only for backward compatibility and will be removed
var globalWorkflowManager *WorkflowManager
var workflowManagerOnce sync.Once

func GetGlobalWorkflowManager() *WorkflowManager {
	workflowManagerOnce.Do(func() {
		globalWorkflowManager = NewWorkflowManager()
	})
	return globalWorkflowManager
}
