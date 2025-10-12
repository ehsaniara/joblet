package jobs

import (
	"context"
	"fmt"
	"os"
	"time"

	pb "github.com/ehsaniara/joblet/api/gen"
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

// GetWorkflowStatus gets the status of a specific workflow
func GetWorkflowStatus(workflowUUID string, detail bool) error {
	// Just call the existing getWorkflowStatus function
	detailFlag = detail
	return getWorkflowStatus(workflowUUID)
}

// DeleteWorkflow deletes a workflow and its jobs
func DeleteWorkflow(workflowUUID string) error {
	return fmt.Errorf("workflow delete not yet implemented - please use 'rnx job delete' for individual jobs")
}
