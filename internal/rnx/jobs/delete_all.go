package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	pb "github.com/ehsaniara/joblet-proto/v2/gen"
	"github.com/ehsaniara/joblet/internal/rnx/common"

	"github.com/spf13/cobra"
)

// NewDeleteAllCmd creates a new cobra command for deleting all non-running jobs.
// This command removes all jobs that are not in running or scheduled state.
// Sends a delete-all request to the Joblet server for bulk job removal.
func NewDeleteAllCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete-all",
		Short: "Delete all non-running jobs",
		Long: `Delete all non-running jobs completely including logs, metadata, and all associated resources.

This command permanently removes all jobs from the system that are not currently running
or scheduled. Jobs in completed, failed, or stopped states will be deleted. Running and
scheduled jobs are preserved and will not be affected.

Complete deletion includes:
- Job records and metadata
- Log files and buffers
- Subscriptions and streams
- Any remaining resources

Examples:
  # Delete all non-running jobs
  rnx job delete-all

  # Delete all non-running jobs with JSON output
  rnx job delete-all --json

Note: This operation is irreversible. Once deleted, job information and logs
cannot be recovered. Only non-running jobs are affected.`,
		Args: cobra.NoArgs,
		RunE: runDeleteAll,
	}

	return cmd
}

// runDeleteAll executes the delete-all command.
// Connects to the server and sends a delete-all request.
// Displays confirmation with counts upon success.
func runDeleteAll(cmd *cobra.Command, args []string) error {
	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	response, err := jobClient.DeleteAllJobs(ctx)
	if err != nil {
		return fmt.Errorf("couldn't delete all jobs: %v", err)
	}

	if common.JSONOutput {
		return outputDeleteAllJobsJSON(response)
	}

	// Display result with appropriate formatting
	if response.Success {
		fmt.Printf("Jobs deleted successfully:\n")
		fmt.Printf("Deleted count: %d\n", response.DeletedCount)
		fmt.Printf("Skipped count: %d (running/scheduled)\n", response.SkippedCount)
		fmt.Printf("Message: %s\n", response.Message)
	} else {
		fmt.Printf("Job deletion failed:\n")
		fmt.Printf("Error: %s\n", response.Message)
		return fmt.Errorf("couldn't delete all jobs: %s", response.Message)
	}

	return nil
}

// outputDeleteAllJobsJSON outputs the delete-all result in JSON format
func outputDeleteAllJobsJSON(response *pb.DeleteAllJobsRes) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}
