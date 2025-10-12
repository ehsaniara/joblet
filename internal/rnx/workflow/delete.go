package workflow

import (
	"github.com/ehsaniara/joblet/internal/rnx/jobs"

	"github.com/spf13/cobra"
)

// NewWorkflowDeleteCmd creates the workflow delete command
func NewWorkflowDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <workflow-uuid>",
		Short: "Delete a workflow and its jobs",
		Long: `Delete a workflow and all associated jobs from the system.

This will remove the workflow record and all jobs that belong to it.
Running workflows cannot be deleted.

UUID supports short-form (first 8 characters) if unique.

Examples:
  rnx workflow delete 386148ef                    # Delete workflow (short UUID)
  rnx workflow delete 386148ef-e591-461a-a823     # Delete workflow (full UUID)`,
		Args: cobra.ExactArgs(1),
		RunE: deleteWorkflow,
	}

	return cmd
}

func deleteWorkflow(cmd *cobra.Command, args []string) error {
	workflowUUID := args[0]

	// Reuse existing workflow delete logic from jobs package
	return jobs.DeleteWorkflow(workflowUUID)
}
