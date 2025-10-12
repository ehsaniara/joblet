package workflow

import (
	"github.com/ehsaniara/joblet/internal/rnx/jobs"

	"github.com/spf13/cobra"
)

// NewWorkflowListCmd creates the workflow list command
func NewWorkflowListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all workflows",
		Long: `List all workflows with their status and progress.

Shows all workflows that have been submitted to the joblet server, including
running, completed, failed, and canceled workflows.

Examples:
  rnx workflow list                        # List all workflows
  rnx workflow list --json                 # JSON output for APIs/UIs`,
		Args: cobra.NoArgs,
		RunE: listWorkflows,
	}

	return cmd
}

func listWorkflows(cmd *cobra.Command, args []string) error {
	// Reuse existing workflow list logic from jobs package
	return jobs.ListWorkflows()
}
