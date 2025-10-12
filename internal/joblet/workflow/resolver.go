// Package workflow provides workflow management, dependency resolution, and expression parsing
// for job orchestration in the Joblet system.
package workflow

import (
	"fmt"
	"sync"
	"time"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
)

// DependencyResolver manages job dependencies and workflow execution
type DependencyResolver struct {
	mu              sync.RWMutex
	workflows       map[int]*WorkflowState
	jobToWorkflow   map[string]int
	workflowCounter int
	jobStateCache   map[string]domain.JobStatus
	expressionCache map[string]bool
	eventChan       chan JobStateEvent
}

// WorkflowState tracks the state of a workflow
type WorkflowState struct {
	ID            int
	Workflow      string // Workflow file path/name
	YamlContent   string // Original YAML content for client access
	Jobs          map[string]*JobDependency
	JobOrder      []string
	Status        WorkflowStatus
	CreatedAt     time.Time
	StartedAt     *time.Time
	CompletedAt   *time.Time
	TotalJobs     int
	CompletedJobs int
	FailedJobs    int
	CanceledJobs  int
}

// JobDependency tracks dependencies for a single job
type JobDependency struct {
	JobID        string
	InternalName string
	Requirements []Requirement
	Status       domain.JobStatus
	CanStart     bool
	Impossible   bool
}

// Requirement represents a job dependency requirement
type Requirement struct {
	Type       RequirementType
	Expression string
	JobID      string
	Status     string
}

// RequirementType defines the type of requirement
type RequirementType int

const (
	RequirementSimple RequirementType = iota
	RequirementExpression
)

// WorkflowStatus represents workflow states
type WorkflowStatus string

const (
	WorkflowPending   WorkflowStatus = "PENDING"
	WorkflowRunning   WorkflowStatus = "RUNNING"
	WorkflowCompleted WorkflowStatus = "COMPLETED"
	WorkflowFailed    WorkflowStatus = "FAILED"
	WorkflowCanceled  WorkflowStatus = "CANCELED"
	WorkflowStopped   WorkflowStatus = "STOPPED"
)

// JobStateEvent represents a job state change event
type JobStateEvent struct {
	JobID     string
	NewStatus domain.JobStatus
	Timestamp time.Time
}

// NewDependencyResolver creates a new dependency resolver for managing workflow orchestration.
// The resolver handles job dependencies, tracks workflow states, and manages job execution order.
// It maintains internal caches for performance and provides thread-safe access to workflow data.
// Used by the WorkflowManager to coordinate job execution based on dependency requirements.
func NewDependencyResolver() *DependencyResolver {
	return &DependencyResolver{
		workflows:       make(map[int]*WorkflowState),
		jobToWorkflow:   make(map[string]int),
		jobStateCache:   make(map[string]domain.JobStatus),
		expressionCache: make(map[string]bool),
		eventChan:       make(chan JobStateEvent, 1000),
	}
}

// CreateWorkflow creates a new workflow with the specified job dependencies and execution order.
// This method performs several key operations:
// 1. Creates a new workflow state with unique ID and metadata
// 2. Maps internal job names to actual job IDs for dependency resolution
// 3. Converts dependency expressions to use actual job IDs instead of internal names
// 4. Evaluates initial job readiness based on dependency requirements
// 5. Sets up job-to-workflow mappings for efficient lookups
// Returns the assigned workflow ID for tracking and monitoring purposes.
func (dr *DependencyResolver) CreateWorkflow(workflow string, jobs map[string]*JobDependency, order []string) (int, error) {
	return dr.CreateWorkflowWithYaml(workflow, "", jobs, order)
}

// CreateWorkflowWithYaml creates a new workflow with YAML content support.
func (dr *DependencyResolver) CreateWorkflowWithYaml(workflow string, yamlContent string, jobs map[string]*JobDependency, order []string) (int, error) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.workflowCounter++
	workflowID := dr.workflowCounter

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

	dr.workflows[workflowID] = workflowState

	// Create mapping between internal names and job IDs
	internalNameToJobID := make(map[string]string)

	// Map jobs to workflow and create name mappings
	for jobID, job := range jobs {
		dr.jobToWorkflow[jobID] = workflowID
		internalNameToJobID[job.InternalName] = jobID
	}

	// Keep requirements using job names for consistency
	// Job names are stable throughout the workflow lifecycle
	// This enables proper dependency validation and circular detection

	// Check which jobs can start immediately
	for _, job := range jobs {
		if dr.canJobStart(job) {
			job.CanStart = true
		}
	}

	return workflowID, nil
}

