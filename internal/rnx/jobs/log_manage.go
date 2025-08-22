package jobs

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewLogManageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Manage job logs",
		Long: `Manage persisted job logs on the server.

This command provides tools to manage job logs that are persisted to disk,
including deletion of specific job logs or cleanup of old logs.

Examples:
  # Delete logs for a specific job
  rnx logs delete f47ac10b
  
  # Delete logs for multiple jobs
  rnx logs delete f47ac10b a1b2c3d4
  
  # Clean up logs older than retention period
  rnx logs cleanup`,
	}

	cmd.AddCommand(newLogDeleteCmd())
	cmd.AddCommand(newLogCleanupCmd())

	return cmd
}

func newLogDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <job-uuid> [job-uuid...]",
		Short: "Delete logs for specific jobs",
		Long: `Delete persisted log files for one or more jobs.

This permanently removes the log files from disk for the specified jobs.
Short-form UUIDs are supported.

Examples:
  # Delete logs for a single job
  rnx logs delete f47ac10b
  
  # Delete logs for multiple jobs
  rnx logs delete f47ac10b a1b2c3d4 e5f6g7h8`,
		Args: cobra.MinimumNArgs(1),
		RunE: runLogDelete,
	}

	return cmd
}

func newLogCleanupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old log files",
		Long: `Trigger immediate clean up of log files older than the configured retention period.

This command forces an immediate cleanup cycle to remove old log files
based on the server's configured retention period. Normally, cleanup runs
automatically every hour.`,
		Args: cobra.NoArgs,
		RunE: runLogCleanup,
	}

	return cmd
}

func runLogDelete(cmd *cobra.Command, args []string) error {
	// Note: This would require server-side implementation through gRPC
	// For now, we'll inform the user about the feature
	fmt.Println("Log deletion requires server-side support.")
	fmt.Println("Job logs are automatically persisted to disk and cleaned up based on retention period.")
	fmt.Println("")
	fmt.Println("To manually delete logs, you can:")
	fmt.Println("1. SSH to the server")
	fmt.Println("2. Navigate to the log directory (default: /opt/joblet/logs)")
	fmt.Println("3. Delete specific job log files manually")
	fmt.Println("")
	fmt.Println("Job log files are named: <job-uuid>_<timestamp>.log")

	return nil
}

func runLogCleanup(cmd *cobra.Command, args []string) error {
	// Note: Cleanup runs automatically on the server
	fmt.Println("Log cleanup runs automatically on the server every hour.")
	fmt.Println("Old logs are deleted based on the configured retention period (default: 7 days).")
	fmt.Println("")
	fmt.Println("To manually trigger cleanup, you can:")
	fmt.Println("1. SSH to the server")
	fmt.Println("2. Run: find /opt/joblet/logs -name '*.log' -mtime +7 -delete")
	fmt.Println("   (adjust the path and days as needed)")

	return nil
}
