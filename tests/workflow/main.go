package main

import (
	"fmt"
	"log"

	"github.com/ehsaniara/joblet/internal/joblet/domain"
	"github.com/ehsaniara/joblet/internal/joblet/workflow"
	"github.com/ehsaniara/joblet/internal/rnx/workflows"
)

func main() {
	fmt.Println("Workflow Engine Test Suite")
	fmt.Println("==========================")

	// Test 1: Load workflow file
	fmt.Println("\n1. Loading workflow file...")
	config, err := workflows.LoadWorkflowConfig("examples/workflows/ml-pipeline/ml-pipeline.yaml")
	if err != nil {
		log.Fatalf("Failed to load workflow config: %v", err)
	}
	fmt.Printf("   ✓ Loaded %d jobs\n", len(config.Jobs))

	// Test 2: Validate dependencies
	fmt.Println("\n2. Validating dependencies...")
	err = workflows.ValidateDependencies(config.Jobs)
	if err != nil {
		log.Fatalf("Invalid dependencies: %v", err)
	}
	fmt.Println("   ✓ No circular dependencies found")

	// Test 3: Build dependency graph
	fmt.Println("\n3. Building dependency graph...")
	jobOrder, err := workflows.BuildDependencyGraph(config.Jobs)
	if err != nil {
		log.Fatalf("Failed to build dependency graph: %v", err)
	}
	fmt.Printf("   ✓ Job execution order: %v\n", jobOrder)

	// Test 4: Test dependency resolver
	fmt.Println("\n4. Testing dependency resolver...")
	resolver := workflow.NewDependencyResolver()

	// Create job dependencies
	jobs := make(map[string]*workflow.JobDependency)
	for name, job := range config.Jobs {
		deps := &workflow.JobDependency{
			JobID:        fmt.Sprintf("job-%s", name),
			InternalName: name,
			Status:       domain.StatusPending,
		}

		// Convert requirements
		for _, req := range job.Requires {
			if req.JobID != "" {
				deps.Requirements = append(deps.Requirements, workflow.Requirement{
					Type:   workflow.RequirementSimple,
					JobID:  req.JobID,
					Status: req.Status,
				})
			} else if req.Expression != "" {
				deps.Requirements = append(deps.Requirements, workflow.Requirement{
					Type:       workflow.RequirementExpression,
					Expression: req.Expression,
				})
			}
		}

		jobs[fmt.Sprintf("job-%s", name)] = deps
	}

	// Create workflow
	workflowID, err := resolver.CreateWorkflow("examples/workflows/ml-pipeline/ml-pipeline.yaml", jobs, jobOrder)
	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}
	fmt.Printf("   ✓ Created workflow ID: %d\n", workflowID)

	// Test 5: Get ready jobs
	fmt.Println("\n5. Getting initial ready jobs...")
	readyJobs := resolver.GetReadyJobs(workflowID)
	fmt.Printf("   ✓ Ready jobs: %v\n", readyJobs)

	// Test 6: Simulate job completion
	fmt.Println("\n6. Simulating job execution...")

	// Complete data-validation
	resolver.OnJobStateChange("job-data-validation", domain.StatusCompleted)
	readyJobs = resolver.GetReadyJobs(workflowID)
	fmt.Printf("   ✓ After data-validation completed, ready jobs: %v\n", readyJobs)

	// Complete feature-engineering
	resolver.OnJobStateChange("job-feature-engineering", domain.StatusCompleted)
	readyJobs = resolver.GetReadyJobs(workflowID)
	fmt.Printf("   ✓ After feature-engineering completed, ready jobs: %v\n", readyJobs)

	// Test 7: Test expression evaluator
	fmt.Println("\n7. Testing expression evaluator...")
	expr := "job-a=COMPLETED AND job-b=FAILED"

	// Test evaluation with domain.JobStatus
	jobStates := map[string]domain.JobStatus{
		"job-a": domain.StatusCompleted,
		"job-b": domain.StatusFailed,
		"job-c": domain.StatusPending,
	}

	evaluator := workflow.NewSimpleExpressionEvaluator(jobStates)
	result := evaluator.Evaluate(expr)
	fmt.Printf("   ✓ Expression '%s' with states %v = %v\n", expr, jobStates, result)

	// Test with different states
	jobStates["job-c"] = domain.StatusCompleted
	result2 := evaluator.Evaluate(expr)
	fmt.Printf("   ✓ Expression '%s' with updated states %v = %v\n", expr, jobStates, result2)

	// Test 8: Test cascading cancellation
	fmt.Println("\n8. Testing cascading cancellation...")

	// Fail a job and see cascading effect
	resolver.OnJobStateChange("job-model-training", domain.StatusFailed)
	wfState, _ := resolver.GetWorkflowStatus(workflowID)
	fmt.Printf("   ✓ After model-training failed, workflow status: %s\n", wfState.Status)

	fmt.Println("\n✅ Workflow engine validation complete!")
}
