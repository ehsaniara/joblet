package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	pb "joblet/api/gen"
	"joblet/internal/rnx/common"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// NewDeleteCmd creates a new cobra command for deleting jobs.
// The command requires exactly one argument: the job UUID to delete.
// Sends a delete request to the Joblet server for complete job removal.
func NewDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <job-uuid>",
		Short: "Remove a job and its data",
		Long: `Permanently remove a job and all its data from the system.

This will delete the job record, logs, and any files it created. You can only
delete jobs that have finished running (completed, failed, or stopped).
Running jobs need to be stopped first.

What gets deleted:
- Job record and details
- Log files and output
- Any temporary files
- Resource allocations

Examples:
  # Delete a finished job
  rnx job delete f47ac10b-58cc-4372-a567-0e02b2c3d479

  # Use a shorter ID if it's unique
  rnx job delete f47ac10b

Warning: This can't be undone! The job and its logs will be gone forever.`,
		Args: cobra.ExactArgs(1),
		RunE: runDelete,
	}

	return cmd
}

// runDelete executes the job delete command.
// Takes the job ID from command arguments, connects to the server,
// and sends a delete request. Displays confirmation upon success.
func runDelete(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	response, err := jobClient.DeleteJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't delete the job: %v", err)
	}

	if common.JSONOutput {
		return outputDeleteJobJSON(response)
	}

	// Display result with appropriate formatting
	if response.Success {
		fmt.Printf("Job deleted successfully:\n")
		fmt.Printf("ID: %s\n", response.Uuid)
		fmt.Printf("Message: %s\n", response.Message)
	} else {
		fmt.Printf("Job deletion failed:\n")
		fmt.Printf("ID: %s\n", response.Uuid)
		fmt.Printf("Sorry, there was an issue: %s\n", response.Message)
		return fmt.Errorf("couldn't delete the job: %s", response.Message)
	}

	return nil
}

// outputDeleteJobJSON outputs the delete job result in JSON format
func outputDeleteJobJSON(response *pb.DeleteJobRes) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}