// UpdateJobID updates the job ID mapping in the dependency resolver.
//
// RESPONSIBILITY:
// - Synchronizes job ID mappings with the workflow manager updates
// - Updates all internal tracking structures to use actual job IDs
// - Maintains dependency resolution consistency during job execution
// - Ensures job state cache integrity across ID transitions
//
// WORKFLOW:
// 1. Updates jobToWorkflow mapping from job name to actual job ID
// 2. Migrates job state cache entries from job name to actual job ID
// 3. Updates JobDependency.JobID field in workflow structures
// 4. Maintains referential integrity across all internal mappings
//
// PARAMETERS:
// - jobName: Original job name from workflow YAML (e.g., "setup-data")
// - actualJobID: Unique job identifier returned by joblet service (e.g., "42")
//
// THREAD SAFETY:
// - Uses write lock to ensure atomic updates
// - Prevents race conditions during concurrent job state changes
//
// INTEGRATION:
// - Called by WorkflowManager.UpdateJobID to maintain consistency
// - Ensures dependency resolution works with actual job IDs
func (dr *DependencyResolver) UpdateJobID(jobName string, actualJobID string) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Update jobToWorkflow mapping
	if workflowID, exists := dr.jobToWorkflow[jobName]; exists {
		delete(dr.jobToWorkflow, jobName)
		dr.jobToWorkflow[actualJobID] = workflowID
	}

	// Keep jobStateCache keyed by job names, not job IDs
	// This ensures requirements can always find their dependencies by name
	// The cache already uses jobName as key, so no changes needed here

	// Update workflow Jobs map and requirements
	for _, workflow := range dr.workflows {
		if jobDep, exists := workflow.Jobs[jobName]; exists {
			// Update the JobID field
			jobDep.JobID = actualJobID
			// Move the entry from jobName key to actualJobID key
			delete(workflow.Jobs, jobName)
			workflow.Jobs[actualJobID] = jobDep

			// Keep requirements using job names for consistency
			// No need to update requirements since they should always reference job names
			break
		}
	}
}

// OnJobStateChange processes job status updates and cascades dependency effects.
// This is the core orchestration method that:
// 1. Updates the job state cache for dependency evaluation
// 2. Updates workflow counters (completed, failed, canceled jobs)
// 3. Handles terminal job states (completed, failed, canceled, stopped)
// 4. Recalculates job readiness for dependent jobs
// 5. Updates overall workflow status based on constituent job states
// 6. Marks jobs as impossible if their dependencies can never be satisfied
// Called by the workflow execution system whenever a job status changes.
func (dr *DependencyResolver) OnJobStateChange(jobID string, newStatus domain.JobStatus) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	// Find workflow
	workflowID, exists := dr.jobToWorkflow[jobID]
	if !exists {
		return // Not part of a workflow
	}

	workflow := dr.workflows[workflowID]
	if workflow == nil {
		return
	}

	// Update job status and cache using job name for consistency
	if job, exists := workflow.Jobs[jobID]; exists {
		// For workflow jobs, update cache using job name so requirements can find it
		// Individual jobs don't have names or dependencies, so they don't need cache entries
		if job.InternalName != "" {
			dr.jobStateCache[job.InternalName] = newStatus
		}
		oldStatus := job.Status
		job.Status = newStatus

		// Update workflow counters
		dr.updateWorkflowCounters(workflow, oldStatus, newStatus)

		// Handle terminal states
		if isTerminalState(newStatus) {
			dr.handleTerminalState(workflow, jobID, newStatus)
		}

		// Update workflow status
		dr.updateWorkflowStatus(workflow)
	}
}

// GetReadyJobs returns a list of job IDs that are ready for execution.
// A job is considered ready when:
// 1. It is in PENDING status (not yet started)
// 2. All of its dependency requirements are satisfied
// 3. It is not marked as impossible due to failed dependencies
// 4. Its CanStart flag is set to true
// This method is called by the workflow orchestration system to determine
// which jobs should be started in the next execution cycle.
func (dr *DependencyResolver) GetReadyJobs(workflowID int) []string {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	workflow := dr.workflows[workflowID]
	if workflow == nil {
		return nil
	}

	var ready []string
	for jobID, job := range workflow.Jobs {
		if job.Status == domain.StatusPending {
			if job.Impossible {
				// Job is marked as impossible, skip it
				continue
			} else if !job.CanStart {
				// Check if job can start now
				canStart := dr.canJobStart(job)
				if canStart {
					job.CanStart = true // Update the flag
					ready = append(ready, jobID)
				}
			} else {
				ready = append(ready, jobID)
			}
		}
		// Skip jobs that are not pending
	}

	return ready
}

