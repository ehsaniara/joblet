package jobs

import (
	"github.com/spf13/cobra"
)

// NewJobCmd creates the main 'job' command that contains all job-related subcommands
func NewJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Job management commands",
		Long: `Job management commands for running and controlling jobs.

Available subcommands:
  run        Run a new job immediately or schedule it for later
  list       List all jobs or workflows
  status     Show status of a specific job
  log        Stream logs from a job
  stop       Stop a running job
  cancel     Cancel a scheduled job (status becomes CANCELED)
  delete     Delete a specific job
  delete-all Delete all non-running jobs`,
	}

	// Add all job-related subcommands
	cmd.AddCommand(NewRunCmd())
	cmd.AddCommand(NewListCmd())
	cmd.AddCommand(NewStatusCmd())
	cmd.AddCommand(NewLogCmd())
	cmd.AddCommand(NewStopCmd())
	cmd.AddCommand(NewCancelCmd())
	cmd.AddCommand(NewDeleteCmd())
	cmd.AddCommand(NewDeleteAllCmd())

	return cmd
}
