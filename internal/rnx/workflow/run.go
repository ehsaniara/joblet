package workflow

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ehsaniara/joblet/internal/rnx/jobs"

	"github.com/spf13/cobra"
)

// NewWorkflowRunCmd creates the workflow run command
func NewWorkflowRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run <workflow-file>",
		Short: "Run a workflow from a YAML file",
		Long: `Run a multi-job workflow defined in a YAML file.

Workflows orchestrate multiple jobs with dependencies, allowing you to create
complex pipelines for data processing, ML training, and automated tasks.

The workflow file must be a valid YAML file defining jobs and their dependencies.

Examples:
  rnx workflow run pipeline.yaml                    # Run workflow from current directory
  rnx workflow run examples/ml-pipeline.yaml        # Run workflow from path
  rnx workflow run /path/to/workflow.yaml           # Run workflow with absolute path`,
		Args: cobra.ExactArgs(1),
		RunE: runWorkflow,
	}

	return cmd
}

func runWorkflow(cmd *cobra.Command, args []string) error {
	workflowFile := args[0]

	// Check if file exists
	if _, err := os.Stat(workflowFile); os.IsNotExist(err) {
		return fmt.Errorf("workflow file not found: %s", workflowFile)
	}

	// Get absolute path
	absPath, err := filepath.Abs(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Reuse existing workflow execution logic from jobs package
	// This calls the same backend implementation
	return jobs.ExecuteWorkflow(absPath)
}