// GetWorkflowStatus retrieves the current state of a workflow including all job statuses.
// Returns a copy of the WorkflowState to prevent race conditions during concurrent access.
// The returned state includes:
// - Workflow metadata (ID, workflow file, creation time)
// - Job dependency information and current statuses
// - Execution statistics (completed, failed, canceled job counts)
// - Overall workflow status and timing information
// Used by monitoring systems and CLI commands to display workflow progress.
func (dr *DependencyResolver) GetWorkflowStatus(workflowID int) (*WorkflowState, error) {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	workflow, exists := dr.workflows[workflowID]
	if !exists {
		return nil, fmt.Errorf("workflow %d not found", workflowID)
	}

	// Create a copy to avoid race conditions
	copy := *workflow
	return &copy, nil
}

// Private helper functions

// canJobStart evaluates whether a job's dependency requirements are satisfied.
// Returns true if all requirements are met, false otherwise.
// A job with no requirements can always start immediately.
func (dr *DependencyResolver) canJobStart(job *JobDependency) bool {
	if len(job.Requirements) == 0 {
		return true
	}

	for _, req := range job.Requirements {
		satisfied := dr.evaluateRequirement(req)
		if !satisfied {
			return false
		}
	}

	return true
}

// evaluateRequirement checks if a single dependency requirement is satisfied.
// Handles both simple requirements (job=status) and complex expression requirements.
// Uses the job state cache to check current job statuses for evaluation.
func (dr *DependencyResolver) evaluateRequirement(req Requirement) bool {
	switch req.Type {
	case RequirementSimple:
		status, exists := dr.jobStateCache[req.JobID]
		if !exists {
			return false
		}
		return string(status) == req.Status

	case RequirementExpression:
		return dr.evaluateExpression(req.Expression)

	default:
		return false
	}
}

// evaluateExpression evaluates complex dependency expressions with caching.
// Checks the expression cache first for performance, then parses and evaluates.
// Caches results to avoid re-parsing identical expressions multiple times.
func (dr *DependencyResolver) evaluateExpression(expr string) bool {
	// Check cache first
	if result, exists := dr.expressionCache[expr]; exists {
		return result
	}

	// Parse and evaluate expression
	result := dr.parseAndEvaluateExpression(expr)

	// Cache result
	dr.expressionCache[expr] = result

	return result
}

// parseAndEvaluateExpression evaluates dependency expressions using simple parsing.
// Supports boolean operations (AND, OR), parentheses, simple comparisons (=),
// and IN expressions. Uses the simplified expression evaluator.
func (dr *DependencyResolver) parseAndEvaluateExpression(expr string) bool {
	evaluator := NewSimpleExpressionEvaluator(dr.jobStateCache)
	return evaluator.Evaluate(expr)
}

// handleTerminalState processes the cascade effects when a job reaches a terminal state.
// When a job completes, fails, or is canceled, this method:
// 1. Checks all other jobs in the workflow for new readiness
// 2. Marks jobs as impossible if their dependencies can never be satisfied
// 3. Recursively handles cancellation chains when dependencies fail
// This ensures workflow execution continues optimally after each job completion.
func (dr *DependencyResolver) handleTerminalState(workflow *WorkflowState, jobID string, status domain.JobStatus) {
	// Check all jobs in workflow to see if any can now start or should be canceled
	for otherJobID, otherJob := range workflow.Jobs {
		if otherJobID == jobID || otherJob.Impossible {
			continue
		}

		// Skip jobs that are not pending
		if otherJob.Status != domain.StatusPending {
			continue
		}

		// Check if this job's requirements are now impossible
		requirementImpossible := false
		for _, req := range otherJob.Requirements {
			if dr.isRequirementImpossible(req, workflow) {
				requirementImpossible = true
				break
			}
		}

		if requirementImpossible {
			otherJob.Impossible = true
			otherJob.Status = domain.StatusCanceled
			// Update cache using job name for consistency
			if otherJob.InternalName != "" {
				dr.jobStateCache[otherJob.InternalName] = domain.StatusCanceled
			}
			workflow.CanceledJobs++
			// Recursively handle this cancellation
			dr.handleTerminalState(workflow, otherJobID, domain.StatusCanceled)
		} else {
			// Check if this job can now start (requirements are satisfied)
			if dr.canJobStart(otherJob) {
				otherJob.CanStart = true
			}
		}
	}
}

// isRequirementImpossible determines if a requirement can never be satisfied.
// A requirement becomes impossible when the target job is in a terminal state
// that doesn't match the required status (e.g., requiring COMPLETED but job FAILED).
// Used to prevent waiting indefinitely for dependencies that can never be met.
func (dr *DependencyResolver) isRequirementImpossible(req Requirement, workflow *WorkflowState) bool {
	switch req.Type {
	case RequirementSimple:
		// Find job by internal name in workflow
		var targetJob *JobDependency
		for _, job := range workflow.Jobs {
			if job.InternalName == req.JobID {
				targetJob = job
				break
			}
		}
		if targetJob == nil {
			return true
		}

		// If the job is in a terminal state and doesn't match the requirement
		if isTerminalState(targetJob.Status) && string(targetJob.Status) != req.Status {
			return true
		}

		return false

	case RequirementExpression:
		// Expression requirements use complex logic evaluation
		// Currently simplified - full implementation would parse and analyze expressions
		return false

	default:
		return false
	}
}

