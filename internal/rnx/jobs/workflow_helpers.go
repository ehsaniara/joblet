package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/rnx/common"
	"github.com/ehsaniara/joblet/internal/rnx/workflows"
)

// ExecuteWorkflow runs a workflow from a YAML file
func ExecuteWorkflow(workflowPath string) error {
	// This is a wrapper around the existing handleWorkflowExecution logic
	// We need to read the file and call the workflow execution
	if _, err := os.Stat(workflowPath); os.IsNotExist(err) {
		return fmt.Errorf("workflow file not found: %s", workflowPath)
	}

	// Call existing workflow execution with ModeWorkflow
	return handleWorkflowExecution(workflowPath, workflows.ModeWorkflow, "", nil)
}

// ListWorkflows lists all workflows
func ListWorkflows() error {
	// Connect to server
	client, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer client.Close()

	workflowClient := pb.NewJobServiceClient(client.GetConn())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Call ListWorkflows RPC
	res, err := workflowClient.ListWorkflows(ctx, &pb.ListWorkflowsRequest{
		IncludeCompleted: true,
	})
	if err != nil {
		return fmt.Errorf("failed to list workflows: %w", err)
	}

	if len(res.Workflows) == 0 {
		fmt.Println("No workflows found")
		return nil
	}

	// Use existing formatting logic
	if common.JSONOutput {
		return outputWorkflowsJSON(res.Workflows)
	}

	formatWorkflowList(res.Workflows)
	return nil
}

// Removed duplicate GetWorkflowStatus - now using the exported one from status.go

// DeleteWorkflow deletes a workflow and its jobs
func DeleteWorkflow(workflowUUID string) error {
	return fmt.Errorf("workflow delete not yet implemented - please use 'rnx job delete' for individual jobs")
}

// formatWorkflowList formats and displays workflows in a table
func formatWorkflowList(workflows []*pb.WorkflowInfo) {
	fmt.Printf("UUID                                 STATUS      PROGRESS\n")
	fmt.Printf("------------------------------------ ----------- ---------\n")
	for _, workflow := range workflows {
		// Get status color
		statusColor, resetColor := getStatusColor(workflow.Status)

		fmt.Printf("%-36s %s%-11s%s %d/%d\n",
			workflow.Uuid,
			statusColor, workflow.Status, resetColor,
			workflow.CompletedJobs,
			workflow.TotalJobs)
	}
}

// outputWorkflowsJSON outputs the workflows in JSON format
func outputWorkflowsJSON(workflows []*pb.WorkflowInfo) error {
	// Convert protobuf workflows to a simpler structure for JSON output
	type jsonWorkflow struct {
		UUID          string `json:"uuid"`
		Status        string `json:"status"`
		TotalJobs     int32  `json:"total_jobs"`
		CompletedJobs int32  `json:"completed_jobs"`
		FailedJobs    int32  `json:"failed_jobs"`
		CreatedAt     string `json:"created_at,omitempty"`
		StartedAt     string `json:"started_at,omitempty"`
		CompletedAt   string `json:"completed_at,omitempty"`
	}

	var jsonWorkflows []jsonWorkflow
	for _, workflow := range workflows {
		jsonWf := jsonWorkflow{
			UUID:          workflow.Uuid,
			Status:        workflow.Status,
			TotalJobs:     workflow.TotalJobs,
			CompletedJobs: workflow.CompletedJobs,
			FailedJobs:    workflow.FailedJobs,
		}

		// Convert timestamps if present
		if workflow.CreatedAt != nil {
			jsonWf.CreatedAt = time.Unix(workflow.CreatedAt.Seconds, int64(workflow.CreatedAt.Nanos)).Format(time.RFC3339)
		}
		if workflow.StartedAt != nil {
			jsonWf.StartedAt = time.Unix(workflow.StartedAt.Seconds, int64(workflow.StartedAt.Nanos)).Format(time.RFC3339)
		}
		if workflow.CompletedAt != nil {
			jsonWf.CompletedAt = time.Unix(workflow.CompletedAt.Seconds, int64(workflow.CompletedAt.Nanos)).Format(time.RFC3339)
		}

		jsonWorkflows = append(jsonWorkflows, jsonWf)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonWorkflows)
}
