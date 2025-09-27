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

// NewCancelCmd creates a new cobra command for canceling scheduled jobs.
// The command requires exactly one argument: the job UUID to cancel.
// This is specifically designed for SCHEDULED jobs and provides a more intuitive
// interface than the stop+delete workflow.
func NewCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <job-uuid>",
		Short: "Cancel a scheduled job",
		Long: `Cancel a job that is scheduled to run in the future.

This command is specifically designed for SCHEDULED jobs and will:
1. Cancel the scheduled job (preventing it from executing)
2. Change the job status to CANCELED (not STOPPED)
3. Preserve the job in history for audit purposes

This provides proper cancel vs stop semantics:
- 'rnx job stop' → for RUNNING jobs (status becomes STOPPED)
- 'rnx job cancel' → for SCHEDULED jobs (status becomes CANCELED)

Examples:
  # Cancel a scheduled job
  rnx job cancel f47ac10b-58cc-4372-a567-0e02b2c3d479

  # Cancel using short UUID (first 8 characters)
  rnx job cancel f47ac10b

Note: This command only works for jobs in SCHEDULED status. For running jobs, use 'rnx job stop'.`,
		Args: cobra.ExactArgs(1),
		RunE: runCancel,
	}

	return cmd
}

// runCancel executes the job cancel command.
// Takes the job ID from command arguments, checks if it's scheduled,
// stops it and then deletes it in one operation.
func runCancel(cmd *cobra.Command, args []string) error {
	jobID := args[0]

	jobClient, err := common.NewJobClient()
	if err != nil {
		return fmt.Errorf("couldn't connect to joblet server: %w", err)
	}
	defer jobClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First, get the job status to verify it's scheduled
	statusResponse, err := jobClient.GetJobStatus(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't get job status: %v", err)
	}

	// Check if the job is in SCHEDULED state
	if statusResponse.Status != "SCHEDULED" {
		return fmt.Errorf("job is not in SCHEDULED status (current status: %s). Use 'rnx job stop' for running jobs", statusResponse.Status)
	}

	// Cancel the scheduled job (server will set status to CANCELED for scheduled jobs)
	stopResponse, err := jobClient.StopJob(ctx, jobID)
	if err != nil {
		return fmt.Errorf("couldn't cancel the scheduled job: %v", err)
	}

	if common.JSONOutput {
		return outputCancelJobJSON(stopResponse)
	}

	fmt.Printf("✅ Scheduled job canceled successfully:\n")
	fmt.Printf("ID: %s\n", stopResponse.Uuid)
	// Display status with color coding (should be CANCELED)
	statusColor, resetColor := getStatusColor(stopResponse.Status)
	fmt.Printf("Status: %s%s%s\n", statusColor, stopResponse.Status, resetColor)
	fmt.Printf("Note: Job preserved in history with CANCELED status for audit purposes\n")

	return nil
}

// outputCancelJobJSON outputs the cancel job result in JSON format
func outputCancelJobJSON(stopResponse *pb.StopJobRes) error {
	result := map[string]interface{}{
		"canceled": true,
		"job_id":   stopResponse.Uuid,
		"status":   stopResponse.Status,
		"message":  "Scheduled job canceled successfully and preserved in history",
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