// updateWorkflowCounters maintains accurate job count statistics for workflow monitoring.
// Adjusts counters when job statuses change, ensuring completed/failed/canceled counts
// remain accurate. Also sets the workflow start time when the first job begins running.
func (dr *DependencyResolver) updateWorkflowCounters(workflow *WorkflowState, oldStatus, newStatus domain.JobStatus) {
	// Decrement old status counter
	switch oldStatus {
	case domain.StatusCompleted:
		workflow.CompletedJobs--
	case domain.StatusFailed:
		workflow.FailedJobs--
	case domain.StatusCanceled:
		workflow.CanceledJobs--
	}

	// Increment new status counter
	switch newStatus {
	case domain.StatusCompleted:
		workflow.CompletedJobs++
	case domain.StatusFailed:
		workflow.FailedJobs++
	case domain.StatusCanceled:
		workflow.CanceledJobs++
	}

	// Update started time
	if workflow.StartedAt == nil && newStatus == domain.StatusRunning {
		now := time.Now()
		workflow.StartedAt = &now
	}
}

// updateWorkflowStatus determines the overall workflow status based on constituent job states.
// Workflow status logic:
// - PENDING: No jobs started yet
// - RUNNING: At least one job is running or has started
// - COMPLETED: All jobs completed successfully
// - FAILED: At least one job failed (and workflow not canceled)
// - CANCELED: At least one job was canceled
// Sets completion timestamp when workflow reaches terminal state.
func (dr *DependencyResolver) updateWorkflowStatus(workflow *WorkflowState) {
	allJobsTerminal := true
	hasRunning := false
	hasFailed := false

	for _, job := range workflow.Jobs {
		if !isTerminalState(job.Status) {
			allJobsTerminal = false
		}
		if job.Status == domain.StatusRunning {
			hasRunning = true
		}
		if job.Status == domain.StatusFailed {
			hasFailed = true
		}
	}

	oldStatus := workflow.Status

	if allJobsTerminal {
		if workflow.CanceledJobs > 0 {
			workflow.Status = WorkflowCanceled
		} else if hasFailed || workflow.FailedJobs > 0 {
			workflow.Status = WorkflowFailed
		} else if workflow.CompletedJobs == workflow.TotalJobs {
			workflow.Status = WorkflowCompleted
		} else {
			workflow.Status = WorkflowFailed
		}

		if workflow.CompletedAt == nil {
			now := time.Now()
			workflow.CompletedAt = &now
		}
	} else if hasRunning || workflow.StartedAt != nil {
		workflow.Status = WorkflowRunning
	} else {
		workflow.Status = WorkflowPending
	}

	// Status change handling - notifications could be added here in the future
	_ = oldStatus // Acknowledge the status change for potential future use
}

// isTerminalState checks if a job status represents a final, unchangeable state.
// Terminal states are: COMPLETED, FAILED, STOPPED, CANCELED.
// Jobs in terminal states will not change status again and affect dependency evaluation.
func isTerminalState(status domain.JobStatus) bool {
	return status == domain.StatusCompleted ||
		status == domain.StatusFailed ||
		status == domain.StatusStopped ||
		status == domain.StatusCanceled
}

// ListWorkflows returns a list of all workflows managed by this resolver.
// Each workflow in the returned slice is a copy to prevent external modifications
// to the internal workflow state. The list includes workflows in all states:
// pending, running, completed, failed, and canceled.
// Used by monitoring and administrative functions to get an overview of all workflows.
func (dr *DependencyResolver) ListWorkflows() []*WorkflowState {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	var workflows []*WorkflowState
	for _, wf := range dr.workflows {
		copy := *wf
		workflows = append(workflows, &copy)
	}

	return workflows
}

// GetJobWorkflow looks up which workflow contains a specific job ID.
// Returns the workflow ID and true if the job belongs to a workflow,
// or 0 and false if the job is standalone (not part of any workflow).
// This mapping is used to route job status updates to the correct workflow
// and to determine if job lifecycle events should trigger workflow updates.
func (dr *DependencyResolver) GetJobWorkflow(jobID string) (int, bool) {
	dr.mu.RLock()
	defer dr.mu.RUnlock()

	workflowID, exists := dr.jobToWorkflow[jobID]
	return workflowID, exists
}
