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
		Short: "Delete a job completely",
		Long: `Delete a job completely including logs, metadata, and all associated resources.

This command permanently removes the specified job from the system. The job must 
be in a completed, failed, or stopped state - running jobs cannot be deleted 
directly and must be stopped first.

Complete deletion includes:
- Job record and metadata
- Log files and buffers  
- Subscriptions and streams
- Any remaining resources

Examples:
  # Delete a completed job
  rnx job delete f47ac10b-58cc-4372-a567-0e02b2c3d479
  
  # Delete using short UUID (if unique)
  rnx job delete f47ac10b

Note: This operation is irreversible. Once deleted, job information and logs 
cannot be recovered.`,
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
		return fmt.Errorf("failed to create client: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	response, err := jobClient.DeleteJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("failed to delete job: %v", err)
	}

	if common.JSONOutput {
		return outputDeleteJobJSON(response)
	}

	// Display result with appropriate formatting
	if response.Success {
		fmt.Printf("✅ Job deleted successfully:\n")
		fmt.Printf("ID: %s\n", response.Uuid)
		fmt.Printf("Message: %s\n", response.Message)
	} else {
		fmt.Printf("❌ Job deletion failed:\n")
		fmt.Printf("ID: %s\n", response.Uuid)
		fmt.Printf("Error: %s\n", response.Message)
		return fmt.Errorf("deletion failed: %s", response.Message)
	}

	return nil
}

// outputDeleteJobJSON outputs the delete job result in JSON format
func outputDeleteJobJSON(response *pb.DeleteJobRes) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(response)
}
