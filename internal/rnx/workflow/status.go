package workflow

import (
	"github.com/ehsaniara/joblet/internal/rnx/jobs"

	"github.com/spf13/cobra"
)

var detailFlag bool

// NewWorkflowStatusCmd creates the workflow status command
func NewWorkflowStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <workflow-uuid>",
		Short: "Show workflow status and progress",
		Long: `Show detailed status of a workflow including all jobs, dependencies, and progress.

Displays workflow execution status, timing information, and the status of each
job within the workflow along with their dependencies.

UUID supports short-form (first 8 characters) if unique.

Examples:
  rnx workflow status 386148ef                    # Short UUID
  rnx workflow status 386148ef-e591-461a-a823     # Full UUID
  rnx workflow status 386148ef --detail           # Include YAML content
  rnx workflow status 386148ef --json             # JSON output`,
		Args: cobra.ExactArgs(1),
		RunE: getWorkflowStatus,
	}

	cmd.Flags().BoolVarP(&detailFlag, "detail", "d", false, "Show YAML content when displaying workflow status")

	return cmd
}

func getWorkflowStatus(cmd *cobra.Command, args []string) error {
	workflowUUID := args[0]

	// Reuse existing workflow status logic from jobs package
	return jobs.GetWorkflowStatus(workflowUUID, detailFlag)
}
